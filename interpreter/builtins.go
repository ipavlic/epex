package interpreter

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/parser"
	"github.com/ipavlic/epex/schema"
	"github.com/ipavlic/epex/tracer"
)

// callBuiltinMethod dispatches built-in method calls on objects.
// Returns the result and true if handled, or nil and false if not a builtin.
func (interp *Interpreter) callBuiltinMethod(obj *Value, methodName string, args []*Value) (*Value, bool) {
	lower := strings.ToLower(methodName)

	if obj == nil {
		return NullValue(), true
	}

	switch obj.Type {
	case TypeString:
		return interp.callStringMethod(obj, lower, args)
	case TypeList:
		return interp.callListMethod(obj, lower, args)
	case TypeMap:
		return interp.callMapMethod(obj, lower, args)
	case TypeSet:
		return interp.callSetMethod(obj, lower, args)
	case TypeSObject:
		// Check for Date/Datetime instance methods (stored as SObject with special SType)
		if obj.SType == "Date" || obj.SType == "Datetime" || obj.SType == "Time" {
			return callDatetimeInstanceMethod(obj, lower, args)
		}
		return interp.callSObjectMethod(obj, lower, args)
	case TypeInteger, TypeLong, TypeDouble:
		return interp.callNumericMethod(obj, lower, args)
	}

	return nil, false
}

func (interp *Interpreter) callStringMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	s := obj.Data.(string)
	switch method {
	case "length":
		return IntegerValue(len(s)), true
	case "substring":
		if len(args) == 1 {
			start, _ := args[0].toInt()
			if start < 0 || start > len(s) {
				return StringValue(""), true
			}
			return StringValue(s[start:]), true
		}
		if len(args) >= 2 {
			start, _ := args[0].toInt()
			end, _ := args[1].toInt()
			if start < 0 {
				start = 0
			}
			if end > len(s) {
				end = len(s)
			}
			if start > end {
				return StringValue(""), true
			}
			return StringValue(s[start:end]), true
		}
		return StringValue(s), true
	case "contains":
		if len(args) >= 1 {
			return BooleanValue(strings.Contains(s, args[0].ToString())), true
		}
		return BooleanValue(false), true
	case "tolowercase":
		return StringValue(strings.ToLower(s)), true
	case "touppercase":
		return StringValue(strings.ToUpper(s)), true
	case "split":
		if len(args) >= 1 {
			parts := strings.Split(s, args[0].ToString())
			elements := make([]*Value, len(parts))
			for i, p := range parts {
				elements[i] = StringValue(p)
			}
			return ListValue(elements), true
		}
		return ListValue(nil), true
	case "trim":
		return StringValue(strings.TrimSpace(s)), true
	case "valueof":
		return StringValue(s), true
	case "equals":
		if len(args) >= 1 {
			return BooleanValue(s == args[0].ToString()), true
		}
		return BooleanValue(false), true
	case "equalsignorecase":
		if len(args) >= 1 {
			return BooleanValue(strings.EqualFold(s, args[0].ToString())), true
		}
		return BooleanValue(false), true
	case "startswith":
		if len(args) >= 1 {
			return BooleanValue(strings.HasPrefix(s, args[0].ToString())), true
		}
		return BooleanValue(false), true
	case "endswith":
		if len(args) >= 1 {
			return BooleanValue(strings.HasSuffix(s, args[0].ToString())), true
		}
		return BooleanValue(false), true
	case "replace":
		if len(args) >= 2 {
			return StringValue(strings.ReplaceAll(s, args[0].ToString(), args[1].ToString())), true
		}
		return StringValue(s), true
	case "indexof":
		if len(args) >= 1 {
			return IntegerValue(strings.Index(s, args[0].ToString())), true
		}
		return IntegerValue(-1), true
	case "getsobjecttype":
		// Id.getSobjectType() — look up the SObject type from the ID prefix
		if interp.engine != nil {
			typeName := interp.engine.SObjectTypeForID(s)
			if typeName != "" {
				fields := map[string]*Value{
					"Name": StringValue(typeName),
				}
				return &Value{Type: TypeSObject, Data: fields, SType: "Schema.SObjectType"}, true
			}
		}
		return NullValue(), true
	}
	// Fall through to extended string methods
	return extendedStringMethod(s, method, args)
}

func (interp *Interpreter) callListMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	elements := obj.Data.([]*Value)
	switch method {
	case "add":
		if len(args) >= 1 {
			obj.Data = append(elements, args[0])
		}
		return NullValue(), true
	case "size":
		return IntegerValue(len(elements)), true
	case "get":
		if len(args) >= 1 {
			idx, _ := args[0].toInt()
			if idx >= 0 && idx < len(elements) {
				return elements[idx], true
			}
		}
		return NullValue(), true
	case "isempty":
		return BooleanValue(len(elements) == 0), true
	case "contains":
		if len(args) >= 1 {
			found := slices.ContainsFunc(elements, func(e *Value) bool {
				return e.Equals(args[0])
			})
			return BooleanValue(found), true
		}
		return BooleanValue(false), true
	case "remove":
		if len(args) >= 1 {
			idx, _ := args[0].toInt()
			if idx >= 0 && idx < len(elements) {
				obj.Data = append(elements[:idx], elements[idx+1:]...)
			}
		}
		return NullValue(), true
	case "clear":
		obj.Data = []*Value{}
		return NullValue(), true
	case "set":
		if len(args) >= 2 {
			idx, _ := args[0].toInt()
			elems := obj.Data.([]*Value)
			if idx >= 0 && idx < len(elems) {
				elems[idx] = args[1]
			}
		}
		return NullValue(), true
	}
	return nil, false
}

