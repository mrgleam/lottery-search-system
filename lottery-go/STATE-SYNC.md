# How in-memory state and the database stay in sync

This is the part of the design most worth understanding, and the part most
often got wrong. Read it before changing anything in `postgres/hybrid.go` or
`postgres/load.go`.

## The two things, and who owns what

| | In-memory hint | PostgreSQL |
|---|---|---|
| Question it answers | "which numbers are worth ASKING about" | "which tickets do you ACTUALLY GET" |
| Can it be wrong? | **Yes, by design** | No |
| Cost to read | ~1 ns (a bit test) | ~1 ms (a network round trip) |
| Size | 4 MB | ~1 GB |
| Rebuilt by | one `GROUP BY` at boot | it is the truth |

The hint exists to stop us shipping hopeless candidate numbers to the database.
It is an optimisation. Delete it and the system still works, just slower as
inventory depletes.

## The safety rule (one direction only)

    Over-reporting:  hint says "available", really sold out
                     -> one wasted number in a query. HARMLESS.

    Under-reporting: hint says "sold out", really available
                     -> those tickets are invisible to this replica.
                        Inventory stranded. NEVER ALLOW THIS.

Every decision in the code leans toward over-reporting. When uncertain, say the
number still has stock and let Postgres correct you.

This asymmetry is why the hint does not need transactions, locks, or
consistency guarantees. It is allowed to be wrong in the direction that costs
nothing.

## Boot: loading the hint

`NewHybrid` runs one query:

    SELECT num, count(*) FROM tickets WHERE status <> 2 GROUP BY num

Two decisions worth noticing:

**One row per NUMBER, not per ticket.** 10,000,000 tickets across 1,000,000
numbers means this returns at most a million rows instead of ten million, and
the aggregation happens inside Postgres where the data already lives.

**`status <> 2` means "not sold", not "available".** A reserved ticket still
counts. Its lease may expire and return it to circulation, so counting only
`status = 0` would be the under-reporting mistake above — tickets would vanish
from this replica until something reloaded.

The hint is 4 MB. Loading it is seconds, once, at startup.

## The claim path, in four phases

    1. GENERATE   pattern -> candidate numbers      pure arithmetic, no I/O
    2. FILTER     drop numbers the hint writes off  ~1 ns each, may be wrong
    3. ASK        one UPDATE ... SKIP LOCKED        authoritative, atomic
    4. RECONCILE  fold the answer back into hint    keeps drift bounded

Phase 2 is the only place the hint is used, and it can only ever *narrow* the
candidate list. It never adds a number the pattern did not generate, so even a
corrupted hint cannot produce a wrong ticket — only fewer of them.

Phase 4 is the step people forget. Without it the hint drifts further from
reality on every request.

## What each event does to the hint

| Event | Unsold count | Why |
|---|---|---|
| **Claim / reserve** | **unchanged** | A reservation is not a sale. The lease may expire and give it back. |
| **Confirm / sold** | **−1** | The only event that permanently removes a ticket from circulation. |
| **Release** | unchanged | It was never sold, so nothing was ever subtracted. |
| **Lease expiry** | unchanged | Same reason. Nothing to undo. |

**The most common bug in this whole system** is decrementing on reserve. It
passes every obvious test. It fails only when a lease expires: the count reads
zero, the number leaves the hint, and those tickets become unreachable on that
replica until the next refresh. Silent, and it looks like inventory
mysteriously going missing.

Claim also calls `MarkAvailable` on every number Postgres actually returned a
ticket for. That corrects a hint which had wrongly written the number off —
correcting upward is always safe.

## Multiple replicas

Each replica has its own hint and only witnesses its own sales.

    replica A sells ticket #5   -> A's count drops, B's does not
    replica B sells ticket #9   -> B's count drops, A's does not

So every replica's count is greater than or equal to the truth. That is the
over-reporting direction, so it is safe — just increasingly wasteful as a draw
sells out.

Two ways to converge:

**Periodic refresh** (default, `LOTTERY_HINT_REFRESH=30s`). Re-run the boot
query and swap the hint. Simple, no extra moving parts, converges within one
interval.

**LISTEN/NOTIFY** (`LOTTERY_USE_LISTEN=true`, needs `schema_notify.sql`). A
trigger fires on the sold transition and every replica applies that one change
immediately. Converges in milliseconds, at the cost of a trigger on the hot
write path and one pooled connection held permanently per replica.

Start with refresh. Move to NOTIFY when refresh interval becomes the
bottleneck, which you will know from `candidates_per_request` climbing between
refreshes.

## Choosing the refresh interval

    too long  -> hint over-reports badly near sell-out, so requests ship many
                 hopeless candidates and latency climbs
    too short -> a full GROUP BY over tickets every few seconds

30 s suits a draw selling out over hours. For a flash sale, use NOTIFY.

## Why the hint holds no ticket IDs

It stores only counts per number, never which tickets. Postgres selects rows
from `num = ANY($1)`, so Go never needs the list.

This matters: **a cached ticket ID would be a double-sale waiting to happen.**
A cached count that is slightly wrong costs one wasted query. Cache the cheap
wrong thing, never the dangerous one.

Note the contrast with `MemoryStore`, which *does* build the full CSR index —
because there it is the storage, not a hint.

## Verifying it yourself

    make db && make itest        # includes the hybrid sync tests

The tests that matter:

| Test | Proves |
|---|---|
| `TestHintLoadsFromDatabase` | boot state matches the table |
| `TestReservingDoesNotReduceUnsoldCount` | the common bug is absent |
| `TestConfirmingReducesUnsoldCount` | sales do move the count |
| `TestNumberLeavesHintWhenSoldOut` | sold-out numbers stop being candidates |
| `TestStaleHintOverReportingIsHarmless` | the safety rule holds |
| `TestHintRecoversFromUnderReporting` | refresh cures the bad direction |

`TestStaleHintOverReportingIsHarmless` is the important one. It sells a ticket
behind the store's back — exactly what another replica looks like from here —
then confirms the claim still returns nothing, because Postgres re-checks under
lock.
