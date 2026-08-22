package compare

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReportRejectsMalformedUnsupportedAndIncomplete(t *testing.T) {
	for name, content := range map[string]string{
		"malformed":     `{`,
		"unsupported":   `{"report_schema_version":99,"workload_model":"open"}`,
		"incomplete":    `{"report_schema_version":1,"workload_model":"open","incomplete":true}`,
		"trailing_json": `{"report_schema_version":1,"workload_model":"open"} extra`,
		"unknown_field": `{"report_schema_version":1,"workload_model":"open","unknown_field":123}`,
		"missing_model": `{"report_schema_version":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name+".json")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadReport(path); err == nil {
				t.Fatalf("%s: expected validation error", name)
			}
		})
	}
}

func TestLoadReportRejectsExceededSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.json")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Write slightly more than MaxReportFileBytes (10MiB + 10 bytes)
	if err := f.Truncate(MaxReportFileBytes + 10); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if _, err := LoadReport(path); err == nil {
		t.Fatal("expected error on oversized report file")
	}
}

func TestLoadReport_ValidReports(t *testing.T) {
	v1Content := `{"report_schema_version":1,"workload_model":"open","daegsa_version":"v0.1.0"}`
	path1 := filepath.Join(t.TempDir(), "v1.json")
	if err := os.WriteFile(path1, []byte(v1Content), 0600); err != nil {
		t.Fatal(err)
	}
	rep1, err := LoadReport(path1)
	if err != nil {
		t.Fatalf("failed to load v1 report: %v", err)
	}
	if rep1.ReportSchemaVersion != 1 {
		t.Errorf("expected v1, got %d", rep1.ReportSchemaVersion)
	}

	v2Content := `{"report_schema_version":2,"workload_model":"open","daegsa_version":"v0.2.0"}`
	path2 := filepath.Join(t.TempDir(), "v2.json")
	if err := os.WriteFile(path2, []byte(v2Content), 0600); err != nil {
		t.Fatal(err)
	}
	rep2, err := LoadReport(path2)
	if err != nil {
		t.Fatalf("failed to load v2 report: %v", err)
	}
	if rep2.ReportSchemaVersion != 2 {
		t.Errorf("expected v2, got %d", rep2.ReportSchemaVersion)
	}
}