func (interp *Interpreter) callMapMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	entries := obj.Data.(map[string]*Value)
	switch method {
	case "put":
		if len(args) >= 2 {
			key := args[0].ToString()
			entries[key] = args[1]
		}
		return NullValue(), true
	case "get":
		if len(args) >= 1 {
			key := args[0].ToString()
			if v, ok := entries[key]; ok {
				return v, true
			}
			return NullValue(), true
		}
		return NullValue(), true
	case "containskey":
		if len(args) >= 1 {
			key := args[0].ToString()
			_, ok := entries[key]
			return BooleanValue(ok), true
		}
		return BooleanValue(false), true
	case "keyset":
		keys := make(map[string]*Value)
		for k := range entries {
			keys[k] = StringValue(k)
		}
		return SetValue(keys), true
	case "values":
		return ListValue(slices.Collect(maps.Values(entries))), true
	case "size":
		return IntegerValue(len(entries)), true
	case "isempty":
		return BooleanValue(len(entries) == 0), true
	case "remove":
		if len(args) >= 1 {
			key := args[0].ToString()
			delete(entries, key)
		}
		return NullValue(), true
	}
	return nil, false
}

func (interp *Interpreter) callSetMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	entries := obj.Data.(map[string]*Value)
	switch method {
	case "add":
		if len(args) >= 1 {
			key := args[0].ToString()
			entries[key] = args[0]
		}
		return NullValue(), true
	case "contains":
		if len(args) >= 1 {
			key := args[0].ToString()
			_, ok := entries[key]
			return BooleanValue(ok), true
		}
		return BooleanValue(false), true
	case "size":
		return IntegerValue(len(entries)), true
	case "isempty":
		return BooleanValue(len(entries) == 0), true
	case "remove":
		if len(args) >= 1 {
			key := args[0].ToString()
			delete(entries, key)
		}
		return NullValue(), true
	}
	return nil, false
}

func (interp *Interpreter) callSObjectMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	switch method {
	case "get":
		if len(args) >= 1 {
			key := strings.ToLower(args[0].ToString())
			for k, v := range fields {
				if strings.ToLower(k) == key {
					return v, true
				}
			}
			return NullValue(), true
		}
		return NullValue(), true
	case "put":
		if len(args) >= 2 {
			key := args[0].ToString()
			fields[key] = args[1]
		}
		return NullValue(), true
	case "getmessage":
		if msg, ok := fields["Message"]; ok {
			return msg, true
		}
		return NullValue(), true
	case "getid":
		if id, ok := fields["Id"]; ok {
			return id, true
		}
		return NullValue(), true
	case "getcause":
		return NullValue(), true
	case "gettypename":
		return StringValue(obj.SType), true
	}

	// Type-specific method dispatch based on SType.
	switch obj.SType {
	case "System.Type":
		return interp.callTypeInstanceMethod(obj, method, args)
	case "Schema.SObjectType":
		return interp.callSObjectTypeMethod(obj, method, args)
	case "Schema.DescribeSObjectResult":
		return interp.callDescribeSObjectMethod(obj, method, args)
	case "Schema.FieldSet":
		return callFieldSetMethod(obj, method, args)
	case "Schema.DescribeFieldResult":
		return interp.callDescribeFieldMethod(obj, method, args)
	case "SObjectAccessDecision":
		return callSObjectAccessDecisionMethod(obj, method, args)
	}

	return nil, false
}

func (interp *Interpreter) callNumericMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	switch method {
	case "intvalue":
		i, _ := obj.toInt()
		return IntegerValue(i), true
	case "longvalue":
		i, _ := obj.toInt()
		return LongValue(int64(i)), true
	case "doublevalue":
		f, _ := obj.toFloat64()
		return DoubleValue(f), true
	case "format":
		return StringValue(obj.ToString()), true
	}
	// Extended Decimal instance methods
	return callDecimalInstanceMethod(obj, method, args)
}

// checkAssert traces and panics if the assertion failed.
func (interp *Interpreter) checkAssert(passed bool, defaultMsg string, args []*Value, customMsgIdx int) {
	msg := defaultMsg
	if customMsgIdx < len(args) {
		msg = args[customMsgIdx].ToString()
	}
	interp.traceAssert(passed, msg)
	if !passed {
		panic(&AssertException{Message: msg})
	}
}

// callSystemMethod handles System.* static method calls.
func (interp *Interpreter) callSystemMethod(methodName string, args []*Value) *Value {
	lower := strings.ToLower(methodName)
	switch lower {
	case "debug":
		if len(args) > 0 {
			fmt.Println("DEBUG|" + args[0].ToString())
		}
		return NullValue()
	case "assert":
		passed := len(args) >= 1 && args[0].IsTruthy()
		interp.checkAssert(passed, "Assertion failed", args, 1)
		return NullValue()
	case "assertequals":
		if len(args) >= 2 {
			passed := args[0].Equals(args[1])
			interp.checkAssert(passed, fmt.Sprintf("Expected: %s, Actual: %s", args[0].ToString(), args[1].ToString()), args, 2)
		}
		return NullValue()
	case "assertnotequals":
		if len(args) >= 2 {
			passed := !args[0].Equals(args[1])
			interp.checkAssert(passed, fmt.Sprintf("Expected not equal: %s", args[0].ToString()), args, 2)
		}
		return NullValue()
	case "abortjob":
		// Stub: no-op
		return NullValue()
	case "enqueuejob":
		// Stub: return a fake job Id
		return StringValue("7071000000000001")
	}
	return NullValue()
}

