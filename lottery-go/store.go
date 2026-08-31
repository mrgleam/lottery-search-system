package lottery

import (
	"context"
	"sync"
	"time"
)

// batchSize is how many candidate numbers we consider at a time.
const batchSize = 64

// MemoryStore keeps every ticket in RAM. It is the reference implementation:
// fast, easy to reason about, and used to develop the HTTP layer before a
// database exists.
//
// It requires ticket IDs to be dense: 0..len(tickets)-1.
type MemoryStore struct {
	mu    sync.Mutex
	index *Index
	now   func() time.Time

	status     []Status
	holder     []string
	leaseUntil []time.Time
	numberOf   []int32

	// unsold[n] counts tickets of number n that are not yet Sold, and
	// unsoldNumbers has a bit set wherever that count is above zero.
	//
	// Counting UNSOLD rather than free is deliberate. An expired reservation
	// is claimable again, so a free-ticket count would read zero, clear the
	// bit, and strand those tickets until a reaper ran.
	unsold        []uint16
	unsoldNumbers *Bitmap
}

// NewMemoryStore indexes the tickets and prepares availability structures.
// Pass time.Now in production; pass a fake clock in tests.
func NewMemoryStore(tickets []Ticket, now func() time.Time) *MemoryStore {
	s := &MemoryStore{
		index:         BuildIndex(tickets),
		now:           now,
		status:        make([]Status, len(tickets)),
		holder:        make([]string, len(tickets)),
		leaseUntil:    make([]time.Time, len(tickets)),
		numberOf:      make([]int32, len(tickets)),
		unsold:        make([]uint16, NumberSpace),
		unsoldNumbers: NewBitmap(NumberSpace),
	}
	for _, t := range tickets {
		s.numberOf[t.ID] = t.Number
		s.unsold[t.Number]++
		s.unsoldNumbers.Set(int(t.Number))
	}
	return s
}

func (s *MemoryStore) Claim(ctx context.Context, p Pattern, k int, holder string, lease time.Duration) ([]Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	until := now.Add(lease)
	out := make([]Reservation, 0, k)

	// PASS 1 -- at most one ticket per number, so the caller sees a spread of
	// different numbers rather than several copies of one.
	s.claimPass(p, k, 1, holder, now, until, &out, ctx)

	// PASS 2 -- top up with duplicates only if pass 1 could not fill the
	// request. Required for patterns with few matching numbers: "123456"
	// matches exactly one number, so five tickets must come from that number.
	if len(out) < k {
		s.claimPass(p, k, 0, holder, now, until, &out, ctx)
	}
	return out, nil
}

// claimPass walks the match set once, taking up to perNumber tickets from each
// number (perNumber == 0 means unlimited), appending to out until it holds k.
func (s *MemoryStore) claimPass(p Pattern, k, perNumber int, holder string, now, until time.Time, out *[]Reservation, ctx context.Context) {
	cands := p.Candidates(RandomSeed())
	for len(*out) < k {
		if ctx.Err() != nil {
			return
		}
		batch := cands.NextBatch(batchSize)
		if len(batch) == 0 {
			return // walked the whole match set
		}
		for _, n := range batch {
			if len(*out) == k {
				return
			}
			if !s.unsoldNumbers.Test(int(n)) {
				continue // sold out
			}
			taken := 0
			for _, id := range s.index.TicketsFor(int(n)) {
				if len(*out) == k || (perNumber > 0 && taken == perNumber) {
					break
				}
				if !s.claimable(id, now) {
					continue
				}
				s.status[id] = Reserved
				s.holder[id] = holder
				s.leaseUntil[id] = until
				*out = append(*out, Reservation{TicketID: int64(id), Number: n, LeaseUntil: until})
				taken++
			}
		}
	}
}

// claimable evaluates expiry at claim time, so no background job is needed for
// correctness.
func (s *MemoryStore) claimable(id int32, now time.Time) bool {
	switch s.status[id] {
	case Available:
		return true
	case Reserved:
		return now.After(s.leaseUntil[id])
	default:
		return false
	}
}

func (s *MemoryStore) Confirm(ctx context.Context, id int64, holder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id < 0 || int(id) >= len(s.status) {
		return ErrTicketNotFound
	}
	if s.status[id] != Reserved || s.holder[id] != holder {
		return ErrNotHeld
	}
	if s.now().After(s.leaseUntil[id]) {
		return ErrLeaseExpired
	}
	s.status[id] = Sold

	n := s.numberOf[id]
	s.unsold[n]--
	if s.unsold[n] == 0 {
		s.unsoldNumbers.Clear(int(n))
	}
	return nil
}

func (s *MemoryStore) Release(ctx context.Context, id int64, holder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id < 0 || int(id) >= len(s.status) {
		return ErrTicketNotFound
	}
	if s.status[id] != Reserved || s.holder[id] != holder {
		return ErrNotHeld
	}
	s.status[id] = Available
	s.holder[id] = ""
	return nil
}

// StatusOf is for tests and admin views.
func (s *MemoryStore) StatusOf(id int64) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[id]
}
