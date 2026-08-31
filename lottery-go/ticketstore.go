package lottery

import (
	"context"
	"errors"
	"time"
)

// Status values are stored as-is in the database, so these numbers are part of
// the schema. Never renumber them without a migration.
type Status int16

const (
	Available Status = 0
	Reserved  Status = 1
	Sold      Status = 2
)

var (
	// ErrNotHeld means the ticket is not reserved by the caller.
	ErrNotHeld = errors.New("ticket is not reserved by this holder")
	// ErrLeaseExpired means the hold ran out before it was confirmed.
	ErrLeaseExpired = errors.New("lease has expired")
	// ErrTicketNotFound means there is no ticket with that id.
	ErrTicketNotFound = errors.New("no such ticket")
)

// Reservation is one ticket held for one user until LeaseUntil.
type Reservation struct {
	TicketID   int64     `json:"id"`
	Number     int32     `json:"number"`
	LeaseUntil time.Time `json:"lease_until"`
}

// TicketStore is everything the HTTP layer needs. Both the in-memory store and
// the Postgres store implement it, and both are checked by the same test suite
// in the storetest package.
//
// Claim may return fewer than k reservations, including none. The returned
// slice is the authority on what the caller actually got.
type TicketStore interface {
	Claim(ctx context.Context, p Pattern, k int, holder string, lease time.Duration) ([]Reservation, error)
	Confirm(ctx context.Context, id int64, holder string) error
	Release(ctx context.Context, id int64, holder string) error
}

// Compile-time proof that MemoryStore satisfies the interface. If someone
// changes a signature, the build breaks here with a clear message rather than
// somewhere confusing at the call site.
var _ TicketStore = (*MemoryStore)(nil)
