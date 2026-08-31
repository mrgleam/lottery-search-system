package lottery

import (
	"errors"
	"testing"
)

func TestParsePatternRejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"too short", "123"},
		{"too long", "1234567"},
		{"empty", ""},
		{"letter", "12a456"},
		{"question mark instead of star", "12?456"},
		{"space", "12 456"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePattern(c.input)
			if !errors.Is(err, ErrBadPattern) {
				t.Errorf("ParsePattern(%q) error = %v, want ErrBadPattern", c.input, err)
			}
		})
	}
}

func TestParsePatternCountsMatches(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"123456", 1},
		{"12345*", 10},
		{"1234**", 100},
		{"1*3*5*", 1000},
		{"******", 1000000},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			p, err := ParsePattern(c.input)
			if err != nil {
				t.Fatalf("ParsePattern(%q) unexpected error: %v", c.input, err)
			}
			if got := p.MatchCount(); got != c.want {
				t.Errorf("MatchCount() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestNumberAtSpinsTheDials(t *testing.T) {
	cases := []struct {
		pattern string
		j       int
		want    int
	}{
		{"123456", 0, 123456},
		{"1*3456", 0, 103456},
		{"1*3456", 7, 173456},
		{"1*3456", 9, 193456},
		{"123***", 0, 123000},
		{"123***", 45, 123045},
		{"******", 123456, 123456},
	}
	for _, c := range cases {
		t.Run(c.pattern, func(t *testing.T) {
			p, err := ParsePattern(c.pattern)
			if err != nil {
				t.Fatalf("ParsePattern(%q): %v", c.pattern, err)
			}
			if got := p.NumberAt(c.j); got != c.want {
				t.Errorf("NumberAt(%d) = %06d, want %06d", c.j, got, c.want)
			}
		})
	}
}

// The property that matters: walking j from 0 to MatchCount()-1 produces every
// matching number, with no repeats and nothing that does not match.
func TestNumberAtCoversEveryMatchExactlyOnce(t *testing.T) {
	p, err := ParsePattern("1*3*5*")
	if err != nil {
		t.Fatalf("ParsePattern: %v", err)
	}
	seen := make(map[int]bool, p.MatchCount())
	for j := 0; j < p.MatchCount(); j++ {
		n := p.NumberAt(j)
		if seen[n] {
			t.Fatalf("number %06d produced twice", n)
		}
		if !p.Matches(n) {
			t.Fatalf("number %06d does not match the pattern", n)
		}
		seen[n] = true
	}
	if len(seen) != p.MatchCount() {
		t.Errorf("produced %d distinct numbers, want %d", len(seen), p.MatchCount())
	}
}
