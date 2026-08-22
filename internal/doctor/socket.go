package doctor

import (
	"context"
	"fmt"
	"net"
	"time"
)

const (
	testSocketPairs = 10
)

// CheckSocketLimits verifies TCP socket allocation, ephemeral port availability, and loopback connectivity (§14).
func CheckSocketLimits(ctx context.Context) CheckResult {
	start := time.Now()

	// 1. Create a local test listener on an ephemeral port
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return CheckResult{
			Name:       "Socket Allocation & Limits",
			Category:   CategorySocket,
			Status:     StatusFail,
			Summary:    "Failed to bind TCP loopback listener",
			Detail:     err.Error(),
			Suggestion: "Check firewall or security software blocking loopback socket binding.",
			Duration:   time.Since(start),
		}
	}
	defer ln.Close()

	addr := ln.Addr().String()

	// 2. Concurrently/rapidly create test connection pairs to verify ephemeral port allocation
	type connPair struct {
		client net.Conn
		server net.Conn
	}
	pairs := make([]connPair, 0, testSocketPairs)
	defer func() {
		for _, p := range pairs {
			if p.client != nil {
				_ = p.client.Close()
			}
			if p.server != nil {
				_ = p.server.Close()
			}
		}
	}()

	errCh := make(chan error, testSocketPairs)

	for i := 0; i < testSocketPairs; i++ {
		if ctx.Err() != nil {
			return CheckResult{
				Name:     "Socket Allocation & Limits",
				Category: CategorySocket,
				Status:   StatusFail,
				Summary:  "Socket check canceled",
				Detail:   ctx.Err().Error(),
				Duration: time.Since(start),
			}
		}

		serverAccepted := make(chan net.Conn, 1)
		go func() {
			sConn, sErr := ln.Accept()
			if sErr != nil {
				errCh <- sErr
				return
			}
			serverAccepted <- sConn
		}()

		var d net.Dialer
		cConn, cErr := d.DialContext(ctx, "tcp", addr)
		if cErr != nil {
			return CheckResult{
				Name:       "Socket Allocation & Limits",
				Category:   CategorySocket,
				Status:     StatusFail,
				Summary:    fmt.Sprintf("Failed to dial loopback connection %d/%d", i+1, testSocketPairs),
				Detail:     cErr.Error(),
				Suggestion: "Inspect ephemeral port pool exhaustion or system socket limits.",
				Duration:   time.Since(start),
			}
		}

		var sConn net.Conn
		select {
		case sConn = <-serverAccepted:
		case <-time.After(1 * time.Second):
			_ = cConn.Close()
			return CheckResult{
				Name:       "Socket Allocation & Limits",
				Category:   CategorySocket,
				Status:     StatusFail,
				Summary:    "Timed out accepting loopback connection",
				Detail:     "Accept took longer than 1s on loopback",
				Suggestion: "Check system load or antivirus inspection of local TCP traffic.",
				Duration:   time.Since(start),
			}
		}

		pairs = append(pairs, connPair{client: cConn, server: sConn})
	}

	elapsed := time.Since(start)
	detail := fmt.Sprintf("Allocated %d loopback TCP socket pairs successfully in %v", testSocketPairs, elapsed.Truncate(time.Microsecond))

	return CheckResult{
		Name:     "Socket Allocation & Limits",
		Category: CategorySocket,
		Status:   StatusPass,
		Summary:  fmt.Sprintf("Ephemeral ports & loopback TCP functional (%d pairs tested)", testSocketPairs),
		Detail:   detail,
		Duration: elapsed,
	}
}
