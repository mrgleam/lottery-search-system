# Instructor Manual — Lottery Search System

Everything you need to teach the 142-step course, including the background that
students never see but you must know to answer their questions.

**Companion documents**

| Document | Whose |
|---|---|
| `lottery-training-course.md` | students — 142 numbered steps |
| `lottery-search-teaching-guide.md` | you — the concept lesson, Module 2 |
| `lottery-go-tutorial.md` | students — TDD walkthrough, Modules 3–10 |
| `lottery-scaling-guide.md` | both — Module 11 |
| `lottery-go/STATE-SYNC.md` | both — Module 9 |
| `lottery-go/` | the reference implementation |

---

# Part 1 — Before you teach

## The one thing you must do

**Run the entire course yourself, end to end, before day one.** The Go code was
written without a compiler available. Work through Modules 0–10 on your own
machine and fix whatever breaks.

You are not just checking for typos. You need to have personally hit the errors
your students will hit, because "I don't know, let's find out" is fine once and
corrosive by the third time.

```
tar -xzf lottery-go-source.tar.gz && cd lottery-go
go mod tidy
go test ./...
make db && go test -tags=integration -race ./postgres/...
make seed-small && make run
```

Record what broke and how long the fix took. That becomes your errata sheet.

## Prepare these physical items

| Item | Used in |
|---|---|
| 12 index cards, ticket ID and number written on each | Module 4, Step 42 |
| Chalk or tape for 12 squares on a desk | Module 4, Step 42 |
| Graph paper, one sheet per student | Module 4, Step 47 |
| A large whiteboard you will not erase for a whole day | Module 2 |

The card exercise is not optional decoration. It is the intervention for
students who did not understand the cursor copy, and it works in about three
seconds where ten minutes of talking fails.

## Prepare these environments

1. A Postgres instance seeded with **10 million** tickets (`make seed`), for
   Module 11. Seeding takes minutes; do not do it in front of the class.
2. A second copy seeded and then **98% sold**, for Module 9 Step 104.
3. Your own machine able to run two API instances on different ports, for
   Module 9 Step 123.

## Timing

Eleven modules, roughly eleven days. Realistic pacing for a group of six to
twelve juniors:

| Module | Planned | Usual reality |
|---|---|---|
| 0 Environment | 2h | 2–4h. Someone's Docker will not start. |
| 1 Go basics | 1 day | 1 day. Do not compress this. |
| 2 Concepts | 1 day | 1 day. Resist "can we start coding?" |
| 3 Patterns | 1 day | often half a day |
| 4 Data structures | 1 day | **frequently 1.5 days** — the hardest module |
| 5 Store | 1 day | 1 day |
| 6 Concurrency | 1 day | 1 day |
| 7 HTTP | 1 day | often half a day |
| 8 Database | 2 days | 2 days |
| 9 The hint | 1 day | 1 day |
| 10 Shipping | 1 day | half a day |
| 11 Scaling | 1 day | 1 day |

If you must cut, cut Modules 10 and 11 and run them later as a follow-up. Never
cut Module 2 or Module 4.

---

# Part 2 — How to teach this course

## The five rules

**1. Make them propose the wrong answer first.**
A student who has *proposed* scanning ten million rows understands the mailbox
idea about three times better than one who was handed it. Never correct the
naive answer — price it.

**2. Never show a finished table.**
This was the single biggest flaw in the first draft of the material. Showing
`offsets` complete lets students read it and believe they understand it. They
cannot build it. Every table gets constructed in front of them, one pass at a
time.

**3. Break things on purpose.**
The course has five deliberate breakages (Steps 45, 49, 60, 98, 122). Students
remember failures they watched far better than rules they were told. Never skip
one to save time.

**4. Measure instead of asserting.**
"Broad patterns are cheaper" is a claim. A counter printed to the terminal is
evidence. Wherever the course offers a measurement, take it.

**5. Silence after a question.**
Junior engineers need five to ten seconds to think. Most instructors wait two
and then answer their own question, which trains the room to wait you out. Count
to ten in your head.