// callAssertMethod handles Assert.* static method calls (API v56+).
func (interp *Interpreter) callAssertMethod(methodName string, args []*Value) *Value {
	mn := strings.ToLower(methodName)
	switch mn {
	case "areequal":
		if len(args) >= 2 {
			passed := args[0].Equals(args[1])
			interp.checkAssert(passed, fmt.Sprintf("Expected: %s, Actual: %s", args[0].ToString(), args[1].ToString()), args, 2)
		}
	case "arenotequal":
		if len(args) >= 2 {
			passed := !args[0].Equals(args[1])
			interp.checkAssert(passed, fmt.Sprintf("Expected not equal: %s", args[0].ToString()), args, 2)
		}
	case "istrue":
		passed := len(args) >= 1 && args[0].IsTruthy()
		interp.checkAssert(passed, "Expected true, got false", args, 1)
	case "isfalse":
		passed := len(args) >= 1 && !args[0].IsTruthy()
		interp.checkAssert(passed, "Expected false, got true", args, 1)
	case "isnull":
		passed := len(args) >= 1 && (args[0].Type == TypeNull || args[0].Data == nil)
		interp.checkAssert(passed, fmt.Sprintf("Expected null, got: %s", safeToString(args, 0)), args, 1)
	case "isnotnull":
		passed := len(args) >= 1 && args[0].Type != TypeNull && args[0].Data != nil
		interp.checkAssert(passed, "Expected non-null value", args, 1)
	case "isinstanceoftype":
		passed := len(args) >= 2 && isInstanceOf(args[0], args[1])
		interp.checkAssert(passed, fmt.Sprintf("Expected instance of %s", safeToString(args, 1)), args, 2)
	case "isnotinstanceoftype":
		passed := len(args) < 2 || !isInstanceOf(args[0], args[1])
		interp.checkAssert(passed, fmt.Sprintf("Expected not instance of %s", safeToString(args, 1)), args, 2)
	case "fail":
		msg := "Assert.fail() called"
		if len(args) >= 1 {
			msg = args[0].ToString()
		}
		interp.traceAssert(false, msg)
		panic(&AssertException{Message: msg})
	}
	return NullValue()
}

func safeToString(args []*Value, idx int) string {
	if idx < len(args) {
		return args[idx].ToString()
	}
	return "<missing>"
}

func isInstanceOf(val *Value, typeVal *Value) bool {
	typeName := strings.ToLower(typeVal.ToString())
	switch typeName {
	case "integer", "int":
		return val.Type == TypeInteger
	case "long":
		return val.Type == TypeLong
	case "double", "decimal":
		return val.Type == TypeDouble
	case "string":
		return val.Type == TypeString
	case "boolean":
		return val.Type == TypeBoolean
	case "list":
		return val.Type == TypeList
	case "map":
		return val.Type == TypeMap
	case "set":
		return val.Type == TypeSet
	default:
		// Check SObject type
		if val.Type == TypeSObject && val.SType != "" {
			return strings.EqualFold(val.SType, typeName)
		}
		return false
	}
}

// traceAssert records an assert event if tracing is enabled.
func (interp *Interpreter) traceAssert(passed bool, message string) {
	if interp.tracer.Enabled() {
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventAssert,
			Timestamp: time.Now(),
			File:      interp.currentFile,
			Detail:    message,
			Passed:    passed,
		})
	}
}

// callStaticMethod handles static method calls on known types (e.g. String.valueOf, Integer.valueOf).
func (interp *Interpreter) callStaticMethod(typeName, methodName string, args []*Value) (*Value, bool) {
	tn := strings.ToLower(typeName)
	mn := strings.ToLower(methodName)

	switch tn {
	case "string":
		if mn == "valueof" && len(args) >= 1 {
			return StringValue(args[0].ToString()), true
		}
		return staticStringMethod(mn, args)
	case "math":
		return callMathMethod(mn, args)
	case "json":
		return callJSONMethod(mn, args)
	case "date":
		return callDateStaticMethod(mn, args)
	case "datetime":
		return callDatetimeStaticMethod(mn, args)
	case "pattern":
		return callPatternStaticMethod(mn, args)
	case "url":
		return callUrlStaticMethod(mn, args)
	case "limits":
		return callLimitsMethod(mn, args)
	case "crypto":
		return callCryptoMethod(mn, args)
	case "encodingutil":
		return callEncodingUtilMethod(mn, args)
	case "integer":
		if mn == "valueof" && len(args) >= 1 {
			s := args[0].ToString()
			i, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return NullValue(), true
			}
			return IntegerValue(i), true
		}
	case "decimal", "double":
		if mn == "valueof" && len(args) >= 1 {
			s := args[0].ToString()
			f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return NullValue(), true
			}
			return DoubleValue(f), true
		}
	case "boolean":
		if mn == "valueof" && len(args) >= 1 {
			s := strings.ToLower(strings.TrimSpace(args[0].ToString()))
			return BooleanValue(s == "true"), true
		}
	case "database":
		return interp.callDatabaseMethod(mn, args)
	case "type":
		return interp.callTypeStaticMethod(mn, args)
	case "id":
		return interp.callIdStaticMethod(mn, args)
	case "schema":
		return interp.callSchemaStaticMethod(mn, args)
	case "security":
		return interp.callSecurityStaticMethod(mn, args)
	}
	return nil, false
}

