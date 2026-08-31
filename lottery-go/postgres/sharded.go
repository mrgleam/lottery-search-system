package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
)

// ShardedStore spreads claims across N logical shards to cut lock contention
// on a hot pattern.
//
// READ THIS BEFORE USING IT. This is the third thing to try, not the first.
// Measure with Metrics.ContendedPerRequest and confirm contention is actually
// your bottleneck; if the real problem is a thinning inventory or a saturated
// connection pool, sharding adds complexity and fixes nothing.
//
// How it works: every ticket carries a shard number, and a claimer starts at
// its own shard, falling through to the others only if that one is dry. Two
// claimers with different starting shards touch disjoint rows and never
// contend, at the cost of a slightly less uniform draw.
type ShardedStore struct {
	pool       *pgxpool.Pool
	shards     int
	batchSize  int
	maxBatches int
}

// NewSharded wraps a pool. shards should be a small multiple of your API
// replica count; more shards means less contention but more fallthrough
// queries when stock thins out.
func NewSharded(pool *pgxpool.Pool, shards int) *ShardedStore {
	if shards < 1 {
		shards = 1
	}
	return &ShardedStore{
		pool:       pool,
		shards:     shards,
		batchSize:  defaultBatchSize,
		maxBatches: defaultMaxBatches,
	}
}

// shardedClaimSQL differs from claimSQL only by the shard predicate. Note the
// index it needs is (shard, num) WHERE status <> 2, not (num).
const shardedClaimSQL = `
UPDATE tickets t
   SET status      = 1,
       holder      = $2,
       lease_until = now() + make_interval(secs => $3),
       version     = version + 1
 WHERE t.id IN (
       SELECT id
         FROM tickets
        WHERE shard = $5
          AND num = ANY($1::int[])
          AND (status = 0 OR (status = 1 AND lease_until < now()))
        LIMIT $4
          FOR UPDATE SKIP LOCKED
       )
RETURNING t.id, t.num, t.lease_until`

func (s *ShardedStore) Claim(ctx context.Context, p lottery.Pattern, k int, holder string, lease time.Duration) ([]lottery.Reservation, error) {
	if k < 1 {
		return nil, nil
	}
	out := make([]lottery.Reservation, 0, k)

	// Start at a shard chosen from the holder, so one user tends to hit the
	// same shard and stays off everyone else's rows.
	start := int(hashString(holder)) % s.shards

	for offset := 0; offset < s.shards && len(out) < k; offset++ {
		shard := (start + offset) % s.shards
		cands := p.Candidates(lottery.RandomSeed())

		for batches := 0; len(out) < k && batches < s.maxBatches; batches++ {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			nums := cands.NextBatch(s.batchSize)
			if len(nums) == 0 {
				break
			}
			rows, err := s.pool.Query(ctx, shardedClaimSQL,
				nums, holder, lease.Seconds(), k-len(out), shard)
			if err != nil {
				return out, fmt.Errorf("claiming from shard %d: %w", shard, err)
			}
			batch, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (lottery.Reservation, error) {
				var res lottery.Reservation
				err := r.Scan(&res.TicketID, &res.Number, &res.LeaseUntil)
				return res, err
			})
			if err != nil {
				return out, fmt.Errorf("reading claimed tickets: %w", err)
			}
			out = append(out, batch...)
		}
	}
	return out, nil
}

// Confirm and Release need no shard awareness: they address one row by id.
func (s *ShardedStore) Confirm(ctx context.Context, id int64, holder string) error {
	return (&Store{pool: s.pool}).Confirm(ctx, id, holder)
}

func (s *ShardedStore) Release(ctx context.Context, id int64, holder string) error {
	return (&Store{pool: s.pool}).Release(ctx, id, holder)
}

// hashString is FNV-1a, inlined to avoid a dependency.
func hashString(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

var _ lottery.TicketStore = (*ShardedStore)(nil)
