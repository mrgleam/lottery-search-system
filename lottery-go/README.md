# Lottery Search System

Search 10,000,000 six-digit lottery tickets by wildcard pattern, and hand them
to concurrent users without ever selling the same ticket twice.

> **Not yet compiled.** This was written in an environment with no Go toolchain
> and no network. Run `make deps && make test` first and fix anything that
> surfaces before teaching from it.

## Quick start

    make deps         # go mod tidy (needs network, pulls pgx)
    make test         # unit tests, no database required
    make db           # postgres in docker
    make itest        # integration tests against the real database
    make seed-small   # 100,000 tickets
    make run          # API on :8080

Then:

    curl -s localhost:8080/search -d '{"pattern":"****23","count":5,"holder":"alice"}'
    curl -s -X POST localhost:8080/reservations/42/confirm -d '{"holder":"alice"}'

## How it works

A six-digit number has only 1,000,000 possible values, and that ceiling is
fixed by the format rather than the data. So matching is arithmetic, not
search: `Pattern.Candidates` generates the numbers a pattern matches without
consulting storage at all. Only then does anything touch the database.

Which tickets a caller actually receives is decided by Postgres, in a single
statement, under row locks:

    UPDATE tickets SET status = 1, ...
     WHERE id IN (SELECT id FROM tickets
                   WHERE num = ANY($1) AND (status = 0 OR lease expired)
                   LIMIT $4 FOR UPDATE SKIP LOCKED)
    RETURNING id, num, lease_until

`SKIP LOCKED` is what lets concurrent claimers proceed in parallel: a
transaction steps over rows another one holds rather than queueing behind them.
Filtering, locking and writing happen in one indivisible step, so there is no
check-then-act window.

## Three backends, one interface

| Implementation | Storage | Use |
|---|---|---|
| `MemoryStore` | RAM only | reference implementation, development, tests |
| `postgres.Store` | Postgres only | straightforward, no caching |
| `postgres.HybridStore` | Postgres + in-memory hint | production; see `STATE-SYNC.md` |

All three satisfy `lottery.TicketStore` and all three pass the same suite in
`storetest`. The API is chosen with `LOTTERY_USE_HINT`.

**How memory and database stay in sync is documented in `STATE-SYNC.md`.**
Read it before touching `postgres/hybrid.go`.

## Layout

| Path | What it holds |
|---|---|
| `pattern.go` | Parsing, `MatchCount`, `NumberAt` -- the odometer |
| `candidates.go` | Batched, scrambled candidate number generation |
| `walker.go` | `j = (a*i + b) mod n`, the permutation that spreads contention |
| `index.go` | CSR index built by counting sort (used by the memory store) |
| `bitmap.go` | Bitmap with `NextSet` for skipping 64 numbers at a time |
| `ticketstore.go` | The `TicketStore` interface, statuses, domain errors |
| `store.go` | `MemoryStore` -- the reference implementation |
| `postgres/store.go` | `Store` -- plain SQL backend, the claim statement |
| `postgres/load.go` | Loading the availability hint from the database |
| `postgres/hybrid.go` | `HybridStore` -- hint in front of Postgres, and the sync path |
| `postgres/sharded.go` | Optional shard-aware store (see the scaling guide) |
| `storetest/` | Conformance suite both implementations must pass |
| `server.go` | HTTP handlers, written against the interface |
| `cmd/api` | The service binary |
| `cmd/seed` | Bulk loader using `COPY` |

## The two implementations

`MemoryStore` and `postgres.Store` both satisfy `lottery.TicketStore`, and both
run **the same test suite** in `storetest`. Nothing in that suite mentions
either backend. If a new implementation passes it unchanged, it behaves like
the others.

The difference worth understanding: `MemoryStore` serialises every claim behind
one `sync.Mutex`, which is the correct-but-serialised design. Postgres with
`SKIP LOCKED` gives the same guarantee while letting claims run in parallel.

## API

| Route | Body | Returns |
|---|---|---|
| `POST /search` | `{"pattern":"****23","count":5,"holder":"alice"}` | reservations, plus `partial` if fewer than requested |
| `POST /reservations/{id}/confirm` | `{"holder":"alice"}` | 200, or 409 / 410 / 404 |
| `POST /reservations/{id}/release` | `{"holder":"alice"}` | 200 |
| `GET /healthz` | | 200 |

`409` means you are not holding that ticket, `410` means the hold expired,
`404` means no such reservation.

## Design notes worth reading before changing anything

**Leases are not database locks.** The row lock lives for milliseconds inside
the claim statement. The user's two-minute hold is a `lease_until` timestamp in
a column. Never hold a transaction open across a payment call.

**Expiry is evaluated at claim time**, in the `WHERE` clause, so correctness
never depends on the reaper running. The reaper exists only to keep the partial
index tidy and admin views clean.

**The `tickets_claimable_num` index is partial on `status <> 2`, not
`status = 0`.** An expired reservation is claimable again and must stay
reachable through the index. Excluding only sold rows is what keeps the index
shrinking as inventory sells out.

**Configuration is environment variables**, listed in `.env.example`.

## Scaling

See `lottery-scaling-guide.md`. The short version:

    make load          # find your current throughput and p99
    make load-sweep    # find the knee
    make stats         # ask the database what it is waiting on

Then fix in this order: connection pool, batch size, Postgres config, API
replicas, Strategy B, sharding. Configuration first, architecture last.

The four metrics that discriminate between failure modes that all look like
"the API got slow":

| Metric | Rising means | Fix |
|---|---|---|
| `candidates_per_request` | Inventory thinning | Strategy B, not more servers |
| `contended_per_request` | Real lock contention | Spread the writes |
| `db_round_trips` | Batch size wrong for the pattern mix | Tune `batchSize` |
| p99 alone | Queueing, usually the pool | Shrink `LOTTERY_MAX_CONNS` |

**The counter-intuitive one:** adding API replicas without shrinking each
replica's connection pool makes throughput *worse*. Total connections across
all replicas should be roughly `(cores × 2)`, not that much per replica.
