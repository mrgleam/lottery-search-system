package lottery

import "testing"

func TestWalkerVisitsEveryValueExactlyOnce(t *testing.T) {
	for _, n := range []int{1, 2, 10, 97, 1000} {
		for seed := int64(0); seed < 5; seed++ {
			seen := make([]bool, n)
			w := newWalker(n, seed)
			count := 0
			for {
				j, ok := w.next()
				if !ok {
					break
				}
				if j < 0 || j >= n {
					t.Fatalf("n=%d seed=%d: produced %d, out of range", n, seed, j)
				}
				if seen[j] {
					t.Fatalf("n=%d seed=%d: produced %d twice", n, seed, j)
				}
				seen[j] = true
				count++
			}
			if count != n {
				t.Errorf("n=%d seed=%d: produced %d values, want %d", n, seed, count, n)
			}
		}
	}
}

func TestWalkerStartsSomewhereDifferentPerSeed(t *testing.T) {
	starts := map[int]bool{}
	for seed := int64(0); seed < 20; seed++ {
		w := newWalker(1000, seed)
		j, ok := w.next()
		if !ok {
			t.Fatal("walker produced nothing")
		}
		starts[j] = true
	}
	if len(starts) < 2 {
		t.Errorf("20 seeds produced %d distinct starting points; randomisation is not working", len(starts))
	}
}

// This test does not check our code. It demonstrates WHY the coprime rule
// exists, by showing what a bad multiplier does. Delete it and the system
// still passes its tests while silently stranding inventory -- which is
// exactly what makes this class of bug dangerous.
func TestNonCoprimeMultiplierStrandsMostOfTheRange(t *testing.T) {
	const n = 10
	badA, badB := 5, 3

	seen := map[int]bool{}
	for i := 0; i < n; i++ {
		seen[(badA*i+badB)%n] = true
	}
	if len(seen) == n {
		t.Fatalf("a=%d was expected to be a bad multiplier for n=%d", badA, n)
	}
	t.Logf("a=%d, n=%d reaches only %d of %d values -- the other %d are unsellable",
		badA, n, len(seen), n, n-len(seen))
}

func TestGCD(t *testing.T) {
	cases := []struct{ a, b, want int }{
		{7, 10, 1},
		{5, 10, 5},
		{2, 10, 2},
		{12, 18, 6},
		{1, 1, 1},
	}
	for _, c := range cases {
		if got := gcd(c.a, c.b); got != c.want {
			t.Errorf("gcd(%d, %d) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
