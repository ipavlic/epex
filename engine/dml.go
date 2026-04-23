package engine

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Insert generates IDs and inserts records into the given SObject table.
func (d *DB) Insert(sobjectName string, records []map[string]any, idGen *IDGenerator) error {
	if len(records) == 0 {
		return nil
	}

	tableName := strings.ToLower(sobjectName)

	for _, rec := range records {
		// Generate ID if not already set.
		if _, ok := rec["Id"]; !ok {
			rec["Id"] = idGen.Generate(sobjectName)
		}

		cols, placeholders, vals := buildInsertParts(rec)
		query := fmt.Sprintf("INSERT INTO %q (%s) VALUES (%s)", tableName, cols, placeholders)
		if _, err := d.db.Exec(query, vals...); err != nil {
			return fmt.Errorf("inserting into %s: %w", tableName, err)
		}
	}
	return nil
}

// Update updates records in the given SObject table by Id.
func (d *DB) Update(sobjectName string, records []map[string]any) error {
	if len(records) == 0 {
		return nil
	}

	tableName := strings.ToLower(sobjectName)

	for _, rec := range records {
		id, ok := rec["Id"]
		if !ok {
			return fmt.Errorf("record missing Id field for update on %s", sobjectName)
		}

		setClauses, vals := buildUpdateParts(rec)
		if len(setClauses) == 0 {
			continue // nothing to update besides Id
		}

		vals = append(vals, id)
		query := fmt.Sprintf("UPDATE %q SET %s WHERE \"id\" = ?", tableName, strings.Join(setClauses, ", "))
		if _, err := d.db.Exec(query, vals...); err != nil {
			return fmt.Errorf("updating %s: %w", tableName, err)
		}
	}
	return nil
}

// Delete deletes records from the given SObject table by Id.
func (d *DB) Delete(sobjectName string, records []map[string]any) error {
	if len(records) == 0 {
		return nil
	}

	tableName := strings.ToLower(sobjectName)

	for _, rec := range records {
		id, ok := rec["Id"]
		if !ok {
			return fmt.Errorf("record missing Id field for delete on %s", sobjectName)
		}
		query := fmt.Sprintf("DELETE FROM %q WHERE \"id\" = ?", tableName)
		if _, err := d.db.Exec(query, id); err != nil {
			return fmt.Errorf("deleting from %s: %w", tableName, err)
		}
	}
	return nil
}

// Upsert inserts or updates records based on the externalIdField.
// If externalIdField is "Id" or empty, it uses the standard Id field.
func (d *DB) Upsert(sobjectName string, records []map[string]any, externalIdField string, idGen *IDGenerator) error {
	if len(records) == 0 {
		return nil
	}

	if externalIdField == "" {
		externalIdField = "Id"
	}

	tableName := strings.ToLower(sobjectName)

	for _, rec := range records {
		extVal, hasExt := rec[externalIdField]

		// Check if a record with this external ID already exists.
		var exists bool
		if hasExt && extVal != nil {
			colName := strings.ToLower(externalIdField)
			row := d.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %q WHERE %q = ?", tableName, colName), extVal)
			var count int
			if err := row.Scan(&count); err != nil {
				return fmt.Errorf("checking existence in %s: %w", tableName, err)
			}
			exists = count > 0
		}

		if exists {
			// Update: build SET clause excluding the external ID field.
			setClauses, vals := buildUpdatePartsExcluding(rec, externalIdField)
			if len(setClauses) == 0 {
				continue
			}
			colName := strings.ToLower(externalIdField)
			vals = append(vals, extVal)
			query := fmt.Sprintf("UPDATE %q SET %s WHERE %q = ?", tableName, strings.Join(setClauses, ", "), colName)
			if _, err := d.db.Exec(query, vals...); err != nil {
				return fmt.Errorf("upserting (update) %s: %w", tableName, err)
			}
		} else {
			// Insert: generate Id if not present.
			if _, ok := rec["Id"]; !ok {
				rec["Id"] = idGen.Generate(sobjectName)
			}
			cols, placeholders, vals := buildInsertParts(rec)
			query := fmt.Sprintf("INSERT INTO %q (%s) VALUES (%s)", tableName, cols, placeholders)
			if _, err := d.db.Exec(query, vals...); err != nil {
				return fmt.Errorf("upserting (insert) %s: %w", tableName, err)
			}
		}
	}
	return nil
}

// buildInsertParts returns column list, placeholder list, and values for an INSERT.
func buildInsertParts(rec map[string]any) (string, string, []any) {
	keys := slices.Sorted(maps.Keys(rec))
	cols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	vals := make([]any, len(keys))
	for i, k := range keys {
		cols[i] = fmt.Sprintf("%q", strings.ToLower(k))
		placeholders[i] = "?"
		vals[i] = rec[k]
	}
	return strings.Join(cols, ", "), strings.Join(placeholders, ", "), vals
}

// buildUpdateParts returns SET clauses and values, excluding the Id field.
func buildUpdateParts(rec map[string]any) ([]string, []any) {
	return buildUpdatePartsExcluding(rec, "Id")
}

// buildUpdatePartsExcluding returns SET clauses and values, excluding the given field.
func buildUpdatePartsExcluding(rec map[string]any, excludeField string) ([]string, []any) {
	keys := slices.Sorted(maps.Keys(rec))
	var setClauses []string
	var vals []any
	for _, k := range keys {
		if strings.EqualFold(k, excludeField) {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%q = ?", strings.ToLower(k)))
		vals = append(vals, rec[k])
	}
	return setClauses, vals
}
