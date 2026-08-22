package report

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteJSONReport serializes the Report to formatted JSON and writes it to targetFilePath (§13).
func WriteJSONReport(targetFilePath string, rep *Report) error {
	if rep == nil {
		return fmt.Errorf("cannot write nil report")
	}
	if targetFilePath == "" {
		return fmt.Errorf("target file path cannot be empty")
	}

	data, err := rep.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal JSON report: %w", err)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(targetFilePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create report directory: %w", err)
		}
	}

	// Write JSON data
	if err := os.WriteFile(targetFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON report to %s: %w", targetFilePath, err)
	}

	return nil
}