## The rhythm inside every step

    ask → let them answer → show the number that hurts → give the fix → check

Skipping the second beat is the most common instructor error.

## What "checkpoint" means

Every module ends with one. It is not a formality. **Do not let a student
proceed past a failed checkpoint** — every module builds on the last, and a
student who did not understand Module 4 will spend Module 5 copying code.

Pair a struggling student with one who finished. Explaining is the best
consolidation the finisher will get.

---

# Part 3 — Module-by-module lesson plans

## Module 0 — Environment (Steps 1–9)

**Goal:** everyone can run `go test`, `docker run`, and `git status`.

Do not lecture. Circulate and unblock. Put the four verification commands from
Step 9 on the board and check each student off personally.

**Expect:** one Docker permission problem on Linux (`usermod -aG docker`, then
log out), one `PATH` problem, one person on Go 1.21 who needs 1.22 for the
routing patterns.

**Do not start Module 1 until every student passes Step 9.** An environment
problem found on day three costs a day.

---

## Module 1 — Go basics (Steps 10–21)

**Goal:** they can write a struct, a method, and a table test unaided.

**Deliberately absent:** interfaces, goroutines, channels. Those arrive in
Modules 6, 8 and 9, at the point the project needs them. Someone will ask why —
tell them. "You'll meet interfaces when we have two implementations that need
one" is a satisfying answer and models good design thinking.

**Spend the most time on Steps 19–20.** Failure messages and table tests are the
habits that make the next ten days bearable. Insist on the Go convention:

    funcName(input) = got, want X

**Checkpoint discipline matters here.** Step 21 asks them to write a struct,
method and table test from scratch. If they cannot, redo Steps 13–20. Everything
afterwards assumes this.

---

## Module 2 — Concepts (Steps 22–30)

**Laptops closed. All day.**

This is the module students most want to skip and the one they can least afford
to. Say so at the start: *"Today we write no code. Tomorrow you'll write code
you understand, instead of code you copied."*

### The arc

| Time | Steps | What happens |
|---|---|---|
| 30 min | 22–23 | Twelve tickets on the board; they find `1*3` by hand |
| 45 min | 24 | They propose scanning; you price it |
| 45 min | 25 | **The hinge** — 10,000,000 vs 1,000,000 |
| 60 min | 26–27 | Mailboxes, then the odometer |
| 45 min | 28 | Early termination |
| 90 min | 29–30 | The race condition, leases, contention |

### Step 25 is the hinge — do not rush it

Write both numbers side by side and leave them there all day:

    Tickets:                    10,000,000    big
    Possible numbers:            1,000,000    small, and FIXED FOREVER

Ask: *"If we sold 100 million tickets tomorrow, how many possible numbers?"*

**Wait for someone to say "still a million."** If nobody does, you have not
landed it, and nothing downstream will work. Ask it three different ways before
moving on.

### Step 27 — the question that produces the insight

After they list the ten numbers matching `1*3`, ask:

> *"Did you look at the ticket data to work that out?"*

They did not. They could not have — you never told them which tickets exist.
Let that land before saying anything else.

### Step 29 — draw the interleaving, do not describe it

```
        User A                          User B
  t1    is #4829112 free?  → yes
  t2                                    is #4829112 free? → yes
  t3    mark it as mine
  t4                                    mark it as mine
```

Write it line by line, pausing at t2. Someone usually sees it before t4.

**Say explicitly: this is a money bug, not a performance bug.** No amount of
clever indexing prevents it.

### Common confusions in this module

| Confusion | Fix |
|---|---|
| "Why not just use a database index?" | You are — that is what the mailbox is. We're deciding what to index and how. |
| "10 tickets per number seems too few" | It's an average. Ask what happens if a number is popular; the design does not care. |
| "Can't we just lock the whole table?" | Yes, and then one user at a time worldwide. Name it as the baseline we improve on. |

---

## Module 3 — Patterns (Steps 31–39)

