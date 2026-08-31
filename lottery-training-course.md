# Lottery Search System — Complete Training Course

A step-by-step course for junior software engineers, from an empty laptop to a scaled production service.

**Every step is numbered and continuous.** Nothing is assumed beyond basic programming (you have written a loop and a function in some language). Where a step needs knowledge the course has not yet given you, that knowledge is a step of its own.

> **Instructor: run everything once before teaching.** The Go code in this course was written without a compiler available. Work through Modules 0–9 yourself first and fix anything that breaks. Notes marked **[Instructor]** are for you, not the students.

---

## How this course is organised

| Module | Steps | Time | You end up with |
|---|---|---|---|
| 0 | 1–9 | 2h | A working environment |
| 1 | 10–21 | 1 day | The Go you need for this project |
| 2 | 22–30 | 1 day | Understanding the problem, no code |
| 3 | 31–39 | 1 day | Pattern matching, tested |
| 4 | 40–51 | 1 day | The index and bitmap |
| 5 | 52–61 | 1 day | A working in-memory store |
| 6 | 62–71 | 1 day | Concurrency and leases |
| 7 | 72–80 | 1 day | An HTTP API |
| 8 | 81–103 | 2 days | SQL, and the Postgres backend |
| 9 | 104–123 | 1 day | The in-memory hint, and keeping it honest |
| 10 | 124–131 | 1 day | A deployable service |
| 11 | 132–142 | 1 day | Load testing and scaling |

**Reference documents** used at specific steps:

- `lottery-search-teaching-guide.md` — the concept lesson (Module 2)
- `lottery-go-tutorial.md` — the TDD walkthrough (Modules 3–10)
- `lottery-scaling-guide.md` — measurement and scaling (Module 11)
- `lottery-go/STATE-SYNC.md` — how memory and database stay in sync (Module 9)
- `lottery-go/` — the finished code, for checking your work

**Rule for students:** do not open `lottery-go/` until a step tells you to. Reading the answer before attempting it feels like learning and is not.

---

# Module 0 — Environment

Two hours. Everyone must finish this before Module 1 or they will be debugging installs while the class moves on.

### Step 1 — Install Go

Download from `go.dev/dl` and install. You need Go 1.22 or later (the course uses routing patterns added in 1.22).

Verify:

```
go version
```

Expected: `go version go1.22.0 <your-platform>` or higher. If the command is not found, Go is installed but not on your `PATH` — that is the most common problem and the fix depends on your OS.

### Step 2 — Understand where Go puts things

```
go env GOPATH GOMODCACHE
```

You do not need to change these. You need to know they exist, because when a download fails, this is where it failed.

### Step 3 — Install Docker

Docker Desktop (Mac/Windows) or Docker Engine (Linux). Verify:

```
docker --version
docker compose version
```

Note it is `docker compose` (two words), not `docker-compose`. The hyphenated version is the old one.

### Step 4 — Prove Docker actually runs containers

```
docker run --rm hello-world
```

Installing Docker and *running* Docker are different achievements. On Linux you may need to add yourself to the `docker` group and log out and back in.

### Step 5 — Install an editor with Go support

VS Code with the official Go extension is the path of least resistance. When it offers to install `gopls`, say yes — that is the language server and it provides the errors-as-you-type that make this course much faster.

### Step 6 — Install git and configure it

```
git --version
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

### Step 7 — Create the project

```
mkdir lottery && cd lottery
go mod init lottery
git init
```

`go mod init lottery` creates `go.mod`, which declares the module name. Every import in this project starts with `lottery` because of this line.

### Step 8 — Prove the toolchain works end to end

Create `hello_test.go`:

```go
package lottery

import "testing"

func TestToolchain(t *testing.T) {
	if 2+2 != 4 {
		t.Error("arithmetic is broken, which is a bigger problem than this course")
	}
}
```

```
go test ./...
```

Expected: `ok  lottery`. Delete the file afterwards.

### Step 9 — Checkpoint

You can proceed when all four commands succeed:

```
go version            # 1.22+
docker run --rm hello-world
go test ./...         # ok
git status            # a repo, not "not a git repository"
```

**[Instructor]** Do not start Module 1 until every student passes Step 9. Pair anyone still stuck with someone who finished. An environment problem discovered on day three costs far more than an hour spent here.

---

# Module 1 — The Go you actually need

One day. This is not a Go course. It teaches only the Go this project uses, and every exercise is a piece you will reuse later.

### Step 10 — Packages and files

Every `.go` file starts with `package <name>`. Files in the same directory share a package and can see each other's code without imports.

Create `notes.go`:

```go
package lottery

// Digits is how many digits a ticket number has.
const Digits = 6
```

A name starting with a capital letter is **exported** — visible outside the package. Lowercase is package-private. This is Go's entire access control system; there is no `public` or `private` keyword.

### Step 11 — Functions and multiple returns

```go
func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}
```

Go functions return multiple values, and returning `(result, error)` is the standard shape. There are no exceptions in Go. **Every error is a value you must handle or explicitly ignore.**

### Step 12 — Handling errors

```go
result, err := divide(10, 2)
if err != nil {
	return err        // or log it, or handle it
}
// use result
```

You will write `if err != nil` hundreds of times. It is verbose and it is deliberate: the error path is as visible as the success path.

**Exercise:** write `divide`, then a test that checks both the success case and the division-by-zero case.

### Step 13 — Structs

```go
type Ticket struct {
	ID     int32
	Number int32
}

t := Ticket{ID: 1, Number: 123456}
fmt.Println(t.Number)
```

Always use field names (`Ticket{ID: 1, Number: 123456}`), never positional (`Ticket{1, 123456}`). Positional breaks silently when someone adds a field.

### Step 14 — Methods

```go
func (t Ticket) IsLucky() bool {
	return t.Number%7 == 0
}
```

`(t Ticket)` is the receiver. A **value receiver** gets a copy; a **pointer receiver** `(t *Ticket)` can modify the original. Use a pointer receiver when the method changes something or when the struct is large.

**Exercise:** add a method to `Ticket` that returns the number as a zero-padded six-character string. (Hint: `fmt.Sprintf("%06d", n)`.)

### Step 15 — Slices

```go
xs := []int{1, 2, 3}          // a slice literal
ys := make([]int, 0, 10)      // length 0, capacity 10
ys = append(ys, 4)            // append RETURNS the new slice
sub := xs[1:3]                // elements 1 and 2 -- a VIEW, not a copy
```

Three things that catch people:

- `append` returns a new slice. `append(ys, 4)` without assigning does nothing useful.
- A slice expression `xs[1:3]` shares memory with `xs`. Modifying one modifies the other.
- `make([]int, 0, 10)` has length 0 but room for 10 — appending will not reallocate until you exceed 10.

That last point matters in this project: we write `make([]Reservation, 0, k)` because we know we will append at most `k` items.

### Step 16 — nil versus empty

```go
var a []int          // nil slice
b := []int{}         // empty slice
len(a) == len(b)     // both 0
a == nil             // true
b == nil             // false
```

Inside Go these behave almost identically. **At a JSON boundary they do not**: a nil slice becomes `null`, an empty slice becomes `[]`. You will meet this in Module 7 and it will break a test.

### Step 17 — Maps

```go
counts := map[string]int{}
counts["a"]++                    // zero value is 0, so this works
v, ok := counts["b"]             // ok is false if the key is absent
```

The two-value lookup is how you distinguish "absent" from "present but zero".

### Step 18 — Writing a test

Create `math_test.go`:

```go
package lottery

import "testing"

