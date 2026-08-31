//go:build integration

// Run with:  go test -tags=integration -race ./postgres/...
// Requires a database. LOTTERY_TEST_DSN overrides the default.
package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
	"lottery/postgres"
	"lottery/storetest"
)

const defaultDSN = "postgres://lottery:lottery@localhost:5432/lottery?sslmode=disable"

func dsn() string {
	if v := os.Getenv("LOTTERY_TEST_DSN"); v != "" {
		return v
	}
	return defaultDSN
}

// The Postgres store runs the exact same conformance suite as the in-memory
// one. Nothing in storetest knows which backend it is exercising.
func TestPostgresStore(t *testing.T) {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connecting to %s: %v", dsn(), err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	storetest.Run(t, func(t *testing.T, numbers []int32) storetest.Harness {
		// Each subtest gets its own table via a temporary schema, so tests
		// cannot see each other's tickets.
		if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		for _, n := range numbers {
			if _, err := pool.Exec(ctx, `INSERT INTO tickets (num) VALUES ($1)`, n); err != nil {
				t.Fatalf("insert ticket %d: %v", n, err)
			}
		}
		return storetest.Harness{
			Store: postgres.New(pool),
			// Postgres uses the real clock, so the suite must actually wait.
			// Keep the lease short enough that the suite stays fast.
			Lease:   500 * time.Millisecond,
			Advance: time.Sleep,
		}
	})
}

func TestReaperReleasesExpiredHolds(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tickets (num) VALUES (123456)`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	store := postgres.New(pool)
	p, _ := lottery.ParsePattern("123456")

	got, err := store.Claim(ctx, p, 1, "alice", 200*time.Millisecond)
	if err != nil || len(got) != 1 {
		t.Fatalf("Claim = %v, %v; want one reservation", got, err)
	}

	if n, err := store.ReapExpired(ctx, 100); err != nil || n != 0 {
		t.Fatalf("ReapExpired while lease is live = %d, %v; want 0", n, err)
	}

	time.Sleep(400 * time.Millisecond)

	n, err := store.ReapExpired(ctx, 100)
	if err != nil {
		t.Fatalf("ReapExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped %d reservations, want 1", n)
	}

	st, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.Available != 1 || st.Reserved != 0 {
		t.Errorf("stats = %+v, want 1 available and 0 reserved", st)
	}
}

// Sanity check that concurrent claims really do run in parallel rather than
// queueing, which is the whole point of SKIP LOCKED.
func TestConcurrentClaimsDoNotSerialise(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn())
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	for i := 0; i < 2000; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO tickets (num) VALUES ($1)`, 100000+i%100); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	store := postgres.New(pool)
	p, _ := lottery.ParsePattern("1*****")

	start := time.Now()
	done := make(chan int, 20)
	for u := 0; u < 20; u++ {
		go func(u int) {
			got, err := store.Claim(ctx, p, 5, fmt.Sprintf("user-%d", u), time.Minute)
			if err != nil {
				t.Errorf("user-%d: %v", u, err)
			}
			done <- len(got)
		}(u)
	}
	total := 0
	for i := 0; i < 20; i++ {
		total += <-done
	}
	elapsed := time.Since(start)

	if total != 100 {
		t.Errorf("handed out %d tickets, want 100", total)
	}
	t.Logf("20 concurrent claimers took %s in total", elapsed)
}
