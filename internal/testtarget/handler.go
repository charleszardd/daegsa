package testtarget

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *TargetServer) buildHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/cookies/set", s.handleCookiesSet)
	mux.HandleFunc("/cookies/inspect", s.handleCookiesInspect)
	mux.HandleFunc("/auth/bearer", s.handleAuthBearer)
	mux.HandleFunc("/auth/header", s.handleAuthHeader)
	mux.HandleFunc("/auth/basic", s.handleAuthBasic)
	mux.HandleFunc("/auth/token-pool", s.handleAuthTokenPool)
	mux.HandleFunc("/", s.handleGeneralRequest)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.recordRequest(r)

		// Check rate limiter if enabled
		if s.rateLimiter != nil {
			allowed, headers := s.rateLimiter.Check()
			for k, v := range headers {
				for _, val := range v {
					w.Header().Add(k, val)
				}
			}
			if !allowed {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error": "rate limited"}`))
				return
			}
		}

		mux.ServeHTTP(w, r)
	})
}

func (s *TargetServer) handleGeneralRequest(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// 1. Check for Hang mode (?hang=true)
	if q.Get("hang") == "true" {
		<-r.Context().Done()
		return
	}

	// 2. Check for Immediate Abrupt Drop (?drop=immediate)
	if q.Get("drop") == "immediate" {
		hijacker, ok := w.(http.Hijacker)
		if ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
	}

	// 3. Check for Midway Abrupt Drop (?drop=midway&after_bytes=N)
	if q.Get("drop") == "midway" {
		afterBytes := 100
		if bStr := q.Get("after_bytes"); bStr != "" {
			if b, err := strconv.Atoi(bStr); err == nil && b >= 0 {
				afterBytes = b
			}
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		// Write partial bytes
		chunk := make([]byte, afterBytes)
		for i := range chunk {
			chunk[i] = byte('A' + (i % 26))
		}
		_, _ = w.Write(chunk)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		// Abruptly hijack and close connection
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
		return
	}

	// 4. Check for Delays (?delay=50ms or X-Target-Delay: 50ms)
	delayStr := q.Get("delay")
	if delayStr == "" {
		delayStr = r.Header.Get("X-Target-Delay")
	}
	if delayStr != "" {
		if d, err := time.ParseDuration(delayStr); err == nil && d > 0 {
			s.clock.Sleep(d)
		}
	}

	// 5. Check for Cross-Origin Redirect (?redirect_url=http://...)
	if redirectURL := q.Get("redirect_url"); redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// 6. Check for Same-Origin Multi-Hop Redirect (?redirect_path=/dest&hops=K)
	if redirectPath := q.Get("redirect_path"); redirectPath != "" {
		hops := 0
		if hopsStr := q.Get("hops"); hopsStr != "" {
			hops, _ = strconv.Atoi(hopsStr)
		}
		if hops > 1 {
			nextURL := fmt.Sprintf("/redirect?redirect_path=%s&hops=%d", redirectPath, hops-1)
			http.Redirect(w, r, nextURL, http.StatusFound)
			return
		}
		http.Redirect(w, r, redirectPath, http.StatusFound)
		return
	}

	// 7. Check for Status Code (?status=NNN or X-Target-Status: NNN)
	statusCode := http.StatusOK
	statusStr := q.Get("status")
	if statusStr == "" {
		statusStr = r.Header.Get("X-Target-Status")
	}
	if statusStr != "" {
		if code, err := strconv.Atoi(statusStr); err == nil && code >= 100 && code <= 599 {
			statusCode = code
		}
	}

	// 8. Check for Byte Payload Sizing (?bytes=N)
	if bytesStr := q.Get("bytes"); bytesStr != "" {
		totalBytes, err := strconv.ParseInt(bytesStr, 10, 64)
		if err == nil && totalBytes >= 0 {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.FormatInt(totalBytes, 10))
			w.WriteHeader(statusCode)

			// Stream chunk by chunk (bounded memory)
			const chunkSize = 4096
			chunk := make([]byte, chunkSize)
			for i := range chunk {
				chunk[i] = byte('A' + (i % 26))
			}

			var written int64
			for written < totalBytes {
				toWrite := int64(chunkSize)
				if totalBytes-written < toWrite {
					toWrite = totalBytes - written
				}
				n, wErr := w.Write(chunk[:toWrite])
				written += int64(n)
				if wErr != nil {
					break
				}
			}
			return
		}
	}

	// Default response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"status":%d,"message":"ok"}`, statusCode)))
}

func (s *TargetServer) handleCookiesSet(w http.ResponseWriter, r *http.Request) {
	for k, vals := range r.URL.Query() {
		for _, v := range vals {
			http.SetCookie(w, &http.Cookie{
				Name:  k,
				Value: v,
				Path:  "/",
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"cookies_set":true}`))
}

func (s *TargetServer) handleCookiesInspect(w http.ResponseWriter, r *http.Request) {
	cookieMap := make(map[string]string)
	for _, c := range r.Cookies() {
		cookieMap[c.Name] = c.Value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cookieMap)
}

func (s *TargetServer) handleAuthBearer(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") && len(strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated":true,"mode":"bearer"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","expected":"Bearer <token>"}`))
}

func (s *TargetServer) handleAuthHeader(w http.ResponseWriter, r *http.Request) {
	headerKey := r.URL.Query().Get("header_name")
	if headerKey == "" {
		headerKey = "X-API-Key"
	}
	val := r.Header.Get(headerKey)
	if val == "" {
		val = r.Header.Get("X-Auth-Token")
	}
	if val == "" {
		val = r.Header.Get("X-API-Key")
	}
	if val != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"authenticated":true,"mode":"custom_header"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","expected_header":"custom header"}`))
}

func (s *TargetServer) handleAuthBasic(w http.ResponseWriter, r *http.Request) {
	user, _, ok := r.BasicAuth()
	if ok && user != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"authenticated":true,"mode":"basic","user":%q}`, user)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","expected":"Basic auth credentials"}`))
}

func (s *TargetServer) handleAuthTokenPool(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	tok := ""
	if strings.HasPrefix(authHeader, "Bearer ") {
		tok = strings.TrimPrefix(authHeader, "Bearer ")
	} else if authHeader != "" {
		tok = authHeader
	} else {
		tok = r.Header.Get("X-API-Key")
		if tok == "" {
			tok = r.Header.Get("X-API-Token")
		}
		if tok == "" {
			tok = r.Header.Get("X-Auth-Token")
		}
	}

	if tok != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		tokenHash := sha256.Sum256([]byte(tok))
		_, _ = w.Write([]byte(fmt.Sprintf(`{"authenticated":true,"token_hash":%q}`, fmt.Sprintf("%x", tokenHash[:8]))))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","message":"missing token"}`))
}