**Goal:** first TDD cycle, first passing tests.

### The three states

Make them observe all three, in order:

1. **Does not compile** — `undefined: ParsePattern`. Say: *"the compiler just
   wrote your to-do list."*
2. **Compiles, fails** — returns a zero value. This proves the test can detect
   the bug.
3. **Passes.**

**Students skip state 2** by writing the full implementation immediately. Then
they never know whether their test works. Insist.

### The demo worth doing live

Change one `%w` to `%v` in `ParsePattern`. Run the tests. Watch `errors.Is` stop
matching. Change it back. Five seconds, permanent memory.

### Step 38 — name the shift

Spot checks prove seven things. The property test proves three things for *all*
inputs. Say out loud: **"this is a step up in testing skill, not just another
test."**

Someone will ask whether testing `NumberAt` with `Matches` is cheating. Good
question, and the answer is instructive: they are *different algorithms* — one
builds digits up, the other masks them out. Testing one against the other
catches errors in either. Copying `NumberAt`'s logic into the test would be
cheating.

---

## Module 4 — Data structures (Steps 40–51)

**The hardest module. Budget 1.5 days.**

### The opening question

> *"How much memory does an empty Go slice use?"*

They say zero. They are wrong, and finding out is the lesson. Roughly 48 bytes
of header per bucket; a million buckets is ~48 MB before a single ticket ID.

### Steps 41–42 — paper before keyboard

**Do not let anyone code until they have filled in all three tables by hand.**
Count, running total, place. Students who code first write the cursor bug and
cannot see why.

If the paper exercise does not land, run the card activity immediately. Twelve
cards, twelve chalk squares, three students: one counts, one computes the
running total, one carries cards while moving the cursor with a finger.

**Moving the cursor with your own finger teaches the copy problem instantly.**

### The two questions students always ask

**"Why does `offsets` have 11 entries for 10 numbers?"**
Every number needs an *end*, and the last has no successor to ask. The sentinel
means the reading rule works for every number with no special case. Generalise
it: **one extra element buys you zero branches.** That trade appears constantly.

**"Why copy `offsets` into `cursor`?"**
Because walking `offsets` directly advances every entry past its own block and
destroys the index. Have them do it (Step 45) and watch the first lookup pass
while the rest fail.

### Step 47 — the graph-paper question

After they tick numbers 5, 13 and 21:

> *"All three landed in box 5. Why don't they collide?"*

This is where two formulas instead of one makes sense. Do not answer it for
them.

### Step 49 — the `=` versus `|=` demo

Have them write `Set` with `=`. Bits 0, 1, 5 and 63 vanish when they set 7. One
word holds 64 members; overwriting it wipes 63 neighbours.

### If a student is drowning here

Go back to twelve tickets and three digits. Everything in this module works at
that scale, and the numbers fit in a person's head.

---

## Module 5 — The store (Steps 52–61)

### Step 54 — the argument worth having

> *"The store tracks which numbers still have tickets. Free tickets, or unsold
> tickets?"*

Most say free. It is wrong, and **the wrong version passes every test they have
written so far.** An expired reservation is claimable again; counting free
tickets makes it read as zero and strands those tickets.

Only the Module 6 lease test catches it. Say that out loud: **it is an argument
for writing that test earlier than feels necessary.**

### Steps 56–57 — the distinct-numbers requirement

Ask them to answer as a *customer*, not a programmer:

> *"You search `****23` and ask for 5 tickets. Do you want five different
> numbers, or is five copies of one acceptable?"*

Five identical numbers means one losing draw wipes you out. Variety is the
reason they searched a pattern at all.

Then the trap: once they accept "distinct", the obvious fix is strictly one per
number. Ask what happens with `123456` and five tickets — exactly one number
matches, so a strict rule returns one ticket. **Prefer, not require.** Two
passes.

Juniors rarely spot this on their own. It is a good demonstration that
requirements have edges.

### Step 60 — the quiet bug

