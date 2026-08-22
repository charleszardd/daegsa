package executor

import (
	"bytes"
	"io"
	"net/http"
)

// Safe drain limit for keep-alive connection reuse (§8).
const safeDrainLimitBytes = 32 * 1024 // 32 KiB

// ReadAndDrainResponseBody reads response payload up to limitBytes, detects whether the body was truncated,
// safely drains remaining bytes up to a safe threshold to preserve keep-alive connection reuse, and closes the body (§8).
func ReadAndDrainResponseBody(resp *http.Response, limitBytes int64) ([]byte, int64, bool, error) {
	if resp == nil || resp.Body == nil {
		return nil, 0, false, nil
	}
	defer resp.Body.Close()

	if limitBytes <= 0 {
		limitBytes = 1024 * 1024 // default 1 MiB
	}

	var buf bytes.Buffer
	limitedReader := io.LimitReader(resp.Body, limitBytes)

	n, err := buf.ReadFrom(limitedReader)
	bytesReceived := n
	if err != nil {
		return buf.Bytes(), bytesReceived, false, err
	}

	truncated := false
	if n == limitBytes {
		// Probe 1 additional byte to see if there is more data
		probeBuf := make([]byte, 1)
		probeN, probeErr := resp.Body.Read(probeBuf)
		if probeN > 0 {
			truncated = true
			bytesReceived += int64(probeN)
		}
		if probeErr != nil && probeErr != io.EOF {
			return buf.Bytes(), bytesReceived, truncated, probeErr
		}
	}

	// Drain remaining up to safeDrainLimitBytes
	if truncated {
		drained, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, safeDrainLimitBytes))
		bytesReceived += drained
	}

	return buf.Bytes(), bytesReceived, truncated, nil
}
