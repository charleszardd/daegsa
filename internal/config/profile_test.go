package config_test

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
)

func TestParseAndValidateYAMLProfileV2(t *testing.T) {
	input := []byte(`schema_version: 2
name: profile
request: {url: "http://127.0.0.1:8080", method: GET}
load:
  model: open
  time_unit: 1s
  max_in_flight: 20
  segments:
    - {name: warm, stage: warmup, duration: 100ms, rate: 10}
    - {name: ramp, stage: measured, duration: 301ms, start_rate: 10, end_rate: 30, steps: 3}
    - {name: cool, stage: cooldown, duration: 100ms, rate: 10}
`)
	cfg, err := config.ParseAndValidateYAML(input)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := config.CompileLoadProfile(&cfg.Load)
	if err != nil {
		t.Fatal(err)
	}
	if compiled.TotalDuration != 501*time.Millisecond {
		t.Fatalf("duration = %v", compiled.TotalDuration)
	}
	if len(compiled.Segments) != 5 {
		t.Fatalf("compiled segments = %d", len(compiled.Segments))
	}
}

func TestProfileRejectsOrderingAndLegacyVersion(t *testing.T) {
	for _, input := range []string{
		`schema_version: 1
name: bad
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: m, stage: measured, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: bad
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: c, stage: cooldown, duration: 1s, rate: 1}, {name: m, stage: measured, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: duplicate-names
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: dup, stage: measured, duration: 1s, rate: 1}, {name: dup, stage: measured, duration: 1s, rate: 2}]}`,
		`schema_version: 2
name: no-measured-stage
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: warm, stage: warmup, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: both-rate-and-ramp
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: bad, stage: measured, duration: 1s, rate: 10, start_rate: 5, end_rate: 15, steps: 3}]}`,
		`schema_version: 2
name: ramp-steps-too-small
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, segments: [{name: bad, stage: measured, duration: 1s, start_rate: 5, end_rate: 15, steps: 1}]}`,
		`schema_version: 2
name: profile-with-rate
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, rate: 10, segments: [{name: m, stage: measured, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: profile-with-duration
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, duration: 5s, segments: [{name: m, stage: measured, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: closed-with-segments
request: {url: "http://127.0.0.1", method: GET}
load: {model: closed, users: 5, duration: 10s, segments: [{name: m, stage: measured, duration: 1s, rate: 1}]}`,
		`schema_version: 2
name: v2-without-segments
request: {url: "http://127.0.0.1", method: GET}
load: {model: open, max_in_flight: 2, rate: 10, duration: 5s}`,
	} {
		if _, err := config.ParseAndValidateYAML([]byte(input)); err == nil {
			t.Fatalf("expected invalid profile config for input:\n%s", input)
		}
	}
}