// callDatabaseMethod handles Database.* static method calls.
func (interp *Interpreter) callDatabaseMethod(methodName string, args []*Value) (*Value, bool) {
	if interp.engine == nil {
		return NullValue(), true
	}

	switch methodName {
	case "insert":
		return interp.databaseDML("INSERT", args), true
	case "update":
		return interp.databaseDML("UPDATE", args), true
	case "delete":
		return interp.databaseDML("DELETE", args), true
	case "upsert":
		return interp.databaseUpsert(args), true
	case "query":
		return interp.databaseQuery(args), true
	case "querywithbinds":
		return interp.databaseQueryWithBinds(args), true
	}
	return nil, false
}

// databaseDML handles Database.insert, Database.update, Database.delete.
// Args: (recordOrList) or (recordOrList, allOrNone)
// Returns List<Database.SaveResult> or List<Database.DeleteResult>.
func (interp *Interpreter) databaseDML(operation string, args []*Value) *Value {
	if len(args) < 1 {
		return ListValue(nil)
	}
	val := args[0]
	typeName := extractSObjectType(val)

	// Fire before triggers
	switch operation {
	case "INSERT":
		interp.fireTriggers("BEFORE", "INSERT", typeName, val, nil)
	case "UPDATE":
		interp.fireTriggers("BEFORE", "UPDATE", typeName, val, val)
	case "DELETE":
		interp.fireTriggers("BEFORE", "DELETE", typeName, nil, val)
	}

	records := valueToRecords(val) // re-extract after before trigger
	start := time.Now()
	var err error
	switch operation {
	case "INSERT":
		err = interp.engine.Insert(typeName, records)
		if err == nil {
			writeBackIds(val, records)
		}
	case "UPDATE":
		err = interp.engine.Update(typeName, records)
	case "DELETE":
		err = interp.engine.Delete(typeName, records)
	}

	interp.traceDML(operation, typeName, len(records), start)

	// Fire after triggers
	switch operation {
	case "INSERT":
		interp.fireTriggers("AFTER", "INSERT", typeName, val, nil)
	case "UPDATE":
		interp.fireTriggers("AFTER", "UPDATE", typeName, val, val)
	case "DELETE":
		interp.fireTriggers("AFTER", "DELETE", typeName, nil, val)
	}

	resultType := "Database.SaveResult"
	if operation == "DELETE" {
		resultType = "Database.DeleteResult"
	}
	return buildDMLResults(records, err, resultType)
}

// databaseUpsert handles Database.upsert(recordOrList, externalIdField, allOrNone).
func (interp *Interpreter) databaseUpsert(args []*Value) *Value {
	if len(args) < 1 {
		return ListValue(nil)
	}
	val := args[0]
	records := valueToRecords(val)
	typeName := extractSObjectType(val)

	externalIdField := "Id"
	if len(args) >= 2 && args[1].Type == TypeString {
		externalIdField = args[1].Data.(string)
	}

	start := time.Now()
	err := interp.engine.Upsert(typeName, records, externalIdField)
	if err == nil {
		writeBackIds(val, records)
	}

	interp.traceDML("UPSERT", typeName, len(records), start)
	return buildDMLResults(records, err, "Database.UpsertResult")
}

// databaseQuery handles Database.query(soqlString).
func (interp *Interpreter) databaseQuery(args []*Value) *Value {
	if len(args) < 1 {
		return ListValue(nil)
	}
	soqlString := args[0].ToString()
	return interp.executeSOQLString(soqlString, nil)
}

// databaseQueryWithBinds handles Database.queryWithBinds(soqlString, bindMap, accessLevel).
func (interp *Interpreter) databaseQueryWithBinds(args []*Value) *Value {
	if len(args) < 2 {
		return ListValue(nil)
	}
	soqlString := args[0].ToString()

	// Convert bind map from Value map to Go map
	var binds map[string]any
	if args[1].Type == TypeMap {
		entries := args[1].Data.(map[string]*Value)
		binds = make(map[string]any, len(entries))
		for k, v := range entries {
			binds[k] = v.ToGoValue()
		}
	}

	return interp.executeSOQLString(soqlString, binds)
}

