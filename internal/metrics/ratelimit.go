package metrics

import "github.com/charleszardd/daegsa/internal/executor"

type HeaderConsistency struct {
	ObservedCount   int64    `json:"observed_count"`
	ParseErrorCount int64    `json:"parse_error_count"`
	AllParsedAgree  bool     `json:"all_parsed_agree"`
	Samples         []string `json:"samples,omitempty"`
}

func (worker *WorkerMetrics) recordHeaderObservations(observations []executor.HeaderParseObservation) {
	if worker.RateLimits.HeaderConsistency == nil {
		worker.RateLimits.HeaderConsistency = make(map[string]HeaderConsistency)
	}
	for _, observation := range observations {
		if !observation.Present {
			continue
		}
		consistency := worker.RateLimits.HeaderConsistency[observation.Name]
		if consistency.ObservedCount == 0 {
			consistency.AllParsedAgree = true
		}
		consistency.ObservedCount++
		if !observation.Valid {
			consistency.ParseErrorCount++
		} else if observation.Value != "" {
			if len(consistency.Samples) > 0 && consistency.Samples[0] != observation.Value {
				consistency.AllParsedAgree = false
			}
			if len(consistency.Samples) < MaxRateLimitSamples && !containsString(consistency.Samples, observation.Value) {
				consistency.Samples = append(consistency.Samples, observation.Value)
			}
		}
		worker.RateLimits.HeaderConsistency[observation.Name] = consistency
	}
}

func mergeHeaderConsistency(destination map[string]HeaderConsistency, source map[string]HeaderConsistency) {
	for name, incoming := range source {
		current := destination[name]
		if current.ObservedCount == 0 {
			current.AllParsedAgree = true
		}
		if !incoming.AllParsedAgree {
			current.AllParsedAgree = false
		}
		current.ObservedCount += incoming.ObservedCount
		current.ParseErrorCount += incoming.ParseErrorCount
		for _, sample := range incoming.Samples {
			if len(current.Samples) > 0 && current.Samples[0] != sample {
				current.AllParsedAgree = false
			}
			if len(current.Samples) < MaxRateLimitSamples && !containsString(current.Samples, sample) {
				current.Samples = append(current.Samples, sample)
			}
		}
		destination[name] = current
	}
}

func copyHeaderConsistency(source map[string]HeaderConsistency) map[string]HeaderConsistency {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]HeaderConsistency, len(source))
	for name, observation := range source {
		observation.Samples = append([]string(nil), observation.Samples...)
		result[name] = observation
	}
	return result
}
