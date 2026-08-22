package scenario

import (
	"fmt"
	"strings"
)

// ErrUnresolvedVariable is returned when a variable referenced in a template is missing from VU state (§11).
type ErrUnresolvedVariable struct {
	VariableName string
}

func (e *ErrUnresolvedVariable) Error() string {
	return fmt.Sprintf("unresolved variable %q", e.VariableName)
}

// SubstituteVariables replaces all occurrences of ${var_name} with their corresponding values in vars (§11).
// Escaped syntax $${var_name} is converted to literal ${var_name} without variable lookup.
// If any referenced variable is missing from vars, an ErrUnresolvedVariable is returned.
func SubstituteVariables(template string, vars map[string]string) (string, error) {
	if !strings.Contains(template, "$") {
		return template, nil
	}

	var sb strings.Builder
	sb.Grow(len(template))

	i := 0
	n := len(template)
	for i < n {
		if template[i] == '$' {
			// Check for escaped $${...}
			if i+1 < n && template[i+1] == '$' && i+2 < n && template[i+2] == '{' {
				end := strings.IndexByte(template[i+3:], '}')
				if end != -1 {
					// Output literal ${...}
					sb.WriteByte('$')
					sb.WriteString(template[i+2 : i+3+end+1])
					i = i + 3 + end + 1
					continue
				}
			}

			// Check for ${var_name}
			if i+1 < n && template[i+1] == '{' {
				end := strings.IndexByte(template[i+2:], '}')
				if end != -1 {
					varName := template[i+2 : i+2+end]
					val, ok := vars[varName]
					if !ok {
						return "", &ErrUnresolvedVariable{VariableName: varName}
					}
					sb.WriteString(val)
					i = i + 2 + end + 1
					continue
				}
			}
		}

		sb.WriteByte(template[i])
		i++
	}

	return sb.String(), nil
}
