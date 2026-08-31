// Package storetest holds the conformance suite that every TicketStore
// implementation must pass.
//
// The point is that these tests never mention MemoryStore or Postgres. If a new
// backend passes them unchanged, it behaves like the others; if it needs the
// tests edited, that difference is worth a conversation.
package storetest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"lottery"
)

// Harness is what an implementation supplies to run the suite.
type Harness struct {
	// Store is loaded with the ticket numbers requested by the factory.
	Store lottery.TicketStore
	// Lease is a hold length suited to this backend: long for a fake clock,
	// short for one that must really wait.
	Lease time.Duration
	// Advance moves time forward, either by fast-forwarding a fake clock or by
	// actually sleeping.
	Advance func(time.Duration)
}

// Factory builds a fresh, isolated store containing tickets with these numbers.
type Factory func(t *testing.T, numbers []int32) Harness

// Run executes the whole suite against one implementation.
func Run(t *testing.T, newStore Factory) {
	t.Run("ClaimReturnsOnlyMatchingTickets", func(t *testing.T) { claimMatchesOnly(t, newStore) })
	t.Run("ClaimStopsAtK", func(t *testing.T) { claimStopsAtK(t, newStore) })
	t.Run("ClaimPrefersDistinctNumbers", func(t *testing.T) { claimPrefersDistinct(t, newStore) })
	t.Run("ClaimAllowsDuplicatesWhenItMust", func(t *testing.T) { claimDuplicatesWhenNeeded(t, newStore) })
	t.Run("ClaimReturnsFewerWhenStockRunsOut", func(t *testing.T) { claimRunsOut(t, newStore) })
	t.Run("ClaimReturnsNothingWhenNoMatches", func(t *testing.T) { claimNoMatches(t, newStore) })
	t.Run("HeldTicketIsNotOfferedTwice", func(t *testing.T) { heldNotOfferedTwice(t, newStore) })
	t.Run("ExpiredLeaseIsReclaimed", func(t *testing.T) { expiredReclaimed(t, newStore) })
	t.Run("ConfirmMakesTheSalePermanent", func(t *testing.T) { confirmPermanent(t, newStore) })
	t.Run("ConfirmRejectsAnotherHolder", func(t *testing.T) { confirmWrongHolder(t, newStore) })
	t.Run("ConfirmRejectsExpiredLease", func(t *testing.T) { confirmExpired(t, newStore) })
	t.Run("ReleasePutsTheTicketBack", func(t *testing.T) { releasePutsBack(t, newStore) })
	t.Run("ConcurrentClaimsNeverOverlap", func(t *testing.T) { concurrentNoOverlap(t, newStore) })
}

func mustParse(t *testing.T, s string) lottery.Pattern {
	t.Helper()
	p, err := lottery.ParsePattern(s)
	if err != nil {
		t.Fatalf("ParsePattern(%q): %v", s, err)
	}
	return p
}

func claim(t *testing.T, h Harness, pattern string, k int, holder string) []lottery.Reservation {
	t.Helper()
	got, err := h.Store.Claim(context.Background(), mustParse(t, pattern), k, holder, h.Lease)
	if err != nil {
		t.Fatalf("Claim(%q, %d, %q): %v", pattern, k, holder, err)
	}
	return got
}

func claimMatchesOnly(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456, 999923, 111123, 555555})

	got := claim(t, h, "****23", 10, "alice")

	if len(got) != 2 {
		t.Fatalf("claimed %d tickets, want 2", len(got))
	}
	for _, r := range got {
		if r.Number%100 != 23 {
			t.Errorf("ticket %d has number %06d, which does not end in 23", r.TicketID, r.Number)
		}
	}
}

func claimStopsAtK(t *testing.T, newStore Factory) {
	numbers := make([]int32, 100)
	for i := range numbers {
		numbers[i] = 100000 + int32(i)
	}
	h := newStore(t, numbers)

	if got := claim(t, h, "1*****", 5, "alice"); len(got) != 5 {
		t.Errorf("claimed %d tickets, want exactly 5", len(got))
	}
}

// A user searching ****23 wants five different numbers, not five copies of
// one. This is the bug that batching introduced: a single query with LIMIT k
// happily fills the whole limit from the first number it finds.
func claimPrefersDistinct(t *testing.T, newStore Factory) {
	// Ten numbers ending in 23, ten tickets each.
	var numbers []int32
	for n := 0; n < 10; n++ {
		for c := 0; c < 10; c++ {
			numbers = append(numbers, int32(n*10000+123))
		}
	}
	h := newStore(t, numbers)

	got := claim(t, h, "****23", 5, "alice")
	if len(got) != 5 {
		t.Fatalf("claimed %d tickets, want 5", len(got))
	}

	seen := map[int32]int{}
	for _, r := range got {
		seen[r.Number]++
	}
	if len(seen) != 5 {
		t.Errorf("got %d distinct numbers across 5 tickets, want 5; counts = %v",
			len(seen), seen)
	}
}

