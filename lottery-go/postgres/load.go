package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
)

// Availability is the in-memory hint: for each of the 1,000,000 possible
// numbers, how many tickets are not yet sold.
//
// READ THIS BEFORE CHANGING ANYTHING HERE.
//
// This structure is a HINT, never the truth. The database decides who gets a
// ticket. The rule that keeps that safe is one-directional:
//
//	Over-reporting  (says available, actually sold out) -> costs one wasted
//	                candidate number in a query. Harmless.
//	Under-reporting (says sold out, actually available) -> those tickets become
//	                invisible to this replica. Inventory is stranded. NEVER
//	                allow this.
//
// Every decision below leans toward over-reporting. When in doubt, say the
// number still has stock and let Postgres correct you.
type Availability struct {
	counts  []uint32       // counts[n] = tickets of number n not yet sold
	nonZero *lottery.Bitmap // bit set wherever counts[n] > 0
	loaded  time.Time
}

// NewAvailability builds an empty hint. Nothing is available until Load runs.
func NewAvailability() *Availability {
	return &Availability{
		counts:  make([]uint32, lottery.NumberSpace),
		nonZero: lottery.NewBitmap(lottery.NumberSpace),
	}
}

// loadSQL asks the database for one row per number, not one row per ticket.
//
// With 10,000,000 tickets across 1,000,000 numbers this returns at most a
// million rows instead of ten million -- and the aggregate happens inside
// Postgres, where the data already is.
//
// status <> 2 means "not sold". A reserved ticket still counts as available,
// because its lease may expire and put it back in circulation. Counting only
// status = 0 here would be the under-reporting mistake described above.
const loadSQL = `
SELECT num, count(*)::int
  FROM tickets
 WHERE status <> 2
 GROUP BY num`

// Load replaces the hint with fresh state from the database.
//
// Safe to call at boot and again at any time; it builds into scratch space and
// swaps at the end, so readers never see a half-built hint.
func (a *Availability) Load(ctx context.Context, pool *pgxpool.Pool) error {
	counts := make([]uint32, lottery.NumberSpace)
	bm := lottery.NewBitmap(lottery.NumberSpace)

	rows, err := pool.Query(ctx, loadSQL)
	if err != nil {
		return fmt.Errorf("loading availability: %w", err)
	}
	defer rows.Close()

	var loaded int
	for rows.Next() {
		var num int32
		var count int32
		if err := rows.Scan(&num, &count); err != nil {
			return fmt.Errorf("scanning availability row: %w", err)
		}
		if num < 0 || int(num) >= lottery.NumberSpace {
			continue // a CHECK constraint should prevent this; skip rather than panic
		}
		counts[num] = uint32(count)
		if count > 0 {
			bm.Set(int(num))
		}
		loaded++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading availability rows: %w", err)
	}

	a.counts = counts
	a.nonZero = bm
	a.loaded = time.Now()
	return nil
}

// MaybeAvailable reports whether this number is worth including as a candidate.
//
// A true answer means "possibly", never "definitely". A false answer is the
// only strong claim, and it is only ever set by information that came from the
// database.
func (a *Availability) MaybeAvailable(n int32) bool {
	if n < 0 || int(n) >= lottery.NumberSpace {
		return false
	}
	return a.nonZero.Test(int(n))
}

// MarkSold records that one ticket of this number was sold.
//
// Called only after Postgres confirms the sale, so the hint is catching up to
// a fact rather than predicting one.
func (a *Availability) MarkSold(n int32) {
	if n < 0 || int(n) >= lottery.NumberSpace {
		return
	}
	if a.counts[n] > 0 {
		a.counts[n]--
	}
	if a.counts[n] == 0 {
		a.nonZero.Clear(int(n))
	}
}

// MarkAvailable records that a number definitely has stock again.
//
// Used when the database hands us a ticket for a number our hint had written
// off -- which happens when another replica released something, or when our
// hint was simply stale. Correcting upward is always safe.
func (a *Availability) MarkAvailable(n int32) {
	if n < 0 || int(n) >= lottery.NumberSpace {
		return
	}
	if a.counts[n] == 0 {
		a.counts[n] = 1
	}
	a.nonZero.Set(int(n))
}

// NextAvailableFrom finds the next number at or after `from` that may have
// stock, or -1. Skips 64 sold-out numbers per comparison.
func (a *Availability) NextAvailableFrom(from int32) int32 {
	m := a.nonZero.NextSet(int(from))
	if m < 0 {
		return -1
	}
	return int32(m)
}

// Stats describes the hint itself, for /debug and for teaching.
type AvailabilityStats struct {
	NumbersWithStock int       `json:"numbers_with_stock"`
	TicketsUnsold    int64     `json:"tickets_unsold"`
	LoadedAt         time.Time `json:"loaded_at"`
	AgeSeconds       float64   `json:"age_seconds"`
}

func (a *Availability) Stats() AvailabilityStats {
	var numbers int
	var tickets int64
	for _, c := range a.counts {
		if c > 0 {
			numbers++
			tickets += int64(c)
		}
	}
	return AvailabilityStats{
		NumbersWithStock: numbers,
		TicketsUnsold:    tickets,
		LoadedAt:         a.loaded,
		AgeSeconds:       time.Since(a.loaded).Seconds(),
	}
}

// StreamTickets walks every ticket in id order, calling fn for each.
//
// The hybrid store does not need this -- Postgres selects rows, so we only ever
// need the per-number counts from Load. It exists for tools that genuinely need
// every row, and it shows the right shape for that: a cursor-backed query that
// streams instead of materialising 10,000,000 rows in Go memory.
func StreamTickets(ctx context.Context, pool *pgxpool.Pool, fn func(lottery.Ticket) error) error {
	rows, err := pool.Query(ctx, `SELECT id, num FROM tickets ORDER BY id`)
	if err != nil {
		return fmt.Errorf("streaming tickets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var num int32
		if err := rows.Scan(&id, &num); err != nil {
			return fmt.Errorf("scanning ticket: %w", err)
		}
		if err := fn(lottery.Ticket{ID: int32(id), Number: num}); err != nil {
			return err
		}
	}
	return rows.Err()
}

var _ = pgx.ErrNoRows // keep the pgx import meaningful if the file is trimmed
