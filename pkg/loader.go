package pkg

import (
	"fmt"
	"path/filepath"

	"github.com/ipavlic/epex/schema"
)

// LoadResult holds all loaded packages and the merged state.
type LoadResult struct {
	LocalPackage *Package
	MockPackages []*Package
	AllPackages  []*Package
	Resolver     *Resolver
	Schema       *schema.Schema // merged schema from all packages
}

// LoadProject loads a complete project: manifest, local source, and mock dependencies.
func LoadProject(projectDir string) (*LoadResult, error) {
	manifest, err := LoadManifest(filepath.Join(projectDir, "epex.json"))
	if err != nil {
		return nil, err
	}

	return LoadProjectWithManifest(projectDir, manifest)
}

// LoadProjectWithManifest loads a project using a pre-parsed manifest.
func LoadProjectWithManifest(projectDir string, manifest *Manifest) (*LoadResult, error) {
	result := &LoadResult{}

	// 1. Load mock packages (dependencies)
	for _, dep := range manifest.Packages {
		mockPath := filepath.Join(projectDir, dep.Mock)
		mockPkg, err := LoadMock(mockPath)
		if err != nil {
			return nil, fmt.Errorf("loading mock package %s: %w", dep.Name, err)
		}
		// Override namespace from manifest if specified
		if dep.Namespace != "" {
			mockPkg.Namespace = dep.Namespace
		}
		result.MockPackages = append(result.MockPackages, mockPkg)
	}

	// 2. Load local package
	sourceDir := filepath.Join(projectDir, manifest.SourceDir)
	localPkg, err := CreateFromSource("local", manifest.DefaultNamespace, sourceDir)
	if err != nil {
		return nil, fmt.Errorf("creating local package: %w", err)
	}
	result.LocalPackage = localPkg

	// 3. Build package list: mocks first, then local
	result.AllPackages = append(result.MockPackages, localPkg)

	// 4. Create resolver
	result.Resolver = NewResolver(result.AllPackages, manifest.DefaultNamespace)

	// 5. Merge schemas
	result.Schema = mergeSchemas(result.AllPackages)

	return result, nil
}

// mergeSchemas combines schemas from all packages.
// Later packages' SObjects override earlier ones.
// Fields from different packages on the same SObject are merged.
func mergeSchemas(packages []*Package) *schema.Schema {
	merged := schema.NewSchema()

	for _, pkg := range packages {
		if pkg.Schema == nil {
			continue
		}
		for name, obj := range pkg.Schema.SObjects {
			existing, ok := merged.SObjects[name]
			if !ok {
				// Copy the SObject
				copy := *obj
				copy.Fields = make(map[string]*schema.SObjectField)
				for k, v := range obj.Fields {
					copy.Fields[k] = v
				}
				merged.SObjects[name] = &copy
			} else {
				// Merge fields from this package into existing
				for k, v := range obj.Fields {
					existing.Fields[k] = v
				}
				// Update metadata if the new package has it
				if obj.Label != "" {
					existing.Label = obj.Label
				}
				if obj.PluralLabel != "" {
					existing.PluralLabel = obj.PluralLabel
				}
			}
		}
	}

	return merged
}
