package lottery

import "math/bits"

const wordBits = 64

// Bitmap is a fixed-size set of non-negative integers, one bit per member.
type Bitmap struct {
	words []uint64
	size  int
}

// NewBitmap makes a bitmap able to hold 0..size-1. Note the rounding UP: a
// size of 65 needs two words, not one.
func NewBitmap(size int) *Bitmap {
	return &Bitmap{
		words: make([]uint64, (size+wordBits-1)/wordBits),
		size:  size,
	}
}

// Size is how many members the bitmap can hold.
func (b *Bitmap) Size() int { return b.size }

// Set ticks the box for n.
func (b *Bitmap) Set(n int) {
	b.words[n/wordBits] |= 1 << (n % wordBits)
}

// Clear unticks the box for n. &^= is Go's "and not", which clears exactly the
// bits set in the operand and leaves every other bit alone.
func (b *Bitmap) Clear(n int) {
	b.words[n/wordBits] &^= 1 << (n % wordBits)
}

// Test reports whether the box for n is ticked.
func (b *Bitmap) Test(n int) bool {
	return b.words[n/wordBits]&(1<<(n%wordBits)) != 0
}

// NextSet returns the smallest m >= from whose bit is set, or -1 if there is
// none. Whole words that are zero are skipped in a single comparison, so empty
// stretches cost one step per 64 members rather than one step per member.
func (b *Bitmap) NextSet(from int) int {
	if from < 0 {
		from = 0
	}
	if from >= b.size {
		return -1
	}
	w := from / wordBits
	// Mask off the bits below `from` in the first word only.
	word := b.words[w] &^ ((1 << (from % wordBits)) - 1)
	for {
		if word != 0 {
			m := w*wordBits + bits.TrailingZeros64(word)
			if m >= b.size {
				return -1
			}
			return m
		}
		w++
		if w >= len(b.words) {
			return -1
		}
		word = b.words[w]
	}
}
