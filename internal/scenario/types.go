package scenario

import (
	"errors"
	"net/http"

	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/plan"
)

// ErrAbortVU signals that the VU worker should terminate its execution entirely (§7).
var ErrAbortVU = errors.New("abort virtual user execution")

// Type aliases to plan package immutable scenario definitions (§6, §7).
type OnFailurePolicy = plan.OnFailurePolicy

const (
	OnFailureStop     = plan.OnFailureStop
	OnFailureAbortVU  = plan.OnFailureAbortVU
	OnFailureContinue = plan.OnFailureContinue
)

type ExtractionSource = plan.ExtractionSource

const (
	SourceJSON     = plan.ExtractSourceJSON
	SourceJSONPath = plan.ExtractSourceJSONPath
	SourceHeader   = plan.ExtractSourceHeader
	SourceCookie   = plan.ExtractSourceCookie
	SourceRegex    = plan.ExtractSourceRegex
)

type ExtractionRule = plan.ExtractionRule
type CompiledStep = plan.CompiledStep
type CompiledScenario = plan.CompiledScenario

// VUState represents the strictly isolated per-VU execution state and session memory (§2, §11).
type VUState struct {
	VUID                int
	Iteration           int64
	Variables           map[string]string
	CookieJar           http.CookieJar
	InitialVariables    map[string]string
	DeterministicTokens []string
}

// NewVUState creates an initialized, isolated state for a specific Virtual User (§11).
func NewVUState(vuID int, jar http.CookieJar, initialVars map[string]string) *VUState {
	vars := make(map[string]string)
	initCopy := make(map[string]string)
	for k, v := range initialVars {
		vars[k] = v
		initCopy[k] = v
	}
	return &VUState{
		VUID:             vuID,
		Iteration:        0,
		Variables:        vars,
		CookieJar:        jar,
		InitialVariables: initCopy,
	}
}

// ResetIteration advances the iteration count.
func (s *VUState) ResetIteration() {
	if s == nil {
		return
	}
	s.Iteration++
}

// StepResult captures the complete execution result of an individual scenario step (§8, §9).
type StepResult struct {
	StepName   string
	StepIndex  int
	Result     *executor.Result
	ExtractErr error
	Succeeded  bool
	AbortedVU  bool
}
