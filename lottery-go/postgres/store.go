// Package postgres implements lottery.TicketStore on top of PostgreSQL.
//
// The database is the system of record. Candidate numbers are still generated
// in Go by arithmetic -- that part needs no storage at all -- but which tickets
// a caller actually gets is decided entirely by Postgres, inside a single
// statement, under row locks.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
)

// Tuning knobs for the claim loop.
const (
	// defaultBatchSize is how many candidate numbers go into one query. Large
	// enough that broad patterns finish in one round trip; small enough that
	// the array parameter stays cheap to plan.
	defaultBatchSize = 256
	// defaultMaxBatches bounds the work one request may do when stock is
	// nearly exhausted. Past this we return what we have rather than scanning
	// the whole match set.
	defaultMaxBatches = 16
	// overFetch is how many candidate NUMBERS we request per ticket wanted,
	// to absorb the ones that turn out to be sold out.
	overFetch = 4
)

// Store is a Postgres-backed TicketStore.
type Store struct {
	pool       *pgxpool.Pool
	batchSize  int
	maxBatches int
}

// New wraps a connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, batchSize: defaultBatchSize, maxBatches: defaultMaxBatches}
}

// Migrate applies the schema. It is idempotent, so it is safe on every boot.
func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("applying schema: %w", err)
	}
	return nil
}

// claimSQL is the heart of the whole system.
//
// The inner SELECT does three things at once: it filters to claimable rows, it
// locks the ones it picks, and SKIP LOCKED makes it step over rows another
// transaction already holds instead of waiting behind them. The outer UPDATE
// then writes rows it is already holding, so it can never block.
//
// Because the predicate is re-checked under the lock, a stale idea of what was
// free costs one skipped candidate and nothing more.
const claimSQL = `
UPDATE tickets t
   SET status      = 1,
       holder      = $2,
       lease_until = now() + make_interval(secs => $3),
       version     = version + 1
 WHERE t.id IN (
       SELECT id
         FROM tickets
        WHERE num = ANY($1::int[])
          AND (status = 0 OR (status = 1 AND lease_until < now()))
        LIMIT $4
          FOR UPDATE SKIP LOCKED
       )
RETURNING t.id, t.num, t.lease_until`

// distinctClaimSQL takes AT MOST ONE ticket per number, so a user searching
// ****23 receives five different numbers rather than five copies of one.
//
// Why the LATERAL: the plain claimSQL sends a batch of candidate numbers with
// LIMIT k, and Postgres happily satisfies that entire limit from the first
// number it finds in the index. With ~10 tickets per number, asking for 10
// gets you 10 identical numbers. The LATERAL runs a separate LIMIT 1 for each
// candidate number, which is what actually spreads the result.
//
// WITH ORDINALITY preserves the scrambled order the walker produced, so
// concurrent users for the same pattern still start in different places.
const distinctClaimSQL = `
WITH candidate(num, ord) AS (
    SELECT * FROM unnest($1::int[]) WITH ORDINALITY
),
picked AS (
    SELECT t.id
      FROM candidate c
      CROSS JOIN LATERAL (
           SELECT id
             FROM tickets
            WHERE tickets.num = c.num
              AND (status = 0 OR (status = 1 AND lease_until < now()))
            LIMIT 1
              FOR UPDATE SKIP LOCKED
      ) t
     ORDER BY c.ord
     LIMIT $4
)
UPDATE tickets t
   SET status      = 1,
       holder      = $2,
       lease_until = now() + make_interval(secs => $3),
       version     = version + 1
  FROM picked
 WHERE t.id = picked.id
RETURNING t.id, t.num, t.lease_until`

// claimRows runs one claim statement and collects the reservations.
func (s *Store) claimRows(ctx context.Context, sql string, nums []int32, holder string, lease time.Duration, want int) ([]lottery.Reservation, error) {
	rows, err := s.pool.Query(ctx, sql, nums, holder, lease.Seconds(), want)
	if err != nil {
		return nil, fmt.Errorf("claiming tickets: %w", err)
	}
	batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (lottery.Reservation, error) {
		var res lottery.Reservation
		err := r.Scan(&res.TicketID, &res.Number, &res.LeaseUntil)
		return res, err
	})
	if err != nil {
		return nil, fmt.Errorf("reading claimed tickets: %w", err)
	}
	return batch, nil
}

