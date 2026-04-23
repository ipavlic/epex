package engine

import (
	"fmt"
	"strings"

	"github.com/ipavlic/epex/schema"
)

// Engine ties together the database, ID generator, and schema.
type Engine struct {
	DB     *DB
	IDGen  *IDGenerator
	Schema *schema.Schema
}

// NewEngine creates a new Engine: opens the DB, creates tables, and
// initialises the ID generator.
func NewEngine(s *schema.Schema) (*Engine, error) {
	db, err := NewDB()
	if err != nil {
		return nil, err
	}
	if err := db.CreateTablesFromSchema(s); err != nil {
		db.Close()
		return nil, err
	}
	return &Engine{
		DB:     db,
		IDGen:  NewIDGenerator(),
		Schema: s,
	}, nil
}

// Close closes the underlying database.
func (e *Engine) Close() error {
	return e.DB.Close()
}

// EnsureTable ensures a table exists for the given SObject, creating it
// dynamically from standard object definitions if not already in the schema.
func (e *Engine) EnsureTable(sobjectName string) error {
	lower := strings.ToLower(sobjectName)
	// Already in schema — table was created at startup.
	for k := range e.Schema.SObjects {
		if strings.ToLower(k) == lower {
			return nil
		}
	}
	// Try standard object definitions.
	obj := schema.GetStandardObject(sobjectName)
	if obj == nil {
		// Not a standard object — create a minimal table with just Id and Name.
		obj = &schema.SObjectSchema{
			Name:   sobjectName,
			Fields: schema.StandardFields(),
		}
	}
	// Register in schema, populate relationship metadata, and create table.
	e.Schema.SObjects[obj.Name] = obj
	e.Schema.PopulateRelationshipNames()
	return e.DB.CreateTable(obj)
}

// ResetDatabase clears all data from all tables, providing test isolation.
func (e *Engine) ResetDatabase() error {
	tables, err := e.DB.ListTables()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := e.DB.db.Exec(fmt.Sprintf("DELETE FROM %q", table)); err != nil {
			return fmt.Errorf("clearing table %s: %w", table, err)
		}
	}
	// Reset ID generator so each test starts with predictable IDs.
	e.IDGen.Reset()
	return nil
}

// Insert inserts records into the named SObject table.
func (e *Engine) Insert(sobjectName string, records []map[string]any) error {
	if err := e.EnsureTable(sobjectName); err != nil {
		return err
	}
	return e.DB.Insert(sobjectName, records, e.IDGen)
}

// Update updates records in the named SObject table by Id.
func (e *Engine) Update(sobjectName string, records []map[string]any) error {
	return e.DB.Update(sobjectName, records)
}

// Delete deletes records from the named SObject table by Id.
func (e *Engine) Delete(sobjectName string, records []map[string]any) error {
	return e.DB.Delete(sobjectName, records)
}

// Upsert inserts or updates records based on the external ID field.
func (e *Engine) Upsert(sobjectName string, records []map[string]any, externalIdField string) error {
	if err := e.EnsureTable(sobjectName); err != nil {
		return err
	}
	return e.DB.Upsert(sobjectName, records, externalIdField, e.IDGen)
}

// GetSchema returns the schema.
func (e *Engine) GetSchema() *schema.Schema {
	return e.Schema
}

// SObjectTypeForID returns the SObject type for a Salesforce-style ID.
func (e *Engine) SObjectTypeForID(id string) string {
	return e.IDGen.SObjectTypeForID(id)
}

// Query executes a SOQL-style query using QueryParams.
func (e *Engine) Query(params *QueryParams) ([]map[string]any, error) {
	return e.queryWithEnsure(params)
}

