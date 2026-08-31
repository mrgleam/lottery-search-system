package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
)

// HybridStore is the design from the architecture document: a fast in-memory
// availability hint in front of an authoritative database.
//
// THE DIVISION OF RESPONSIBILITY -- this is the whole idea:
//
//	The hint answers  "which numbers are WORTH ASKING ABOUT"   (may be wrong)
//	Postgres answers  "which tickets do you ACTUALLY GET"      (always right)
//
// Because the database re-checks every row under lock before writing to it, a
// wrong hint costs one wasted candidate and can never cause a double sale.
// That asymmetry is what lets the hint be cheap, lock-free-ish, and eventually
// consistent instead of transactional.
type HybridStore struct {
	pool *pgxpool.Pool
	log  *slog.Logger

	// mu guards avail only. It is never held across a database call, so a slow
	// query can never block hint readers.
	mu    sync.RWMutex
	avail *Availability

	batchSize  int
	maxBatches int

	// numberOf caches ticket id -> number for reservations we handed out, so
	// Confirm can update the hint without a second query. Entries are removed
	// once the ticket reaches a terminal state.
	numMu    sync.Mutex
	numberOf map[int64]int32
}

// NewHybrid builds the store and loads the hint from the database.
//
// Loading at construction rather than lazily means a replica that starts up
// serves correct-shaped traffic from its first request, instead of reporting
// everything sold out until some background job runs.
func NewHybrid(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger) (*HybridStore, error) {
	if log == nil {
		log = slog.Default()
	}
	avail := NewAvailability()

	start := time.Now()
	if err := avail.Load(ctx, pool); err != nil {
		return nil, err
	}
	st := avail.Stats()
	log.Info("availability hint loaded",
		"numbers_with_stock", st.NumbersWithStock,
		"tickets_unsold", st.TicketsUnsold,
		"took", time.Since(start).Round(time.Millisecond))

	return &HybridStore{
		pool:       pool,
		log:        log,
		avail:      avail,
		batchSize:  defaultBatchSize,
		maxBatches: defaultMaxBatches,
		numberOf:   make(map[int64]int32),
	}, nil
}

// Claim is the read path. Four phases, and it is worth reading them as four:
//
//  1. GENERATE candidate numbers from the pattern (pure arithmetic, no I/O)
//  2. FILTER them through the in-memory hint (fast, may be wrong)
//  3. ASK Postgres, which locks and writes atomically (slow, always right)
//  4. RECONCILE the hint with what Postgres actually said
//
// Phase 4 is the part people forget, and it is what stops the hint drifting
// further from the truth on every request.
func (s *HybridStore) Claim(ctx context.Context, p lottery.Pattern, k int, holder string, lease time.Duration) ([]lottery.Reservation, error) {
	if k < 1 {
		return nil, nil
	}
	out := make([]lottery.Reservation, 0, k)
	cands := p.Candidates(lottery.RandomSeed())

	for batches := 0; len(out) < k && batches < s.maxBatches; batches++ {
		if err := ctx.Err(); err != nil {
			return out, err
		}

		// Phase 1 + 2: generate, then keep only what the hint thinks is worth
		// asking about. Filtering here is what makes a nearly-sold-out draw
		// cheap: we stop shipping hopeless numbers to the database.
		raw := cands.NextBatch(min((k-len(out))*overFetch, s.batchSize))
		if len(raw) == 0 {
			break // walked the whole match set
		}
		nums := s.filterByHint(raw)
		if len(nums) == 0 {
			continue // every candidate in this batch looks sold out; next batch
		}

		// Phase 3: the database decides. Everything before this was advisory.
		// distinctClaimSQL takes at most one ticket per number.
		batch, err := (&Store{pool: s.pool}).claimRows(ctx, distinctClaimSQL, nums, holder, lease, k-len(out))
		if err != nil {
			return out, err
		}

		// Phase 4: reconcile.
		s.reconcileAfterClaim(batch)
		out = append(out, batch...)
	}

	// Top up with duplicates if there were not enough distinct numbers.
	if len(out) < k {
		cands = p.Candidates(lottery.RandomSeed())
		for batches := 0; len(out) < k && batches < s.maxBatches; batches++ {
			if err := ctx.Err(); err != nil {
				return out, err
			}
			raw := cands.NextBatch(s.batchSize)
			if len(raw) == 0 {
				break
			}
			nums := s.filterByHint(raw)
			if len(nums) == 0 {
				continue
			}
			batch, err := (&Store{pool: s.pool}).claimRows(ctx, claimSQL, nums, holder, lease, k-len(out))
			if err != nil {
				return out, err
			}
			s.reconcileAfterClaim(batch)
			out = append(out, batch...)
		}
	}
	return out, nil
}

// filterByHint drops candidates the hint believes are sold out.
//
// Note what this does NOT do: it never adds a number the pattern did not
// generate. The hint can only ever narrow the candidate set, so a corrupted
// hint degrades results but cannot produce a wrong ticket.
func (s *HybridStore) filterByHint(raw []int32) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]int32, 0, len(raw))
	for _, n := range raw {
		if s.avail.MaybeAvailable(n) {
			out = append(out, n)
		}
	}
	return out
}

