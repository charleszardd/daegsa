package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
)

var (
	// ErrMissingEnvVar indicates a required environment variable is not set.
	ErrMissingEnvVar = errors.New("missing environment variable")

	// ErrInvalidEnvSyntax indicates invalid syntax in a variable placeholder.
	ErrInvalidEnvSyntax = errors.New("invalid environment variable syntax")
)

var validIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ExpandEnv expands ${VAR_NAME} environment variable placeholders in input bytes.
// Escaped placeholders formatted as $${VAR_NAME} are converted to literal ${VAR_NAME}.
// If getenv is nil, os.LookupEnv is used. If a referenced environment variable is not set
// or returns an empty string, ErrMissingEnvVar is returned.
func ExpandEnv(input []byte, getenv func(string) string) ([]byte, error) {
	if len(input) == 0 {
		return input, nil
	}

	var lookup func(string) (string, bool)
	if getenv != nil {
		lookup = func(key string) (string, bool) {
			val := getenv(key)
			if val == "" {
				return "", false
			}
			return val, true
		}
	} else {
		lookup = func(key string) (string, bool) {
			val, ok := os.LookupEnv(key)
			if !ok || val == "" {
				return "", false
			}
			return val, true
		}
	}

	return ExpandEnvWithLookup(input, lookup)
}

// ExpandEnvWithLookup expands ${VAR_NAME} placeholders using a custom lookup function.
func ExpandEnvWithLookup(input []byte, lookup func(string) (string, bool)) ([]byte, error) {
	if len(input) == 0 {
		return input, nil
	}

	var buf bytes.Buffer
	buf.Grow(len(input))

	i := 0
	for i < len(input) {
		if input[i] == '$' {
			// Check for escape: $$
			if i+1 < len(input) && input[i+1] == '$' {
				// Check if followed by {
				if i+2 < len(input) && input[i+2] == '{' {
					// Escaped placeholder: $${VAR} -> literal ${VAR}
					buf.WriteByte('$')
					i += 2 // skip one '$', leave next '${' to be handled as literal
					// find matching '}'
					closeIdx := bytes.IndexByte(input[i:], '}')
					if closeIdx == -1 {
						return nil, fmt.Errorf("%w: unclosed escaped variable placeholder", ErrInvalidEnvSyntax)
					}
					varName := string(input[i+1 : i+closeIdx])
					if !validIdentifierRegex.MatchString(varName) {
						return nil, fmt.Errorf("%w: invalid variable name %q in escaped placeholder", ErrInvalidEnvSyntax, varName)
					}
					buf.Write(input[i : i+closeIdx+1])
					i += closeIdx + 1
					continue
				}
				// Plain double $$
				buf.WriteByte('$')
				i += 2
				continue
			}

			// Check for ${
			if i+1 < len(input) && input[i+1] == '{' {
				closeIdx := bytes.IndexByte(input[i+2:], '}')
				if closeIdx == -1 {
					return nil, fmt.Errorf("%w: unclosed variable placeholder", ErrInvalidEnvSyntax)
				}

				varNameBytes := input[i+2 : i+2+closeIdx]
				varName := string(varNameBytes)

				if !validIdentifierRegex.MatchString(varName) {
					return nil, fmt.Errorf("%w: invalid variable name %q", ErrInvalidEnvSyntax, varName)
				}

				val, ok := lookup(varName)
				if !ok {
					return nil, fmt.Errorf("%w: environment variable %q is not set", ErrMissingEnvVar, varName)
				}

				buf.WriteString(val)
				i = i + 2 + closeIdx + 1
				continue
			}

			// Standalone $
			buf.WriteByte(input[i])
			i++
			continue
		}

		buf.WriteByte(input[i])
		i++
	}

	return buf.Bytes(), nil
}
