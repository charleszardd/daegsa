package core_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestAllOutcomes_CountAndValidity(t *testing.T) {
	if len(core.AllOutcomes) != 12 {
		t.Fatalf("expected 12 canonical outcomes, got %d", len(core.AllOutcomes))
	}

	seen := make(map[core.Outcome]bool)
	for _, o := range core.AllOutcomes {
		if !o.IsValid() {
			t.Errorf("outcome %q reports IsValid() == false", o)
		}
		if seen[o] {
			t.Errorf("duplicate outcome found in AllOutcomes: %s", o)
		}
		seen[o] = true
	}
}

func TestOutcome_ClassificationHelpers(t *testing.T) {
	tests := []struct {
		outcome            core.Outcome
		isSuccess          bool
		isHTTPResponse     bool
		isTransportFailure bool
		isClientDrop       bool
		isCanceled         bool
	}{
		{
			outcome:            core.OutcomeSuccess,
			isSuccess:          true,
			isHTTPResponse:     true,
			isTransportFailure: false,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeUnexpectedStatus,
			isSuccess:          false,
			isHTTPResponse:     true,
			isTransportFailure: false,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeRateLimited,
			isSuccess:          false,
			isHTTPResponse:     true,
			isTransportFailure: false,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeTimeout,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeDNSError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeConnectError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeTLSError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeRequestBuildError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: false,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeResponseBodyError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeCanceled,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: false,
			isClientDrop:       false,
			isCanceled:         true,
		},
		{
			outcome:            core.OutcomeOtherTransportError,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: true,
			isClientDrop:       false,
			isCanceled:         false,
		},
		{
			outcome:            core.OutcomeDropped,
			isSuccess:          false,
			isHTTPResponse:     false,
			isTransportFailure: false,
			isClientDrop:       true,
			isCanceled:         false,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.outcome), func(t *testing.T) {
			if got := tt.outcome.IsSuccess(); got != tt.isSuccess {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.isSuccess)
			}
			if got := tt.outcome.IsHTTPResponse(); got != tt.isHTTPResponse {
				t.Errorf("IsHTTPResponse() = %v, want %v", got, tt.isHTTPResponse)
			}
			if got := tt.outcome.IsTransportFailure(); got != tt.isTransportFailure {
				t.Errorf("IsTransportFailure() = %v, want %v", got, tt.isTransportFailure)
			}
			if got := tt.outcome.IsClientDrop(); got != tt.isClientDrop {
				t.Errorf("IsClientDrop() = %v, want %v", got, tt.isClientDrop)
			}
			if got := tt.outcome.IsCanceled(); got != tt.isCanceled {
				t.Errorf("IsCanceled() = %v, want %v", got, tt.isCanceled)
			}
		})
	}
}

func TestOutcomeClassifier_Classify(t *testing.T) {
	classifier := core.NewOutcomeClassifier()

	tests := []struct {
		name     string
		input    core.ClassifyInput
		expected core.Outcome
	}{
		{
			name: "dropped request produces OutcomeDropped",
			input: core.ClassifyInput{
				Dropped: true,
			},
			expected: core.OutcomeDropped,
		},
		{
			name: "explicitly canceled request produces OutcomeCanceled",
			input: core.ClassifyInput{
				Canceled: true,
			},
			expected: core.OutcomeCanceled,
		},
		{
			name: "context canceled error produces OutcomeCanceled",
			input: core.ClassifyInput{
				Err: context.Canceled,
			},
			expected: core.OutcomeCanceled,
		},
		{
			name: "context deadline exceeded produces OutcomeTimeout",
			input: core.ClassifyInput{
				Err: context.DeadlineExceeded,
			},
			expected: core.OutcomeTimeout,
		},
		{
			name: "os deadline exceeded produces OutcomeTimeout",
			input: core.ClassifyInput{
				Err: os.ErrDeadlineExceeded,
			},
			expected: core.OutcomeTimeout,
		},
		{
			name: "dns error produces OutcomeDNSError",
			input: core.ClassifyInput{
				Err: &net.DNSError{Name: "unknown.invalid", IsNotFound: true},
			},
			expected: core.OutcomeDNSError,
		},
		{
			name: "connect refused error produces OutcomeConnectError",
			input: core.ClassifyInput{
				Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			},
			expected: core.OutcomeConnectError,
		},
		{
			name: "tls certificate error produces OutcomeTLSError",
			input: core.ClassifyInput{
				Err: &tls.CertificateVerificationError{},
			},
			expected: core.OutcomeTLSError,
		},
		{
			name: "request build error produces OutcomeRequestBuildError",
			input: core.ClassifyInput{
				RequestBuildErr: true,
			},
			expected: core.OutcomeRequestBuildError,
		},
		{
			name: "response body error produces OutcomeResponseBodyError",
			input: core.ClassifyInput{
				ResponseBodyErr: true,
			},
			expected: core.OutcomeResponseBodyError,
		},
		{
			name: "unclassified network error produces OutcomeOtherTransportError",
			input: core.ClassifyInput{
				Err: errors.New("abrupt transport protocol reset"),
			},
			expected: core.OutcomeOtherTransportError,
		},
		{
			name: "HTTP 200 with default expected statuses produces OutcomeSuccess",
			input: core.ClassifyInput{
				StatusCode: http.StatusOK,
			},
			expected: core.OutcomeSuccess,
		},
		{
			name: "HTTP 404 with expected [200] produces OutcomeUnexpectedStatus",
			input: core.ClassifyInput{
				StatusCode:       http.StatusNotFound,
				ExpectedStatuses: []int{http.StatusOK},
			},
			expected: core.OutcomeUnexpectedStatus,
		},
		{
			name: "HTTP 404 with expected [404] produces OutcomeSuccess",
			input: core.ClassifyInput{
				StatusCode:       http.StatusNotFound,
				ExpectedStatuses: []int{http.StatusNotFound},
			},
			expected: core.OutcomeSuccess,
		},
		{
			name: "HTTP 429 produces OutcomeRateLimited regardless of expected statuses",
			input: core.ClassifyInput{
				StatusCode:       http.StatusTooManyRequests,
				ExpectedStatuses: []int{http.StatusOK, http.StatusTooManyRequests},
			},
			expected: core.OutcomeRateLimited,
		},
		{
			name: "HTTP 500 with expected [200, 201] produces OutcomeUnexpectedStatus",
			input: core.ClassifyInput{
				StatusCode:       http.StatusInternalServerError,
				ExpectedStatuses: []int{http.StatusOK, http.StatusCreated},
			},
			expected: core.OutcomeUnexpectedStatus,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.Classify(tt.input)
			if got != tt.expected {
				t.Errorf("Classify() = %q, want %q", got, tt.expected)
			}
		})
	}
}