With `a = 5, n = 10` the walker reaches two values out of ten. Eight boxes are
permanently unreachable.

**Emphasise:** no crash, no error, no log line. The system looks healthy while
inventory is unsellable. It surfaces months later when the books do not
balance.

*"The quiet bugs are the expensive ones"* is the sentence to leave them with.

---

## Module 6 — Concurrency (Steps 62–71)

### Step 65 — the demo that must be run twice

The counter test, once without `-race` and once with. Without it, the count is
sometimes 100 and sometimes not. With it, Go names the conflicting lines.

**The lesson: a passing concurrent test proves very little on its own.** From
here on, `-race` is part of "the tests pass."

### Two traps to warn about before they hit them

**`t.Fatalf` inside a goroutine hangs the test.** It calls `runtime.Goexit`, so
`wg.Wait()` never returns. Use `t.Errorf`.

**The test needs its own mutex** for the shared map. Students find it funny that
the test for a concurrency bug can itself have one. Let them.

### Step 69 — why the clock is injected

Ask how they would test a two-minute lease otherwise. The answer —
`time.Sleep(3 * time.Minute)` — is its own argument.

Someone will ask why a `func` rather than a `Clock` interface. Answer: we need
one method, so a function value suffices. **Define an interface when you have
two implementations that need it, not before.** They will meet the counterpoint
in Module 8, where Postgres cannot take a fake clock at all.

---

## Module 7 — HTTP (Steps 72–80)

Usually the easiest module. Two traps carry the weight.

### Step 78 — nil versus empty

A nil slice marshals to `null`, an empty slice to `[]`. Clients doing
`for (const t of data.tickets)` break on `null`.

### Step 79 — the wire type

`{"number": 2323}` is not a lottery number. It is `002323` — six characters.

Ask: *"Should we change `Reservation.Number` to a string?"*

**No.** Internally `int32` is right — the hint indexes by it, the pattern
generates it. What is wrong is using the domain type as the wire type. Give the
HTTP layer its own view struct.

**The principle: the shape of your data in memory and the shape you promise
clients are two different contracts.** Couple them and you cannot change one
without breaking the other.

---

## Module 8 — Database (Steps 81–103)

Two days. Day one is interfaces and SQL; day two is the implementation.

### Step 85 — the harness awkwardness is a teaching moment

The conformance suite cannot give Postgres a fake clock, because `now()` lives
inside SQL. So the harness asks for a lease length *and* a way to move past it.

Ask why we do not just fake the clock for Postgres too. Answer: time would then
come from the application, and two app servers with skewed clocks would disagree
about expiry. **The database must own time.**

*"Test friction usually points at something true about the design"* is worth
saying.

### Steps 92–93 — the demonstration that anchors the whole course

Two `psql` windows on the projector.

Window 1:

```sql
BEGIN;
SELECT * FROM tickets WHERE id = 1 FOR UPDATE;
```

Window 2:

```sql
SELECT * FROM tickets WHERE id = 1 FOR UPDATE;   -- hangs
```

Let it hang for a full ten seconds. Then `COMMIT` in window 1 and watch it
unblock.

Now repeat with `SKIP LOCKED` in window 2 — **instant, different row, no
error.**

**This is the single most important demonstration in the course.** Module 2
said "checking and taking must be one indivisible step." This is the mechanism.
Do not rush it, and do not replace it with slides.

### Step 95 — the index predicate question

> *"Why is the claim index partial on `status <> 2` rather than `status = 0`?"*

Let them argue. `status = 0` looks obviously right and is smaller.

It is broken. An expired reservation has `status = 1`, so it falls out of that
index and the claim query never finds it. **Correctness would silently depend on
the cleanup job running.**

### Steps 98–100 — the best lesson in the course

Have them run the real curl against a seeded database:

```
curl -s localhost:8080/search -d '{"pattern":"****23","count":10,"holder":"alice"}'
```

Ten tickets, all the same number. **The bug they fixed in Module 5 is back.**

