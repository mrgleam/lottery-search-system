package lottery

import "math/rand"

// Candidates produces the numbers matching a pattern, in a scrambled order
// that differs per seed, in batches.
//
// Batching is what lets the same generator serve both stores: the in-memory
// store walks a batch and checks its own index, while the Postgres store sends
// the batch to the database as an array parameter.
type Candidates struct {
	p Pattern
	w *walker
}

// Candidates starts a fresh traversal. Pass a different seed per request so
// concurrent users for the same pattern do not collide on the same numbers.
func (p Pattern) Candidates(seed int64) *Candidates {
	return &Candidates{p: p, w: newWalker(p.MatchCount(), seed)}
}

// RandomSeed is what production passes to Candidates.
func RandomSeed() int64 { return rand.Int63() }

// NextBatch returns up to max further candidate numbers, or an empty slice
// once the whole match set has been walked.
func (c *Candidates) NextBatch(max int) []int32 {
	out := make([]int32, 0, max)
	for len(out) < max {
		j, ok := c.w.next()
		if !ok {
			break
		}
		out = append(out, int32(c.p.NumberAt(j)))
	}
	return out
}
