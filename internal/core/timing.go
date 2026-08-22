package core

import "time"

// TimingBoundary documents the canonical timing boundaries enforced across DAEGSA (§7, §8).
//
// Latency Contract:
//   - DispatchedAt: recorded immediately before the HTTP transport RoundTrip begins.
//   - HeadersReceivedAt: recorded when RoundTrip returns the initial *http.Response header block.
//   - BodyCompletedAt: recorded when the response body is read to completion (or up to response_body_limit)
//     and the response body is closed, or when a transport error occurs.
//   - Latency = BodyCompletedAt - DispatchedAt (dispatch-to-body-consumed).
//   - Time to First Byte (TTFB) = HeadersReceivedAt - DispatchedAt.
//   - Scheduler Lag = ScheduledAt - PlannedAt (in open model).
type TimingBoundary struct{}

// RequestTimestamps captures precise monotonic timestamps for a single request execution lifecycle.
type RequestTimestamps struct {
	// PlannedAt is the target arrival timestamp calculated by the open scheduler.
	PlannedAt time.Time `json:"planned_at"`

	// ScheduledAt is the time when the request was dequeued or dispatched into an execution lane.
	ScheduledAt time.Time `json:"scheduled_at"`

	// DispatchedAt is the time immediately before invoking the HTTP transport RoundTrip.
	DispatchedAt time.Time `json:"dispatched_at"`

	// HeadersReceivedAt is the time when the initial response headers were received from the server.
	HeadersReceivedAt time.Time `json:"headers_received_at"`

	// BodyCompletedAt is the time when response body consumption finished or transport error occurred.
	BodyCompletedAt time.Time `json:"body_completed_at"`
}

// Latency calculates the total request latency (dispatch-to-body-consumed).
// Returns 0 if DispatchedAt or BodyCompletedAt is zero or if BodyCompletedAt is before DispatchedAt.
func (t RequestTimestamps) Latency() time.Duration {
	if t.DispatchedAt.IsZero() || t.BodyCompletedAt.IsZero() {
		return 0
	}
	if t.BodyCompletedAt.Before(t.DispatchedAt) {
		return 0
	}
	return t.BodyCompletedAt.Sub(t.DispatchedAt)
}

// TTFB calculates the time to first byte (time between dispatch and header receipt).
func (t RequestTimestamps) TTFB() time.Duration {
	if t.DispatchedAt.IsZero() || t.HeadersReceivedAt.IsZero() {
		return 0
	}
	if t.HeadersReceivedAt.Before(t.DispatchedAt) {
		return 0
	}
	return t.HeadersReceivedAt.Sub(t.DispatchedAt)
}

// ScheduleLag calculates the delay between the planned open arrival tick and actual scheduling.
func (t RequestTimestamps) ScheduleLag() time.Duration {
	if t.PlannedAt.IsZero() || t.ScheduledAt.IsZero() {
		return 0
	}
	if t.ScheduledAt.Before(t.PlannedAt) {
		return 0
	}
	return t.ScheduledAt.Sub(t.PlannedAt)
}

// TotalDuration calculates the entire lifecycle duration from planned tick to completion.
func (t RequestTimestamps) TotalDuration() time.Duration {
	start := t.PlannedAt
	if start.IsZero() {
		start = t.DispatchedAt
	}
	if start.IsZero() || t.BodyCompletedAt.IsZero() {
		return 0
	}
	if t.BodyCompletedAt.Before(start) {
		return 0
	}
	return t.BodyCompletedAt.Sub(start)
}
