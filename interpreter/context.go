package interpreter

import (
	"fmt"
	"strings"

	"github.com/ipavlic/epex/engine"
)

// AccessMode determines whether permission checks are enforced.
type AccessMode int

const (
	AccessModeSystem AccessMode = iota // no checks — full access (default)
	AccessModeUser                     // FLS/CRUD/sharing enforced
)

// executionContext tracks the running user for System.runAs blocks.
type executionContext struct {
	userID     string            // Id of the running user
	userFields map[string]*Value // cached User SObject fields (ProfileId, Username, etc.)
}

// dbToBool converts a database value to a Go bool.
// Handles bool, int/int64, string "0"/"1"/"true"/"false".
func dbToBool(val any) bool {
	switch v := val.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case string:
		lower := strings.ToLower(v)
		return lower != "0" && lower != "false" && lower != ""
	case float64:
		return v != 0
	default:
		return fmt.Sprintf("%v", v) != "0"
	}
}

// effectiveSharingMode returns "with" or "without" based on the current class
// modifier and execution context.
//
// Salesforce rules:
//   - "with sharing": always enforces sharing
//   - "without sharing": never enforces sharing
//   - "inherited sharing": inherits caller's mode; at entry point defaults to "with sharing"
//   - No modifier (omitted): at entry point defaults to "without sharing";
//     when called by another class, inherits caller's mode
func (interp *Interpreter) effectiveSharingMode() string {
	if interp.currentClass != nil {
		for _, mod := range interp.currentClass.Modifiers {
			lower := strings.ToLower(mod)
			if lower == "withsharing" || lower == "with sharing" {
				return "with"
			}
			if lower == "withoutsharing" || lower == "without sharing" {
				return "without"
			}
			if lower == "inheritedsharing" || lower == "inherited sharing" {
				// Inherit caller's sharing mode; at entry point default to "with"
				if interp.callerSharingMode != "" {
					return interp.callerSharingMode
				}
				return "with"
			}
		}
	}
	// No modifier (omitted): inherit caller's mode; at entry point default to "without"
	if interp.callerSharingMode != "" {
		return interp.callerSharingMode
	}
	return "without"
}

// checkCRUDPermission checks whether the current user's profile has the given
// permission (create/read/edit/delete) on the specified SObject type.
// "No permission row = full access" — only enforced when rows exist.
func (interp *Interpreter) checkCRUDPermission(sobjectType, operation string) {
	if interp.execCtx == nil || interp.engine == nil {
		return // system context — allow everything
	}
	profileID := interp.getProfileID()
	if profileID == "" {
		return
	}

	permField := ""
	switch strings.ToLower(operation) {
	case "create":
		permField = "permissionscreate"
	case "read":
		permField = "permissionsread"
	case "edit":
		permField = "permissionsedit"
	case "delete":
		permField = "permissionsdelete"
	}
	if permField == "" {
		return
	}

	params := &engine.QueryParams{
		Fields:    []string{permField},
		SObject:   "ObjectPermissions",
		Where:     `"parentid" = ? AND LOWER("sobjecttype") = LOWER(?)`,
		WhereArgs: []any{profileID, sobjectType},
		Limit:     1,
	}
	results, err := interp.engine.QueryWithFullParams(params)
	if err != nil || len(results) == 0 {
		return // no row = full access
	}

	if val, ok := results[0][permField]; ok && !dbToBool(val) {
		panic(&ThrowSignal{Value: StringValue(
			"Insufficient privileges: " + operation + " on " + sobjectType)})
	}
}

// checkFieldPermission checks whether the current user's profile can access a
// specific field. Returns true if accessible. "No row = full access."
func (interp *Interpreter) checkFieldPermission(sobjectType, fieldName, access string) bool {
	if interp.execCtx == nil || interp.engine == nil {
		return true
	}
	profileID := interp.getProfileID()
	if profileID == "" {
		return true
	}

	qualifiedField := sobjectType + "." + fieldName
	permField := "permissionsread"
	if strings.ToLower(access) == "edit" {
		permField = "permissionsedit"
	}

	params := &engine.QueryParams{
		Fields:    []string{permField},
		SObject:   "FieldPermissions",
		Where:     `"parentid" = ? AND LOWER("field") = LOWER(?)`,
		WhereArgs: []any{profileID, qualifiedField},
		Limit:     1,
	}
	results, err := interp.engine.QueryWithFullParams(params)
	if err != nil || len(results) == 0 {
		return true // no row = full access
	}

	if val, ok := results[0][permField]; ok {
		return dbToBool(val)
	}
	return true
}

// getProfileID returns the profile ID from the execution context, or empty string.
func (interp *Interpreter) getProfileID() string {
	if interp.execCtx != nil {
		if v, ok := interp.execCtx.userFields["ProfileId"]; ok && v != nil {
			return v.ToString()
		}
	}
	return ""
}

// checkObjectPermissionBool queries ObjectPermissions for a specific permission
// and returns its boolean value. Returns true if no row exists.
func (interp *Interpreter) checkObjectPermissionBool(sobjectType, permField string) bool {
	if interp.execCtx == nil || interp.engine == nil {
		return true
	}
	profileID := interp.getProfileID()
	if profileID == "" {
		return true
	}

	lowerField := strings.ToLower(permField)
	params := &engine.QueryParams{
		Fields:    []string{lowerField},
		SObject:   "ObjectPermissions",
		Where:     `"parentid" = ? AND LOWER("sobjecttype") = LOWER(?)`,
		WhereArgs: []any{profileID, sobjectType},
		Limit:     1,
	}
	results, err := interp.engine.QueryWithFullParams(params)
	if err != nil || len(results) == 0 {
		return true // no row = full access
	}
	if val, ok := results[0][lowerField]; ok {
		return dbToBool(val)
	}
	return true
}
