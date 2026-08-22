package core

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
)

// Outcome represents the canonical terminal classification for every scheduled request (§9).
type Outcome string

const (
	// OutcomeSuccess indicates the request completed and returned an expected HTTP status code.
	OutcomeSuccess Outcome = "success"

	// OutcomeUnexpectedStatus indicates the request completed with an HTTP status code not in expected_statuses.
	OutcomeUnexpectedStatus Outcome = "unexpected_status"

	// OutcomeRateLimited indicates the server responded with HTTP 429 Too Many Requests.
	OutcomeRateLimited Outcome = "rate_limited"

	// OutcomeTimeout indicates the request failed due to deadline exceeded or client/network timeout.
	OutcomeTimeout Outcome = "timeout"

	// OutcomeDNSError indicates host lookup or name resolution failed.
	OutcomeDNSError Outcome = "dns_error"

	// OutcomeConnectError indicates TCP/socket connection establishment failed (e.g. connection refused).
	OutcomeConnectError Outcome = "connect_error"

	// OutcomeTLSError indicates a TLS handshake, certificate validation, or record error.
	OutcomeTLSError Outcome = "tls_error"

	// OutcomeRequestBuildError indicates request construction, URL parsing, or header assembly failed.
	OutcomeRequestBuildError Outcome = "request_build_error"

	// OutcomeResponseBodyError indicates an error occurred while reading the response body up to the limit.
	OutcomeResponseBodyError Outcome = "response_body_error"

	// OutcomeCanceled indicates the request was explicitly canceled by test shutdown or context abort.
	OutcomeCanceled Outcome = "canceled"

	// OutcomeOtherTransportError indicates an unclassified network or transport failure.
	OutcomeOtherTransportError Outcome = "other_transport_error"

	// OutcomeDropped indicates scheduled open-model work was dropped because max_in_flight was saturated.
	OutcomeDropped Outcome = "dropped"
)

// AllOutcomes contains all 12 canonical outcome values in deterministic order.
var AllOutcomes = []Outcome{
	OutcomeSuccess,
	OutcomeUnexpectedStatus,
	OutcomeRateLimited,
	OutcomeTimeout,
	OutcomeDNSError,
	OutcomeConnectError,
	OutcomeTLSError,
	OutcomeRequestBuildError,
	OutcomeResponseBodyError,
	OutcomeCanceled,
	OutcomeOtherTransportError,
	OutcomeDropped,
}

// IsValid reports whether o is one of the 12 canonical outcomes.
func (o Outcome) IsValid() bool {
	switch o {
	case OutcomeSuccess,
		OutcomeUnexpectedStatus,
		OutcomeRateLimited,
		OutcomeTimeout,
		OutcomeDNSError,
		OutcomeConnectError,
		OutcomeTLSError,
		OutcomeRequestBuildError,
		OutcomeResponseBodyError,
		OutcomeCanceled,
		OutcomeOtherTransportError,
		OutcomeDropped:
		return true
	default:
		return false
	}
}

// String returns the string representation of the outcome.
func (o Outcome) String() string {
	return string(o)
}

// IsSuccess reports whether the outcome represents an expected HTTP success.
func (o Outcome) IsSuccess() bool {
	return o == OutcomeSuccess
}

// IsHTTPResponse reports whether the outcome represents a completed HTTP response from the target.
func (o Outcome) IsHTTPResponse() bool {
	return o == OutcomeSuccess || o == OutcomeUnexpectedStatus || o == OutcomeRateLimited
}

// IsTransportFailure reports whether the outcome represents a network or transport-level failure.
func (o Outcome) IsTransportFailure() bool {
	switch o {
	case OutcomeTimeout,
		OutcomeDNSError,
		OutcomeConnectError,
		OutcomeTLSError,
		OutcomeResponseBodyError,
		OutcomeOtherTransportError:
		return true
	default:
		return false
	}
}

// IsClientDrop reports whether the outcome represents work dropped by the generator before dispatch.
func (o Outcome) IsClientDrop() bool {
	return o == OutcomeDropped
}

// IsCanceled reports whether the outcome was canceled.
func (o Outcome) IsCanceled() bool {
	return o == OutcomeCanceled
}

// ClassifyInput holds all parameters necessary to deterministically classify a request outcome.
type ClassifyInput struct {
	StatusCode       int
	ExpectedStatuses []int
	Err              error
	Canceled         bool
	Dropped          bool
	ResponseBodyErr  bool
	RequestBuildErr  bool
}

// OutcomeClassifier classifies request results into one of the 12 canonical outcomes.
type OutcomeClassifier struct{}

// NewOutcomeClassifier returns a ready-to-use OutcomeClassifier.
func NewOutcomeClassifier() OutcomeClassifier {
	return OutcomeClassifier{}
}

// Classify deterministically classifies a request result into an Outcome.
// It is designed to be allocation-free on the happy path and for standard errors.
func (OutcomeClassifier) Classify(input ClassifyInput) Outcome {
	if input.Dropped {
		return OutcomeDropped
	}

	if input.Canceled {
		return OutcomeCanceled
	}

	if input.RequestBuildErr {
		return OutcomeRequestBuildError
	}

	if input.ResponseBodyErr {
		return OutcomeResponseBodyError
	}

	if input.Err != nil {
		return classifyError(input.Err)
	}

	// No error: evaluate HTTP status code
	if input.StatusCode == http.StatusTooManyRequests {
		return OutcomeRateLimited
	}

	if isStatusExpected(input.StatusCode, input.ExpectedStatuses) {
		return OutcomeSuccess
	}

	return OutcomeUnexpectedStatus
}

// isStatusExpected checks whether statusCode is in expectedStatuses.
// If expectedStatuses is empty, default expected status is 200 OK.
func isStatusExpected(statusCode int, expectedStatuses []int) bool {
	if len(expectedStatuses) == 0 {
		return statusCode == http.StatusOK
	}
	for _, expected := range expectedStatuses {
		if statusCode == expected {
			return true
		}
	}
	return false
}

// classifyError inspects Go standard library errors to categorize transport failures.
func classifyError(err error) Outcome {
	if err == nil {
		return OutcomeSuccess
	}

	// Unwrap url.Error if present
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}

	// Context cancellation and deadlines
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, syscall.ETIMEDOUT) {
		return OutcomeTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return OutcomeTimeout
	}

	// Check for DNS error
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return OutcomeDNSError
	}

	// Check for TLS errors
	var certErr *tls.CertificateVerificationError
	var recordErr tls.RecordHeaderError
	if errors.As(err, &certErr) || errors.As(err, &recordErr) {
		return OutcomeTLSError
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "tls:") ||
		strings.Contains(errMsg, "certificate") ||
		strings.Contains(errMsg, "handshake failure") {
		return OutcomeTLSError
	}

	// Check for connection / dial errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" {
			return OutcomeConnectError
		}
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			if errors.Is(sysErr.Err, syscall.ECONNREFUSED) {
				return OutcomeConnectError
			}
		}
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return OutcomeConnectError
		}
	}

	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connect: connection refused") ||
		strings.Contains(errMsg, "No connection could be made") {
		return OutcomeConnectError
	}

	// Check for cancellation
	if errors.Is(err, context.Canceled) || errors.Is(err, os.ErrClosed) || strings.Contains(errMsg, "context canceled") {
		return OutcomeCanceled
	}

	return OutcomeOtherTransportError
}
