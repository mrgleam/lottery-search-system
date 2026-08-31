package lottery

import "testing"

// The same twelve tickets used on the whiteboard, so students can check the
// code against the tables they filled in by hand.
func twelveTickets() []Ticket {
	return []Ticket{
		{ID: 1, Number: 3}, {ID: 2, Number: 1}, {ID: 3, Number: 4},
		{ID: 4, Number: 1}, {ID: 5, Number: 5}, {ID: 6, Number: 9},
		{ID: 7, Number: 2}, {ID: 8, Number: 6}, {ID: 9, Number: 5},
		{ID: 10, Number: 3}, {ID: 11, Number: 5}, {ID: 12, Number: 8},
	}
}

func equalIDs(got, want []int32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestIndexGroupsTicketsByNumber(t *testing.T) {
	ix := BuildIndex(twelveTickets())

	cases := []struct {
		number int
		want   []int32
	}{
		{1, []int32{2, 4}},
		{2, []int32{7}},
		{3, []int32{1, 10}},
		{4, []int32{3}},
		{5, []int32{5, 9, 11}},
		{6, []int32{8}},
		{8, []int32{12}},
		{9, []int32{6}},
	}
	for _, c := range cases {
		if got := ix.TicketsFor(c.number); !equalIDs(got, c.want) {
			t.Errorf("TicketsFor(%d) = %v, want %v", c.number, got, c.want)
		}
	}
}

func TestIndexReturnsNothingForEmptyNumbers(t *testing.T) {
	ix := BuildIndex(twelveTickets())
	for _, n := range []int{0, 7, 500000, NumberSpace - 1} {
		if got := ix.TicketsFor(n); len(got) != 0 {
			t.Errorf("TicketsFor(%d) = %v, want empty", n, got)
		}
	}
}

func TestIndexHoldsEveryTicketExactlyOnce(t *testing.T) {
	tickets := twelveTickets()
	ix := BuildIndex(tickets)

	if ix.Len() != len(tickets) {
		t.Fatalf("index holds %d tickets, want %d", ix.Len(), len(tickets))
	}
	seen := map[int32]int{}
	for n := 0; n < NumberSpace; n++ {
		for _, id := range ix.TicketsFor(n) {
			seen[id]++
		}
	}
	for _, tk := range tickets {
		if seen[tk.ID] != 1 {
			t.Errorf("ticket %d appears %d times, want 1", tk.ID, seen[tk.ID])
		}
	}
}

func BenchmarkBuildIndex(b *testing.B) {
	tickets := make([]Ticket, 1000000)
	for i := range tickets {
		tickets[i] = Ticket{ID: int32(i), Number: int32(i % NumberSpace)}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildIndex(tickets)
	}
}
