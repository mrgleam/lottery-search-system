package lottery

import "math/rand"

// walker visits every value in [0, n) exactly once, in a scrambled order that
// differs per seed.
//
// It uses j = (a*i + b) mod n. When a shares no factor with n, that formula
// is a permutation: it hits every value once and never repeats. When a DOES
// share a factor, it visits only a fraction of the range and the rest of the
// tickets become permanently unreachable.
type walker struct {
	n, a, b, i int
}

func newWalker(n int, seed int64) *walker {
	if n <= 0 {
		return &walker{}
	}
	r := rand.New(rand.NewSource(seed))
	return &walker{n: n, a: coprimeTo(n, r), b: r.Intn(n)}
}

// next returns the next value and whether there was one.
func (w *walker) next() (int, bool) {
	if w.i >= w.n {
		return 0, false
	}
	// a and i are both < n <= 1e6, so a*i stays well inside int64.
	j := (w.a*w.i + w.b) % w.n
	w.i++
	return j, true
}

// coprimeTo picks a random multiplier that shares no factor with n.
func coprimeTo(n int, r *rand.Rand) int {
	for {
		a := 1 + r.Intn(n)
		if gcd(a, n) == 1 {
			return a
		}
	}
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