// reconcileAfterClaim folds what the database said back into the hint.
//
// A claim does not change how many tickets are UNSOLD -- a reservation is not
// a sale -- so no count moves here. What we do learn is that this number
// definitely had stock, which lets us correct a hint that had wrongly written
// it off (because another replica released it, or because we were stale).
//
// Correcting upward is always safe. See the note in load.go.
func (s *HybridStore) reconcileAfterClaim(batch []lottery.Reservation) {
	if len(batch) == 0 {
		return
	}
	s.mu.Lock()
	for _, r := range batch {
		s.avail.MarkAvailable(r.Number)
	}
	s.mu.Unlock()

	// Remember id -> number so Confirm can update the hint without a query.
	s.numMu.Lock()
	for _, r := range batch {
		s.numberOf[r.TicketID] = r.Number
	}
	s.numMu.Unlock()
}

// Confirm turns a reservation into a sale. THIS is where the hint's count
// actually moves, because a sale is the only thing that permanently removes a
// ticket from circulation.
func (s *HybridStore) Confirm(ctx context.Context, id int64, holder string) error {
	tag, err := s.pool.Exec(ctx, confirmSQL, id, holder)
	if err != nil {
		return fmt.Errorf("confirming ticket %d: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return (&Store{pool: s.pool}).explainFailure(ctx, id, holder)
	}

	// The write succeeded, so the hint may now be one too high for this
	// number. Bring it down.
	if n, ok := s.forgetNumber(id); ok {
		s.mu.Lock()
		s.avail.MarkSold(n)
		s.mu.Unlock()
	}
	// If we did not know the number -- another replica handed out this
	// reservation -- we simply leave the hint alone. It over-reports until the
	// next refresh, which is the safe direction.
	return nil
}

// Release returns a reservation to circulation. The ticket was never sold, so
// the unsold count never changed and there is nothing to decrement. We do
// re-assert availability, because our hint may have been stale.
func (s *HybridStore) Release(ctx context.Context, id int64, holder string) error {
	tag, err := s.pool.Exec(ctx, releaseSQL, id, holder)
	if err != nil {
		return fmt.Errorf("releasing ticket %d: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return (&Store{pool: s.pool}).explainFailure(ctx, id, holder)
	}
	if n, ok := s.forgetNumber(id); ok {
		s.mu.Lock()
		s.avail.MarkAvailable(n)
		s.mu.Unlock()
	}
	return nil
}

func (s *HybridStore) forgetNumber(id int64) (int32, bool) {
	s.numMu.Lock()
	defer s.numMu.Unlock()
	n, ok := s.numberOf[id]
	if ok {
		delete(s.numberOf, id)
	}
	return n, ok
}

// Refresh reloads the hint from the database.
//
// This is how a replica learns about sales made by OTHER replicas. Without it,
// each replica only ever hears about its own sales, so its counts drift
// upward -- over-reporting, which is safe but increasingly wasteful as a draw
// sells out.
func (s *HybridStore) Refresh(ctx context.Context) error {
	fresh := NewAvailability()
	if err := fresh.Load(ctx, s.pool); err != nil {
		return err
	}
	s.mu.Lock()
	s.avail = fresh
	s.mu.Unlock()

	// Reservations we are tracking are unaffected by the swap; ids still map
	// to the same numbers.
	return nil
}

// RunRefresher reloads the hint periodically until the context is cancelled.
//
// Choosing the interval is a real trade-off:
//
//	too long  -> the hint over-reports badly near sell-out, so requests ship
//	             many hopeless candidates and latency climbs
//	too short -> a full GROUP BY over the tickets table every few seconds
//
// Thirty seconds is a reasonable default for a draw that sells out over hours.
// For a flash sale, drive it from NOTIFY instead (see listenSQL below).
func (s *HybridStore) RunRefresher(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			if err := s.Refresh(ctx); err != nil {
				s.log.Error("refreshing availability hint", "error", err)
				continue
			}
			st := s.HintStats()
			s.log.Info("availability hint refreshed",
				"numbers_with_stock", st.NumbersWithStock,
				"tickets_unsold", st.TicketsUnsold,
				"took", time.Since(start).Round(time.Millisecond))
		}
	}
}

// listenSQL is the faster alternative to polling. A trigger on the tickets
// table sends a notification whenever a ticket is sold, and each replica
// applies that one change to its own hint immediately.
//
// See schema_notify.sql for the trigger. This is worth enabling when refresh
// intervals become the bottleneck; it is not worth the moving parts before
// then.
const listenSQL = `LISTEN ticket_sold`

// RunListener applies sold-notifications from other replicas as they happen.
//
// Note it takes a dedicated connection from the pool for the lifetime of the
// call: LISTEN is a session-level feature, so it cannot share a pooled
// connection with normal queries. Budget for it when sizing the pool.
func (s *HybridStore) RunListener(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring listener connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, listenSQL); err != nil {
		return fmt.Errorf("issuing LISTEN: %w", err)
	}
	s.log.Info("listening for ticket_sold notifications")

	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("waiting for notification: %w", err)
		}
		var num int32
		if _, err := fmt.Sscanf(n.Payload, "%d", &num); err != nil {
			s.log.Warn("unparseable notification payload", "payload", n.Payload)
			continue
		}
		s.mu.Lock()
		s.avail.MarkSold(num)
		s.mu.Unlock()
	}
}

// HintStats exposes the hint's own state, for /debug/hint and for teaching.
func (s *HybridStore) HintStats() AvailabilityStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.avail.Stats()
}

var _ lottery.TicketStore = (*HybridStore)(nil)