// executeSOQLString parses and executes a SOQL string, returning List<SObject>.
// Uses the ANTLR grammar to parse the SOQL, then extracts query parameters from the parse tree.
func (interp *Interpreter) executeSOQLString(soqlString string, binds map[string]any) *Value {
	if interp.engine == nil {
		return ListValue(nil)
	}

	queryCtx, err := apex.ParseSOQLString(soqlString)
	if err != nil {
		return ListValue(nil)
	}
	qCtx, ok := queryCtx.(*parser.QueryContext)
	if !ok || qCtx == nil {
		return ListValue(nil)
	}

	// Build a bind resolver that looks up variables from the binds map.
	var resolver bindResolver
	if len(binds) > 0 {
		resolver = func(be *parser.BoundExpressionContext) any {
			// The bound expression text is :varName — extract the variable name.
			if expr := be.Expression(); expr != nil {
				varName := expr.GetText()
				if val, ok := binds[varName]; ok {
					return val
				}
				for k, v := range binds {
					if strings.EqualFold(k, varName) {
						return v
					}
				}
			}
			return nil
		}
	}
	params := extractQueryParams(qCtx, resolver)

	// Apply sharing filter
	interp.applySharingFilter(params)

	start := time.Now()
	results, err := interp.engine.QueryWithFullParams(params)
	if err != nil {
		return ListValue(nil)
	}
	if interp.tracer.Enabled() {
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventSOQL,
			Timestamp: time.Now(),
			File:      interp.currentFile,
			Detail:    soqlString,
			Duration:  time.Since(start),
			RowCount:  len(results),
		})
	}

	sobjectType := params.SObject
	if params.IsAggregate {
		sobjectType = "AggregateResult"
	}
	return queryResultsToList(results, sobjectType)
}

// extractSObjectType gets the SObject type name from a Value.
func extractSObjectType(val *Value) string {
	if val.Type == TypeSObject {
		return val.SType
	}
	if val.Type == TypeList {
		elems := val.Data.([]*Value)
		if len(elems) > 0 && elems[0].Type == TypeSObject {
			return elems[0].SType
		}
	}
	return ""
}

// traceDML records a DML trace event if tracing is enabled.
func (interp *Interpreter) traceDML(operation, typeName string, rowCount int, start time.Time) {
	if !interp.tracer.Enabled() {
		return
	}
	interp.tracer.Record(tracer.TraceEvent{
		Type:      tracer.EventDML,
		Timestamp: time.Now(),
		File:      interp.currentFile,
		Class:     typeName,
		Detail:    operation,
		Duration:  time.Since(start),
		RowCount:  rowCount,
	})
}

// buildDMLResults creates a List of Database result objects (SaveResult, DeleteResult, or UpsertResult).
func buildDMLResults(records []map[string]any, err error, resultType string) *Value {
	results := make([]*Value, len(records))
	for i, rec := range records {
		fields := map[string]*Value{
			"Id":      NullValue(),
			"success": BooleanValue(err == nil),
			"errors":  ListValue(nil),
		}
		if id, ok := rec["Id"]; ok {
			fields["Id"] = StringValue(fmt.Sprintf("%v", id))
		}
		if resultType == "Database.UpsertResult" {
			fields["created"] = BooleanValue(true) // simplified: assume created
		}
		results[i] = &Value{Type: TypeSObject, Data: fields, SType: resultType}
	}
	return ListValue(results)
}

// queryResultsToList converts raw query results to a List<SObject> Value.
func queryResultsToList(results []map[string]any, sobjectType string) *Value {
	elements := make([]*Value, len(results))
	for i, row := range results {
		fields := make(map[string]*Value)

		for k, v := range row {
			// Check for TYPEOF polymorphic result (map[string]any)
			if refRecord, ok := v.(map[string]any); ok {
				refType, _ := refRecord["type"].(string)
				if refType == "" {
					refType = k
				}
				refFields := make(map[string]*Value)
				for rk, rv := range refRecord {
					if rk == "type" {
						continue
					}
					refFields[rk] = goValueToValue(rv)
				}
				fields[k] = &Value{Type: TypeSObject, Data: refFields, SType: refType}
				continue
			}

			// Check for child subquery results ([]map[string]any)
			if childRows, ok := v.([]map[string]any); ok {
				childType := k
				childElements := make([]*Value, len(childRows))
				for j, childRow := range childRows {
					childFields := make(map[string]*Value)
					for ck, cv := range childRow {
						childFields[ck] = goValueToValue(cv)
					}
					childElements[j] = &Value{Type: TypeSObject, Data: childFields, SType: childType}
				}
				fields[k] = ListValue(childElements)
				continue
			}

			// Check for dotted parent field (e.g. "account.name" or "account.owner.name")
			if strings.ContainsRune(k, '.') {
				setNestedField(fields, k, goValueToValue(v))
				continue
			}

			fields[k] = goValueToValue(v)
		}

		elements[i] = &Value{Type: TypeSObject, Data: fields, SType: sobjectType}
	}
	return ListValue(elements)
}

// setNestedField sets a value at a dotted path within a field map, creating
// intermediate SObject values as needed. For example, "account.owner.name"
// creates fields["account"] (SObject) → .Data["owner"] (SObject) → .Data["name"].
func setNestedField(fields map[string]*Value, dottedKey string, val *Value) {
	parts := strings.Split(dottedKey, ".")
	current := fields
	for i := 0; i < len(parts)-1; i++ {
		seg := parts[i]
		existing, ok := current[seg]
		if !ok || existing.Type != TypeSObject {
			nested := make(map[string]*Value)
			existing = &Value{Type: TypeSObject, Data: nested, SType: seg}
			current[seg] = existing
		}
		current = existing.Data.(map[string]*Value)
	}
	current[parts[len(parts)-1]] = val
}

