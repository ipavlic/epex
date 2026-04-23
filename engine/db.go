package engine

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/ipavlic/epex/schema"
	_ "modernc.org/sqlite"
)

// DB wraps a *sql.DB backed by an in-memory SQLite database.
type DB struct {
	db *sql.DB
}

// NewDB opens an in-memory SQLite database.
func NewDB() (*DB, error) {
	sqlDB, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory sqlite: %w", err)
	}
	// Force single connection to avoid concurrency issues with in-memory databases.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("pinging sqlite: %w", err)
	}
	return &DB{db: sqlDB}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// fieldTypeToSQL maps a schema.FieldType to an appropriate SQLite column type.
func fieldTypeToSQL(ft schema.FieldType) string {
	switch ft {
	case schema.FieldTypeCheckbox:
		return "INTEGER"
	case schema.FieldTypeNumber, schema.FieldTypeCurrency, schema.FieldTypePercent:
		return "REAL"
	case schema.FieldTypeAutoNumber:
		return "INTEGER"
	default:
		// Id, Text, Email, Phone, Url, Picklist, MultiselectPicklist,
		// Date, DateTime, Time, Lookup, MasterDetail, and all others → TEXT
		return "TEXT"
	}
}

// CreateTable creates a SQLite table from an SObjectSchema.
// The table name is the SObject name lowercased.
func (d *DB) CreateTable(sobject *schema.SObjectSchema) error {
	tableName := strings.ToLower(sobject.Name)

	// Sort field names for deterministic DDL.
	fieldNames := make([]string, 0, len(sobject.Fields))
	for name := range sobject.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	var cols []string
	for _, name := range fieldNames {
		field := sobject.Fields[name]
		colName := strings.ToLower(name)
		colType := fieldTypeToSQL(field.Type)
		colDef := fmt.Sprintf("%q %s", colName, colType)
		if colName == "id" {
			colDef += " PRIMARY KEY"
		}
		cols = append(cols, colDef)
	}

	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %q (%s)", tableName, strings.Join(cols, ", "))
	_, err := d.db.Exec(ddl)
	if err != nil {
		return fmt.Errorf("creating table %s: %w", tableName, err)
	}
	return nil
}

// CreateTablesFromSchema creates tables for all SObjects in the schema.
func (d *DB) CreateTablesFromSchema(s *schema.Schema) error {
	for _, sobject := range s.SObjects {
		if err := d.CreateTable(sobject); err != nil {
			return err
		}
	}
	return nil
}

// ListTables returns the names of all user tables in the database.
func (d *DB) ListTables() ([]string, error) {
	rows, err := d.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
