package schema

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// customObjectXML maps to the XML structure of .object-meta.xml files.
type customObjectXML struct {
	XMLName          xml.Name `xml:"CustomObject"`
	Label            string   `xml:"label"`
	PluralLabel      string   `xml:"pluralLabel"`
	DeploymentStatus string   `xml:"deploymentStatus"`
	SharingModel     string   `xml:"sharingModel"`
	NameField        struct {
		Label string `xml:"label"`
		Type  string `xml:"type"`
	} `xml:"nameField"`
}

// customFieldXML maps to the XML structure of .field-meta.xml files.
type customFieldXML struct {
	XMLName          xml.Name `xml:"CustomField"`
	FullName         string   `xml:"fullName"`
	Label            string   `xml:"label"`
	Type             string   `xml:"type"`
	Required         bool     `xml:"required"`
	Unique           bool     `xml:"unique"`
	ExternalId       bool     `xml:"externalId"`
	Length           int      `xml:"length"`
	Precision        int      `xml:"precision"`
	Scale            int      `xml:"scale"`
	ReferenceTo      string   `xml:"referenceTo"`
	RelationshipName string   `xml:"relationshipName"`
	DefaultValue     string   `xml:"defaultValue"`
}

// BuildSchema reads SFDX object metadata from the given project root and
// constructs an in-memory Schema. It looks for objects under the standard
// SFDX path: force-app/main/default/objects/
func BuildSchema(rootDir string) (*Schema, error) {
	objectsDir := filepath.Join(rootDir, "force-app", "main", "default", "objects")
	return BuildSchemaFromDir(objectsDir)
}

// BuildSchemaFromDir reads SObject metadata from an objects directory.
// Each subdirectory is expected to be an SObject with an optional
// .object-meta.xml and a fields/ subdirectory.
func BuildSchemaFromDir(objectsDir string) (*Schema, error) {
	schema := NewSchema()

	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		return nil, fmt.Errorf("reading objects directory %s: %w", objectsDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		objName := entry.Name()
		objDir := filepath.Join(objectsDir, objName)

		sobject, err := buildSObject(objName, objDir)
		if err != nil {
			return nil, fmt.Errorf("building SObject %s: %w", objName, err)
		}

		schema.SObjects[objName] = sobject
	}

	schema.PopulateRelationshipNames()
	return schema, nil
}

func buildSObject(name, dir string) (*SObjectSchema, error) {
	sobject := &SObjectSchema{
		Name:   name,
		Fields: make(map[string]*SObjectField),
	}

	// Add standard fields present on every SObject.
	for k, v := range StandardFields() {
		sobject.Fields[k] = v
	}

	// If this is a known standard object, merge in its object-specific fields.
	if stdObj := GetStandardObject(name); stdObj != nil {
		for k, v := range stdObj.Fields {
			if _, exists := sobject.Fields[k]; !exists {
				sobject.Fields[k] = v
			}
		}
		if sobject.Label == "" {
			sobject.Label = stdObj.Label
		}
	}

	// Parse .object-meta.xml if it exists
	metaFile := filepath.Join(dir, name+".object-meta.xml")
	if data, err := os.ReadFile(metaFile); err == nil {
		var obj customObjectXML
		if err := xml.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", metaFile, err)
		}
		sobject.Label = obj.Label
		sobject.PluralLabel = obj.PluralLabel
		sobject.DeploymentStatus = obj.DeploymentStatus
		sobject.SharingModel = obj.SharingModel
		sobject.NameFieldLabel = obj.NameField.Label
		sobject.NameFieldType = obj.NameField.Type
	}

	// Parse field definitions from fields/ subdirectory
	fieldsDir := filepath.Join(dir, "fields")
	fieldEntries, err := os.ReadDir(fieldsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return sobject, nil
		}
		return nil, fmt.Errorf("reading fields directory %s: %w", fieldsDir, err)
	}

	for _, fe := range fieldEntries {
		if fe.IsDir() || !strings.HasSuffix(fe.Name(), ".field-meta.xml") {
			continue
		}

		fieldPath := filepath.Join(fieldsDir, fe.Name())
		field, err := parseField(fieldPath)
		if err != nil {
			return nil, err
		}

		// Derive API name from filename if not set in XML
		if field.FullName == "" {
			field.FullName = strings.TrimSuffix(fe.Name(), ".field-meta.xml")
		}

		sobject.Fields[field.FullName] = field
	}

	return sobject, nil
}

func parseField(path string) (*SObjectField, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var f customFieldXML
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return &SObjectField{
		FullName:              f.FullName,
		Label:                 f.Label,
		Type:                  FieldType(f.Type),
		Required:              f.Required,
		Unique:                f.Unique,
		ExternalId:            f.ExternalId,
		Length:                f.Length,
		Precision:             f.Precision,
		Scale:                 f.Scale,
		ReferenceTo:           f.ReferenceTo,
		ChildRelationshipName: f.RelationshipName, // XML relationshipName is the child direction
		DefaultValue:          f.DefaultValue,
	}, nil
}
