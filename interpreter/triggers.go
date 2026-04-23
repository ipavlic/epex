package interpreter

import (
	"strings"
	"time"

	"github.com/ipavlic/epex/parser"
	"github.com/ipavlic/epex/tracer"
)

// triggerContext holds the Trigger.* context variables during trigger execution.
type triggerContext struct {
	newList  *Value // Trigger.new - List<SObject>
	oldList  *Value // Trigger.old - List<SObject>
	newMap   *Value // Trigger.newMap - Map<Id, SObject>
	oldMap   *Value // Trigger.oldMap - Map<Id, SObject>
	isBefore bool
	isAfter  bool
	isInsert bool
	isUpdate bool
	isDelete bool
	isUndelete bool
	size     int
}

// fireTriggers finds and executes all matching triggers for the given operation.
// phase is "BEFORE" or "AFTER", op is "INSERT"/"UPDATE"/"DELETE"/"UNDELETE".
func (interp *Interpreter) fireTriggers(phase, op, sobjectType string, val *Value, oldVal *Value) {
	if interp.registry == nil {
		return
	}

	for _, ti := range interp.registry.Triggers {
		if !strings.EqualFold(ti.SObject, sobjectType) {
			continue
		}
		for _, ev := range ti.Events {
			if !strings.EqualFold(ev.Op, op) {
				continue
			}
			isBefore := phase == "BEFORE"
			if isBefore && !ev.IsBefore {
				continue
			}
			if !isBefore && !ev.IsAfter {
				continue
			}
			interp.executeTrigger(ti, phase, op, val, oldVal)
		}
	}
}

func (interp *Interpreter) executeTrigger(ti *TriggerInfo, phase, op string, val *Value, oldVal *Value) {
	tuCtx, ok := ti.Node.(*parser.TriggerUnitContext)
	if !ok || tuCtx == nil {
		return
	}
	block := tuCtx.TriggerBlock()
	if block == nil {
		return
	}
	blockCtx, ok := block.(*parser.TriggerBlockContext)
	if !ok || blockCtx == nil {
		return
	}

	// Build trigger context
	tc := &triggerContext{
		isBefore:   phase == "BEFORE",
		isAfter:    phase == "AFTER",
		isInsert:   op == "INSERT",
		isUpdate:   op == "UPDATE",
		isDelete:   op == "DELETE",
		isUndelete: op == "UNDELETE",
	}

	// Build Trigger.new and Trigger.old
	newRecords := valToList(val)
	oldRecords := valToList(oldVal)
	tc.newList = newRecords
	tc.oldList = oldRecords
	tc.size = listLen(newRecords)
	if tc.size == 0 {
		tc.size = listLen(oldRecords)
	}

	// Build maps
	tc.newMap = buildTriggerMap(newRecords)
	tc.oldMap = buildTriggerMap(oldRecords)

	// Save interpreter state
	prevEnv := interp.env
	prevFile := interp.currentFile
	prevTriggerCtx := interp.triggerCtx

	interp.env = NewEnvironment(prevEnv)
	interp.currentFile = ti.SourceFile
	interp.triggerCtx = tc

	start := time.Now()

	// Trace trigger entry
	if interp.tracer.Enabled() {
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventMethodEntry,
			Timestamp: start,
			File:      ti.SourceFile,
			Class:     ti.Name,
			Method:    phase + "_" + op,
		})
	}

	// Execute trigger body statements
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Re-panic assertion errors and throw signals
				switch r.(type) {
				case *AssertException, *ThrowSignal, *NullPointerError, *ArithmeticError:
					panic(r)
				}
			}
		}()

		for _, stmt := range blockCtx.AllTriggerStatement() {
			tsCtx, ok := stmt.(*parser.TriggerStatementContext)
			if !ok || tsCtx == nil {
				continue
			}
			if s := tsCtx.Statement(); s != nil {
				interp.visitNode(s)
			}
			if bmd := tsCtx.BlockMemberDeclaration(); bmd != nil {
				interp.visitNode(bmd)
			}
		}
	}()

	// Trace trigger exit
	if interp.tracer.Enabled() {
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventMethodExit,
			Timestamp: time.Now(),
			File:      ti.SourceFile,
			Class:     ti.Name,
			Method:    phase + "_" + op,
			Duration:  time.Since(start),
		})
	}

	// Restore state
	interp.env = prevEnv
	interp.currentFile = prevFile
	interp.triggerCtx = prevTriggerCtx
}

// resolveTriggerField handles Trigger.new, Trigger.old, Trigger.isBefore, etc.
func (interp *Interpreter) resolveTriggerField(fieldName string) *Value {
	tc := interp.triggerCtx
	if tc == nil {
		return NullValue()
	}
	switch strings.ToLower(fieldName) {
	case "new":
		return tc.newList
	case "old":
		return tc.oldList
	case "newmap":
		return tc.newMap
	case "oldmap":
		return tc.oldMap
	case "isbefore":
		return BooleanValue(tc.isBefore)
	case "isafter":
		return BooleanValue(tc.isAfter)
	case "isinsert":
		return BooleanValue(tc.isInsert)
	case "isupdate":
		return BooleanValue(tc.isUpdate)
	case "isdelete":
		return BooleanValue(tc.isDelete)
	case "isundelete":
		return BooleanValue(tc.isUndelete)
	case "size":
		return IntegerValue(tc.size)
	case "isexecuting":
		return BooleanValue(true)
	case "operationtype":
		var op string
		if tc.isBefore {
			op = "BEFORE_"
		} else {
			op = "AFTER_"
		}
		switch {
		case tc.isInsert:
			op += "INSERT"
		case tc.isUpdate:
			op += "UPDATE"
		case tc.isDelete:
			op += "DELETE"
		case tc.isUndelete:
			op += "UNDELETE"
		}
		return StringValue(op)
	}
	return NullValue()
}

// valToList ensures we have a List<SObject> value.
func valToList(v *Value) *Value {
	if v == nil {
		return ListValue(nil)
	}
	if v.Type == TypeList {
		return v
	}
	if v.Type == TypeSObject {
		return ListValue([]*Value{v})
	}
	return ListValue(nil)
}

func listLen(v *Value) int {
	if v == nil || v.Type != TypeList {
		return 0
	}
	return len(v.Data.([]*Value))
}

func buildTriggerMap(listVal *Value) *Value {
	entries := make(map[string]*Value)
	if listVal == nil || listVal.Type != TypeList {
		return MapValue(entries)
	}
	for _, elem := range listVal.Data.([]*Value) {
		if elem.Type == TypeSObject {
			fields := elem.Data.(map[string]*Value)
			for k, v := range fields {
				if strings.EqualFold(k, "Id") && v.Type == TypeString {
					entries[v.Data.(string)] = elem
					break
				}
			}
		}
	}
	return MapValue(entries)
}