func TestDivide(t *testing.T) {
	got, err := divide(10, 2)
	if err != nil {
		t.Fatalf("divide(10, 2) unexpected error: %v", err)
	}
	if got != 5 {
		t.Errorf("divide(10, 2) = %d, want 5", got)
	}
}
```

Rules:

- File must end in `_test.go`
- Function must start with `Test` and take `*testing.T`
- `t.Errorf` records a failure and continues; `t.Fatalf` stops this test immediately

**Use `Fatalf` when continuing would be meaningless** (you got an error instead of a value, so checking that value is pointless). Use `Errorf` otherwise, because one run reporting three failures beats three runs reporting one each.

### Step 19 — Failure messages that help

This is the habit that separates a useful test suite from a frustrating one.

```go
// Bad: tells you nothing
if got != want {
	t.Error("wrong result")
}

// Good: tells you the input, the result, and the expectation
if got != want {
	t.Errorf("divide(%d, %d) = %d, want %d", a, b, got, want)
}
```

**The standard Go phrasing is `funcName(input) = got, want X`.** Follow it and every failure reads the same way.

### Step 20 — Table tests and subtests

```go
func TestDivide(t *testing.T) {
	cases := []struct {
		name    string
		a, b    int
		want    int
		wantErr bool
	}{
		{"simple", 10, 2, 5, false},
		{"rounds down", 7, 2, 3, false},
		{"by zero", 1, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := divide(c.a, c.b)
			if (err != nil) != c.wantErr {
				t.Fatalf("divide(%d, %d) error = %v, wantErr = %v", c.a, c.b, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("divide(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}
```

The `name` field exists so `go test -v` prints `by_zero` rather than `#3`. With twenty cases this is the difference between a two-second fix and a five-minute hunt.

**This is the dominant test shape in this project.** Get comfortable with it now.

### Step 21 — Checkpoint

Write, from scratch, without looking back:

1. A struct `Student` with `Name string` and `Score int`
2. A method `Passed() bool` returning whether the score is 50 or more
3. A table test with at least four cases, including boundary values 49, 50, 51
4. `go test -v ./...` showing four named subtests

**You are ready for Module 2 when you can do this without help.** If you cannot, redo Steps 13–20 — everything afterwards is built on them.

**[Instructor]** Interfaces, goroutines and channels are deliberately *not* here. They arrive in Modules 6 and 8, at the point where the project needs them, so students meet each one solving a problem rather than in the abstract.

---

# Module 2 — Understanding the problem

One day, no code. **Laptops closed.**

**[Instructor]** Teach this from `lottery-search-teaching-guide.md`. The steps below map onto it directly. Resist every request to "just start coding" — students who skip this write code they do not understand and cannot debug.

### Step 22 — The problem statement

> 10,000,000 lottery tickets, each a 6-digit number. Users search with a pattern like `1**4*7`, where `*` matches any digit. Two users must never receive the same ticket at the same time.

### Step 23 — Shrink it to twelve tickets

Teaching guide, Step 0. Twelve tickets, three digits, on the whiteboard. Find everything matching `1*3` by hand.

**This table stays on the board for the rest of the day.** Every idea gets demonstrated on it before scaling up.

### Step 24 — Price the obvious solution

Teaching guide, Step 1. Everyone proposes "loop through all ten million." Do not correct it — cost it. 10,000,000 checks for ~100 useful results is 99.999% waste.

### Step 25 — The observation that changes everything

Teaching guide, Step 2. Six digits means exactly 1,000,000 possible numbers, and that ceiling is fixed by the *format*, not the data. Ten million tickets across a million numbers means about ten tickets per number.

**Checkpoint question:** *"If we sold 100 million tickets tomorrow, how many possible numbers?"* Still 1,000,000. If a student cannot answer this, stop and go back — nothing downstream will land.

### Step 26 — Mailboxes

Teaching guide, Step 3. One box per possible number. Finding tickets for number 142 is not a search; it is walking to box 142.

### Step 27 — Wildcards are an odometer

Teaching guide, Step 4. `1*3` matches ten numbers, and you can list them *without looking at any data*. Fixed digits are locked dials; wildcards spin.

Search becomes: spin the dials → get numbers → open those boxes.

### Step 28 — You never wanted all of them

Teaching guide, Step 5. Nobody wants 100,000 tickets; they want five. So you stop early, which makes broad patterns *cheaper* than narrow ones.

### Step 29 — The real problem: two users at once

Teaching guide, Step 6. Draw the interleaving:

```
        User A                          User B
  t1    is #4829112 free?  → yes
  t2                                    is #4829112 free? → yes
  t3    mark it as mine
  t4                                    mark it as mine
```

Both checks passed because both ran before either write. **This is a money bug, not a performance bug.** Name it: race condition, or check-then-act.

### Step 30 — Leases, contention, and checkpoint

Teaching guide, Steps 7–8. Reservations expire so abandoned sessions do not strand inventory. Everyone starting their search at the same place causes needless collisions, so each user starts somewhere random.

**Checkpoint — each student explains, out loud, without notes:**

1. Why we do not scan ten million tickets
2. Why `1*3` matches ten numbers regardless of the data
3. Why "check if free, then take it" is broken
4. Why a reservation must expire

**Do not proceed until every student can do this.** Module 3 starts writing code, and code written without this understanding is code that cannot be debugged.

---

# Module 3 — Pattern matching in code

One day. From here on you follow `lottery-go-tutorial.md`, but this course tells you exactly where to start, stop, and check.

### Step 31 — The bridge from whiteboard to keyboard

Yesterday you decided the pattern generates numbers. Today you build exactly that, and nothing else.

Two functions:

- `ParsePattern("1**4*7")` — validate and prepare
- `NumberAt(j)` — give me the j-th matching number

Notice what is *not* here: no tickets, no database, no HTTP. **Build the smallest piece that is completely understood, then the next.**

### Step 32 — Write the test before the code

Tutorial Chapter 1. Create `pattern_test.go` with `TestParsePatternRejectsBadInput` and run it.

It will not compile:

```
./pattern_test.go:18:13: undefined: ParsePattern
```

**This is success.** The compiler just wrote your to-do list. Read the error; it names exactly what to declare next.

### Step 33 — Make it compile, then fail properly

Write the minimum: a `Pattern` struct, `ErrBadPattern`, and a `ParsePattern` that returns `(Pattern{}, nil)`.

Run again. Now it compiles and *fails* — `error = <nil>, want ErrBadPattern`.

**The three states matter:** does not compile → compiles but fails → passes. Skipping the middle state means you never proved the test can detect the bug.

### Step 34 — Make it pass

Tutorial Chapter 1. The parse loop with the `switch`.

New Go concept you will meet: `%w` in `fmt.Errorf` wraps an error so `errors.Is` can see it. `%v` would flatten it to text and the test would fail. Try changing one to the other and watch what happens — five seconds, and you will remember it.

### Step 35 — Add MatchCount

Tutorial Chapter 1. Predict each expected value before running:

| Pattern | Matches |
|---|---|
| `123456` | 1 |
| `12345*` | 10 |
| `1234**` | 100 |
| `******` | 1,000,000 |

If you can predict these, Step 27 landed.

### Step 36 — The odometer

Tutorial Chapter 2. `NumberAt(j)`.

Walk one case on paper before coding: pattern `1*3456`, `j = 7`. Base is `103456`, one wildcard at position 1, place value 10,000, so `103456 + 7×10000 = 173456`.

### Step 37 — Format numbers so failures are readable

Use `%06d` in test messages. `103456` and `3456` look confusingly similar; `103456` and `003456` do not. Small habit, large payoff.

### Step 38 — Test the property, not just examples

Tutorial Chapter 2, `TestNumberAtCoversEveryMatchExactlyOnce`.

Seven spot checks prove seven things. This test proves three properties for *all* inputs: nothing repeats, nothing wrong appears, nothing is missing.

**This is a real step up in testing skill.** Spot checks catch typos; property tests catch design errors.

### Step 39 — Checkpoint

```
go test -v ./...
```

All pattern tests pass. You can explain:

- Why `pow10` has seven entries for six digits
- Why we parse into numbers instead of keeping the string
- What `%w` does that `%v` does not

Commit: `git add -A && git commit -m "pattern matching"`

---

# Module 4 — Data structures

One day. The hardest module conceptually, and the one where the mailbox metaphor becomes real memory.

### Step 40 — Understand why the obvious version fails

**Before writing code**, read Appendix A.1–A.3 of the teaching guide.

Answer for yourself: how much memory does an *empty* Go slice-of-slices use for a million buckets? The answer is not zero, and that is the point.

### Step 41 — Build the index by hand on paper

Teaching guide A.4.1. Three passes on the twelve-ticket table:

1. **Count** how many tickets per number
2. **Running total** to get each number's start position
3. **Place** each ticket, advancing a cursor

Fill in all three tables with a pencil. **Do not skip this.** Students who code before doing it by hand write the cursor bug and cannot see why.

### Step 42 — The classroom activity if you are stuck

Teaching guide A.4.1, the index-card exercise. Twelve cards, twelve chalk squares, move the cursor with your finger. If the paper exercise did not click, this will.

### Step 43 — Write the index test first

Tutorial Chapter 3. Use the same twelve tickets, so your test checks the code against the table you filled in by hand.

Include `NumberSpace - 1` in the empty-number cases. That is the one that panics if you forget the sentinel slot.

### Step 44 — Implement BuildIndex

Tutorial Chapter 3, the three passes.

### Step 45 — Break it deliberately

Delete `copy(cursor, offsets)` and use `offsets` directly. Run the tests.

The first lookup passes; the rest fail. **You just reproduced the most common bug in this topic and watched your test catch it.** Put the copy back.

### Step 46 — Benchmark it

Tutorial Chapter 3.

```
go test -bench=BuildIndex -benchmem ./...
```

"Boot takes a few seconds" is now a number you measured rather than a claim you were given. Note `b.ResetTimer()` — without it you benchmark the setup.

### Step 47 — Understand bitmaps before coding them

Teaching guide A.6.1. The attendance-sheet analogy, the two formulas, the graph-paper exercise:

- Set number 5 → row 0, box 5
- Set number 13 → row 1, box 5
- Set number 21 → row 2, box 5
- **Why do all three land in box 5 without colliding?**

Answer that and you understand why there are two formulas rather than one.

### Step 48 — Write the bitmap tests

Tutorial Chapter 4, including `TestBitmapSetLeavesNeighboursAlone`.

### Step 49 — Break it deliberately, again

Write `Set` using `=` instead of `|=`. Run the tests.

Bits 0, 1, 5 and 63 all vanish when you set 7, because one 64-bit word holds sixty-four members. **Overwriting the word wipes 63 innocent neighbours.** Fix it.

### Step 50 — NextSet

Tutorial Chapter 4. `math/bits.TrailingZeros64` compiles to one CPU instruction; the standard library gives you hardware intrinsics for free.

### Step 51 — Checkpoint

All tests pass. You can explain, on a whiteboard:

- Why `offsets` has one more entry than there are numbers
- Why pass 3 needs a copy of `offsets`
- Why `NewBitmap(65)` allocates two words
- How `NextSet` skips 64 numbers in one comparison

Commit: `git commit -am "index and bitmap"`

---

# Module 5 — The in-memory store

One day. Assembling the pieces into something that works.

### Step 52 — Name the three outcomes

Before coding, answer: a claim can end three ways. What are they?

1. Got everything asked for
2. Got some
3. Got none

Each needs a test, and the caller must be able to tell them apart. Juniors write the first and stop.

### Step 53 — Write the four claim tests

Tutorial Chapter 5: matching only, stops at k, runs out, no matches.

### Step 54 — The design decision worth arguing about

Read the tutorial's "design decision worth arguing about" carefully, then answer before reading on:

> The store tracks which numbers still have tickets. Should the counter track *free* tickets or *unsold* tickets?

**Free is wrong.** An expired reservation is claimable again. If the counter tracks free tickets, an expired hold reads as zero, the number gets marked sold out, and those tickets are unreachable until a cleanup job runs.

**The wrong version passes every test you have written so far.** Only the lease test in Module 6 catches it — which is a strong argument for writing that test earlier than feels necessary.

### Step 55 — Implement Claim

Tutorial Chapter 5.

`for len(out) < k` is Step 28 in one line: stop as soon as you have enough.

### Step 56 — Ask what the user actually wants

Your `Claim` returns `k` tickets. Before moving on, answer this:

> A user searches `****23` and asks for 5 tickets. Should they receive five
> **different** numbers, or is five copies of one number acceptable?

Think about it as a lottery customer, not as a programmer. If all five tickets
carry `002323` and that number does not win, you lose everything. Five
different numbers spread the risk. **A customer asking for five tickets from a
wildcard search expects variety** — that is the whole reason they searched a
pattern instead of typing one number.

Now look at your loop from the previous step:

```go
for _, id := range s.index.TicketsFor(int(n)) {
    // takes EVERY claimable ticket of this number before moving on
}
```

With about ten tickets per number, asking for five gets you five copies of the
first number you touch. **Your code has this bug right now.**

### Step 57 — Write the test, then fix it with two passes

```go
func TestClaimPrefersDistinctNumbers(t *testing.T) {
	// ten numbers ending in 23, ten tickets each
	got := claim(t, h, "****23", 5, "alice")

	seen := map[int32]int{}
	for _, r := range got {
		seen[r.Number]++
	}
	if len(seen) != 5 {
		t.Errorf("got %d distinct numbers across 5 tickets, want 5; counts = %v",
			len(seen), seen)
	}
}
```

Watch it fail, then think before coding the fix — because the obvious fix is
wrong.

**Ask yourself:** if you take at most one ticket per number, what happens when
someone searches `123456` and asks for five?

That pattern matches exactly **one** number. A strict one-per-number rule would
hand back a single ticket and call it a day, which is clearly not what someone
buying five copies of their lucky number wants.

So the rule is *prefer* distinct, not *require* it — two passes:

```go
// PASS 1 -- at most one ticket per number, so the caller sees a spread.
s.claimPass(p, k, 1, holder, now, until, &out, ctx)

// PASS 2 -- top up with duplicates only if pass 1 could not fill the request.
if len(out) < k {
    s.claimPass(p, k, 0, holder, now, until, &out, ctx)
}
```

`claimPass` takes a `perNumber` limit, where `0` means unlimited. Write the
second test too:

```go
func TestClaimAllowsDuplicatesWhenItMust(t *testing.T) {
	// five tickets, all numbered 123456
	got := claim(t, h, "123456", 4, "alice")
	if len(got) != 4 {
		t.Fatalf("claimed %d tickets of the only matching number, want 4", len(got))
	}
}
```

**Both tests belong in the conformance suite**, not in a memory-store-specific
file. This is a behaviour every backend must share, and you will find out in
Module 8 why that matters.
### Step 58 — Measure the counter-intuitive result

Add a temporary counter for how many numbers `Claim` visits. Compare `123456` against `1*****`.

The broad pattern visits fewer. **You were told this in Module 2; now you have measured it.** Remove the counter afterwards.

### Step 59 — The walker

Tutorial Chapter 6. Test `n = 1` and `n = 2` — permutation code breaks at the edges, and `n = 1` is a real input (any pattern with no wildcards).

### Step 60 — Watch a bad multiplier strand inventory

Tutorial Chapter 6, `TestNonCoprimeMultiplierStrandsMostOfTheRange`.

With `a = 5, n = 10`, the walker reaches two values out of ten. Eight boxes are permanently unreachable and their tickets can never be sold.

**No crash. No error. No log line.** The system looks healthy while inventory is silently unsellable. This is the failure mode juniors most need to see: **the quiet bugs are the expensive ones.**

### Step 61 — Checkpoint

All store tests pass. You can explain:

- Why the store counts unsold rather than free tickets
- Why a claim prefers distinct numbers but must allow duplicates
- What happens if `a` shares a factor with `n`

Commit: `git commit -am "claiming and traversal"`

---

# Module 6 — Concurrency

One day. New Go concepts, introduced where the project needs them.

### Step 62 — Goroutines

```go
go doSomething()      // runs concurrently; the caller does not wait
```

That is the entire syntax. The difficulty is never starting them — it is knowing when they finished and coordinating shared data.

### Step 63 — WaitGroup

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
	wg.Add(1)              // BEFORE starting the goroutine
	go func(i int) {
		defer wg.Done()    // always deferred, so it runs even on panic
		work(i)
	}(i)
}
wg.Wait()                  // blocks until all Done
```

`wg.Add(1)` goes outside the goroutine. Inside, you race between starting and waiting.

### Step 64 — Mutex

```go
var mu sync.Mutex
mu.Lock()
shared++          // only one goroutine at a time reaches here
mu.Unlock()
```

Prefer `defer mu.Unlock()` immediately after `Lock()` so an early return cannot leave it locked.

### Step 65 — See a race with your own eyes

Write this, run it, then run it with `-race`:

```go
func TestRaceDemo(t *testing.T) {
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); counter++ }()
	}
	wg.Wait()
	t.Logf("counter = %d (expected 100)", counter)
}
```

```
go test -run RaceDemo -v ./...
go test -race -run RaceDemo -v ./...
```

Without `-race` the counter is sometimes 100 and sometimes not. With `-race`, Go names the exact lines in conflict.

**Take the lesson seriously: a passing concurrent test proves very little on its own.** Delete this file afterwards.

### Step 66 — Write the concurrency test

Tutorial Chapter 7.

Two traps to internalise:

- **`t.Fatalf` inside a goroutine hangs the test.** It calls `runtime.Goexit`, so `wg.Wait()` never returns. Use `t.Errorf`.
- **The test needs its own mutex** for the shared `seen` map. The test for a concurrency bug can itself have a concurrency bug.

### Step 67 — Make it pass with one mutex

Be honest about what this is: **correct but serialised.** Every claim waits for every other claim. In memory that is microseconds, so it holds up fine. In Module 8 you will see what a database does instead.

### Step 68 — Run the whole suite with -race

```
go test -race ./...
```

From now on, `-race` is part of "the tests pass".

### Step 69 — Controlling time

Tutorial Chapter 8, the fake clock.

**Ask yourself first:** how would you test a two-minute lease without one? The answer — `time.Sleep(3 * time.Minute)` — is its own argument. A suite that takes three minutes is a suite nobody runs.

### Step 70 — Leases, confirm, release

Tutorial Chapter 8. Include the test that confirms a *sold* ticket stays sold after the clock advances an hour. Expiry applies to reservations, never to sales.

This is also the test that catches the Step 54 mistake. If you chose "free tickets" back then, it fails now.

### Step 71 — Checkpoint

```
go test -race ./...
```

Everything green. You can explain what `-race` does, why `t.Fatalf` in a goroutine is wrong, and why the clock is injected rather than called directly.

Commit: `git commit -am "concurrency and leases"`

---

# Module 7 — The HTTP API

One day.

### Step 72 — Handlers

```go
func handle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello"))
}
```

`w` is where you write the response; `r` is the request. That is the whole interface.

### Step 73 — Routing

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /search", handleSearch)
mux.HandleFunc("POST /reservations/{id}/confirm", handleConfirm)
```

The method prefix and `{id}` placeholders need Go 1.22. Read the placeholder with `r.PathValue("id")`.

### Step 74 — JSON in and out

```go
type searchRequest struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

var req searchRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	// 400
}
json.NewEncoder(w).Encode(response)
```

The backtick strings are **struct tags**. Without them, Go would expect `Pattern` with a capital P in the JSON.

### Step 75 — Test handlers without starting a server

```go
req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
rec := httptest.NewRecorder()
srv.ServeHTTP(rec, req)
// rec.Code, rec.Body
```

No port opened, no server started. Tests run in microseconds and cannot collide in CI.

### Step 76 — Write the handler tests

Tutorial Chapter 9. Include `body = %s` in failure messages — when a handler test fails, the body usually contains the actual reason.

### Step 77 — Validate everything

Tutorial Chapter 9, the validation table. The `count` cap is a real requirement, not paperwork: without it, one request can drain inventory.

### Step 78 — Meet the nil-slice trap

Tutorial Chapter 9, `TestSearchReturnsEmptyArrayNotNull`.

A nil slice marshals to `null`; an empty slice to `[]`. Clients doing `for (const t of data.tickets)` break on `null`. Fix with `make([]T, 0, n)`.

**This is Step 16 arriving as a real bug.** You will hit it in every Go service you write.

### Step 79 — The number is not an integer

One more wire-format trap, and it is the same lesson as the last one from a
different angle.

Your ticket numbers are stored as `int32`. Serialise one directly and a client
receives:

```json
{"id": 3576966, "number": 2323}
```

**`2323` is not a lottery number.** The number is `002323` — six characters.
The integer is an implementation detail chosen because arithmetic on integers
is fast, and it has just leaked to every client you have.

**Ask yourself:** should you fix this by changing the field type on
`Reservation`?

No. Internally, `int32` is right: the hint indexes by it, the pattern generates
it, comparisons are cheap. What is wrong is using the *domain* type as the
*wire* type. Give the HTTP layer its own view:

```go
// ticketView is the WIRE representation, deliberately separate from the
// domain type.
type ticketView struct {
	ID         int64     `json:"id"`
	Number     string    `json:"number"`
	LeaseUntil time.Time `json:"lease_until"`
}

func viewOf(r Reservation) ticketView {
	return ticketView{
		ID:         r.TicketID,
		Number:     fmt.Sprintf("%0*d", Digits, r.Number),
		LeaseUntil: r.LeaseUntil,
	}
}
```

```go
func TestSearchPadsNumbersToSixDigits(t *testing.T) {
	// ...
	for _, tk := range resp.Tickets {
		if len(tk.Number) != 6 {
			t.Errorf("number = %q, want six characters", tk.Number)
		}
	}
}
```

**The principle worth keeping:** the shape of your data in memory and the shape
you promise to clients are two different contracts. Coupling them means you
cannot change one without breaking the other.

While you are here, add `distinct_numbers` to the search response. It costs
three lines and makes the Step-B behaviour visible in the payload instead of
requiring someone to eyeball ten rows.
### Step 80 — Checkpoint

All handler tests pass. You can explain what struct tags do, why `httptest` beats starting a real server, the nil-versus-empty difference, and why the wire type is separate from the domain type.

Commit: `git commit -am "http api"`

---

# Module 8 — Database

Two days. The biggest module, and where most gaps used to be. Day one is SQL and interfaces; day two is the Postgres implementation.

## Day one — interfaces and SQL

### Step 81 — Interfaces in Go

```go
type Shape interface {
	Area() float64
}
```

Any type with an `Area() float64` method satisfies `Shape` — **no `implements` keyword, no declaration**. If the methods match, it fits.

```go
var _ Shape = (*Circle)(nil)
```

That line does nothing at runtime. It is a compile-time assertion: if `*Circle` stops satisfying `Shape`, the build breaks *here*, with a clear message, instead of somewhere confusing later.

### Step 82 — Extract TicketStore

Tutorial Chapter 10. Rename `Store` to `MemoryStore`, define the interface, add `context.Context`, widen IDs to `int64`.

**Nothing behavioural changes.** Your tests prove the refactor was safe. That is the payoff for having written them.

### Step 83 — context.Context

```go
func Claim(ctx context.Context, ...) error {
	if err := ctx.Err(); err != nil {
		return err          // caller gave up; stop working
	}
}
```

A context carries cancellation. When an HTTP client disconnects, `r.Context()` is cancelled, and work stops instead of burning a database connection for nobody.

Convention: always the first parameter, always named `ctx`.

### Step 84 — One suite, many implementations

Tutorial Chapter 10, the `storetest` package.

This is the module's central idea: **behavioural tests that mention no backend at all.** Any implementation passing them is interchangeable with the others.

### Step 85 — Why the harness is shaped oddly

The suite cannot hand Postgres a fake clock, because `now()` lives inside SQL. So it asks for a lease length and a way to move past it — memory fast-forwards a fake clock; Postgres sleeps for real.

**The awkwardness is telling you something true:** application clocks drift between servers, so the database must own time. Test friction usually points at a real constraint.

### Step 86 — Start Postgres

```
docker compose up -d db
docker compose exec db psql -U lottery -d lottery
```

You are now at a `lottery=#` prompt. `\q` exits, `\dt` lists tables, `\d tickets` describes one.

### Step 87 — SQL you need, part 1: creating tables

```sql
CREATE TABLE tickets (
    id          BIGSERIAL PRIMARY KEY,
    num         INTEGER     NOT NULL,
    status      SMALLINT    NOT NULL DEFAULT 0,
    holder      TEXT,
    lease_until TIMESTAMPTZ
);
```

- `BIGSERIAL` — auto-incrementing 64-bit integer
- `PRIMARY KEY` — unique, indexed, not null
- `NOT NULL` — the database rejects missing values
- `TIMESTAMPTZ` — timestamp *with time zone*. Always use this, never `TIMESTAMP`.

Type it in `psql` and run `\d tickets`.

### Step 88 — SQL you need, part 2: reading and writing

```sql
INSERT INTO tickets (num) VALUES (123456);
SELECT * FROM tickets WHERE num = 123456;
UPDATE tickets SET status = 1 WHERE id = 1;
DELETE FROM tickets WHERE id = 1;
```

Practise each. Then:

```sql
SELECT status, count(*) FROM tickets GROUP BY status;
```

### Step 89 — SQL you need, part 3: RETURNING

```sql
UPDATE tickets SET status = 1 WHERE id = 1 RETURNING id, num;
```

`RETURNING` is a Postgres feature that gives back the rows you changed. **This eliminates the read-then-write gap** — you learn what you got in the same statement that got it. It is central to the claim query.

### Step 90 — SQL you need, part 4: indexes and EXPLAIN

```sql
CREATE INDEX tickets_num ON tickets (num);
EXPLAIN ANALYZE SELECT * FROM tickets WHERE num = 123456;
```

Read the output for `Seq Scan` (reads everything) versus `Index Scan` (jumps straight there). Drop the index, run it again, compare. **Watch the plan change with your own eyes** — it is far more convincing than being told.

### Step 91 — Partial indexes

```sql
CREATE INDEX tickets_claimable ON tickets (num) WHERE status <> 2;
```

Indexes only the rows matching the condition. Smaller index, faster lookups, and it shrinks permanently as tickets sell.

### Step 92 — See the race in SQL

Open **two** `psql` windows side by side.

Window 1:

```sql
BEGIN;
SELECT * FROM tickets WHERE id = 1 FOR UPDATE;
-- do not commit yet
```

Window 2:

```sql
SELECT * FROM tickets WHERE id = 1 FOR UPDATE;
-- hangs, waiting for window 1
```

Window 2 is blocked. Now `COMMIT` in window 1 and watch window 2 unblock.

### Step 93 — See SKIP LOCKED fix it

Window 1: hold a lock as before. Window 2:

```sql
SELECT * FROM tickets WHERE status = 0 FOR UPDATE SKIP LOCKED LIMIT 1;
```

**Returns instantly with a different row.** No waiting, no error, no duplicate.

**This is the single most important demonstration in the course.** Module 2 said "checking and taking must be one indivisible step." You have now seen the mechanism that makes it so.

### Step 94 — Checkpoint (day one)

You can, in `psql`, create a table, insert and query rows, read an `EXPLAIN` plan, and demonstrate `SKIP LOCKED` in two windows.

## Day two — the Postgres store

### Step 95 — The schema

Tutorial Chapter 11.

**Answer before reading the explanation:** why is the claim index partial on `status <> 2` rather than `status = 0`?

Because an expired reservation has `status = 1` and is claimable again. Indexing only `status = 0` would hide it, and correctness would silently depend on the cleanup job.

### Step 96 — Add pgx

```
go get github.com/jackc/pgx/v5
go mod tidy
```

pgx is the Postgres driver. `go mod tidy` records it in `go.mod` and downloads it.

### Step 97 — The claim statement

Tutorial Chapter 12. Read it line by line and be able to explain each:

```sql
UPDATE tickets t SET status = 1, holder = $2, lease_until = now() + ...
 WHERE t.id IN (
       SELECT id FROM tickets
        WHERE num = ANY($1::int[])
          AND (status = 0 OR (status = 1 AND lease_until < now()))
        LIMIT $4 FOR UPDATE SKIP LOCKED)
RETURNING t.id, t.num, t.lease_until
```

**Why the subquery?** `UPDATE` has no `SKIP LOCKED` clause. The inner `SELECT` is where locking is expressible. It locks the rows; the outer `UPDATE` then writes rows it already holds and can never block.

### Step 98 — The optimisation that broke a guarantee

You fixed distinct numbers back in Module 5. Seed a real database and try it
again:

```
make seed-small && make run
curl -s localhost:8080/search -d '{"pattern":"****23","count":10,"holder":"alice"}'
```

```json
{"tickets": [
    {"id": 3576966, "number": "002323"},
    {"id": 535393,  "number": "002323"},
    {"id": 7070752, "number": "002323"},
    ...
```

**The bug is back.** Ten tickets, one number.

**Work out why before reading on.** The walker still scrambles the candidate
order. The two-pass logic is still in `MemoryStore`. The conformance suite still
passes. So what changed?

**Batching.** We generate 256 candidate numbers and send them to Postgres in one
query with `LIMIT 10`. Postgres scans `tickets_claimable_num`, which is ordered
by `num`, finds ten tickets of the first matching number, satisfies the limit,
and stops. It has no reason to spread across numbers, and we never asked it to.

**This is the lesson in the module, and it is worth stating out loud:** an
optimisation at one layer silently undid a guarantee established at another.
Neither change was wrong in isolation. Nothing crashed. Every existing test
passed — because the conformance suite's distinct test was written against
`MemoryStore`, which does not batch.

Real systems break this way far more often than they break loudly.

### Step 99 — Fix it with LATERAL

`LIMIT k` on the whole batch is the problem, so run a separate `LIMIT 1` per
candidate number:

```sql
WITH candidate(num, ord) AS (
    SELECT * FROM unnest($1::int[]) WITH ORDINALITY
),
picked AS (
    SELECT t.id
      FROM candidate c
      CROSS JOIN LATERAL (
           SELECT id
             FROM tickets
            WHERE tickets.num = c.num
              AND (status = 0 OR (status = 1 AND lease_until < now()))
            LIMIT 1
              FOR UPDATE SKIP LOCKED
      ) t
     ORDER BY c.ord
     LIMIT $4
)
UPDATE tickets t
   SET status = 1, holder = $2, lease_until = now() + make_interval(secs => $3)
  FROM picked
 WHERE t.id = picked.id
RETURNING t.id, t.num, t.lease_until
```

New SQL to understand, one piece at a time:

- **`unnest($1::int[])`** turns the candidate array into rows you can join against
- **`WITH ORDINALITY`** adds a position column, preserving the walker's scrambled order — without it, Postgres may reorder and reintroduce the convoy from Step 8
- **`CROSS JOIN LATERAL`** runs the subquery once *per row* of `candidate`, with access to `c.num`. A plain subquery cannot see the outer row; that is exactly what `LATERAL` unlocks
- **`LIMIT 1` inside** is the fix: one ticket per number, not `k` from whichever number comes first

Then add the duplicate-fallback pass, same as Module 5: if pass 1 could not fill
the order, run the original `claimSQL` for the remainder.

Also shrink the batch. You now take at most one ticket per number, so requesting
256 numbers to fill 5 slots locks rows you will not use:

```go
nums := cands.NextBatch(min((k-len(out))*overFetch, s.batchSize))
```

With `overFetch = 4`, asking for 5 tickets fetches 20 candidate numbers — enough
to absorb the sold-out ones, few enough to avoid locking rows you discard.

### Step 100 — Move the test where it belongs

The bug survived because the distinct test only ever ran against `MemoryStore`.
Move it into `storetest/suite.go` so **every** backend must satisfy it:

```
go test ./...                              # MemoryStore
go test -tags=integration ./postgres/...   # Store and HybridStore
```

**This is what the conformance suite is for.** A behaviour every implementation
must share belongs in the shared suite, not in whichever file you happened to be
editing. Verify the plan too:

```sql
EXPLAIN (ANALYZE, BUFFERS)
-- paste the distinct claim query with a small array
```

Confirm it uses `tickets_claimable_num` rather than scanning.
### Step 101 — Implement the Postgres store

Tutorial Chapter 12. Batching, `k - len(out)` in the `LIMIT`, and the `explainFailure` path that only runs when a write affects zero rows.

### Step 102 — The reaper

Tutorial Chapter 13.

**Answer before reading:** expiry is already handled in the claim query, so why have a reaper at all?

For index accuracy and clean admin views — **not for correctness.** Turn it off and the system still never loses a ticket. Design so background jobs are optimisations, never load-bearing.

### Step 103 — Run the same suite against Postgres

Tutorial Chapter 14.

```
make db
go test -tags=integration -race ./postgres/...
```

**This is the moment the course has been building to.** The same eleven tests that check `MemoryStore` now check Postgres. Not similar tests — the same code.

Then: change `SKIP LOCKED` to plain `FOR UPDATE` and rerun `TestConcurrentClaimsDoNotSerialise`. Correctness assertions still pass; elapsed time climbs. **The argument for `SKIP LOCKED`, as a measurement.**

Commit: `git commit -am "postgres backend"`

---

# Module 9 — The availability hint

One day. You have a working Postgres backend. This module puts a small
in-memory layer in front of it — and, more importantly, teaches you how to keep
memory and a database in agreement without lying to your users.

**Reference:** `lottery-go/STATE-SYNC.md` and `lottery-go/postgres/hybrid.go`.

## Part one — see the problem before building the cure

### Step 104 — Sell almost everything

An optimisation you cannot measure is a guess. Create the condition it fixes.

```
make db && make seed-small
docker compose exec db psql -U lottery -d lottery
```

```sql
-- Sell 98% of the inventory, leaving a thin scattering of stock.
UPDATE tickets SET status = 2 WHERE id % 50 <> 0;
SELECT status, count(*) FROM tickets GROUP BY status;
```

### Step 105 — Watch it get slow

```
make run
go run ./cmd/loadgen -c 20 -d 20s -pattern '****23'
```

Note the p99. Then compare against a fresh database (`make seed-small` again,
no sell-off) and run the same test.

**The nearly-empty one is dramatically slower.** Nothing in the code changed.

### Step 106 — Work out why

Ask yourself before reading on: the query is the same, the indexes are the
same, so where did the time go?

Each request generates candidate numbers and ships batches of them to Postgres.
When 98% is sold, almost every candidate is a number with nothing left. The
database dutifully looks each one up, finds nothing, and returns empty. **We
are paying a network round trip and an index lookup to be told "no" over and
over.**

The fix is not a faster database. It is *not asking*.

### Step 107 — The two-layer idea

Whiteboard, no code:

```
   pattern ──► candidate numbers ──► [ HINT ] ──► [ POSTGRES ] ──► tickets
                (arithmetic)          in RAM        on disk
                                      may be        always
                                      wrong         right
```

| | Hint | Postgres |
|---|---|---|
| Answers | "which numbers are worth ASKING about" | "which tickets do you GET" |
| Can be wrong? | **yes, by design** | no |
| Read cost | ~1 ns | ~1 ms |
| Size | 4 MB | ~1 GB |

The hint is an optimisation. Delete it and the system still works.

### Step 108 — The safety rule, and why it is one-directional

**This is the most important idea in the module.** Write it on the board and
leave it there.

```
Over-reporting:   hint says "available", really sold out
                  → one wasted number in a query.  HARMLESS.

Under-reporting:  hint says "sold out", really available
                  → those tickets are invisible to this replica.
                    Inventory stranded.  NEVER ALLOW THIS.
```

**Ask yourself:** why is one harmless and the other serious?

Because Postgres re-checks every row under lock before writing to it. A hint
that over-reports sends a hopeless candidate; the database says no; we lose one
lookup. A hint that under-reports means we never *ask* about tickets that
exist — and nothing downstream can correct a question we never posed.

Every decision in the rest of this module leans toward over-reporting.

### Step 109 — Predict the trap

Before writing any code, answer on paper:

> The hint counts how many tickets each number still has. A user **reserves** a
> ticket. Should the count go down?

Write your answer down. Do not skip this — the whole module turns on it.

**The answer is no.** A reservation is not a sale. The lease may expire and
return the ticket to circulation. If you decrement on reserve, then when the
lease expires your count reads zero, the number leaves the hint, and those
tickets become invisible on this replica.

That is the under-reporting failure, and it is the most common bug in this
design. **It passes every obvious test.** You will prove that at Step 115.

## Part two — build the hint

### Step 110 — sync.RWMutex

A new Go concept, because this is the first thing in the project read far more
often than written.

```go
var mu sync.RWMutex

mu.RLock()          // many readers at once
_ = data[i]
mu.RUnlock()

mu.Lock()           // one writer, excluding all readers
data[i] = v
mu.Unlock()
```

`sync.Mutex` allows one goroutine at a time, full stop. `RWMutex` allows any
number of concurrent readers, but a writer excludes everyone.

The hint is read on every candidate of every request and written only when a
ticket sells, so this is exactly the right shape.

**One rule to internalise:** never hold the lock across a database call. A slow
query would block every reader. Look for that rule being followed in the code
you write below.

### Step 111 — The Availability struct

Create `postgres/load.go`:

```go
type Availability struct {
	counts  []uint32        // counts[n] = tickets of number n not yet sold
	nonZero *lottery.Bitmap // bit set wherever counts[n] > 0
	loaded  time.Time
}

func NewAvailability() *Availability {
	return &Availability{
		counts:  make([]uint32, lottery.NumberSpace),
		nonZero: lottery.NewBitmap(lottery.NumberSpace),
	}
}
```

Two structures for one job. **Ask yourself why we keep both**, when the bitmap
is derivable from the counts.

Because they answer different questions. `counts` answers "how many left"
(needed to know when to clear the bit). The bitmap answers "any left at all" in
one bit test, and — via `NextSet` — skips 64 sold-out numbers per comparison.
That is the Module 4 bitmap earning its place.

Memory: 4 MB of counts, 125 KB of bitmap.

### Step 112 — Write the loading query

```sql
SELECT num, count(*)::int
  FROM tickets
 WHERE status <> 2
 GROUP BY num
```

Two decisions in one small query. Justify each before moving on.

**Why `GROUP BY num` rather than selecting every ticket?** Ten million tickets
across a million numbers means this returns at most a million rows instead of
ten million, and Postgres aggregates where the data already lives. Pulling 10M
rows into Go to count them yourself is the version to avoid.

**Why `status <> 2` (not sold) rather than `status = 0` (available)?** A
reserved ticket still counts, because its lease may expire. Counting only
`status = 0` is the under-reporting mistake from Step 102 — reserved tickets
would vanish from the hint the moment anyone held them.

### Step 113 — Implement Load

Build into scratch space, swap at the end:

```go
func (a *Availability) Load(ctx context.Context, pool *pgxpool.Pool) error {
	counts := make([]uint32, lottery.NumberSpace)
	bm := lottery.NewBitmap(lottery.NumberSpace)

	rows, err := pool.Query(ctx, loadSQL)
	if err != nil {
		return fmt.Errorf("loading availability: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var num, count int32
		if err := rows.Scan(&num, &count); err != nil {
			return fmt.Errorf("scanning availability row: %w", err)
		}
		counts[num] = uint32(count)
		if count > 0 {
			bm.Set(int(num))
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading availability rows: %w", err)
	}

	a.counts = counts
	a.nonZero = bm
	a.loaded = time.Now()
	return nil
}
```

**Why build into locals and assign at the end?** So a reader never sees a
half-built hint. If we wrote into `a.counts` directly, a concurrent request
during a refresh would see a structure that is partly new and partly old.

**And why `rows.Err()` after the loop?** `rows.Next()` returns false both for
"finished" and "broke". Without this check, a connection dropped halfway
through gives you a silently incomplete hint — under-reporting, the dangerous
direction.

### Step 114 — The three mutators

```go
// MaybeAvailable: true means "possibly", false is the only strong claim.
func (a *Availability) MaybeAvailable(n int32) bool {
	return a.nonZero.Test(int(n))
}

// MarkSold: one ticket of this number is permanently gone.
func (a *Availability) MarkSold(n int32) {
	if a.counts[n] > 0 {
		a.counts[n]--
	}
	if a.counts[n] == 0 {
		a.nonZero.Clear(int(n))
	}
}

// MarkAvailable: this number definitely has stock. Correcting upward is
// always safe.
func (a *Availability) MarkAvailable(n int32) {
	if a.counts[n] == 0 {
		a.counts[n] = 1
	}
	a.nonZero.Set(int(n))
}
```

Note the guard `if a.counts[n] > 0` in `MarkSold`. Without it, an unsigned
counter at zero would wrap to 4,294,967,295 and the number would look
inexhaustible forever. **Defensive, because the hint may be stale and we would
rather be slightly wrong than catastrophically wrong.**

### Step 115 — Test the load

```go
func TestHintLoadsFromDatabase(t *testing.T) {
	// insert 123456 three times and 999999 once
	store, _ := postgres.NewHybrid(ctx, pool, nil)

	st := store.HintStats()
	if st.NumbersWithStock != 2 {
		t.Errorf("numbers with stock = %d, want 2", st.NumbersWithStock)
	}
	if st.TicketsUnsold != 4 {
		t.Errorf("tickets unsold = %d, want 4", st.TicketsUnsold)
	}
}
```

## Part three — the hybrid store

### Step 116 — Write the trap test first

Before building `Claim`, write the test for Step 103's answer:

```go
func TestReservingDoesNotReduceUnsoldCount(t *testing.T) {
	// two tickets numbered 123456
	before := store.HintStats().TicketsUnsold
	store.Claim(ctx, p, 1, "alice", time.Minute)
	after := store.HintStats().TicketsUnsold

	if after != before {
		t.Errorf("unsold count changed on reserve: %d -> %d; a reservation is not a sale",
			before, after)
	}
}
```

Writing this first means the trap cannot survive to production.

### Step 117 — The four phases

`Claim` in `postgres/hybrid.go` is four distinct phases. Read them as four:

```
1. GENERATE   pattern → candidate numbers        pure arithmetic, no I/O
2. FILTER     drop what the hint writes off      ~1 ns each, may be wrong
3. ASK        UPDATE ... SKIP LOCKED             authoritative, atomic
4. RECONCILE  fold the answer back into the hint keeps drift bounded
```

Phases 1 and 3 you already built in Modules 3 and 8. This module adds 2 and 4.

### Step 118 — Phase 2, the filter

```go
func (s *HybridStore) filterByHint(raw []int32) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]int32, 0, len(raw))
	for _, n := range raw {
		if s.avail.MaybeAvailable(n) {
			out = append(out, n)
		}
	}
	return out
}
```

**The critical property:** this can only ever *remove* numbers, never add one.
A corrupted hint therefore produces fewer results, never a wrong ticket. Say
that out loud — it is why a cache is acceptable in a system that must never
double-sell.

`RLock`, not `Lock`, because many requests filter concurrently. And the lock is
released before the database call, per Step 104.

### Step 119 — Phase 4, reconcile

```go
func (s *HybridStore) reconcileAfterClaim(batch []lottery.Reservation) {
	s.mu.Lock()
	for _, r := range batch {
		s.avail.MarkAvailable(r.Number)
	}
	s.mu.Unlock()

	s.numMu.Lock()
	for _, r := range batch {
		s.numberOf[r.TicketID] = r.Number
	}
	s.numMu.Unlock()
}
```

**No count decreases here** — that is Step 103's answer in code.

What we *do* learn is that this number definitely had stock, so we correct any
hint that had wrongly written it off. And we remember `id → number`, so
`Confirm` can update the hint later without another query.

**Phase 4 is the step people forget.** Skip it and the hint drifts further from
reality on every single request.

### Step 120 — Confirm is where the count moves

```go
func (s *HybridStore) Confirm(ctx context.Context, id int64, holder string) error {
	tag, err := s.pool.Exec(ctx, confirmSQL, id, holder)
	if err != nil {
		return fmt.Errorf("confirming ticket %d: %w", id, err)
	}
	if tag.RowsAffected() != 1 {
		return (&Store{pool: s.pool}).explainFailure(ctx, id, holder)
	}

	if n, ok := s.forgetNumber(id); ok {
		s.mu.Lock()
		s.avail.MarkSold(n)
		s.mu.Unlock()
	}
	return nil
}
```

The hint is updated **after** Postgres confirms the write — catching up to a
fact, never predicting one.

If we do not recognise the id (another replica handed out that reservation), we
leave the hint alone. It over-reports until the next refresh, which is the safe
direction.

Full event table:

| Event | Unsold count | Why |
|---|---|---|
| Claim / reserve | **unchanged** | a reservation is not a sale |
| Confirm / sold | **−1** | the only permanent removal |
| Release | unchanged | it was never sold |
| Lease expiry | unchanged | same reason |

### Step 121 — Prove the conformance suite still passes

```
make db
go test -tags=integration -race ./postgres/...
```

`HybridStore` must pass **the same eleven tests** as `MemoryStore` and
`postgres.Store`. A cache that changes observable behaviour is not a cache; it
is a bug.

## Part four — break it, then fix it

### Step 122 — Introduce the trap deliberately

Add a decrement to `reconcileAfterClaim`:

```go
s.avail.MarkSold(r.Number)      // WRONG on purpose
```

Run the suite.

- `TestReservingDoesNotReduceUnsoldCount` fails, because you wrote it at Step 110
- `ExpiredLeaseIsReclaimed` in the conformance suite also fails

**Now delete your Step 110 test and run again.** The suite still fails, but only
on the lease test — the failure now appears far from its cause, in a test about
something else entirely.

**That is the lesson.** Without a test aimed at the invariant, this bug surfaces
as "expired reservations sometimes do not come back", days later, in production.
Restore both the test and the correct code.

### Step 123 — The stale hint experiment, and checkpoint

Simulate another replica selling behind your back:

```go
func TestStaleHintOverReportingIsHarmless(t *testing.T) {
	// one ticket numbered 123456; hint loaded and says it exists
	pool.Exec(ctx, `UPDATE tickets SET status = 2 WHERE num = 123456`)

	// hint says yes, database says no — the database wins
	got, _ := store.Claim(ctx, p, 1, "bob", time.Minute)
	if len(got) != 0 {
		t.Errorf("claimed %d tickets from a sold-out number", len(got))
	}

	store.Refresh(ctx)
	// hint is now correct again
}
```

This is the safety rule from Step 102, executable.

Then start two API instances on different ports against the same database. Sell
tickets through one and watch the other's `HintStats` stay stale until its
refresh fires. **Each replica only witnesses its own sales**, so every replica's
count is greater than or equal to the truth — the over-reporting direction, safe
but increasingly wasteful as a draw sells out.

Two ways to converge, and the trade-off is worth stating:

| Approach | Converges in | Cost |
|---|---|---|
| Periodic refresh (default, 30 s) | one interval | a `GROUP BY` every interval |
| `LISTEN/NOTIFY` (`schema_notify.sql`) | milliseconds | a trigger on the hot write path, plus one held connection per replica |

Start with refresh. Move to NOTIFY when you can *show* the refresh interval is
the bottleneck.

**Checkpoint — you can explain, without notes:**

1. Why over-reporting is harmless and under-reporting is not
2. Why reserving does not change the count, but confirming does
3. Why the filter can only narrow the candidate list, never widen it
4. Why the hint stores counts and never ticket IDs
5. What happens when two replicas each sell tickets the other cannot see

On point 4: a cached ticket ID would be a double-sale waiting to happen; a
count that is slightly wrong costs one query. **Cache the cheap wrong thing,
never the dangerous one.**

Re-run Step 99's load test with the hint enabled and compare the p99 against
what you recorded. That number is what this module bought you.

Commit: `git commit -am "availability hint"`

---

# Module 10 — Shipping it

One day.

### Step 124 — Seed ten million tickets

Tutorial Chapter 15.

Seed 100,000 rows with `INSERT` in a loop, then with `COPY`. The gap is usually two orders of magnitude.

### Step 125 — Do not skip ANALYZE

Without fresh statistics the planner may ignore your partial index, and you will wrongly conclude the index does not work. Run `ANALYZE tickets` after bulk loading, always.

### Step 126 — main that returns an error

```go
func main() {
	if err := run(log); err != nil {
		log.Error("fatal", "error", err)
		os.Exit(1)
	}
}
```

`main` does almost nothing. `run` returns an error. This keeps `os.Exit` in exactly one place and makes startup testable.

### Step 127 — Graceful shutdown

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
```

SIGTERM becomes context cancellation, which unwinds the reaper and the server through one mechanism.

**Ask yourself:** what happens to a user mid-claim during a deploy? With `srv.Shutdown`, their request finishes. Without it, the connection drops and they lose their tickets.

### Step 128 — Set your HTTP timeouts

```go
srv := &http.Server{
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       15 * time.Second,
	WriteTimeout:      15 * time.Second,
	IdleTimeout:       60 * time.Second,
}
```

**Go's defaults are all zero, meaning no timeout at all.** That is a slow-loris vulnerability in every Go service that forgets them. Not optional.

### Step 129 — Configuration from the environment

Environment variables with defaults, documented in `.env.example`. No config file parser, no flags library. Works unchanged in Docker and Kubernetes.

### Step 130 — Run the whole thing

```
make db
make seed-small
make run
```

```
curl -s localhost:8080/search -d '{"pattern":"****23","count":5,"holder":"alice"}'
```

**You have built a working service.**

### Step 131 — Checkpoint

The service starts, serves searches, confirms reservations, and shuts down cleanly on Ctrl-C. You can explain why `main` is three lines and why HTTP timeouts are mandatory.

Commit and tag: `git commit -am "deployable service" && git tag v1.0`

---

# Module 11 — Scaling

One day. Follow `lottery-scaling-guide.md`.

### Step 132 — Measure before changing anything

```
make load
```

Record throughput, p50, p99.

### Step 133 — Understand p50 versus p99

An average of 20 ms tells you nothing. A p50 of 11 ms with a p99 of 185 ms means one request in a hundred is sixteen times slower — and at 3,400 req/sec that happens 34 times a second, to real people.

**Tail latency is the number that matters.**

### Step 134 — Find the knee

```
make load-sweep
```

Three shapes: throughput rising with flat latency (headroom), throughput flat with latency rising (saturated — this is the knee), throughput falling with latency spiking (overloaded).

### Step 135 — Learn the four diagnostic metrics

Scaling guide, Part 1. Four failure modes that all look like "the API got slow", each with a different fix. **Choosing wrong makes things worse.**

### Step 136 — Ask the database what it is waiting on

```
make stats
```

`pg_stat_activity` wait events, index scan counts, status distribution.

### Step 137 — The counter-intuitive experiment

Set `LOTTERY_MAX_CONNS=100`, run the load test. Then set it to `5` and run it again.

**It gets faster with fewer connections.** Four replicas × 20 connections against an 8-core database means Postgres context-switches instead of working. Predict the result before running it — most people guess wrong, and that surprise is the lesson.

### Step 138 — Why read replicas do not help here

Every request in this system writes. A claim is an `UPDATE`. There is nothing to send to a replica.

**The standard scaling playbook assumes read-heavy traffic.** Always check which kind you have before reaching for a technique.

### Step 139 — Diagnose three sabotaged systems

Scaling guide, Part 7. Your instructor breaks the system three ways; you diagnose each from metrics alone.

1. Oversized connection pool
2. Nearly-empty inventory with broad patterns
3. Fixed walker seed

Same symptom, three causes, three fixes. **Reading the metrics is the skill.**

### Step 140 — Understand where sharding sits

Sharding is step 7 of 9, after four configuration changes and two other fixes. It is the interesting problem, which is why everyone jumps to it, which is why it is the most common scaling mistake in industry.

### Step 141 — The non-technical constraint

Sharding makes ticket selection non-uniform across inventory. **For a real lottery that may be a regulatory question, not an engineering one.**

"Technically better" and "allowed" are different tests. Engineering does not get to decide this alone.

### Step 142 — Final checkpoint

You can:

- Run a load test and read the percentiles
- Find the knee in the throughput curve
- Name the four diagnostic metrics and what each implies
- Explain why more connections can mean less throughput
- List the scaling steps in order and justify the ordering

---

# When you are stuck

**Read the error message.** Go's compiler errors name the file, line, and problem. Most juniors skim them and start guessing.

**Run one test.** `go test -run TestName -v ./...` beats staring at fifty results.

**Print things.** `t.Logf("got %+v", value)` in a test, visible with `-v`. `%+v` prints struct field names.

**Check what you changed.** `git diff`. The bug is almost always in the last thing you touched.

**Reproduce it smaller.** If the twelve-ticket case works and ten million does not, the difference is scale. If both fail, the logic is wrong and the small case will show you where.

**Rubber-duck it.** Explain the failing code out loud, line by line, to a colleague or an object. Explaining it usually finds it.

**Then look at `lottery-go/`.** Compare your version to the reference. Do this last, not first — comparing before attempting teaches nothing.

---

# Assessment

| Level | Evidence |
|---|---|
| **Pass** | All modules complete, `go test -race ./...` green, service runs and serves requests |
| **Good** | The above, plus can explain *why* each design decision was made, not just what it does |
| **Strong** | The above, plus completed an exercise from the tutorial's list — Strategy B, idempotency keys, or a third `TicketStore` implementation |

### Final task

Write one page explaining the system to a colleague who has never seen it. Cover:

- Why we do not scan ten million tickets
- How wildcards become numbers
- How two users are prevented from getting the same ticket
- Why reservations expire
- What you would measure first if it got slow

**If you can write that page, you have learned the course.** Everything else is detail.