// goValueToValue converts a Go any value to an interpreter Value.
func goValueToValue(v any) *Value {
	if v == nil {
		return NullValue()
	}
	switch tv := v.(type) {
	case int64:
		return IntegerValue(int(tv))
	case float64:
		return DoubleValue(tv)
	case bool:
		return BooleanValue(tv)
	case string:
		return StringValue(tv)
	default:
		return StringValue(fmt.Sprintf("%v", tv))
	}
}

// --- Type.forName / newInstance ---

// callTypeStaticMethod handles Type.forName() static method calls.
func (interp *Interpreter) callTypeStaticMethod(methodName string, args []*Value) (*Value, bool) {
	if methodName != "forname" || len(args) < 1 {
		return nil, false
	}
	// Type.forName(typeName) or Type.forName(namespace, typeName)
	typeName := ""
	if len(args) >= 2 {
		// Type.forName(namespace, typeName) — ignore namespace
		typeName = args[1].ToString()
	} else {
		typeName = args[0].ToString()
	}
	// Return a Type object (SObject with SType "System.Type" and a TypeName field)
	fields := map[string]*Value{
		"TypeName": StringValue(typeName),
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "System.Type"}, true
}

// --- Id.valueOf / getSobjectType ---

// callIdStaticMethod handles Id.valueOf() static method calls.
func (interp *Interpreter) callIdStaticMethod(methodName string, args []*Value) (*Value, bool) {
	if methodName == "valueof" && len(args) >= 1 {
		return StringValue(args[0].ToString()), true
	}
	return nil, false
}

// --- Schema.getGlobalDescribe ---

// callSchemaStaticMethod handles Schema.* static method calls.
func (interp *Interpreter) callSchemaStaticMethod(methodName string, args []*Value) (*Value, bool) {
	if methodName == "getglobaldescribe" {
		return interp.buildGlobalDescribe(), true
	}
	return nil, false
}

// buildGlobalDescribe builds a Map<String, Schema.SObjectType> from the engine schema.
func (interp *Interpreter) buildGlobalDescribe() *Value {
	entries := make(map[string]*Value)
	if interp.engine != nil {
		s := interp.engine.GetSchema()
		if s != nil {
			for name := range s.SObjects {
				// Each value is a Schema.SObjectType object
				fields := map[string]*Value{
					"Name": StringValue(name),
				}
				entries[strings.ToLower(name)] = &Value{Type: TypeSObject, Data: fields, SType: "Schema.SObjectType"}
			}
		}
	}
	return &Value{Type: TypeMap, Data: entries}
}

// buildDescribeSObjectResult creates a DescribeSObjectResult for an SObject schema.
func buildDescribeSObjectResult(obj *schema.SObjectSchema) *Value {
	fields := map[string]*Value{
		"Name":      StringValue(obj.Name),
		"Label":     StringValue(obj.Label),
		"KeyPrefix": NullValue(),
		"fields":    buildFieldsDescribe(obj),
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "Schema.DescribeSObjectResult"}
}

// buildFieldsDescribe creates the fields token object with getMap() support.
func buildFieldsDescribe(obj *schema.SObjectSchema) *Value {
	fieldMap := make(map[string]*Value)
	for name, f := range obj.Fields {
		fieldMap[strings.ToLower(name)] = buildDescribeFieldResult(f)
	}
	fields := map[string]*Value{
		"__fieldMap": &Value{Type: TypeMap, Data: fieldMap},
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "Schema.FieldSet"}
}

// buildDescribeFieldResult creates a DescribeFieldResult for a single field.
func buildDescribeFieldResult(f *schema.SObjectField) *Value {
	fields := map[string]*Value{
		"Name":     StringValue(f.FullName),
		"Label":    StringValue(f.Label),
		"Type":     StringValue(string(f.Type)),
		"Required": BooleanValue(f.Required),
		"Unique":   BooleanValue(f.Unique),
		"Length":   IntegerValue(f.Length),
	}
	return &Value{Type: TypeSObject, Data: fields, SType: "Schema.DescribeFieldResult"}
}

// --- Type instance methods ---

// callTypeInstanceMethod handles instance method calls on System.Type values.
func (interp *Interpreter) callTypeInstanceMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	typeName := ""
	if tn, ok := fields["TypeName"]; ok {
		typeName = tn.ToString()
	}

	switch method {
	case "newinstance":
		return interp.typeNewInstance(typeName), true
	case "getname":
		return StringValue(typeName), true
	case "tostring":
		return StringValue(typeName), true
	}
	return nil, false
}

// typeNewInstance creates a new instance of the given type name.
func (interp *Interpreter) typeNewInstance(typeName string) *Value {
	lower := strings.ToLower(typeName)

	// Primitive types
	switch lower {
	case "string":
		return StringValue("")
	case "integer", "int":
		return IntegerValue(0)
	case "long":
		return LongValue(0)
	case "double", "decimal":
		return DoubleValue(0)
	case "boolean":
		return BooleanValue(false)
	}

	// Check user-defined classes in the registry
	if classInfo, ok := interp.registry.Classes[lower]; ok {
		instance := make(map[string]*Value)
		for fname := range classInfo.Fields {
			instance[fname] = NullValue()
		}
		return &Value{Type: TypeSObject, Data: instance, SType: classInfo.Name}
	}

	// SObject type — create empty SObject
	fields := make(map[string]*Value)
	return &Value{Type: TypeSObject, Data: fields, SType: typeName}
}

