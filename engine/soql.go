package engine

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ipavlic/epex/schema"
)

// AggregateField represents an aggregate function in a SOQL SELECT.
type AggregateField struct {
	FunctionSQL string // e.g. COUNT(*), SUM("amount")
	Alias       string // user alias or auto "expr0"
}

// QueryParams represents a pre-parsed SOQL query.
type QueryParams struct {
	Fields          []string
	ParentFields    []ParentField   // dotted parent fields like Account.Name
	SubQueries      []ChildSubQuery // child subqueries like (SELECT Id FROM Contacts)
	TypeOfFields    []TypeOfField   // TYPEOF polymorphic field expressions
	AggregateFields []AggregateField
	SObject         string
	Where           string // SQL WHERE clause (already translated from SOQL)
	WhereArgs       []any  // bind parameters for the WHERE clause
	GroupBy         string
	Having          string
	HavingArgs      []any
	OrderBy         string
	Limit           int
	Offset          int
	IsAggregate     bool
	SubQueryTables  []string // tables referenced by semi-join subqueries in WHERE
	AccessMode      string   // "", "USER_MODE", "SYSTEM_MODE", "SECURITY_ENFORCED"
}

// ParentField represents a dotted parent relationship field in a SOQL SELECT.
// For multi-level traversals like Account.Owner.Name, Path is ["Account", "Owner"]
// and FieldName is "Name".
type ParentField struct {
	Path      []string // relationship hops, e.g. ["Account", "Owner"]
	FieldName string   // leaf field, e.g. "Name"
}

// ChildSubQuery represents a child subquery in a SOQL SELECT.
type ChildSubQuery struct {
	RelationshipName string   // e.g. "Contacts" (from the FROM clause)
	Fields           []string // e.g. ["Id", "Name"]
	Where            string
	WhereArgs        []any
	OrderBy          string
	Limit            int
}

// TypeOfWhen represents a single WHEN clause in a TYPEOF expression.
type TypeOfWhen struct {
	SObjectType string   // e.g. "Account"
	Fields      []string // e.g. ["Phone", "NumberOfEmployees"]
}

// TypeOfField represents a TYPEOF polymorphic field expression in a SOQL SELECT.
type TypeOfField struct {
	FieldName   string       // polymorphic relationship: "What", "Who"
	FKField     string       // FK column: "WhatId", "WhoId"
	WhenClauses []TypeOfWhen
	ElseFields  []string
}

// joinInfo tracks a resolved parent JOIN alias and its target SObject.
type joinInfo struct {
	alias   string
	sobject string
}

// buildParentJoins resolves parent relationship fields into LEFT JOINs and
// appends dotted-column SELECT expressions to cols. Returns the JOIN clauses.
func buildParentJoins(params *QueryParams, s *schema.Schema, mainAlias string, cols *[]string) []string {
	var joins []string
	joinCache := map[string]joinInfo{}
	aliasIdx := 1

	for _, pf := range params.ParentFields {
		currentAlias := mainAlias
		currentSObject := params.SObject
		var pathSoFar []string

		for _, hop := range pf.Path {
			pathSoFar = append(pathSoFar, strings.ToLower(hop))
			cacheKey := strings.Join(pathSoFar, ".")

			if ji, ok := joinCache[cacheKey]; ok {
				currentAlias = ji.alias
				currentSObject = ji.sobject
				continue
			}

			rel := s.ResolveParentRelationship(currentSObject, hop)
			if rel == nil {
				currentAlias = ""
				break
			}

			alias := fmt.Sprintf("t%d", aliasIdx)
			aliasIdx++
			parentTable := strings.ToLower(rel.ParentSObject)
			fkCol := strings.ToLower(rel.FKField)
			joins = append(joins, fmt.Sprintf(
				"LEFT JOIN %q %s ON %s.%q = %s.\"id\"",
				parentTable, alias, currentAlias, fkCol, alias,
			))
			joinCache[cacheKey] = joinInfo{alias: alias, sobject: rel.ParentSObject}
			currentAlias = alias
			currentSObject = rel.ParentSObject
		}

		if currentAlias == "" {
			continue
		}

		pathParts := make([]string, len(pf.Path))
		for i, hop := range pf.Path {
			pathParts[i] = strings.ToLower(hop)
		}
		pathParts = append(pathParts, strings.ToLower(pf.FieldName))
		colAlias := strings.Join(pathParts, ".")
		*cols = append(*cols, fmt.Sprintf("%s.%q AS \"%s\"", currentAlias, strings.ToLower(pf.FieldName), colAlias))
	}

	return joins
}

// executeChildSubqueries runs child subqueries for each parent result row,
// attaching child records under the relationship name key.
func (d *DB) executeChildSubqueries(params *QueryParams, results []map[string]any, s *schema.Schema) {
	for _, sq := range params.SubQueries {
		rel := s.ResolveChildRelationship(params.SObject, sq.RelationshipName)
		lowerRel := strings.ToLower(sq.RelationshipName)

		if rel == nil {
			for _, row := range results {
				row[lowerRel] = []map[string]any{}
			}
			continue
		}

		childTable := strings.ToLower(rel.ChildSObject)
		fkCol := strings.ToLower(rel.FKField)

		for _, row := range results {
			parentId := row["id"]
			if parentId == nil {
				row[lowerRel] = []map[string]any{}
				continue
			}
			row[lowerRel] = d.queryChildRows(sq, childTable, fkCol, parentId)
		}
	}
}

