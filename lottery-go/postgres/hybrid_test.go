//go:build integration

// Run with:  go test -tags=integration -race ./postgres/...
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
	"lottery/postgres"
	"lottery/storetest"
)

// The hybrid store runs the same conformance suite as everything else. The
// in-memory hint must not change observable behaviour in any way.
func TestHybridStore(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	storetest.Run(t, func(t *testing.T, numbers []int32) storetest.Harness {
		if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		for _, n := range numbers {
			if _, err := pool.Exec(ctx, `INSERT INTO tickets (num) VALUES ($1)`, n); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
		// Built AFTER the inserts, so the hint reflects the seeded state.
		store, err := postgres.NewHybrid(ctx, pool, nil)
		if err != nil {
			t.Fatalf("NewHybrid: %v", err)
		}
		return storetest.Harness{
			Store:   store,
			Lease:   500 * time.Millisecond,
			Advance: time.Sleep,
		}
	})
}

// The hint must reflect what is in the database at boot.
func TestHintLoadsFromDatabase(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456, 123456, 123456, 999999)

	store, err := postgres.NewHybrid(ctx, pool, nil)
	if err != nil {
		t.Fatalf("NewHybrid: %v", err)
	}

	st := store.HintStats()
	if st.NumbersWithStock != 2 {
		t.Errorf("numbers with stock = %d, want 2", st.NumbersWithStock)
	}
	if st.TicketsUnsold != 4 {
		t.Errorf("tickets unsold = %d, want 4", st.TicketsUnsold)
	}
}

// Reserving must NOT reduce the unsold count: a reservation can expire, and a
// hint that counted it as gone would strand the ticket.
func TestReservingDoesNotReduceUnsoldCount(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456, 123456)

	store, _ := postgres.NewHybrid(ctx, pool, nil)
	p, _ := lottery.ParsePattern("123456")

	before := store.HintStats().TicketsUnsold
	if _, err := store.Claim(ctx, p, 1, "alice", time.Minute); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	after := store.HintStats().TicketsUnsold

	if after != before {
		t.Errorf("unsold count changed on reserve: %d -> %d; a reservation is not a sale", before, after)
	}
}

// Confirming a sale is the only thing that reduces the count.
func TestConfirmingReducesUnsoldCount(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456, 123456)

	store, _ := postgres.NewHybrid(ctx, pool, nil)
	p, _ := lottery.ParsePattern("123456")

	got, err := store.Claim(ctx, p, 1, "alice", time.Minute)
	if err != nil || len(got) != 1 {
		t.Fatalf("Claim = %v, %v", got, err)
	}
	before := store.HintStats().TicketsUnsold

	if err := store.Confirm(ctx, got[0].TicketID, "alice"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	after := store.HintStats().TicketsUnsold

	if after != before-1 {
		t.Errorf("unsold count = %d after a sale, want %d", after, before-1)
	}
}

// When the last ticket of a number sells, the number must drop out of the hint
// so future requests stop shipping it as a candidate.
func TestNumberLeavesHintWhenSoldOut(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456)

	store, _ := postgres.NewHybrid(ctx, pool, nil)
	p, _ := lottery.ParsePattern("123456")

	got, _ := store.Claim(ctx, p, 1, "alice", time.Minute)
	if err := store.Confirm(ctx, got[0].TicketID, "alice"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	if n := store.HintStats().NumbersWithStock; n != 0 {
		t.Errorf("numbers with stock = %d after selling the only ticket, want 0", n)
	}
	if again, _ := store.Claim(ctx, p, 1, "bob", time.Minute); len(again) != 0 {
		t.Errorf("bob got %d tickets from a sold-out number", len(again))
	}
}

// THE SAFETY PROPERTY. A stale hint that over-reports availability must never
// change the answer, because Postgres re-checks under lock.
//
// We simulate the stale case by selling a ticket behind the store's back --
// exactly what another replica doing its own sales looks like from here.
func TestStaleHintOverReportingIsHarmless(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456)

	store, _ := postgres.NewHybrid(ctx, pool, nil)
	p, _ := lottery.ParsePattern("123456")

	// Another replica sells the only ticket. Our hint still says it exists.
	if _, err := pool.Exec(ctx, `UPDATE tickets SET status = 2 WHERE num = 123456`); err != nil {
		t.Fatalf("simulating another replica: %v", err)
	}
	if store.HintStats().NumbersWithStock != 1 {
		t.Fatal("precondition: the hint should still be stale here")
	}

	// The hint says yes; the database says no. The database wins.
	got, err := store.Claim(ctx, p, 1, "bob", time.Minute)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("claimed %d tickets from a sold-out number; the database should have refused", len(got))
	}

	// And a refresh brings the hint back in line.
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := store.HintStats().NumbersWithStock; n != 0 {
		t.Errorf("numbers with stock = %d after refresh, want 0", n)
	}
}

// The opposite direction: a hint that has wrongly written a number off must be
// corrected when the database proves stock exists.
func TestHintRecoversFromUnderReporting(t *testing.T) {
	ctx := context.Background()
	pool := freshPool(t, ctx)
	defer pool.Close()

	seed(t, ctx, pool, 123456)

	store, _ := postgres.NewHybrid(ctx, pool, nil)
	p, _ := lottery.ParsePattern("123456")

	// Sell it, then have another replica release it back.
	got, _ := store.Claim(ctx, p, 1, "alice", time.Minute)
	store.Confirm(ctx, got[0].TicketID, "alice")
	if store.HintStats().NumbersWithStock != 0 {
		t.Fatal("precondition: the number should have left the hint")
	}
	if _, err := pool.Exec(ctx, `UPDATE tickets SET status = 0, holder = NULL WHERE num = 123456`); err != nil {
		t.Fatalf("simulating another replica: %v", err)
	}

	// The hint filters this number out, so this claim finds nothing. That is
	// the under-reporting failure -- inventory invisible to this replica.
	if again, _ := store.Claim(ctx, p, 1, "bob", time.Minute); len(again) != 0 {
		t.Log("claim succeeded despite the stale hint, which is fine too")
	}

	// Refresh is the cure.
	if err := store.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	after, _ := store.Claim(ctx, p, 1, "bob", time.Minute)
	if len(after) != 1 {
		t.Errorf("bob got %d tickets after refresh, want 1", len(after))
	}
}

func freshPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func seed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, numbers ...int32) {
	t.Helper()
	for _, n := range numbers {
		if _, err := pool.Exec(ctx, `INSERT INTO tickets (num) VALUES ($1)`, n); err != nil {
			t.Fatalf("insert %d: %v", n, err)
		}
	}
}
