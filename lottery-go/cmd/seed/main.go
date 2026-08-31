// Command seed loads the tickets table with random tickets.
//
//	go run ./cmd/seed -n 10000000
//
// It uses COPY rather than INSERT. Ten million rows takes seconds this way and
// a very long time the other way -- worth measuring both in class.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lottery"
	"lottery/postgres"
)

func main() {
	var (
		count = flag.Int("n", 10000000, "how many tickets to create")
		reset = flag.Bool("reset", true, "empty the table first")
		dsn   = flag.String("dsn", envOr("LOTTERY_DSN",
			"postgres://lottery:lottery@localhost:5432/lottery?sslmode=disable"), "database DSN")
	)
	flag.Parse()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool, postgres.Schema); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if *reset {
		if _, err := pool.Exec(ctx, `TRUNCATE tickets RESTART IDENTITY`); err != nil {
			log.Fatalf("truncate: %v", err)
		}
	}

	start := time.Now()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// CopyFromFunc streams rows without building a giant slice in memory.
	rowsLeft := *count
	src := pgx.CopyFromFunc(func() ([]any, error) {
		if rowsLeft == 0 {
			return nil, nil
		}
		rowsLeft--
		return []any{int32(rng.Intn(lottery.NumberSpace))}, nil
	})

	n, err := pool.CopyFrom(ctx, pgx.Identifier{"tickets"}, []string{"num"}, src)
	if err != nil {
		log.Fatalf("copy: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("inserted %d tickets in %s (%.0f rows/sec)\n",
		n, elapsed.Round(time.Millisecond), float64(n)/elapsed.Seconds())

	// ANALYZE so the planner has fresh statistics for the partial indexes.
	if _, err := pool.Exec(ctx, `ANALYZE tickets`); err != nil {
		log.Fatalf("analyze: %v", err)
	}
	fmt.Println("analyzed")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
