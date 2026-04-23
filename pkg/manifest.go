package pkg

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manifest is the project configuration file (epex.json).
type Manifest struct {
	DefaultNamespace string              `json:"defaultNamespace"`
	SourceDir        string              `json:"sourceDir"`
	Packages         []PackageDependency `json:"packages"`
}

// PackageDependency declares a dependency on another package.
type PackageDependency struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Mock      string `json:"mock"` // path to .apkg file
}

// LoadManifest reads and parses an epex.json file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}

	// Defaults
	if m.SourceDir == "" {
		m.SourceDir = "force-app/main/default"
	}

	return &m, nil
}

// SaveManifest writes a manifest to disk.
func SaveManifest(path string, m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
