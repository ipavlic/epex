package pkg

import (
	"strings"

	"github.com/ipavlic/epex/interpreter"
	"github.com/ipavlic/epex/schema"
)

// Package represents a unit of Apex code with namespace isolation.
type Package struct {
	Name      string                            `json:"name"`
	Namespace string                            `json:"namespace"`
	Classes   map[string]*interpreter.ClassInfo `json:"-"` // keyed by lowercase name
	Schema    *schema.Schema                    `json:"schema"`
	SourceDir string                            `json:"sourceDir,omitempty"`
	IsMock    bool                              `json:"isMock"`
}

// NewPackage creates an empty package.
func NewPackage(name, namespace string) *Package {
	return &Package{
		Name:      name,
		Namespace: namespace,
		Classes:   make(map[string]*interpreter.ClassInfo),
		Schema:    schema.NewSchema(),
	}
}

// AccessLevel determines cross-package visibility.
type AccessLevel int

const (
	AccessPrivate   AccessLevel = iota // same class only
	AccessProtected                    // same class + subclasses
	AccessPublic                       // same package/namespace
	AccessGlobal                       // any package
)

// IsAccessibleFrom returns whether a class from this package is accessible
// from code in the given namespace.
func (p *Package) IsAccessibleFrom(callerNamespace string, classInfo *interpreter.ClassInfo) bool {
	// Same namespace always has access
	if callerNamespace == p.Namespace {
		return true
	}
	// Cross-namespace: require global or @NamespaceAccessible
	for _, mod := range classInfo.Modifiers {
		switch mod {
		case "global", "GLOBAL":
			return true
		}
	}
	for _, ann := range classInfo.Annotations {
		if ann == "NamespaceAccessible" {
			return true
		}
	}
	return false
}

// GetClass returns a class by name (case-insensitive) if it exists.
func (p *Package) GetClass(name string) (*interpreter.ClassInfo, bool) {
	ci, ok := p.Classes[strings.ToLower(name)]
	return ci, ok
}
