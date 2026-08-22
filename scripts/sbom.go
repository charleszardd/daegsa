//go:build ignore

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CycloneDX 1.5 JSON BOM Models (§11, §15)
type CycloneDXBOM struct {
	BOMFormat    string      `json:"bomFormat"`
	SpecVersion  string      `json:"specVersion"`
	SerialNumber string      `json:"serialNumber"`
	Version      int         `json:"version"`
	Metadata     BOMMetadata `json:"metadata"`
	Components   []Component `json:"components"`
}

type BOMMetadata struct {
	Timestamp string    `json:"timestamp"`
	Tools     []BOMTool `json:"tools"`
	Component Component `json:"component"`
}

type BOMTool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Component struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	PURL        string `json:"purl,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

func main() {
	distDir := "dist"
	outputFile := filepath.Join(distDir, "sbom-cyclonedx.json")

	if err := os.MkdirAll(distDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating dist directory: %v\n", err)
		os.Exit(1)
	}

	components, err := parseGoModDependencies("go.mod")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing go.mod: %v\n", err)
		os.Exit(1)
	}

	// Generate deterministic serial number based on module components
	hash := sha256.New()
	for _, c := range components {
		hash.Write([]byte(c.Name + "@" + c.Version))
	}
	serialHash := fmt.Sprintf("%x", hash.Sum(nil))
	serialNumber := fmt.Sprintf("urn:uuid:%s-%s-%s-%s-%s",
		serialHash[0:8], serialHash[8:12], serialHash[12:16], serialHash[16:20], serialHash[20:32])

	bom := CycloneDXBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: serialNumber,
		Version:      1,
		Metadata: BOMMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tools: []BOMTool{
				{
					Vendor:  "DAEGSA",
					Name:    "daegsa-sbom-generator",
					Version: "v0.1.0",
				},
			},
			Component: Component{
				Type:        "application",
				Name:        "github.com/charleszardd/daegsa",
				Version:     "v0.1.0-dev",
				Description: "Portable CLI for repeatable REST API load, capacity, and rate-limit testing with explicit open and closed workload models",
				PURL:        "pkg:golang/github.com/charleszardd/daegsa@v0.1.0-dev",
			},
		},
		Components: components,
	}

	jsonData, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling CycloneDX SBOM: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Generated CycloneDX SBOM at %s (%d components)\n", outputFile, len(components))
}

func parseGoModDependencies(path string) ([]Component, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer file.Close()

	var components []Component
	scanner := bufio.NewScanner(file)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		if inRequireBlock || strings.HasPrefix(line, "require ") {
			raw := line
			if strings.HasPrefix(raw, "require ") {
				raw = strings.TrimPrefix(raw, "require ")
			}

			parts := strings.Fields(raw)
			if len(parts) >= 2 {
				modName := parts[0]
				modVer := parts[1]
				scope := "required"
				if len(parts) >= 3 && parts[2] == "//" && len(parts) >= 4 && parts[3] == "indirect" {
					scope = "optional"
				}

				components = append(components, Component{
					Type:    "library",
					Name:    modName,
					Version: modVer,
					PURL:    fmt.Sprintf("pkg:golang/%s@%s", modName, modVer),
					Scope:   scope,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}

	return components, nil
}
