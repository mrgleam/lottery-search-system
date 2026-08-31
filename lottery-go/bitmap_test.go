package lottery

import "testing"

func TestBitmapSetTestClear(t *testing.T) {
	b := NewBitmap(200)

	if b.Test(5) {
		t.Error("a fresh bitmap should have nothing set")
	}
	b.Set(5)
	if !b.Test(5) {
		t.Error("Test(5) should be true after Set(5)")
	}
	b.Clear(5)
	if b.Test(5) {
		t.Error("Test(5) should be false after Clear(5)")
	}
}

// This is the test that catches writing `=` where `|=` was meant. One word
// holds 64 members, so a careless Set wipes the other 63.
func TestBitmapSetLeavesNeighboursAlone(t *testing.T) {
	b := NewBitmap(200)
	for _, n := range []int{0, 1, 5, 63} {
		b.Set(n)
	}
	b.Set(7)

	for _, n := range []int{0, 1, 5, 7, 63} {
		if !b.Test(n) {
			t.Errorf("bit %d should still be set", n)
		}
	}
}

func TestBitmapRoundsSizeUp(t *testing.T) {
	// 65 members need two 64-bit words. Sizing down would panic here.
	b := NewBitmap(65)
	b.Set(64)
	if !b.Test(64) {
		t.Error("bit 64 should be settable in a 65-member bitmap")
	}
}

func TestBitmapNextSetSkipsEmptyRuns(t *testing.T) {
	b := NewBitmap(1000)
	b.Set(3)
	b.Set(700)

	if got := b.NextSet(0); got != 3 {
		t.Errorf("NextSet(0) = %d, want 3", got)
	}
	if got := b.NextSet(4); got != 700 {
		t.Errorf("NextSet(4) = %d, want 700", got)
	}
	if got := b.NextSet(701); got != -1 {
		t.Errorf("NextSet(701) = %d, want -1", got)
	}
}

func TestBitmapNextSetFindsBitAtTheQueryPosition(t *testing.T) {
	b := NewBitmap(1000)
	b.Set(64)
	if got := b.NextSet(64); got != 64 {
		t.Errorf("NextSet(64) = %d, want 64", got)
	}
}