Make them work out why before you say anything. The walker still scrambles. The
two-pass logic is still in `MemoryStore`. The conformance suite still passes.

**Batching.** We send 256 numbers with `LIMIT 10`; Postgres scans the index in
`num` order, fills the limit from the first number, and stops.

Then the three points that matter:

1. **An optimisation at one layer silently undid a guarantee at another.**
   Neither change was wrong alone.
2. Nothing crashed and no test failed — because the distinct test lived in a
   `MemoryStore` file, and `MemoryStore` does not batch.
3. **Real systems break this way far more often than they break loudly.**

Step 100 moves the test into the shared suite. That is what a conformance suite
is *for*.

### Step 103 — the payoff

The same eleven tests now check Postgres. Not similar tests — the same code.

Then have someone change `SKIP LOCKED` to plain `FOR UPDATE` and rerun the
concurrent test. **Correctness assertions still pass; elapsed time climbs.** The
argument for `SKIP LOCKED`, delivered as a measurement.

---

## Module 9 — The hint (Steps 104–123)

### Steps 104–106 — problem before cure

Sell 98% of inventory, run the load test, note the p99. Compare against a fresh
database. Dramatically slower with no code change.

Ask where the time went. Answer: almost every candidate is a sold-out number, so
we pay a round trip and an index lookup to be told "no", over and over.

**The fix is not a faster database. It is not asking.**

### Step 108 — the safety rule goes on the board and stays there

```
Over-reporting:   says "available", really sold out
                  → one wasted lookup.  HARMLESS.

Under-reporting:  says "sold out", really available
                  → tickets invisible to this replica. STRANDED. NEVER.
```

Ask why one is harmless and the other serious. **Because Postgres re-checks
under lock — but nothing can correct a question we never asked.**

### Step 109 — make them commit to an answer in writing

> *"A user reserves a ticket. Should the count go down?"*

Have them write it down before continuing. Most say yes. The answer is no: a
reservation is not a sale, and the lease may expire.

Committing to a wrong answer in writing makes the correction stick.

### Step 122 — break it, then break it worse

They add the decrement. Two tests fail.

Then have them **delete their own Step 116 test** and run again. The suite still
fails — but now only on a lease test about something else entirely.

**That is the lesson.** Without a test aimed at the invariant, this surfaces
days later as "expired reservations sometimes don't come back."

### Step 123 — two replicas

Start two instances on different ports. Sell through one, watch the other's
`HintStats` stay stale until refresh.

Each replica only witnesses its own sales, so every count is ≥ the truth — the
over-reporting direction, safe but wasteful.

---

## Module 10 — Shipping (Steps 124–131)

Usually half a day.

**The point to press:** Go's `http.Server` timeout defaults are all **zero**,
meaning no timeout. That is a slow-loris vulnerability in every Go service that
forgets them. Not optional.

Ask what happens to a user mid-claim during a rolling deploy. With
`srv.Shutdown`, their request finishes. Without it, the connection drops and
they lose their tickets.

---

## Module 11 — Scaling (Steps 132–142)

### Step 133 — p50 versus p99

An average of 20 ms tells you nothing. A p50 of 11 ms with a p99 of 185 ms
means one request in a hundred is sixteen times slower — and at 3,400 req/sec
that happens 34 times a second, to real people.

### Step 137 — the experiment that surprises everyone

Set `LOTTERY_MAX_CONNS=100`, run the load test. Then set it to `5`.

**Have them predict first.** Most expect fewer connections to be slower. It gets
faster, because Postgres stops context-switching. That surprise is the most
valuable five minutes in the module.

### Step 139 — diagnose three sabotaged systems

You break it three ways; they diagnose from metrics alone without being told
which.

1. Oversized connection pool → p99 explodes, other metrics flat
2. Nearly-empty inventory, broad patterns → `candidates_per_request` climbs
3. Fixed walker seed → `contended_per_request` climbs

**Same symptom, three causes, three fixes.** Reading the metrics is the skill.

### Step 141 — the non-technical constraint

