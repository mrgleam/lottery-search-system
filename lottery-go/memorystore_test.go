package lottery_test

import (
	"sync"
	"testing"
	"time"

	"lottery"
	"lottery/storetest"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// The whole conformance suite, run against the in-memory implementation.
func TestMemoryStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T, numbers []int32) storetest.Harness {
		tickets := make([]lottery.Ticket, len(numbers))
		for i, n := range numbers {
			tickets[i] = lottery.Ticket{ID: int32(i), Number: n}
		}
		clock := newFakeClock()
		return storetest.Harness{
			Store:   lottery.NewMemoryStore(tickets, clock.Now),
			Lease:   time.Minute,
			Advance: clock.Advance,
		}
	})
}