// queryChildRows executes a single child subquery for one parent record.
func (d *DB) queryChildRows(sq ChildSubQuery, childTable, fkCol string, parentId any) []map[string]any {
	childCols := make([]string, len(sq.Fields))
	for i, f := range sq.Fields {
		childCols[i] = fmt.Sprintf("%q", strings.ToLower(f))
	}
	query := fmt.Sprintf("SELECT %s FROM %q WHERE %q = ?",
		strings.Join(childCols, ", "), childTable, fkCol)

	args := []any{parentId}
	if sq.Where != "" {
		query += " AND (" + sq.Where + ")"
		args = append(args, sq.WhereArgs...)
	}
	if sq.OrderBy != "" {
		query += " ORDER BY " + sq.OrderBy
	}
	if sq.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", sq.Limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return []map[string]any{}
	}

	return scanRows(rows, columns)
}

// scanRows reads all rows from the result set into a slice of maps.
func scanRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}, columns []string) []map[string]any {
	var results []map[string]any
	for rows.Next() {
		vals := make([]any, len(columns))
		valPtrs := make([]any, len(columns))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		if err := rows.Scan(valPtrs...); err != nil {
			continue
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = vals[i]
		}
		results = append(results, row)
	}
	if results == nil {
		return []map[string]any{}
	}
	return results
}

// QueryWithParams builds a SQL SELECT from QueryParams and executes it.
func (d *DB) QueryWithParams(params *QueryParams, s *schema.Schema) ([]map[string]any, error) {
	tableName := strings.ToLower(params.SObject)
	needsAlias := len(params.ParentFields) > 0

	// Ensure FK columns for TYPEOF fields are in the field list.
	for _, tof := range params.TypeOfFields {
		if !slices.ContainsFunc(params.Fields, func(f string) bool {
			return strings.EqualFold(f, tof.FKField)
		}) {
			params.Fields = append(params.Fields, tof.FKField)
		}
	}

	// Salesforce always includes Id in results, so ensure it's selected
	// (except for aggregate queries which don't return Id).
	hasId := slices.ContainsFunc(params.Fields, func(f string) bool {
		return strings.EqualFold(f, "Id")
	})

	var cols []string
	mainAlias := ""
	if needsAlias {
		mainAlias = "t0"
	}

	qualifyMain := func(col string) string {
		if mainAlias != "" {
			return fmt.Sprintf("%s.%q", mainAlias, col)
		}
		return fmt.Sprintf("%q", col)
	}

	if !hasId && !params.IsAggregate {
		cols = append(cols, qualifyMain("id"))
	}
	for _, f := range params.Fields {
		cols = append(cols, qualifyMain(strings.ToLower(f)))
	}
	for _, af := range params.AggregateFields {
		cols = append(cols, fmt.Sprintf("%s AS %q", af.FunctionSQL, af.Alias))
	}

	// Build parent relationship JOINs.
	var joins []string
	if needsAlias {
		joins = buildParentJoins(params, s, mainAlias, &cols)
	}

	// Assemble the query.
	var query string
	if mainAlias != "" {
		query = fmt.Sprintf("SELECT %s FROM %q %s", strings.Join(cols, ", "), tableName, mainAlias)
	} else {
		query = fmt.Sprintf("SELECT %s FROM %q", strings.Join(cols, ", "), tableName)
	}
	for _, j := range joins {
		query += " " + j
	}

	var args []any
	if params.Where != "" {
		where := params.Where
		if mainAlias != "" {
			where = qualifyWhereColumns(where, mainAlias)
		}
		query += " WHERE " + where
		args = append(args, params.WhereArgs...)
	}
	if params.GroupBy != "" {
		query += " GROUP BY " + params.GroupBy
	}
	if params.Having != "" {
		query += " HAVING " + params.Having
		args = append(args, params.HavingArgs...)
	}
	if params.OrderBy != "" {
		query += " ORDER BY " + params.OrderBy
	}
	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", params.Limit)
	} else if params.Offset > 0 {
		query += " LIMIT -1"
	}
	if params.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", params.Offset)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query on %s: %w", tableName, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("getting columns: %w", err)
	}

	results := scanRows(rows, columns)
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	// Execute child subqueries.
	if len(params.SubQueries) > 0 && len(results) > 0 {
		d.executeChildSubqueries(params, results, s)
	}

	return results, nil
}

// whereIdentRe matches identifier-like tokens (field names) in a WHERE clause.
// These are sequences of word chars that aren't SQL keywords or bind placeholders.
var whereIdentRe = regexp.MustCompile(`[a-z_][a-z0-9_]*`)

// whereSQLKeywords are SQL keywords that should not be alias-qualified.
var whereSQLKeywords = map[string]bool{
	"and": true, "or": true, "not": true, "in": true, "like": true,
	"is": true, "null": true, "true": true, "false": true,
	"count": true, "sum": true, "avg": true, "min": true, "max": true,
	"distinct": true,
}

// qualifyWhereColumns prefixes unqualified column identifiers in a WHERE clause
// with the given table alias. For example, `id = ?` becomes `t0."id" = ?`.
func qualifyWhereColumns(where, alias string) string {
	return whereIdentRe.ReplaceAllStringFunc(where, func(match string) string {
		if whereSQLKeywords[match] {
			return match
		}
		return fmt.Sprintf("%s.\"%s\"", alias, match)
	})
}