Sharding makes ticket selection non-uniform across inventory. For a real
lottery **that may be a regulatory question, not an engineering one.**

"Technically better" and "allowed" are different tests. Engineering does not
decide this alone. Worth ninety seconds — most juniors have never been told
that some technically-correct changes are not theirs to make.

---

# Part 4 — Background you need but students do not

## SKIP LOCKED, in more depth

Three answers to "someone holds this row":

| Strategy | Behaviour | Cost |
|---|---|---|
| `FOR UPDATE` | wait for the holder to commit | correct; throughput collapses to one at a time |
| `FOR UPDATE NOWAIT` | error immediately | application must catch and retry |
| `FOR UPDATE SKIP LOCKED` | pretend the row is absent, move on | no wait, no error; incomplete view |

**Details that matter if asked:**

- Skipping happens **after** the `WHERE` filter and **before** `LIMIT`, so
  `LIMIT 5` returns five rows you actually got.
- The lock lives until `COMMIT`, not until the statement ends. This is the
  biggest source of production bugs with the pattern.
- It requires reading the heap, so it defeats index-only scans. That is why the
  partial index matters.
- Postgres' own docs say it is unsuitable for general queries — only for
  queue-like access. `SELECT count(*) ... SKIP LOCKED` is meaningless.

**Under `READ COMMITTED`**, if you lock a row a concurrent transaction just
updated, Postgres re-fetches the newest version and re-evaluates your `WHERE`
against it. If it no longer qualifies, the row is silently dropped. **That is
exactly what we want** — it closes the last gap. Under `REPEATABLE READ` you
would get a serialization failure instead, which is why queue workloads use
`READ COMMITTED`.

Availability: PostgreSQL 9.5+, MySQL 8.0+, Oracle, SQL Server (`READPAST`).
SQLite has no row locking at all.

## The two locks students conflate

| | Database row lock | Lease |
|---|---|---|
| Lives for | milliseconds | minutes |
| Enforced by | Postgres | your application |
| Released by | `COMMIT` | clock expiry |
| Survives a crash? | no | yes — it is a column |

**The disaster this prevents:** holding a transaction open across a payment
gateway call. A row lock then lives for thirty seconds, every worker scans past
it, and a slow payment provider takes the system down.

Never hold a database lock across a network call to something you do not
control.

## Do you need two API replicas?

A student will ask. **Not for throughput.**

| Phase | Cost |
|---|---|
| Parse JSON | ~2 µs |
| Generate candidates | ~5 µs |
| **Postgres round trip** | **~1–3 ms** |
| Marshal JSON | ~3 µs |

Over 99% of wall-clock time is waiting on the database. A second Go process
gives you twice as much of almost nothing.

**Why juniors get this wrong:** in Node, Python or Ruby, one process uses one
core, so you run N processes for N cores. Go's scheduler already spreads
goroutines across every core. **"Run more copies to use more cores" is a
language artifact, not a law.**

The ceiling is connections, not Go:

    pool of 20 ÷ 2 ms per claim ≈ 10,000 claims/sec

And Postgres on 8 cores doing `SKIP LOCKED` updates with WAL fsyncs sustains a
few thousand writes/sec. **The database hits its limit first.**

Reasons to run two anyway, none about throughput: zero-downtime deploys
(usually the real reason), surviving a node failure, and somewhere to drain
traffic during a restart.

**The cost specific to this system:** each replica has its own hint and only
witnesses its own sales. Four replicas means four hints drifting apart. Worth
naming so students see "stateless" is a spectrum.

## Why read replicas do not help

Every request writes. A claim is an `UPDATE`. There is nothing to send to a
replica. **The standard scaling playbook assumes read-heavy traffic.** Always
check which kind you have first.

## Why we cache counts and never ticket IDs

A cached ticket ID is a double-sale waiting to happen. A count that is slightly
wrong costs one query.

**Cache the cheap wrong thing, never the dangerous one.**

## Why the design uses arithmetic rather than a text index