// --- Id instance methods ---

// getSobjectType is dispatched via callBuiltinMethod for string values with
// the method name "getsobjecttype". We handle it in callStringMethod.

// --- Schema.SObjectType instance methods ---

func (interp *Interpreter) callSObjectTypeMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	switch method {
	case "getdescribe":
		name := ""
		if n, ok := fields["Name"]; ok {
			name = n.ToString()
		}
		if interp.engine != nil {
			s := interp.engine.GetSchema()
			if s != nil {
				if sobject := s.FindSObject(name); sobject != nil {
					return buildDescribeSObjectResult(sobject), true
				}
			}
		}
		return NullValue(), true
	case "newsobject":
		name := ""
		if n, ok := fields["Name"]; ok {
			name = n.ToString()
		}
		return &Value{Type: TypeSObject, Data: make(map[string]*Value), SType: name}, true
	}
	return nil, false
}

// --- Schema.DescribeSObjectResult instance methods ---

func (interp *Interpreter) callDescribeSObjectMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	sobjectName := ""
	if v, ok := fields["Name"]; ok {
		sobjectName = v.ToString()
	}
	switch method {
	case "getname":
		if v, ok := fields["Name"]; ok {
			return v, true
		}
		return NullValue(), true
	case "getlabel":
		if v, ok := fields["Label"]; ok {
			return v, true
		}
		return NullValue(), true
	case "getkeyprefix":
		return NullValue(), true
	case "iscreateable":
		return BooleanValue(interp.checkObjectPermissionBool(sobjectName, "PermissionsCreate")), true
	case "isupdateable":
		return BooleanValue(interp.checkObjectPermissionBool(sobjectName, "PermissionsEdit")), true
	case "isdeletable":
		return BooleanValue(interp.checkObjectPermissionBool(sobjectName, "PermissionsDelete")), true
	case "isqueryable", "isaccessible":
		return BooleanValue(interp.checkObjectPermissionBool(sobjectName, "PermissionsRead")), true
	case "iscustom":
		return BooleanValue(strings.HasSuffix(sobjectName, "__c")), true
	}
	return nil, false
}

// --- Schema.FieldSet instance methods ---

func callFieldSetMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	if method == "getmap" {
		if fm, ok := fields["__fieldMap"]; ok {
			return fm, true
		}
		return &Value{Type: TypeMap, Data: make(map[string]*Value)}, true
	}
	return nil, false
}

// --- Schema.DescribeFieldResult instance methods ---

func (interp *Interpreter) callDescribeFieldMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	getField := func(key string, fallback *Value) (*Value, bool) {
		if v, ok := fields[key]; ok {
			return v, true
		}
		return fallback, true
	}
	switch method {
	case "getname":
		return getField("Name", NullValue())
	case "getlabel":
		return getField("Label", NullValue())
	case "gettype":
		return getField("Type", NullValue())
	case "isrequired":
		return getField("Required", BooleanValue(false))
	case "isnillable":
		if v, ok := fields["Required"]; ok && v.Type == TypeBoolean {
			return BooleanValue(!v.Data.(bool)), true
		}
		return BooleanValue(false), true
	case "isunique":
		return getField("Unique", BooleanValue(false))
	case "getlength":
		return getField("Length", IntegerValue(0))
	case "iscustom":
		name := ""
		if v, ok := fields["Name"]; ok {
			name = v.ToString()
		}
		return BooleanValue(strings.HasSuffix(name, "__c")), true
	case "isaccessible":
		// Check field-level read permission
		sobjectType := ""
		fieldName := ""
		if v, ok := fields["SObjectType"]; ok && v != nil {
			sobjectType = v.ToString()
		}
		if v, ok := fields["Name"]; ok && v != nil {
			fieldName = v.ToString()
		}
		if sobjectType != "" && fieldName != "" {
			return BooleanValue(interp.checkFieldPermission(sobjectType, fieldName, "read")), true
		}
		return BooleanValue(true), true
	}
	return nil, false
}

