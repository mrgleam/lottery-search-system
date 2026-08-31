package lottery

import (
	"context"
	"encoding/json"
	"fmt"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// Server exposes a TicketStore over HTTP. It depends on the interface, not on
// any particular backend, so the same handlers serve the in-memory store in
// tests and Postgres in production.
type Server struct {
	store TicketStore
	mux   *http.ServeMux
	lease time.Duration
	log   *slog.Logger
}

// NewServer wires a store to routes.
func NewServer(store TicketStore, lease time.Duration, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{store: store, mux: http.NewServeMux(), lease: lease, log: log}
	s.mux.HandleFunc("POST /search", s.handleSearch)
	s.mux.HandleFunc("POST /reservations/{id}/confirm", s.handleConfirm)
	s.mux.HandleFunc("POST /reservations/{id}/release", s.handleRelease)
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

type searchRequest struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	Holder  string `json:"holder"`
}

// ticketView is the WIRE representation of a reservation, deliberately
// separate from the domain type.
//
// A lottery number is six characters, not an integer -- "002323", never 2323.
// Storing it as an int32 internally is an implementation detail that must not
// leak to clients, who would otherwise render a number that does not exist.
type ticketView struct {
	ID         int64     `json:"id"`
	Number     string    `json:"number"`
	LeaseUntil time.Time `json:"lease_until"`
}

func viewOf(r Reservation) ticketView {
	return ticketView{
		ID:         r.TicketID,
		Number:     fmt.Sprintf("%0*d", Digits, r.Number),
		LeaseUntil: r.LeaseUntil,
	}
}

type searchResponse struct {
	Tickets        []ticketView `json:"tickets"`
	Requested      int          `json:"requested"`
	Partial        bool         `json:"partial"`
	DistinctNumbers int         `json:"distinct_numbers"`
}

const maxCount = 20

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}
	p, err := ParsePattern(req.Pattern)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Count < 1 || req.Count > maxCount {
		writeError(w, http.StatusBadRequest, "count must be between 1 and 20")
		return
	}
	if req.Holder == "" {
		writeError(w, http.StatusBadRequest, "holder is required")
		return
	}

	got, err := s.store.Claim(r.Context(), p, req.Count, req.Holder, s.lease)
	if err != nil {
		s.log.Error("claim failed", "pattern", req.Pattern, "holder", req.Holder, "error", err)
		writeError(w, http.StatusInternalServerError, "could not complete search")
		return
	}

	// make(...) not var: a nil slice marshals to null, which breaks clients
	// that iterate the array without checking.
	tickets := make([]ticketView, 0, len(got))
	distinct := map[int32]struct{}{}
	for _, r := range got {
		tickets = append(tickets, viewOf(r))
		distinct[r.Number] = struct{}{}
	}

	writeJSON(w, http.StatusOK, searchResponse{
		Tickets:         tickets,
		Requested:       req.Count,
		Partial:         len(tickets) < req.Count,
		DistinctNumbers: len(distinct),
	})
}

type holderRequest struct {
	Holder string `json:"holder"`
}

// reservationAction is the shape of both Confirm and Release, so one handler
// serves both routes.
type reservationAction func(ctx context.Context, id int64, holder string) error

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	s.finishReservation(w, r, s.store.Confirm)
}

func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	s.finishReservation(w, r, s.store.Release)
}

func (s *Server) finishReservation(w http.ResponseWriter, r *http.Request, action reservationAction) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "reservation id must be a non-negative integer")
		return
	}
	var req holderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "malformed JSON body")
		return
	}
	if req.Holder == "" {
		writeError(w, http.StatusBadRequest, "holder is required")
		return
	}

	if err := action(r.Context(), id, req.Holder); err != nil {
		code, msg := statusForError(err)
		if code == http.StatusInternalServerError {
			s.log.Error("reservation action failed", "id", id, "holder", req.Holder, "error", err)
		}
		writeError(w, code, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// statusForError maps domain errors onto HTTP codes in one place, so handlers
// never sprinkle status codes around.
func statusForError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrTicketNotFound):
		return http.StatusNotFound, "no such reservation"
	case errors.Is(err, ErrNotHeld):
		return http.StatusConflict, "you are not holding this ticket"
	case errors.Is(err, ErrLeaseExpired):
		return http.StatusGone, "the hold on this ticket has expired"
	default:
		return http.StatusInternalServerError, "unexpected error"
	}
}

func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id >= 0
}
