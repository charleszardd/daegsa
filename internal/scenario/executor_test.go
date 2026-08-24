package scenario_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestScenarioExecutor_ChainingAndExtraction(t *testing.T) {
	// Setup test server
	var loginCount, itemsCount, logoutCount int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			loginCount++
			http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "sess_secret_789", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token":   "tok_jwt_abc123",
				"user_id": "user_42",
			})

		case "/api/items":
			itemsCount++
			authHdr := r.Header.Get("Authorization")
			cookie, _ := r.Cookie("session_id")

			if authHdr != "Bearer tok_jwt_abc123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if cookie == nil || cookie.Value != "sess_secret_789" {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"items": []map[string]string{
					{"id": "item-1", "name": "Item One"},
					{"id": "item-2", "name": "Item Two"},
				},
			})

		case "/api/logout":
			logoutCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	sc := &scenario.CompiledScenario{
		Name: "test_chaining",
		Steps: []*scenario.CompiledStep{
			{
				Name:             "login",
				URL:              ts.URL + "/auth/login",
				Method:           "POST",
				ExpectedStatuses: []int{200},
				Timeout:          5 * time.Second,
				ExtractRules: map[string]scenario.ExtractionRule{
					"token": {
						From:       scenario.SourceJSON,
						Expression: "token",
					},
					"user_id": {
						From:       scenario.SourceJSON,
						Expression: "user_id",
					},
				},
				OnFailure: scenario.OnFailureStop,
			},
			{
				Name:             "get_items",
				URL:              ts.URL + "/api/items",
				Method:           "GET",
				Headers:          http.Header{"Authorization": []string{"Bearer ${token}"}},
				ExpectedStatuses: []int{200},
				Timeout:          5 * time.Second,
				ExtractRules: map[string]scenario.ExtractionRule{
					"first_item": {
						From:       scenario.SourceJSONPath,
						Expression: "$.items[0].id",
					},
				},
				OnFailure: scenario.OnFailureStop,
			},
			{
				Name:             "logout",
				URL:              ts.URL + "/api/logout",
				Method:           "POST",
				ExpectedStatuses: []int{200},
				Timeout:          5 * time.Second,
				OnFailure:        scenario.OnFailureStop,
			},
		},
	}

	exec := scenario.NewScenarioExecutor(sc, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}
	state := scenario.NewVUState(0, jar, nil)

	var executedSteps []string
	onStepDone := func(res *scenario.StepResult) {
		executedSteps = append(executedSteps, res.StepName)
		if !res.Succeeded {
			t.Logf("Step %q failed: outcome=%v status=%d extractErr=%v err=%v",
				res.StepName, res.Result.Outcome, res.Result.StatusCode, res.ExtractErr, res.Result.Err)
		}
	}

	success, err := exec.ExecuteIteration(context.Background(), state, onStepDone)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if !success {
		t.Fatalf("expected iteration to succeed")
	}

	if len(executedSteps) != 3 {
		t.Errorf("expected 3 executed steps, got %d (%v)", len(executedSteps), executedSteps)
	}
	if state.Variables["token"] != "tok_jwt_abc123" {
		t.Errorf("expected token 'tok_jwt_abc123', got %q", state.Variables["token"])
	}
	if state.Variables["first_item"] != "item-1" {
		t.Errorf("expected first_item 'item-1', got %q", state.Variables["first_item"])
	}
}

func TestScenarioExecutor_OnFailurePolicies(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	tests := []struct {
		name          string
		policy        scenario.OnFailurePolicy
		expectedSteps []string
		expectSuccess bool
		expectErr     error
	}{
		{
			name:          "on_failure: stop skips subsequent steps in iteration",
			policy:        scenario.OnFailureStop,
			expectedSteps: []string{"step1", "step2_fail"},
			expectSuccess: false,
			expectErr:     nil,
		},
		{
			name:          "on_failure: continue executes subsequent steps",
			policy:        scenario.OnFailureContinue,
			expectedSteps: []string{"step1", "step2_fail", "step3"},
			expectSuccess: false,
			expectErr:     nil,
		},
		{
			name:          "on_failure: abort_vu returns ErrAbortVU",
			policy:        scenario.OnFailureAbortVU,
			expectedSteps: []string{"step1", "step2_fail"},
			expectSuccess: false,
			expectErr:     scenario.ErrAbortVU,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &scenario.CompiledScenario{
				Name: "test_policy",
				Steps: []*scenario.CompiledStep{
					{
						Name:             "step1",
						URL:              ts.URL + "/ok",
						Method:           "GET",
						ExpectedStatuses: []int{200},
						OnFailure:        scenario.OnFailureStop,
					},
					{
						Name:             "step2_fail",
						URL:              ts.URL + "/fail",
						Method:           "GET",
						ExpectedStatuses: []int{200}, // server returns 500, so this fails
						OnFailure:        tt.policy,
					},
					{
						Name:             "step3",
						URL:              ts.URL + "/ok",
						Method:           "GET",
						ExpectedStatuses: []int{200},
						OnFailure:        scenario.OnFailureStop,
					},
				},
			}

			exec := scenario.NewScenarioExecutor(sc, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())
			state := scenario.NewVUState(0, nil, nil)

			var executedSteps []string
			onStepDone := func(res *scenario.StepResult) {
				executedSteps = append(executedSteps, res.StepName)
			}

			success, err := exec.ExecuteIteration(context.Background(), state, onStepDone)
			if tt.expectErr != nil {
				if !errors.Is(err, tt.expectErr) {
					t.Errorf("expected error %v, got %v", tt.expectErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if success != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, success)
			}

			if len(executedSteps) != len(tt.expectedSteps) {
				t.Fatalf("expected executed steps %v, got %v", tt.expectedSteps, executedSteps)
			}
			for i := range executedSteps {
				if executedSteps[i] != tt.expectedSteps[i] {
					t.Errorf("step %d: expected %q, got %q", i, tt.expectedSteps[i], executedSteps[i])
				}
			}
		})
	}
}

func TestScenarioExecutor_ExtractionErrorFailsStep(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Body does NOT contain "token"
		_, _ = w.Write([]byte(`{"other_key": "val"}`))
	}))
	defer ts.Close()

	sc := &scenario.CompiledScenario{
		Name: "test_extract_fail",
		Steps: []*scenario.CompiledStep{
			{
				Name:             "step_with_missing_extract",
				URL:              ts.URL + "/endpoint",
				Method:           "GET",
				ExpectedStatuses: []int{200},
				ExtractRules: map[string]scenario.ExtractionRule{
					"token": {
						From:       scenario.SourceJSON,
						Expression: "token",
					},
				},
				OnFailure: scenario.OnFailureStop,
			},
		},
	}

	exec := scenario.NewScenarioExecutor(sc, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())
	state := scenario.NewVUState(0, nil, nil)

	var recordedResult *scenario.StepResult
	onStepDone := func(res *scenario.StepResult) {
		recordedResult = res
	}

	success, err := exec.ExecuteIteration(context.Background(), state, onStepDone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if success {
		t.Errorf("expected iteration to fail due to missing extraction key")
	}

	if recordedResult == nil {
		t.Fatalf("expected recorded result, got nil")
	}
	if recordedResult.Succeeded {
		t.Errorf("expected step to not succeed when extraction fails")
	}
	if recordedResult.ExtractErr == nil {
		t.Errorf("expected non-nil ExtractErr")
	}
}