// The other direction: when the pattern matches only one number, the caller
// must still be able to buy several tickets of it.
func claimDuplicatesWhenNeeded(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456, 123456, 123456, 123456, 123456})

	got := claim(t, h, "123456", 4, "alice")
	if len(got) != 4 {
		t.Fatalf("claimed %d tickets of the only matching number, want 4", len(got))
	}
	for _, r := range got {
		if r.Number != 123456 {
			t.Errorf("got number %06d, want 123456", r.Number)
		}
	}
}

func claimRunsOut(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456, 123456})

	if got := claim(t, h, "123456", 10, "alice"); len(got) != 2 {
		t.Errorf("claimed %d tickets, want 2 (all that exist)", len(got))
	}
}

func claimNoMatches(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	if got := claim(t, h, "999999", 5, "alice"); len(got) != 0 {
		t.Errorf("claimed %d tickets, want none", len(got))
	}
}

func heldNotOfferedTwice(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	if got := claim(t, h, "123456", 1, "alice"); len(got) != 1 {
		t.Fatalf("alice claimed %d tickets, want 1", len(got))
	}
	if got := claim(t, h, "123456", 1, "bob"); len(got) != 0 {
		t.Errorf("bob claimed %d tickets while alice holds the only one", len(got))
	}
}

func expiredReclaimed(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	if got := claim(t, h, "123456", 1, "alice"); len(got) != 1 {
		t.Fatalf("alice should have claimed the ticket")
	}
	h.Advance(2 * h.Lease)

	if got := claim(t, h, "123456", 1, "bob"); len(got) != 1 {
		t.Errorf("bob should get the ticket once alice's lease expired, got %d", len(got))
	}
}

func confirmPermanent(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	got := claim(t, h, "123456", 1, "alice")
	if err := h.Store.Confirm(context.Background(), got[0].TicketID, "alice"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	h.Advance(2 * h.Lease)

	// Expiry applies to reservations, never to sales.
	if again := claim(t, h, "123456", 1, "bob"); len(again) != 0 {
		t.Errorf("a sold ticket must never be claimable again, got %d", len(again))
	}
}

func confirmWrongHolder(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	got := claim(t, h, "123456", 1, "alice")
	err := h.Store.Confirm(context.Background(), got[0].TicketID, "bob")
	if err != lottery.ErrNotHeld {
		t.Errorf("Confirm by bob = %v, want ErrNotHeld", err)
	}
}

func confirmExpired(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	got := claim(t, h, "123456", 1, "alice")
	h.Advance(2 * h.Lease)

	err := h.Store.Confirm(context.Background(), got[0].TicketID, "alice")
	if err != lottery.ErrLeaseExpired {
		t.Errorf("Confirm after expiry = %v, want ErrLeaseExpired", err)
	}
}

func releasePutsBack(t *testing.T, newStore Factory) {
	h := newStore(t, []int32{123456})

	got := claim(t, h, "123456", 1, "alice")
	if err := h.Store.Release(context.Background(), got[0].TicketID, "alice"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if again := claim(t, h, "123456", 1, "bob"); len(again) != 1 {
		t.Errorf("bob should get the released ticket, got %d", len(again))
	}
}

// The requirement the entire system exists for. Run with -race.
func concurrentNoOverlap(t *testing.T, newStore Factory) {
	const users, perUser = 40, 5

	var numbers []int32
	for n := 0; n < 50; n++ {
		for c := 0; c < 10; c++ {
			numbers = append(numbers, int32(100000+n))
		}
	}
	h := newStore(t, numbers)
	p := mustParse(t, "1*****")

	var (
		mu   sync.Mutex
		seen = map[int64]string{}
		wg   sync.WaitGroup
	)
	for u := 0; u < users; u++ {
		wg.Add(1)
		go func(u int) {
			defer wg.Done()
			holder := fmt.Sprintf("user-%d", u)
			got, err := h.Store.Claim(context.Background(), p, perUser, holder, h.Lease)
			if err != nil {
				t.Errorf("%s: Claim: %v", holder, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, r := range got {
				if other, dup := seen[r.TicketID]; dup {
					t.Errorf("ticket %d given to both %s and %s", r.TicketID, other, holder)
				}
				seen[r.TicketID] = holder
			}
		}(u)
	}
	wg.Wait()

	if len(seen) != users*perUser {
		t.Errorf("handed out %d tickets, want %d (there is stock for everyone)",
			len(seen), users*perUser)
	}
}
