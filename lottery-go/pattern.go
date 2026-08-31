package lottery

import (
	"errors"
	"fmt"
)

// Digits is how many digits a ticket number has.
const Digits = 6

// NumberSpace is how many distinct ticket numbers exist: 10^Digits.
const NumberSpace = 1000000

// ErrBadPattern is returned by ParsePattern for anything malformed.
var ErrBadPattern = errors.New("bad pattern")

// pow10[i] == 10^i. We need index Digits, so the array has Digits+1 entries.
var pow10 = [Digits + 1]int{1, 10, 100, 1000, 10000, 100000, 1000000}

// Pattern is a parsed search pattern such as "1**4*7".
//
// base holds the value contributed by the fixed digits; wildcards holds the
// positions that may vary, counting from the left.
type Pattern struct {
	base      int
	wildcards []int
}

// ParsePattern validates and parses a six-character pattern of digits and '*'.
func ParsePattern(s string) (Pattern, error) {
	if len(s) != Digits {
		return Pattern{}, fmt.Errorf("%w: want %d characters, got %d", ErrBadPattern, Digits, len(s))
	}
	var p Pattern
	for i := 0; i < Digits; i++ {
		c := s[i]
		switch {
		case c == '*':
			p.wildcards = append(p.wildcards, i)
		case c >= '0' && c <= '9':
			p.base += int(c-'0') * pow10[Digits-1-i]
		default:
			return Pattern{}, fmt.Errorf("%w: unexpected character %q", ErrBadPattern, c)
		}
	}
	return p, nil
}

// MatchCount is how many numbers this pattern matches: 10^(number of wildcards).
func (p Pattern) MatchCount() int {
	return pow10[len(p.wildcards)]
}

// NumberAt returns the j-th matching number, for j in [0, MatchCount()).
//
// This is the odometer: fixed digits stay still, wildcard dials spin.
func (p Pattern) NumberAt(j int) int {
	n := p.base
	for i := len(p.wildcards) - 1; i >= 0; i-- {
		d := j % 10
		j /= 10
		n += d * pow10[Digits-1-p.wildcards[i]]
	}
	return n
}

// Matches reports whether a number fits the pattern. Handy for tests; the
// search path never needs it, because it generates matches instead.
func (p Pattern) Matches(number int) bool {
	rest := number
	for _, pos := range p.wildcards {
		place := pow10[Digits-1-pos]
		rest -= ((number / place) % 10) * place
	}
	return rest == p.base
}
