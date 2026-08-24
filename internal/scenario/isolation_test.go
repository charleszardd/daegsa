package scenario_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestScenarioIsolation_ConcurrentVUs(t *testing.T) {
	const numVUs = 10
	const iterationsPerVU = 5

	// Mock server that returns VU-specific tokens and validates that Step 2 receives the exact matching VU token
	var mu sync.Mutex
	issuedTokens := make(map[string]int) // token -> vuID

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			vuStr := r.URL.Query().Get("vu")
			token := fmt.Sprintf("token_for_vu_%s_%d", vuStr, time.Now().UnixNano())

			mu.Lock()
			issuedTokens[token] = 1
			mu.Unlock()

			http.SetCookie(w, &http.Cookie{
				Name:  "vu_cookie",
				Value: fmt.Sprintf("cookie_for_vu_%s", vuStr),
				Path:  "/",
			})

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token": token,
			})

		case "/validate":
			authHdr := r.Header.Get("Authorization")
			expectedVU := r.URL.Query().Get("vu")
			expectedTokenPrefix := fmt.Sprintf("Bearer token_for_vu_%s_", expectedVU)

			if len(authHdr) < len(expectedTokenPrefix) || authHdr[:len(expectedTokenPrefix)] != expectedTokenPrefix {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"token mismatch across VUs"}`))
				return
			}

			cookie, err := r.Cookie("vu_cookie")
			expectedCookie := fmt.Sprintf("cookie_for_vu_%s", expectedVU)
			if err != nil || cookie == nil || cookie.Value != expectedCookie {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"cookie mismatch across VUs"}`))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"valid":true}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	sc := &scenario.CompiledScenario{
		Name: "isolation_check",
		Steps: []*scenario.CompiledStep{
			{
				Name:             "login",
				URL:              ts.URL + "/login?vu=${vu_id}",
				Method:           "POST",
				ExpectedStatuses: []int{200},
				ExtractRules: map[string]scenario.ExtractionRule{
					"extracted_token": {
						From:       scenario.SourceJSON,
						Expression: "token",
					},
					"extracted_cookie": {
						From:       scenario.SourceCookie,
						Expression: "vu_cookie",
					},
				},
				OnFailure: scenario.OnFailureStop,
			},
			{
				Name:             "validate",
				URL:              ts.URL + "/validate?vu=${vu_id}",
				Method:           "GET",
				Headers:          http.Header{"Authorization": []string{"Bearer ${extracted_token}"}},
				ExpectedStatuses: []int{200},
				OnFailure:        scenario.OnFailureStop,
			},
		},
	}

	exec := scenario.NewScenarioExecutor(sc, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())

	var wg sync.WaitGroup
	errorsChan := make(chan error, numVUs*iterationsPerVU)

	for vu := 0; vu < numVUs; vu++ {
		wg.Add(1)
		go func(vuID int) {
			defer wg.Done()

			jar, err := cookiejar.New(nil)
			if err != nil {
				errorsChan <- err
				return
			}

			initialVars := map[string]string{
				"vu_id": fmt.Sprintf("%d", vuID),
			}
			state := scenario.NewVUState(vuID, jar, initialVars)

			for iter := 0; iter < iterationsPerVU; iter++ {
				success, execErr := exec.ExecuteIteration(context.Background(), state, nil)
				if execErr != nil {
					errorsChan <- fmt.Errorf("VU %d iteration %d error: %w", vuID, iter, execErr)
					return
				}
				if !success {
					errorsChan <- fmt.Errorf("VU %d iteration %d failed validation", vuID, iter)
					return
				}

				// Verify state variables are strictly this VU's
				expectedCookie := fmt.Sprintf("cookie_for_vu_%d", vuID)
				if state.Variables["extracted_cookie"] != expectedCookie {
					errorsChan <- fmt.Errorf("VU %d has leaked cookie from another VU: %q", vuID, state.Variables["extracted_cookie"])
					return
				}
			}
		}(vu)
	}

	wg.Wait()
	close(errorsChan)

	for err := range errorsChan {
		t.Errorf("Isolation violation: %v", err)
	}
}
