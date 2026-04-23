package pkg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/interpreter"
	"github.com/ipavlic/epex/schema"
)

// CreateFromSource scans an SFDX source directory and builds a Package.
// sourceDir should be the path containing classes/ and objects/ subdirectories
// (e.g. force-app/main/default).
func CreateFromSource(name, namespace, sourceDir string) (*Package, error) {
	pkg := NewPackage(name, namespace)
	pkg.SourceDir = sourceDir

	// Parse classes
	classesDir := filepath.Join(sourceDir, "classes")
	results, err := apex.ParseDirectory(classesDir)
	if err != nil {
		// Classes directory may not exist — that's OK
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("parsing classes in %s: %w", classesDir, err)
		}
	}

	reg := interpreter.NewRegistry()
	for _, r := range results {
		if len(r.Errors) > 0 {
			continue // skip files with parse errors
		}
		reg.RegisterClass(r.Tree)
	}
	pkg.Classes = reg.Classes

	// Build schema
	objectsDir := filepath.Join(sourceDir, "objects")
	s, err := schema.BuildSchemaFromDir(objectsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("building schema from %s: %w", objectsDir, err)
		}
		s = schema.NewSchema()
	}
	pkg.Schema = s

	return pkg, nil
}

// CreateFromProject reads a manifest and builds the local package.
func CreateFromProject(projectDir string) (*Package, *Manifest, error) {
	manifestPath := filepath.Join(projectDir, "epex.json")
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, nil, err
	}

	sourceDir := filepath.Join(projectDir, manifest.SourceDir)
	pkg, err := CreateFromSource("local", manifest.DefaultNamespace, sourceDir)
	if err != nil {
		return nil, nil, err
	}

	return pkg, manifest, nil
}
