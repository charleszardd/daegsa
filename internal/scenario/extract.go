package scenario

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ExtractValue extracts a string value from an HTTP response according to the given rule (§11).
func ExtractValue(resp *http.Response, body []byte, rule ExtractionRule, state *VUState) (string, error) {
	switch rule.From {
	case SourceJSON, SourceJSONPath:
		return extractJSON(body, rule.Expression)

	case SourceHeader:
		return extractHeader(resp, rule.Expression)

	case SourceCookie:
		return extractCookie(resp, rule.Expression, state)

	case SourceRegex:
		return extractRegex(body, rule.Expression)

	default:
		return "", fmt.Errorf("unsupported extraction source %q", rule.From)
	}
}

// ExtractAll applies all extraction rules against the HTTP response and updates state.Variables (§11).
// Rules are evaluated in deterministic sorted order of variable names.
func ExtractAll(resp *http.Response, body []byte, rules map[string]ExtractionRule, state *VUState) error {
	if len(rules) == 0 || state == nil {
		return nil
	}

	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, varName := range keys {
		rule := rules[varName]
		val, err := ExtractValue(resp, body, rule, state)
		if err != nil {
			return fmt.Errorf("failed to extract %q (from %s: %q): %w", varName, rule.From, rule.Expression, err)
		}
		state.Variables[varName] = val
	}

	return nil
}

func extractJSON(body []byte, expr string) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("cannot extract JSON from empty response body")
	}

	var root interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return "", fmt.Errorf("invalid JSON response: %w", err)
	}

	steps, err := parseJSONPath(expr)
	if err != nil {
		return "", err
	}

	curr := root
	for _, step := range steps {
		if step.hasKey {
			objMap, ok := curr.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("key %q cannot be traversed on non-object value", step.key)
			}
			val, exists := objMap[step.key]
			if !exists {
				return "", fmt.Errorf("key %q not found in JSON object", step.key)
			}
			curr = val
		}

		if step.hasIndex {
			arr, ok := curr.([]interface{})
			if !ok {
				return "", fmt.Errorf("index [%d] cannot be traversed on non-array value", step.index)
			}
			if step.index < 0 || step.index >= len(arr) {
				return "", fmt.Errorf("array index [%d] out of bounds (length: %d)", step.index, len(arr))
			}
			curr = arr[step.index]
		}
	}

	return jsonValueToString(curr)
}

type pathStep struct {
	key      string
	hasKey   bool
	index    int
	hasIndex bool
}

func parseJSONPath(expr string) ([]pathStep, error) {
	trimmed := strings.TrimSpace(expr)
	if strings.HasPrefix(trimmed, "$.") {
		trimmed = trimmed[2:]
	} else if strings.HasPrefix(trimmed, "$") {
		trimmed = trimmed[1:]
	}
	if trimmed == "" {
		return nil, nil
	}

	var steps []pathStep
	// Tokens are split by '.'
	parts := strings.Split(trimmed, ".")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle array brackets within part, e.g. "items[0]", "[1]", "items[0][1]"
		for len(part) > 0 {
			bracketStart := strings.IndexByte(part, '[')
			if bracketStart == -1 {
				// Pure key name
				steps = append(steps, pathStep{key: part, hasKey: true})
				break
			}

			if bracketStart > 0 {
				keyName := part[:bracketStart]
				steps = append(steps, pathStep{key: keyName, hasKey: true})
				part = part[bracketStart:]
				continue
			}

			// Starts with '['
			bracketEnd := strings.IndexByte(part, ']')
			if bracketEnd == -1 {
				return nil, fmt.Errorf("unclosed bracket in jsonpath %q", expr)
			}
			idxStr := part[1:bracketEnd]
			idx, err := strconv.Atoi(idxStr)
			if err != nil || idx < 0 {
				return nil, fmt.Errorf("invalid array index %q in jsonpath %q", idxStr, expr)
			}
			steps = append(steps, pathStep{index: idx, hasIndex: true})
			part = part[bracketEnd+1:]
		}
	}

	return steps, nil
}

func jsonValueToString(v interface{}) (string, error) {
	if v == nil {
		return "", nil
	}

	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		if val == math.Trunc(val) && !math.IsNaN(val) && !math.IsInf(val, 0) {
			return strconv.FormatInt(int64(val), 10), nil
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(val), nil
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(val)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return fmt.Sprintf("%v", val), nil
	}
}

func extractHeader(resp *http.Response, headerName string) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("response is nil")
	}

	val := resp.Header.Get(headerName)
	if val == "" {
		return "", fmt.Errorf("header %q not found in response", headerName)
	}
	return val, nil
}

func extractCookie(resp *http.Response, cookieName string, state *VUState) (string, error) {
	if resp != nil {
		for _, c := range resp.Cookies() {
			if c.Name == cookieName {
				return c.Value, nil
			}
		}

		if state != nil && state.CookieJar != nil && resp.Request != nil && resp.Request.URL != nil {
			for _, c := range state.CookieJar.Cookies(resp.Request.URL) {
				if c.Name == cookieName {
					return c.Value, nil
				}
			}
		}
	}

	return "", fmt.Errorf("cookie %q not found", cookieName)
}

func extractRegex(body []byte, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex %q: %w", pattern, err)
	}

	matches := re.FindSubmatch(body)
	if len(matches) == 0 {
		return "", fmt.Errorf("regex %q matched nothing in response", pattern)
	}

	if len(matches) > 1 {
		return string(matches[1]), nil
	}
	return string(matches[0]), nil
}
