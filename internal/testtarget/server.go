package testtarget

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
)

// Default maximum recorded requests in memory to prevent memory exhaustion (§14, §15).
const maxRecordedRequests = 10000

// RecordedRequest captures full metadata of an incoming HTTP request for test assertions.
type RecordedRequest struct {
	Method     string      `json:"method"`
	URL        string      `json:"url"`
	Path       string      `json:"path"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
	RemoteAddr string      `json:"remote_addr"`
	Time       time.Time   `json:"time"`
}

// Option configures the TargetServer.
type Option func(*TargetServer)

// WithRateLimiter attaches a RateLimiter to the TargetServer.
func WithRateLimiter(rl *RateLimiter) Option {
	return func(s *TargetServer) {
		s.rateLimiter = rl
	}
}

// WithClock attaches a custom Clock for deterministic delays and rate limits.
func WithClock(c clock.Clock) Option {
	return func(s *TargetServer) {
		s.clock = c
	}
}

// TargetServer wraps httptest.Server to simulate real API target behavior (§8, §14, §15).
type TargetServer struct {
	server       *httptest.Server
	rateLimiter  *RateLimiter
	clock        clock.Clock
	mu           sync.RWMutex
	recordedReqs []RecordedRequest
}

// NewServer starts a new loopback TargetServer with the provided options.
func NewServer(options ...Option) *TargetServer {
	ts := &TargetServer{
		clock:        clock.NewRealClock(),
		recordedReqs: make([]RecordedRequest, 0, 128),
	}

	for _, opt := range options {
		opt(ts)
	}

	if ts.rateLimiter != nil && ts.rateLimiter.clock == nil {
		ts.rateLimiter.clock = ts.clock
	}

	handler := ts.buildHandler()
	ts.server = httptest.NewServer(handler)

	return ts
}

// URL returns the base URL of the running test server (e.g. "http://127.0.0.1:54321").
func (s *TargetServer) URL() string {
	return s.server.URL
}

// Close gracefully stops the test server and cleans up resources.
func (s *TargetServer) Close() {
	s.server.Close()
}

// RecordedRequests returns a copy of all recorded requests received by the server.
func (s *TargetServer) RecordedRequests() []RecordedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	copied := make([]RecordedRequest, len(s.recordedReqs))
	copy(copied, s.recordedReqs)
	return copied
}

// ResetRecordedRequests clears all recorded requests from memory.
func (s *TargetServer) ResetRecordedRequests() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordedReqs = s.recordedReqs[:0]
}

func (s *TargetServer) recordRequest(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.recordedReqs) >= maxRecordedRequests {
		return // bounded memory safety
	}

	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	s.recordedReqs = append(s.recordedReqs, RecordedRequest{
		Method:     r.Method,
		URL:        r.URL.String(),
		Path:       r.URL.Path,
		Header:     r.Header.Clone(),
		Body:       bodyBytes,
		RemoteAddr: r.RemoteAddr,
		Time:       s.clock.Now(),
	})
}