Students who know Elasticsearch will ask. It is built for *text* search — finding
words where you do not know the shape of the data. Here the data has exactly one
shape: six digits. Leading-wildcard queries like `*****3` are also its weakest
case. Using it means operating a cluster to do a job arithmetic does for free.

---

# Part 5 — Questions students ask

**"Why not `LIKE '____23'` in SQL?"**
It works, and it is the right first instinct. But it makes the database examine
rows to find matches — the work we showed was unnecessary. We already know which
numbers match; computing them takes microseconds and touches no disk.

**"Isn't a million mailboxes a lot of memory?"**
~46 MB for the memory store's full index, ~4 MB for the hint. Show the
arithmetic rather than asserting.

**"What if two patterns match the same ticket?"**
They can — `123456` matches both `1*****` and `****56`. This is exactly why the
guarantee is per-*ticket*, not per-pattern. The requirement as originally worded
would have permitted a real bug.

**"Why not lock the whole table?"**
Correct, and one user at a time worldwide. Name it as the baseline `SKIP LOCKED`
improves on — it helps them see locking as a spectrum, not a binary.

**"Does the hint ever get out of sync?"**
Constantly, by design. It is a hint; the database decides under lock. Plant the
general principle: **be sloppy where it is cheap, strict where it counts.**

**"Why is `Confirm` a separate call? Why not sell immediately?"**
Because payment happens between them, and it can fail or be abandoned. The
two-phase shape is the only honest way to hold a ticket during checkout without
losing inventory to closed laptops.

**"What happens if the server crashes mid-claim?"**
The transaction rolls back; nothing was reserved. If it crashes after commit,
the reservation exists with its lease and expires normally. **Nothing is lost
because nothing lives only in memory.**

**"Could we just use Redis?"**
Excellent for leases, wrong for durable ownership of financial inventory. Also
worth noting `SKIP LOCKED` has no clean Redis equivalent.

---

# Part 6 — Diagnosing stuck students

| What you see | Likely cause | Intervention |
|---|---|---|
| Copying code without reading it | lost two modules ago | back to the last checkpoint they genuinely passed |
| Cannot build `offsets` | never did the paper exercise | the card activity, Step 42 |
| Bitmap formulas swapped | never did graph paper | Step 47, and ask why 5/13/21 all hit box 5 |
| Tests pass, cannot explain why | wrote code first, test after | make them break it deliberately |
| Concurrency test flaky | not running `-race` | Step 65 demo again |
| Lost in SQL | skipped Steps 87–93 | back to `psql`, hands on keys |
| Silent in discussions | often the ones who understand least | ask them a direct checkpoint question privately |

**The general rule:** when a student is stuck, go back to twelve tickets and
three digits. Every idea in this course works at that scale.

---

# Part 7 — Assessment

| Level | Evidence |
|---|---|
| **Pass** | all modules done, `go test -race ./...` green, service runs |
| **Good** | can explain *why* each decision was made, not just what it does |
| **Strong** | completed an exercise — Strategy B, idempotency keys, or a third backend |

## The final task

One page explaining the system to a colleague who has never seen it: why we do
not scan ten million tickets, how wildcards become numbers, how two users are
prevented from getting the same ticket, why reservations expire, and what they
would measure first if it got slow.

**Mark it on the fourth point above all.** Anyone can recite the mailbox idea.
Understanding that a reservation is not a sale, and why that distinction matters
to the hint, is the mark of someone who followed the reasoning rather than the
steps.

## Questions for a viva, if you run one

1. Why is the claim index partial on `status <> 2` rather than `status = 0`?
2. Reserving does not change the unsold count, but confirming does. Why?
3. Why is over-reporting availability safe and under-reporting dangerous?
4. Batching broke distinct numbers. Why did no test catch it?
5. You add a second API replica and throughput drops. What did you forget?
6. Why do read replicas not help this system?

**All six have appeared as deliberate bugs or demonstrations in the course.** A
student who watched those moments can answer all of them; one who copied code
can answer none.