func (s *Store) Claim(ctx context.Context, p lottery.Pattern, k int, holder string, lease time.Duration) ([]lottery.Reservation, error) {
	if k < 1 {
		return nil, nil
	}
	out := make([]lottery.Reservation, 0, k)

	// PASS 1 -- one ticket per number, so the user sees a spread of numbers.
	//
	// We over-fetch candidate numbers because some will be sold out: asking for
	// k*overFetch numbers to fill k slots usually completes in one round trip.
	cands := p.Candidates(lottery.RandomSeed())
	for batches := 0; len(out) < k && batches < s.maxBatches; batches++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		nums := cands.NextBatch(min((k-len(out))*overFetch, s.batchSize))
		if len(nums) == 0 {
			break // walked the whole match set
		}
		batch, err := s.claimRows(ctx, distinctClaimSQL, nums, holder, lease, k-len(out))
		if err != nil {
			return out, err
		}
		out = append(out, batch...)
	}

	// PASS 2 -- top up with duplicates if there were not enough DISTINCT
	// numbers to satisfy the request.
	//
	// This is not a fallback for tidiness; it is required. A pattern with no
	// wildcards ("123456") matches exactly one number, so pass 1 can never
	// return more than one ticket. Someone asking for five copies of 123456
	// should get five, not one.
	if len(out) < k {
		cands = p.Candidates(lottery.RandomSeed())
		for batches := 0; len(out) < k && batches < s.maxBatches; batches++ {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			nums := cands.NextBatch(s.batchSize)
			if len(nums) == 0 {
				break
			}
			batch, err := s.claimRows(ctx, claimSQL, nums, holder, lease, k-len(out))
			if err != nil {
				return out, err
			}
			if len(batch) == 0 {
				continue
			}
			out = append(out, batch...)
		}
	}
	return out, nil
}


const confirmSQL = `
UPDATE tickets
   SET status = 2, version = version + 1
 WHERE id = $1 AND status = 1 AND holder = $2 AND lease_until >= now()`

func (s *Store) Confirm(ctx context.Context, id int64, holder string) error {
	tag, err := s.pool.Exec(ctx, confirmSQL, id, holder)
	if err != nil {
		return fmt.Errorf("confirming ticket %d: %w", id, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	return s.explainFailure(ctx, id, holder)
}

const releaseSQL = `
UPDATE tickets
   SET status = 0, holder = NULL, lease_until = NULL, version = version + 1
 WHERE id = $1 AND status = 1 AND holder = $2`

func (s *Store) Release(ctx context.Context, id int64, holder string) error {
	tag, err := s.pool.Exec(ctx, releaseSQL, id, holder)
	if err != nil {
		return fmt.Errorf("releasing ticket %d: %w", id, err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	return s.explainFailure(ctx, id, holder)
}

// explainFailure turns "no rows matched" into a specific error. The write
// itself stays a single statement; this extra read only runs on the failure
// path, where an extra round trip costs nothing.
func (s *Store) explainFailure(ctx context.Context, id int64, holder string) error {
	var (
		status  int16
		owner   *string
		expired *bool
	)
	err := s.pool.QueryRow(ctx,
		`SELECT status, holder, lease_until < now() FROM tickets WHERE id = $1`, id,
	).Scan(&status, &owner, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return lottery.ErrTicketNotFound
	}
	if err != nil {
		return fmt.Errorf("inspecting ticket %d: %w", id, err)
	}
	if lottery.Status(status) != lottery.Reserved || owner == nil || *owner != holder {
		return lottery.ErrNotHeld
	}
	if expired != nil && *expired {
		return lottery.ErrLeaseExpired
	}
	// Someone else changed it between the update and this read. Retrying is
	// the caller's business.
	return lottery.ErrNotHeld
}

// reapSQL returns expired reservations to circulation.
//
// This is housekeeping, not correctness: the claim predicate already treats an
// expired reservation as claimable. The reaper exists so the partial index
// stays accurate and so admin views do not fill with dead holds.
const reapSQL = `
WITH expired AS (
    SELECT id
      FROM tickets
     WHERE status = 1 AND lease_until < now()
     LIMIT $1
       FOR UPDATE SKIP LOCKED
)
UPDATE tickets t
   SET status = 0, holder = NULL, lease_until = NULL, version = version + 1
  FROM expired
 WHERE t.id = expired.id`

// ReapExpired releases up to limit expired reservations and reports how many.
func (s *Store) ReapExpired(ctx context.Context, limit int) (int64, error) {
	tag, err := s.pool.Exec(ctx, reapSQL, limit)
	if err != nil {
		return 0, fmt.Errorf("reaping expired reservations: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RunReaper sweeps on a timer until the context is cancelled.
func (s *Store) RunReaper(ctx context.Context, every time.Duration, batch int, onError func(error)) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.ReapExpired(ctx, batch); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}

// Stats is a small operational view.
type Stats struct {
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Sold      int64 `json:"sold"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.pool.QueryRow(ctx, `
        SELECT count(*) FILTER (WHERE status = 0),
               count(*) FILTER (WHERE status = 1),
               count(*) FILTER (WHERE status = 2)
          FROM tickets`).Scan(&st.Available, &st.Reserved, &st.Sold)
	if err != nil {
		return st, fmt.Errorf("reading stats: %w", err)
	}
	return st, nil
}

// Compile-time proof that Store satisfies the interface.
var _ lottery.TicketStore = (*Store)(nil)
