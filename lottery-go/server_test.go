package lottery_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"lottery"
)

func newTestServer(numbers ...int32) *lottery.Server {
	tickets := make([]lottery.Ticket, len(numbers))
	for i, n := range numbers {
		tickets[i] = lottery.Ticket{ID: int32(i), Number: n}
	}
	store := lottery.NewMemoryStore(tickets, newFakeClock().Now)
	return lottery.NewServer(store, time.Minute, nil)
}

func do(t *testing.T, srv *lottery.Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestSearchReturnsMatchingTickets(t *testing.T) {
	srv := newTestServer(123456, 999923, 111123)

	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"****23","count":2,"holder":"alice"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Tickets []struct {
			ID     int64  `json:"id"`
			Number string `json:"number"`
		} `json:"tickets"`
		Partial bool `json:"partial"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Tickets) != 2 {
		t.Fatalf("got %d tickets, want 2", len(resp.Tickets))
	}
	if resp.Partial {
		t.Error("partial = true, but the full request was satisfied")
	}
	for _, tk := range resp.Tickets {
		if !strings.HasSuffix(tk.Number, "23") {
			t.Errorf("ticket number %s does not end in 23", tk.Number)
		}
	}
}

// The wire format must be six characters. A client rendering "2323" shows a
// number that does not exist.
func TestSearchPadsNumbersToSixDigits(t *testing.T) {
	srv := newTestServer(2323, 12323)

	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"**2323","count":1,"holder":"alice"}`)

	var resp struct {
		Tickets []struct {
			Number string `json:"number"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding: %v; body = %s", err, rec.Body.String())
	}
	if len(resp.Tickets) == 0 {
		t.Fatalf("no tickets returned; body = %s", rec.Body.String())
	}
	for _, tk := range resp.Tickets {
		if len(tk.Number) != 6 {
			t.Errorf("number = %q, want six characters", tk.Number)
		}
	}
}

func TestSearchFlagsPartialResults(t *testing.T) {
	srv := newTestServer(123456)

	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"123456","count":5,"holder":"alice"}`)

	var resp struct {
		Tickets []struct {
			ID     int64  `json:"id"`
			Number string `json:"number"`
		} `json:"tickets"`
		Partial bool `json:"partial"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp.Tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(resp.Tickets))
	}
	if !resp.Partial {
		t.Error("partial = false, but only 1 of 5 requested tickets was available")
	}
}

func TestSearchRejectsBadRequests(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad pattern", `{"pattern":"12","count":1,"holder":"a"}`, http.StatusBadRequest},
		{"malformed json", `{"pattern":`, http.StatusBadRequest},
		{"zero count", `{"pattern":"******","count":0,"holder":"a"}`, http.StatusBadRequest},
		{"huge count", `{"pattern":"******","count":9999,"holder":"a"}`, http.StatusBadRequest},
		{"no holder", `{"pattern":"******","count":1}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := newTestServer(123456)
			rec := do(t, srv, http.MethodPost, "/search", c.body)
			if rec.Code != c.want {
				t.Errorf("status = %d, want %d; body = %s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
}

func TestSearchRejectsGET(t *testing.T) {
	srv := newTestServer(123456)
	rec := do(t, srv, http.MethodGet, "/search", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// A nil slice marshals to null and breaks clients that iterate without a check.
func TestSearchReturnsEmptyArrayNotNull(t *testing.T) {
	srv := newTestServer(123456)
	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"999999","count":5,"holder":"alice"}`)

	if got := rec.Body.String(); !strings.Contains(got, `"tickets":[]`) {
		t.Errorf("body = %s, want an empty JSON array rather than null", got)
	}
}

func TestConfirmAndReleaseLifecycle(t *testing.T) {
	srv := newTestServer(123456, 123456)

	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"123456","count":2,"holder":"alice"}`)
	var resp struct {
		Tickets []struct {
			ID int64 `json:"id"`
		} `json:"tickets"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Tickets) != 2 {
		t.Fatalf("expected 2 reservations, got %d", len(resp.Tickets))
	}

	confirm := do(t, srv, http.MethodPost,
		"/reservations/"+itoa(resp.Tickets[0].ID)+"/confirm", `{"holder":"alice"}`)
	if confirm.Code != http.StatusOK {
		t.Errorf("confirm status = %d, want 200; body = %s", confirm.Code, confirm.Body.String())
	}

	release := do(t, srv, http.MethodPost,
		"/reservations/"+itoa(resp.Tickets[1].ID)+"/release", `{"holder":"alice"}`)
	if release.Code != http.StatusOK {
		t.Errorf("release status = %d, want 200; body = %s", release.Code, release.Body.String())
	}
}

func TestConfirmByTheWrongHolderIsAConflict(t *testing.T) {
	srv := newTestServer(123456)

	rec := do(t, srv, http.MethodPost, "/search", `{"pattern":"123456","count":1,"holder":"alice"}`)
	var resp struct {
		Tickets []struct {
			ID int64 `json:"id"`
		} `json:"tickets"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	bad := do(t, srv, http.MethodPost,
		"/reservations/"+itoa(resp.Tickets[0].ID)+"/confirm", `{"holder":"bob"}`)
	if bad.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409; body = %s", bad.Code, bad.Body.String())
	}
}

func TestConfirmUnknownReservationIsNotFound(t *testing.T) {
	srv := newTestServer(123456)
	rec := do(t, srv, http.MethodPost, "/reservations/9999/confirm", `{"holder":"alice"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(123456)
	rec := do(t, srv, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