// QueryFields executes a structured SOQL query with individual parameters.
func (e *Engine) QueryFields(fields []string, sobject, where string, whereArgs []any, orderBy string, limit, offset int) ([]map[string]any, error) {
	return e.QueryWithFullParams(&QueryParams{
		Fields:    fields,
		SObject:   sobject,
		Where:     where,
		WhereArgs: whereArgs,
		OrderBy:   orderBy,
		Limit:     limit,
		Offset:    offset,
	})
}

// QueryWithFullParams executes a SOQL query with full relationship support.
func (e *Engine) QueryWithFullParams(params *QueryParams) ([]map[string]any, error) {
	return e.queryWithEnsure(params)
}

// queryWithEnsure ensures all required tables exist, then executes the query.
func (e *Engine) queryWithEnsure(params *QueryParams) ([]map[string]any, error) {
	if err := e.EnsureTable(params.SObject); err != nil {
		return nil, err
	}
	// Ensure tables for all hops in parent relationship paths.
	for _, pf := range params.ParentFields {
		currentSObject := params.SObject
		for _, hop := range pf.Path {
			rel := e.Schema.ResolveParentRelationship(currentSObject, hop)
			if rel == nil {
				break
			}
			if err := e.EnsureTable(rel.ParentSObject); err != nil {
				return nil, err
			}
			currentSObject = rel.ParentSObject
		}
	}
	// Ensure tables for semi-join subquery targets.
	for _, tbl := range params.SubQueryTables {
		if err := e.EnsureTable(tbl); err != nil {
			return nil, err
		}
	}
	// Ensure tables for child subquery targets.
	for _, sq := range params.SubQueries {
		rel := e.Schema.ResolveChildRelationship(params.SObject, sq.RelationshipName)
		if rel != nil {
			if err := e.EnsureTable(rel.ChildSObject); err != nil {
				return nil, err
			}
		}
	}
	results, err := e.DB.QueryWithParams(params, e.Schema)
	if err != nil {
		return nil, err
	}

	// Resolve TYPEOF polymorphic fields.
	if len(params.TypeOfFields) > 0 {
		if err := e.resolveTypeOfFields(results, params.TypeOfFields); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// resolveTypeOfFields resolves TYPEOF polymorphic fields in query results.
// For each row, it reads the FK value, determines the SObject type, matches
// the appropriate WHEN clause (or ELSE), queries the referenced table for
// those fields, and stores the result as a map under the relationship name.
func (e *Engine) resolveTypeOfFields(results []map[string]any, typeOfFields []TypeOfField) error {
	for _, tof := range typeOfFields {
		fkKey := strings.ToLower(tof.FKField)
		relKey := strings.ToLower(tof.FieldName)

		for _, row := range results {
			fkVal, ok := row[fkKey]
			if !ok || fkVal == nil {
				row[relKey] = nil
				continue
			}

			fkStr := fmt.Sprintf("%v", fkVal)
			sobjectType := e.IDGen.SObjectTypeForID(fkStr)
			if sobjectType == "" {
				row[relKey] = nil
				continue
			}

			// Find matching WHEN clause.
			var fields []string
			for _, wc := range tof.WhenClauses {
				if strings.EqualFold(wc.SObjectType, sobjectType) {
					fields = wc.Fields
					break
				}
			}
			if fields == nil {
				fields = tof.ElseFields
			}
			if fields == nil {
				row[relKey] = nil
				continue
			}

			// Ensure the referenced table exists.
			if err := e.EnsureTable(sobjectType); err != nil {
				return err
			}

			// Query the referenced record.
			refResults, err := e.DB.QueryWithParams(&QueryParams{
				Fields:    fields,
				SObject:   sobjectType,
				Where:     "\"id\" = ?",
				WhereArgs: []any{fkStr},
				Limit:     1,
			}, e.Schema)
			if err != nil || len(refResults) == 0 {
				row[relKey] = nil
				continue
			}

			// Add a "type" key so downstream can set the SType.
			refRecord := refResults[0]
			refRecord["type"] = sobjectType
			row[relKey] = refRecord
		}
	}
	return nil
}
