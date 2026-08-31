package lottery

// Ticket is one lottery ticket.
type Ticket struct {
	ID     int32
	Number int32
}

// Index maps a ticket number to the IDs of the tickets carrying it.
//
// It is two flat arrays, not a million slices:
//
//	offsets[n] .. offsets[n+1]  is the slice of ticketIDs belonging to number n
//
// offsets has NumberSpace+1 entries. The extra one at the end is the sentinel,
// which is what lets the rule above work for the last number with no special
// case.
type Index struct {
	offsets   []int32
	ticketIDs []int32
}

// BuildIndex groups tickets by number in three passes: count, running total,
// place. This is counting sort; we keep the index it builds along the way and
// throw away the sorted order.
func BuildIndex(tickets []Ticket) *Index {
	// Pass 1: count. Counting into offsets[n+1] means pass 2 can turn the
	// counts into start positions in place.
	offsets := make([]int32, NumberSpace+1)
	for _, t := range tickets {
		offsets[t.Number+1]++
	}

	// Pass 2: running total. offsets[n] now means "how many tickets have a
	// number smaller than n", which is exactly where n's block starts.
	for n := 0; n < NumberSpace; n++ {
		offsets[n+1] += offsets[n]
	}

	// Pass 3: place. We walk a COPY, because writing through offsets itself
	// would advance every entry past its own block and destroy the index.
	cursor := make([]int32, NumberSpace+1)
	copy(cursor, offsets)

	ids := make([]int32, len(tickets))
	for _, t := range tickets {
		ids[cursor[t.Number]] = t.ID
		cursor[t.Number]++
	}

	return &Index{offsets: offsets, ticketIDs: ids}
}

// TicketsFor returns the IDs of tickets carrying this number. The result
// aliases the index's own storage, so callers must not modify it.
func (ix *Index) TicketsFor(number int) []int32 {
	return ix.ticketIDs[ix.offsets[number]:ix.offsets[number+1]]
}

// Len is the total number of tickets indexed.
func (ix *Index) Len() int { return len(ix.ticketIDs) }
