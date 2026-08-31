package lottery

import (
	"sort"
	"sync"
	"time"
)

// Metrics records what the claim path actually did, so scaling decisions rest
// on measurements rather than intuition.
//
// The fields worth watching are not the obvious ones. Throughput tells you
// when you have a problem; CandidatesWalked and RowsContended tell you WHICH
// problem, which is what decides the fix.
type Metrics struct {
	mu sync.Mutex

	Requests   int64
	Empty      int64 // requests that matched nothing at all
	Partial    int64 // requests served fewer tickets than asked for
	Issued     int64 // tickets actually handed out
	DBRoundTri int64 // database queries issued by the claim path

	// CandidatesWalked counts candidate numbers considered. Rising sharply
	// while Issued stays flat means inventory is thinning, and the fix is
	// Strategy B, not more servers.
	CandidatesWalked int64

	// RowsContended counts rows skipped because someone else held them.
	// Rising while everything else is stable means genuine lock contention,
	// and the fix is spreading writes.
	RowsContended int64

	latencies []time.Duration // capped sample
}

const maxLatencySamples = 10000

// Observe records one completed claim.
func (m *Metrics) Observe(d time.Duration, requested, issued, candidates, contended, roundTrips int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Requests++
	m.Issued += int64(issued)
	m.CandidatesWalked += int64(candidates)
	m.RowsContended += int64(contended)
	m.DBRoundTri += int64(roundTrips)
	if issued == 0 {
		m.Empty++
	} else if issued < requested {
		m.Partial++
	}

	// Reservoir-free cap: once full we stop sampling. Good enough for a
	// teaching system; a real one would use a proper histogram (HdrHistogram
	// or Prometheus buckets) that does not lose the tail.
	if len(m.latencies) < maxLatencySamples {
		m.latencies = append(m.latencies, d)
	}
}

// Snapshot is a point-in-time view, safe to serialise.
type Snapshot struct {
	Requests            int64   `json:"requests"`
	Empty               int64   `json:"empty"`
	Partial             int64   `json:"partial"`
	Issued              int64   `json:"issued"`
	DBRoundTrips        int64   `json:"db_round_trips"`
	CandidatesPerRequest float64 `json:"candidates_per_request"`
	ContendedPerRequest  float64 `json:"contended_per_request"`
	P50ms               float64 `json:"p50_ms"`
	P99ms               float64 `json:"p99_ms"`
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := Snapshot{
		Requests:     m.Requests,
		Empty:        m.Empty,
		Partial:      m.Partial,
		Issued:       m.Issued,
		DBRoundTrips: m.DBRoundTri,
	}
	if m.Requests > 0 {
		s.CandidatesPerRequest = float64(m.CandidatesWalked) / float64(m.Requests)
		s.ContendedPerRequest = float64(m.RowsContended) / float64(m.Requests)
	}
	if len(m.latencies) > 0 {
		sorted := make([]time.Duration, len(m.latencies))
		copy(sorted, m.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		s.P50ms = ms(sorted[len(sorted)*50/100])
		s.P99ms = ms(sorted[min(len(sorted)*99/100, len(sorted)-1)])
	}
	return s
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

