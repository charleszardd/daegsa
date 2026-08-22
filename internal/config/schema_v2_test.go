package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaV2DocumentsAreValidJSON(t *testing.T) {
	for _, name := range []string{"config.schema.json", "report.schema.json"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "schemas", "v2", name))
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if document["additionalProperties"] != false {
			t.Fatalf("%s must reject unknown root fields", name)
		}
	}
}
