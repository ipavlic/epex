package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ipavlic/epex/interpreter"
	"github.com/ipavlic/epex/schema"
)

// MockClassDef is a serializable class stub for mock packages.
type MockClassDef struct {
	Name        string           `json:"name"`
	Modifiers   []string         `json:"modifiers"`
	Annotations []string         `json:"annotations"`
	Methods     []MockMethodDef  `json:"methods"`
	Fields      []MockFieldDef   `json:"fields"`
	SuperClass  string           `json:"superClass,omitempty"`
	Interfaces  []string         `json:"interfaces,omitempty"`
}

// MockMethodDef is a serializable method stub.
type MockMethodDef struct {
	Name       string              `json:"name"`
	ReturnType string              `json:"returnType"`
	Params     []interpreter.ParamInfo `json:"params"`
	IsStatic   bool                `json:"isStatic"`
	Modifiers  []string            `json:"modifiers"`
}

// MockFieldDef is a serializable field stub.
type MockFieldDef struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	IsStatic  bool     `json:"isStatic"`
	Modifiers []string `json:"modifiers"`
}

// MockPackageData is the serialized format of a .apkg mock file.
type MockPackageData struct {
	Name      string                          `json:"name"`
	Namespace string                          `json:"namespace"`
	Classes   []MockClassDef                  `json:"classes"`
	Schema    *schema.Schema                  `json:"schema"`
}

// SaveMock writes a package's metadata to an .apkg file.
func SaveMock(path string, pkg *Package) error {
	data := MockPackageData{
		Name:      pkg.Name,
		Namespace: pkg.Namespace,
		Schema:    pkg.Schema,
	}

	for _, ci := range pkg.Classes {
		mcd := MockClassDef{
			Name:        ci.Name,
			Modifiers:   ci.Modifiers,
			Annotations: ci.Annotations,
			SuperClass:  ci.SuperClass,
			Interfaces:  ci.Interfaces,
		}
		for _, mi := range ci.Methods {
			mcd.Methods = append(mcd.Methods, MockMethodDef{
				Name:       mi.Name,
				ReturnType: mi.ReturnType,
				Params:     mi.Params,
				IsStatic:   mi.IsStatic,
				Modifiers:  mi.Modifiers,
			})
		}
		for _, fi := range ci.Fields {
			mcd.Fields = append(mcd.Fields, MockFieldDef{
				Name:      fi.Name,
				Type:      fi.Type,
				IsStatic:  fi.IsStatic,
				Modifiers: fi.Modifiers,
			})
		}
		data.Classes = append(data.Classes, mcd)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mock package: %w", err)
	}
	return os.WriteFile(path, jsonData, 0644)
}

// LoadMock reads a .apkg mock file and reconstructs a Package with stub classes.
func LoadMock(path string) (*Package, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading mock package %s: %w", path, err)
	}

	var mpd MockPackageData
	if err := json.Unmarshal(data, &mpd); err != nil {
		return nil, fmt.Errorf("parsing mock package %s: %w", path, err)
	}

	pkg := NewPackage(mpd.Name, mpd.Namespace)
	pkg.IsMock = true
	if mpd.Schema != nil {
		pkg.Schema = mpd.Schema
	}

	for _, mcd := range mpd.Classes {
		ci := &interpreter.ClassInfo{
			Name:         mcd.Name,
			Modifiers:    mcd.Modifiers,
			Annotations:  mcd.Annotations,
			SuperClass:   mcd.SuperClass,
			Interfaces:   mcd.Interfaces,
			Methods:      make(map[string]*interpreter.MethodInfo),
			Fields:       make(map[string]*interpreter.FieldInfo),
			Constructors: []*interpreter.ConstructorInfo{},
			InnerClasses: make(map[string]*interpreter.ClassInfo),
			// Node is nil — this is a stub, not parseable source
		}
		for _, mmd := range mcd.Methods {
			ci.Methods[strings.ToLower(mmd.Name)] = &interpreter.MethodInfo{
				Name:       mmd.Name,
				ReturnType: mmd.ReturnType,
				Params:     mmd.Params,
				IsStatic:   mmd.IsStatic,
				Modifiers:  mmd.Modifiers,
				// Node is nil — stub method, returns default value
			}
		}
		for _, mfd := range mcd.Fields {
			ci.Fields[strings.ToLower(mfd.Name)] = &interpreter.FieldInfo{
				Name:      mfd.Name,
				Type:      mfd.Type,
				IsStatic:  mfd.IsStatic,
				Modifiers: mfd.Modifiers,
			}
		}
		pkg.Classes[strings.ToLower(mcd.Name)] = ci
	}

	return pkg, nil
}
