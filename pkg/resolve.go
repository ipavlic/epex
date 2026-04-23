package pkg

import (
	"strings"

	"github.com/ipavlic/epex/interpreter"
)

// Resolver handles namespace-qualified name resolution across packages.
type Resolver struct {
	packages         []*Package          // all loaded packages
	byNamespace      map[string]*Package // lookup by namespace (lowercase)
	defaultNamespace string
}

// NewResolver creates a resolver for the given packages.
func NewResolver(packages []*Package, defaultNamespace string) *Resolver {
	r := &Resolver{
		packages:         packages,
		byNamespace:      make(map[string]*Package),
		defaultNamespace: defaultNamespace,
	}
	for _, p := range packages {
		if p.Namespace != "" {
			r.byNamespace[strings.ToLower(p.Namespace)] = p
		}
	}
	return r
}

// ResolveClass resolves a possibly namespace-qualified class name.
// Examples: "MyClass", "RDNACadence.MyClass"
// Returns the ClassInfo, its owning Package, and whether it was found.
func (r *Resolver) ResolveClass(qualifiedName string, callerNamespace string) (*interpreter.ClassInfo, *Package, bool) {
	parts := strings.SplitN(qualifiedName, ".", 2)

	if len(parts) == 2 {
		// Try namespace.ClassName
		ns := strings.ToLower(parts[0])
		className := strings.ToLower(parts[1])

		if pkg, ok := r.byNamespace[ns]; ok {
			if ci, ok := pkg.GetClass(className); ok {
				if pkg.IsAccessibleFrom(callerNamespace, ci) {
					return ci, pkg, true
				}
			}
		}
	}

	// Try unqualified name in caller's namespace first
	name := strings.ToLower(qualifiedName)
	if callerPkg, ok := r.byNamespace[strings.ToLower(callerNamespace)]; ok {
		if ci, ok := callerPkg.GetClass(name); ok {
			return ci, callerPkg, true
		}
	}

	// Try default namespace
	if r.defaultNamespace != "" && !strings.EqualFold(callerNamespace, r.defaultNamespace) {
		if defPkg, ok := r.byNamespace[strings.ToLower(r.defaultNamespace)]; ok {
			if ci, ok := defPkg.GetClass(name); ok {
				return ci, defPkg, true
			}
		}
	}

	// Try all packages (for global classes)
	for _, pkg := range r.packages {
		if ci, ok := pkg.GetClass(name); ok {
			if pkg.IsAccessibleFrom(callerNamespace, ci) {
				return ci, pkg, true
			}
		}
	}

	return nil, nil, false
}

// BuildMergedRegistry creates a single Registry containing all classes
// from all packages, with namespace prefixing for disambiguation.
// The local package's classes are registered both with and without namespace prefix.
func (r *Resolver) BuildMergedRegistry() *interpreter.Registry {
	reg := interpreter.NewRegistry()

	for _, pkg := range r.packages {
		for key, ci := range pkg.Classes {
			// Register with plain name
			reg.Classes[key] = ci

			// Also register with namespace prefix if package has one
			if pkg.Namespace != "" {
				nsKey := strings.ToLower(pkg.Namespace) + "." + key
				reg.Classes[nsKey] = ci
			}
		}
	}

	return reg
}

