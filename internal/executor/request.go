package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/charleszardd/daegsa/internal/plan"
)

// BuildHTTPRequest constructs an *http.Request from an immutable execution plan and computes
// estimated wire bytes sent (§8).
func BuildHTTPRequest(ctx context.Context, p *plan.Plan) (*http.Request, int64, error) {
	if p == nil {
		return nil, 0, fmt.Errorf("plan cannot be nil")
	}

	var bodyReader io.Reader
	bodyLen := int64(0)
	if len(p.Body) > 0 {
		bodyReader = bytes.NewReader(p.Body)
		bodyLen = int64(len(p.Body))
	}

	req, err := http.NewRequestWithContext(ctx, p.Method, p.TargetURL.String(), bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create http request: %w", err)
	}

	// Copy headers
	for k, vals := range p.Headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}

	// Host header override if explicitly set in headers
	if hostVal := p.Headers.Get("Host"); hostVal != "" {
		req.Host = hostVal
	}

	// Estimate bytes sent (Request Line + Headers + Body)
	bytesSent := estimateRequestBytes(req, bodyLen)

	return req, bytesSent, nil
}

func estimateRequestBytes(req *http.Request, bodyLen int64) int64 {
	// Request line: METHOD /path HTTP/1.1\r\n
	uri := req.URL.RequestURI()
	if uri == "" {
		uri = "/"
	}
	lineLen := len(req.Method) + 1 + len(uri) + len(" HTTP/1.1\r\n")

	// Host header
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headerBytes := len("Host: ") + len(host) + len("\r\n")

	// Additional headers
	for k, vals := range req.Header {
		for _, v := range vals {
			headerBytes += len(k) + len(": ") + len(v) + len("\r\n")
		}
	}
	headerBytes += len("\r\n") // end of headers

	return int64(lineLen + headerBytes) + bodyLen
}