// callSecurityStaticMethod handles Security.* static method calls.
func (interp *Interpreter) callSecurityStaticMethod(methodName string, args []*Value) (*Value, bool) {
	if strings.ToLower(methodName) != "stripinaccessible" {
		return nil, false
	}
	if len(args) < 2 {
		return NullValue(), true
	}

	accessType := strings.ToUpper(args[0].ToString())
	sourceRecords := args[1]

	// Optional third arg: enforceRootObjectCRUD (default true)
	enforceRootCRUD := true
	if len(args) >= 3 && args[2].Type == TypeBoolean {
		enforceRootCRUD = args[2].Data.(bool)
	}

	// Collect the source list
	var records []*Value
	if sourceRecords.Type == TypeList {
		records = sourceRecords.Data.([]*Value)
	} else if sourceRecords.Type == TypeSObject {
		records = []*Value{sourceRecords}
	}

	if len(records) == 0 {
		return buildAccessDecision(nil, nil), true
	}

	sobjectType := records[0].SType

	// Determine which permission to check per field
	fieldAccess := "read"
	crudOp := "read"
	switch accessType {
	case "CREATABLE":
		fieldAccess = "edit" // field-level create = edit permission
		crudOp = "create"
	case "UPDATABLE":
		fieldAccess = "edit"
		crudOp = "edit"
	case "READABLE":
		fieldAccess = "read"
		crudOp = "read"
	case "UPSERTABLE":
		fieldAccess = "edit"
		crudOp = "create"
	}

	// Enforce root object CRUD if requested
	if enforceRootCRUD {
		interp.checkCRUDPermission(sobjectType, crudOp)
	}

	// Build stripped copies and track removed fields
	removedFields := make(map[string]map[string]bool) // sobjectType -> set of field names
	var strippedRecords []*Value
	modifiedIndexes := make(map[int]bool)

	for i, rec := range records {
		if rec.Type != TypeSObject {
			strippedRecords = append(strippedRecords, rec)
			continue
		}
		srcFields := rec.Data.(map[string]*Value)
		newFields := make(map[string]*Value)
		recModified := false

		for k, v := range srcFields {
			lower := strings.ToLower(k)
			// Always keep Id and standard fields
			if lower == "id" || lower == "name" {
				newFields[k] = v
				continue
			}
			if interp.checkFieldPermission(sobjectType, k, fieldAccess) {
				newFields[k] = v
			} else {
				recModified = true
				if removedFields[sobjectType] == nil {
					removedFields[sobjectType] = make(map[string]bool)
				}
				removedFields[sobjectType][k] = true
			}
		}

		stripped := &Value{Type: TypeSObject, Data: newFields, SType: rec.SType}
		strippedRecords = append(strippedRecords, stripped)
		if recModified {
			modifiedIndexes[i] = true
		}
	}

	return buildAccessDecision(strippedRecords, removedFields), true
}

// buildAccessDecision creates an SObjectAccessDecision value.
func buildAccessDecision(records []*Value, removedFields map[string]map[string]bool) *Value {
	fields := map[string]*Value{}

	// records list
	if records != nil {
		fields["__records"] = ListValue(records)
	} else {
		fields["__records"] = ListValue(nil)
	}

	// removedFields map: Map<String, Set<String>>
	rfMap := make(map[string]*Value)
	for stype, fieldSet := range removedFields {
		setEntries := make(map[string]*Value)
		for f := range fieldSet {
			setEntries[strings.ToLower(f)] = BooleanValue(true)
		}
		rfMap[strings.ToLower(stype)] = &Value{Type: TypeMap, Data: setEntries}
	}
	fields["__removedFields"] = &Value{Type: TypeMap, Data: rfMap}

	return &Value{Type: TypeSObject, Data: fields, SType: "SObjectAccessDecision"}
}

// callSObjectAccessDecisionMethod handles instance methods on SObjectAccessDecision.
func callSObjectAccessDecisionMethod(obj *Value, method string, args []*Value) (*Value, bool) {
	fields := obj.Data.(map[string]*Value)
	switch method {
	case "getrecords":
		if v, ok := fields["__records"]; ok {
			return v, true
		}
		return ListValue(nil), true
	case "getremovedfields":
		if v, ok := fields["__removedFields"]; ok {
			return v, true
		}
		return &Value{Type: TypeMap, Data: make(map[string]*Value)}, true
	case "getmodifiedindexes":
		// Simplified: return empty set
		return &Value{Type: TypeMap, Data: make(map[string]*Value)}, true
	}
	return nil, false
}

// callTestMethod handles Test.* static method calls.
func (interp *Interpreter) callTestMethod(methodName string, args []*Value) *Value {
	mn := strings.ToLower(methodName)
	switch mn {
	case "starttest", "stoptest":
		// Stub: no-op
		return NullValue()
	case "isrunningtest":
		return BooleanValue(true)
	case "getstandardpricebookid":
		return StringValue("01s000000000001")
	case "loaddata":
		// Stub: return empty list
		return ListValue(nil)
	}
	return NullValue()
}

// callUserInfoMethod handles UserInfo.* static method calls.
func (interp *Interpreter) callUserInfoMethod(methodName string, args []*Value) *Value {
	mn := strings.ToLower(methodName)
	switch mn {
	case "getuserid":
		if interp.execCtx != nil && interp.execCtx.userID != "" {
			return StringValue(interp.execCtx.userID)
		}
		return StringValue("005000000000001")
	case "getusername":
		if interp.execCtx != nil {
			if v, ok := interp.execCtx.userFields["Username"]; ok && v != nil {
				return v
			}
		}
		return StringValue("test@example.com")
	case "getprofileid":
		if interp.execCtx != nil {
			if v, ok := interp.execCtx.userFields["ProfileId"]; ok && v != nil {
				return v
			}
		}
		return StringValue("00e000000000001")
	case "getorganizationid":
		return StringValue("00D000000000001")
	case "ismulticurrencyorganization":
		return BooleanValue(false)
	case "getdefaultcurrency":
		return StringValue("USD")
	case "getlocale":
		return StringValue("en_US")
	case "gettimezone":
		// Return a stub TimeZone SObject with getId() support
		fields := map[string]*Value{
			"Id": StringValue("America/Los_Angeles"),
		}
		return &Value{Type: TypeSObject, Data: fields, SType: "TimeZone"}
	}
	return NullValue()
}

// AssertException is thrown on assertion failure.
type AssertException struct {
	Message string
}

func (e *AssertException) Error() string {
	return "System.AssertException: " + e.Message
}
