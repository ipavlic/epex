package interpreter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/antlr4-go/antlr/v4"
	"github.com/ipavlic/epex/engine"
	"github.com/ipavlic/epex/parser"
	"github.com/ipavlic/epex/schema"
	"github.com/ipavlic/epex/tracer"
)

// EngineInterface abstracts DML and SOQL operations.
type EngineInterface interface {
	Insert(sobjectName string, records []map[string]any) error
	Update(sobjectName string, records []map[string]any) error
	Delete(sobjectName string, records []map[string]any) error
	Upsert(sobjectName string, records []map[string]any, externalIdField string) error
	QueryFields(fields []string, sobject, where string, whereArgs []any, orderBy string, limit, offset int) ([]map[string]any, error)
	QueryWithFullParams(params *engine.QueryParams) ([]map[string]any, error)
	// EnsureTable ensures a table exists for the given SObject, creating it
	// dynamically from standard object definitions if needed.
	EnsureTable(sobjectName string) error
	// ResetDatabase clears all data from all tables, providing test isolation.
	ResetDatabase() error
	// GetSchema returns the schema for Schema describe support.
	GetSchema() *schema.Schema
	// SObjectTypeForID returns the SObject type for a Salesforce-style ID.
	SObjectTypeForID(id string) string
}

// knownStaticClasses is the set of well-known Apex class names that are
// resolved as static class references (e.g. System, Assert, String).
var knownStaticClasses = map[string]bool{
	"system": true, "assert": true, "string": true, "integer": true,
	"double": true, "decimal": true, "boolean": true, "long": true,
	"math": true, "json": true, "type": true, "date": true,
	"datetime": true, "time": true, "database": true, "pattern": true,
	"url": true, "limits": true, "crypto": true, "encodingutil": true,
	"trigger": true, "id": true, "schema": true,
	"test": true, "userinfo": true, "security": true,
}

// ReturnSignal is used for control flow: returning from a method.
type ReturnSignal struct {
	Value *Value
}

// BreakSignal is used for control flow: breaking from a loop.
type BreakSignal struct{}

// ContinueSignal is used for control flow: continuing a loop.
type ContinueSignal struct{}

// ThrowSignal is used for control flow: throwing an exception.
type ThrowSignal struct {
	Value *Value
}

// Interpreter walks the AST and evaluates Apex code.
type Interpreter struct {
	parser.BaseApexParserVisitor
	registry    *Registry
	env         *Environment
	engine      EngineInterface
	tracer      tracer.Tracer
	thisObj     *Value
	currentClass *ClassInfo
	currentFile  string // current source file being executed
	debugOutput  []string // captured debug output for testing
	triggerCtx   *triggerContext // active trigger context, nil when not in trigger
	execCtx      *executionContext // active user context for System.runAs, nil = system admin
	callerSharingMode string // sharing mode of the caller ("with"/"without"/""), used for inherited/omitted sharing
}

// NewInterpreter creates a new interpreter.
func NewInterpreter(registry *Registry, engine EngineInterface) *Interpreter {
	interp := &Interpreter{
		registry: registry,
		env:      NewEnvironment(nil),
		engine:   engine,
		tracer:   &tracer.NoopTracer{},
	}
	return interp
}

// SetTracer sets the tracer for this interpreter.
func (interp *Interpreter) SetTracer(t tracer.Tracer) {
	if t != nil {
		interp.tracer = t
	}
}

// SetCurrentFile sets the current source file name for trace events.
func (interp *Interpreter) SetCurrentFile(file string) {
	interp.currentFile = file
}

// safeValue ensures we always return a *Value from visit results.
func safeValue(v any) *Value {
	if v == nil {
		return NullValue()
	}
	if val, ok := v.(*Value); ok {
		if val == nil {
			return NullValue()
		}
		return val
	}
	return NullValue()
}

// visitNode visits an ANTLR parse tree node.
func (interp *Interpreter) visitNode(node antlr.ParseTree) any {
	if node == nil {
		return NullValue()
	}
	return node.Accept(interp)
}

// ExecuteMethod runs a specific method on a class.
func (interp *Interpreter) ExecuteMethod(className, methodName string, args []*Value) *Value {
	classInfo, ok := interp.registry.Classes[strings.ToLower(className)]
	if !ok {
		return NullValue()
	}

	methodInfo, ok := classInfo.Methods[strings.ToLower(methodName)]
	if !ok {
		return NullValue()
	}

	prevClass := interp.currentClass
	prevFile := interp.currentFile
	interp.currentClass = classInfo
	interp.currentFile = classInfo.SourceFile

	// Create instance if not static
	if !methodInfo.IsStatic {
		interp.thisObj = interp.createInstance(classInfo, nil)
	}

	result := interp.executeMethodNode(methodInfo, args)

	interp.currentClass = prevClass
	interp.currentFile = prevFile
	return result
}

// ExecuteAnonymous runs a block of anonymous Apex code (parsed as a CompilationUnit).
func (interp *Interpreter) ExecuteAnonymous(tree parser.ICompilationUnitContext) *Value {
	return safeValue(interp.visitNode(tree))
}

// executeMethodInClass runs a method in the context of a (possibly different) class,
// properly saving and restoring currentClass, currentFile, and callerSharingMode
// so that sharing keywords (inherited sharing, omitted modifier) inherit correctly.
func (interp *Interpreter) executeMethodInClass(classInfo *ClassInfo, methodInfo *MethodInfo, args []*Value) *Value {
	prevClass := interp.currentClass
	prevFile := interp.currentFile
	prevCallerSharing := interp.callerSharingMode

	// Record the caller's effective sharing mode before switching classes
	interp.callerSharingMode = interp.effectiveSharingMode()
	interp.currentClass = classInfo
	interp.currentFile = classInfo.SourceFile

	result := interp.executeMethodNode(methodInfo, args)

	interp.currentClass = prevClass
	interp.currentFile = prevFile
	interp.callerSharingMode = prevCallerSharing
	return result
}

func (interp *Interpreter) executeMethodNode(methodInfo *MethodInfo, args []*Value) *Value {
	mdCtx, ok := methodInfo.Node.(*parser.MethodDeclarationContext)
	if !ok || mdCtx == nil {
		return NullValue()
	}

	className := ""
	if interp.currentClass != nil {
		className = interp.currentClass.Name
	}

	// Trace method entry
	var methodStart time.Time
	if interp.tracer.Enabled() {
		methodStart = time.Now()
		line := 0
		if mdCtx.GetStart() != nil {
			line = mdCtx.GetStart().GetLine()
		}
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventMethodEntry,
			Timestamp: methodStart,
			File:      interp.currentFile,
			Line:      line,
			Class:     className,
			Method:    methodInfo.Name,
		})
	}

	// Create new scope
	prevEnv := interp.env
	interp.env = NewEnvironment(interp.env)

	// Bind parameters
	for i, param := range methodInfo.Params {
		if i < len(args) {
			interp.env.Define(param.Name, args[i])
		} else {
			interp.env.Define(param.Name, NullValue())
		}
	}

	// Execute block
	block := mdCtx.Block()
	var result *Value
	if block != nil {
		r := interp.visitNode(block)
		if ret, ok := r.(*ReturnSignal); ok {
			result = ret.Value
		} else {
			result = safeValue(r)
		}
	} else {
		result = NullValue()
	}

	// Trace method exit
	if interp.tracer.Enabled() {
		interp.tracer.Record(tracer.TraceEvent{
			Type:     tracer.EventMethodExit,
			Timestamp: time.Now(),
			Class:    className,
			Method:   methodInfo.Name,
			Duration: time.Since(methodStart),
		})
	}

	interp.env = prevEnv
	return result
}

func (interp *Interpreter) createInstance(classInfo *ClassInfo, args []*Value) *Value {
	obj := SObjectValue(classInfo.Name, make(map[string]*Value))

	// Initialize fields
	for _, fi := range classInfo.Fields {
		if !fi.IsStatic {
			obj.Data.(map[string]*Value)[fi.Name] = NullValue()
		}
	}

	// Initialize fields with values from field declarations
	for _, fi := range classInfo.Fields {
		if fi.IsStatic {
			continue
		}
		if fi.Node != nil {
			fdCtx, ok := fi.Node.(*parser.FieldDeclarationContext)
			if !ok || fdCtx == nil {
				continue
			}
			vds := fdCtx.VariableDeclarators()
			if vds == nil {
				continue
			}
			vdsCtx, ok := vds.(*parser.VariableDeclaratorsContext)
			if !ok {
				continue
			}
			for _, vd := range vdsCtx.AllVariableDeclarator() {
				vdCtx, ok := vd.(*parser.VariableDeclaratorContext)
				if !ok {
					continue
				}
				name := getIdText(vdCtx.Id())
				if vdCtx.Expression() != nil {
					val := safeValue(interp.visitNode(vdCtx.Expression()))
					obj.Data.(map[string]*Value)[name] = val
				}
			}
		}
	}

	// Run constructor if present and args are provided
	if len(classInfo.Constructors) > 0 && args != nil {
		for _, ci := range classInfo.Constructors {
			if len(ci.Params) == len(args) {
				interp.executeConstructor(ci, obj, args)
				break
			}
		}
	}

	return obj
}

func (interp *Interpreter) executeConstructor(ci *ConstructorInfo, obj *Value, args []*Value) {
	cdCtx, ok := ci.Node.(*parser.ConstructorDeclarationContext)
	if !ok || cdCtx == nil {
		return
	}

	prevEnv := interp.env
	prevThis := interp.thisObj
	interp.env = NewEnvironment(interp.env)
	interp.thisObj = obj

	for i, param := range ci.Params {
		if i < len(args) {
			interp.env.Define(param.Name, args[i])
		}
	}

	if cdCtx.Block() != nil {
		interp.visitNode(cdCtx.Block())
	}

	interp.env = prevEnv
	interp.thisObj = prevThis
}

// ============================================================
// Visitor methods
// ============================================================

func (interp *Interpreter) VisitCompilationUnit(ctx *parser.CompilationUnitContext) any {
	if ctx == nil {
		return NullValue()
	}
	td := ctx.TypeDeclaration()
	if td != nil {
		return interp.visitNode(td)
	}
	return NullValue()
}

func (interp *Interpreter) VisitTypeDeclaration(ctx *parser.TypeDeclarationContext) any {
	if ctx == nil {
		return NullValue()
	}
	if cd := ctx.ClassDeclaration(); cd != nil {
		return interp.visitNode(cd)
	}
	return NullValue()
}

func (interp *Interpreter) VisitClassDeclaration(ctx *parser.ClassDeclarationContext) any {
	// Class declarations are handled by the registry; nothing to execute directly.
	return NullValue()
}

// --- Literals ---

func (interp *Interpreter) VisitLiteral(ctx *parser.LiteralContext) any {
	if ctx == nil {
		return NullValue()
	}
	if ctx.IntegerLiteral() != nil {
		text := ctx.IntegerLiteral().GetText()
		i, err := strconv.Atoi(text)
		if err != nil {
			return NullValue()
		}
		return IntegerValue(i)
	}
	if ctx.LongLiteral() != nil {
		text := ctx.LongLiteral().GetText()
		text = strings.TrimSuffix(text, "L")
		text = strings.TrimSuffix(text, "l")
		i, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return NullValue()
		}
		return LongValue(i)
	}
	if ctx.NumberLiteral() != nil {
		text := ctx.NumberLiteral().GetText()
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return NullValue()
		}
		return DoubleValue(f)
	}
	if ctx.StringLiteral() != nil {
		text := ctx.StringLiteral().GetText()
		// Strip surrounding single quotes
		if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
			text = text[1 : len(text)-1]
		}
		// Handle escape sequences
		text = strings.ReplaceAll(text, "\\'", "'")
		text = strings.ReplaceAll(text, "\\n", "\n")
		text = strings.ReplaceAll(text, "\\t", "\t")
		text = strings.ReplaceAll(text, "\\f", "\f")
		text = strings.ReplaceAll(text, "\\b", "\b")
		text = strings.ReplaceAll(text, "\\r", "\r")
		text = strings.ReplaceAll(text, "\\\\", "\\")
		return StringValue(text)
	}
	if ctx.BooleanLiteral() != nil {
		text := strings.ToLower(ctx.BooleanLiteral().GetText())
		return BooleanValue(text == "true")
	}
	if ctx.NULL() != nil {
		return NullValue()
	}
	return NullValue()
}

// --- Primary expressions ---

func (interp *Interpreter) VisitLiteralPrimary(ctx *parser.LiteralPrimaryContext) any {
	if ctx == nil {
		return NullValue()
	}
	for _, child := range ctx.GetChildren() {
		if litCtx, ok := child.(parser.ILiteralContext); ok {
			return interp.visitNode(litCtx)
		}
	}
	return NullValue()
}

func (interp *Interpreter) VisitIdPrimary(ctx *parser.IdPrimaryContext) any {
	if ctx == nil {
		return NullValue()
	}
	name := ctx.Id().GetText()

	// Check local variables
	if val, ok := interp.env.Get(name); ok {
		return val
	}

	// Check if it's a well-known static class name
	lower := strings.ToLower(name)
	if knownStaticClasses[lower] {
		return StringValue(name)
	}

	// Check if it's a class name (for static access)
	if _, ok := interp.registry.Classes[lower]; ok {
		// Return a special value representing the class reference
		return StringValue(name)
	}

	// Check this object fields
	if interp.thisObj != nil && interp.thisObj.Type == TypeSObject {
		fields := interp.thisObj.Data.(map[string]*Value)
		for k, v := range fields {
			if strings.EqualFold(k, name) {
				return v
			}
		}
	}

	return NullValue()
}

func (interp *Interpreter) VisitThisPrimary(ctx *parser.ThisPrimaryContext) any {
	if interp.thisObj != nil {
		return interp.thisObj
	}
	return NullValue()
}

func (interp *Interpreter) VisitSuperPrimary(ctx *parser.SuperPrimaryContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitTypeRefPrimary(ctx *parser.TypeRefPrimaryContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitSoqlPrimary(ctx *parser.SoqlPrimaryContext) any {
	if ctx == nil {
		return NullValue()
	}
	for _, child := range ctx.GetChildren() {
		if soqlCtx, ok := child.(parser.ISoqlLiteralContext); ok {
			return interp.visitNode(soqlCtx)
		}
	}
	return NullValue()
}

// --- Expressions ---

func (interp *Interpreter) VisitPrimaryExpression(ctx *parser.PrimaryExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	for _, child := range ctx.GetChildren() {
		if rc, ok := child.(antlr.RuleContext); ok {
			return interp.visitNode(rc)
		}
	}
	return NullValue()
}

func (interp *Interpreter) VisitArth1Expression(ctx *parser.Arth1ExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	right := safeValue(interp.visitNode(exprs[1]))

	if ctx.MUL() != nil {
		return left.Multiply(right)
	}
	if ctx.DIV() != nil {
		return left.Divide(right)
	}
	if ctx.MOD() != nil {
		return left.Modulo(right)
	}
	return NullValue()
}

func (interp *Interpreter) VisitArth2Expression(ctx *parser.Arth2ExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	right := safeValue(interp.visitNode(exprs[1]))

	if ctx.ADD() != nil {
		return left.Add(right)
	}
	if ctx.SUB() != nil {
		return left.Subtract(right)
	}
	return NullValue()
}

func (interp *Interpreter) VisitCmpExpression(ctx *parser.CmpExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	right := safeValue(interp.visitNode(exprs[1]))

	// Parse the operator from children tokens
	// The grammar for cmpExpression: expression (GT | LT) ASSIGN? expression
	hasGT := ctx.GT() != nil
	hasLT := ctx.LT() != nil
	hasAssign := ctx.ASSIGN() != nil

	if hasLT && hasAssign {
		// <=
		return BooleanValue(left.LessThan(right) || left.Equals(right))
	}
	if hasGT && hasAssign {
		// >=
		return BooleanValue(left.GreaterThan(right) || left.Equals(right))
	}
	if hasLT {
		return BooleanValue(left.LessThan(right))
	}
	if hasGT {
		return BooleanValue(left.GreaterThan(right))
	}
	return NullValue()
}

func (interp *Interpreter) VisitEqualityExpression(ctx *parser.EqualityExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	right := safeValue(interp.visitNode(exprs[1]))

	if ctx.EQUAL() != nil || ctx.TRIPLEEQUAL() != nil {
		return BooleanValue(left.Equals(right))
	}
	if ctx.NOTEQUAL() != nil || ctx.TRIPLENOTEQUAL() != nil || ctx.LESSANDGREATER() != nil {
		return BooleanValue(!left.Equals(right))
	}
	return NullValue()
}

func (interp *Interpreter) VisitLogAndExpression(ctx *parser.LogAndExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	if !left.IsTruthy() {
		return BooleanValue(false)
	}
	right := safeValue(interp.visitNode(exprs[1]))
	return BooleanValue(right.IsTruthy())
}

func (interp *Interpreter) VisitLogOrExpression(ctx *parser.LogOrExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	if left.IsTruthy() {
		return BooleanValue(true)
	}
	right := safeValue(interp.visitNode(exprs[1]))
	return BooleanValue(right.IsTruthy())
}

func (interp *Interpreter) VisitNegExpression(ctx *parser.NegExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	if ctx.BANG() != nil {
		return BooleanValue(!val.IsTruthy())
	}
	return val
}

func (interp *Interpreter) VisitPreOpExpression(ctx *parser.PreOpExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))

	if ctx.SUB() != nil {
		// Unary minus
		if val.Type == TypeInteger {
			return IntegerValue(-val.Data.(int))
		}
		if val.Type == TypeDouble {
			return DoubleValue(-val.Data.(float64))
		}
		if val.Type == TypeLong {
			return LongValue(-val.Data.(int64))
		}
	}
	if ctx.ADD() != nil {
		return val
	}
	if ctx.INC() != nil {
		result := val.Add(IntegerValue(1))
		interp.assignToExpression(expr, result)
		return result
	}
	if ctx.DEC() != nil {
		result := val.Subtract(IntegerValue(1))
		interp.assignToExpression(expr, result)
		return result
	}
	return val
}

func (interp *Interpreter) VisitPostOpExpression(ctx *parser.PostOpExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	oldVal := val

	if ctx.INC() != nil {
		newVal := val.Add(IntegerValue(1))
		interp.assignToExpression(expr, newVal)
		return oldVal
	}
	if ctx.DEC() != nil {
		newVal := val.Subtract(IntegerValue(1))
		interp.assignToExpression(expr, newVal)
		return oldVal
	}
	return val
}

func (interp *Interpreter) VisitCondExpression(ctx *parser.CondExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 3 {
		return NullValue()
	}
	cond := safeValue(interp.visitNode(exprs[0]))
	if cond.IsTruthy() {
		return interp.visitNode(exprs[1])
	}
	return interp.visitNode(exprs[2])
}

func (interp *Interpreter) VisitCoalExpression(ctx *parser.CoalExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	left := safeValue(interp.visitNode(exprs[0]))
	if left.Type != TypeNull {
		return left
	}
	return interp.visitNode(exprs[1])
}

func (interp *Interpreter) VisitSubExpression(ctx *parser.SubExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	return interp.visitNode(expr)
}

func (interp *Interpreter) VisitCastExpression(ctx *parser.CastExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	// For now, just evaluate the expression (type checking not implemented)
	return interp.visitNode(expr)
}

func (interp *Interpreter) VisitInstanceOfExpression(ctx *parser.InstanceOfExpressionContext) any {
	return BooleanValue(false)
}

func (interp *Interpreter) VisitBitAndExpression(ctx *parser.BitAndExpressionContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitBitOrExpression(ctx *parser.BitOrExpressionContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitBitExpression(ctx *parser.BitExpressionContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitBitNotExpression(ctx *parser.BitNotExpressionContext) any {
	return NullValue()
}

// --- Assignment ---

func (interp *Interpreter) VisitAssignExpression(ctx *parser.AssignExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	right := safeValue(interp.visitNode(exprs[1]))

	// Handle compound assignments
	if ctx.ASSIGN() == nil {
		left := safeValue(interp.visitNode(exprs[0]))
		if ctx.ADD_ASSIGN() != nil {
			right = left.Add(right)
		} else if ctx.SUB_ASSIGN() != nil {
			right = left.Subtract(right)
		} else if ctx.MUL_ASSIGN() != nil {
			right = left.Multiply(right)
		} else if ctx.DIV_ASSIGN() != nil {
			right = left.Divide(right)
		}
	}

	interp.assignToExpression(exprs[0], right)
	return right
}

func (interp *Interpreter) assignToExpression(expr parser.IExpressionContext, value *Value) {
	// Handle IdPrimary (simple variable)
	if primary, ok := expr.(*parser.PrimaryExpressionContext); ok {
		for _, child := range primary.GetChildren() {
			if idPrimary, ok := child.(*parser.IdPrimaryContext); ok {
				name := idPrimary.Id().GetText()
				interp.env.Set(name, value)
				// Also set on this object if applicable
				if interp.thisObj != nil && interp.thisObj.Type == TypeSObject {
					fields := interp.thisObj.Data.(map[string]*Value)
					for k := range fields {
						if strings.EqualFold(k, name) {
							fields[k] = value
							return
						}
					}
				}
				return
			}
		}
	}

	// Handle dot expression (a.b = value)
	if dotExpr, ok := expr.(*parser.DotExpressionContext); ok {
		obj := safeValue(interp.visitNode(dotExpr.Expression()))
		fieldName := ""
		if dotExpr.AnyId() != nil {
			fieldName = dotExpr.AnyId().GetText()
		}
		// Static field assignment: ClassName.field = value
		if obj.Type == TypeString {
			className := obj.Data.(string)
			key := className + "." + fieldName
			// Static fields live in the root environment so they persist across scopes
			root := interp.env
			for root.parent != nil {
				root = root.parent
			}
			root.Set(key, value)
			return
		}
		if obj.Type == TypeSObject {
			m := obj.Data.(map[string]*Value)
			// Case-insensitive: update existing key if it matches
			for k := range m {
				if strings.EqualFold(k, fieldName) {
					m[k] = value
					return
				}
			}
			m[fieldName] = value
		} else if obj.Type == TypeMap {
			m := obj.Data.(map[string]*Value)
			m[fieldName] = value
		}
		return
	}

	// Handle array expression (a[i] = value)
	if arrExpr, ok := expr.(*parser.ArrayExpressionContext); ok {
		exprs := arrExpr.AllExpression()
		if len(exprs) >= 2 {
			obj := safeValue(interp.visitNode(exprs[0]))
			idx := safeValue(interp.visitNode(exprs[1]))
			if obj.Type == TypeList {
				i, ok := idx.toInt()
				if ok {
					elems := obj.Data.([]*Value)
					if i >= 0 && i < len(elems) {
						elems[i] = value
					}
				}
			} else if obj.Type == TypeMap {
				m := obj.Data.(map[string]*Value)
				m[idx.ToString()] = value
			}
		}
		return
	}
}

// --- Dot expression (field/method access) ---

func (interp *Interpreter) VisitDotExpression(ctx *parser.DotExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}

	obj := safeValue(interp.visitNode(ctx.Expression()))

	// Dot method call: obj.method(args)
	if dmc := ctx.DotMethodCall(); dmc != nil {
		dmcCtx, ok := dmc.(*parser.DotMethodCallContext)
		if !ok || dmcCtx == nil {
			return NullValue()
		}
		methodName := ""
		if dmcCtx.AnyId() != nil {
			methodName = dmcCtx.AnyId().GetText()
		}

		args := interp.evaluateExpressionList(dmcCtx.ExpressionList())

		// Check if obj is a class name (static call like System.debug)
		if obj.Type == TypeString {
			possibleClassName := obj.Data.(string)
			lowerClassName := strings.ToLower(possibleClassName)

			// System methods
			if lowerClassName == "system" {
				return interp.callSystemMethod(methodName, args)
			}

			// Assert methods (Assert.areEqual, Assert.isTrue, etc.)
			if lowerClassName == "assert" {
				return interp.callAssertMethod(methodName, args)
			}

			// Test methods (Test.startTest, Test.isRunningTest, etc.)
			if lowerClassName == "test" {
				return interp.callTestMethod(methodName, args)
			}

			// UserInfo methods (UserInfo.getUserId, etc.)
			if lowerClassName == "userinfo" {
				return interp.callUserInfoMethod(methodName, args)
			}

			// Static methods on known types
			if result, ok := interp.callStaticMethod(possibleClassName, methodName, args); ok {
				return result
			}

			// Check registry for static methods on user classes
			if classInfo, ok := interp.registry.Classes[lowerClassName]; ok {
				if mi, ok := classInfo.Methods[strings.ToLower(methodName)]; ok {
					return interp.executeMethodInClass(classInfo, mi, args)
				}
			}
		}

		// Built-in instance methods
		if result, ok := interp.callBuiltinMethod(obj, methodName, args); ok {
			return result
		}

		// User-defined instance methods on SObject-typed objects
		if obj.Type == TypeSObject && obj.SType != "" {
			if classInfo, ok := interp.registry.Classes[strings.ToLower(obj.SType)]; ok {
				if mi, ok := classInfo.Methods[strings.ToLower(methodName)]; ok {
					prevThis := interp.thisObj
					interp.thisObj = obj
					result := interp.executeMethodInClass(classInfo, mi, args)
					interp.thisObj = prevThis
					return result
				}
			}
		}

		return NullValue()
	}

	// Dot field access: obj.field
	if ctx.AnyId() != nil {
		fieldName := ctx.AnyId().GetText()

		// Trigger context property access
		if obj.Type == TypeString && strings.EqualFold(obj.Data.(string), "trigger") {
			return interp.resolveTriggerField(fieldName)
		}

		// Class name for static field access
		if obj.Type == TypeString {
			possibleClassName := obj.Data.(string)
			lowerClassName := strings.ToLower(possibleClassName)
			if classInfo, ok := interp.registry.Classes[lowerClassName]; ok {
				if _, ok := classInfo.Fields[strings.ToLower(fieldName)]; ok {
					// Return static field value from env
					key := possibleClassName + "." + fieldName
					if val, ok := interp.env.Get(key); ok {
						return val
					}
					return NullValue()
				}
			}
		}

		// SObject / Map field access
		if obj.Type == TypeSObject || obj.Type == TypeMap {
			m := obj.Data.(map[string]*Value)
			for k, v := range m {
				if strings.EqualFold(k, fieldName) {
					return v
				}
			}
			return NullValue()
		}
	}

	return NullValue()
}

// --- Method call expression ---

func (interp *Interpreter) VisitMethodCallExpression(ctx *parser.MethodCallExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	mc := ctx.MethodCall()
	if mc == nil {
		return NullValue()
	}
	return interp.visitNode(mc)
}

func (interp *Interpreter) VisitMethodCall(ctx *parser.MethodCallContext) any {
	if ctx == nil {
		return NullValue()
	}

	args := interp.evaluateExpressionList(ctx.ExpressionList())

	if ctx.THIS() != nil {
		// this() constructor call
		if interp.currentClass != nil && interp.thisObj != nil {
			for _, ci := range interp.currentClass.Constructors {
				if len(ci.Params) == len(args) {
					interp.executeConstructor(ci, interp.thisObj, args)
					break
				}
			}
		}
		return NullValue()
	}

	if ctx.SUPER() != nil {
		return NullValue()
	}

	if ctx.Id() == nil {
		return NullValue()
	}

	methodName := ctx.Id().GetText()
	lowerName := strings.ToLower(methodName)

	// Check if it's a method on the current class
	if interp.currentClass != nil {
		if mi, ok := interp.currentClass.Methods[lowerName]; ok {
			if mi.IsStatic {
				return interp.executeMethodNode(mi, args)
			}
			if interp.thisObj != nil {
				prevThis := interp.thisObj
				result := interp.executeMethodNode(mi, args)
				interp.thisObj = prevThis
				return result
			}
		}
	}

	return NullValue()
}

// --- Array expression ---

func (interp *Interpreter) VisitArrayExpression(ctx *parser.ArrayExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	exprs := ctx.AllExpression()
	if len(exprs) < 2 {
		return NullValue()
	}
	obj := safeValue(interp.visitNode(exprs[0]))
	idx := safeValue(interp.visitNode(exprs[1]))

	if obj.Type == TypeList {
		i, ok := idx.toInt()
		if ok {
			elems := obj.Data.([]*Value)
			if i >= 0 && i < len(elems) {
				return elems[i]
			}
		}
	}
	if obj.Type == TypeMap {
		m := obj.Data.(map[string]*Value)
		key := idx.ToString()
		if v, ok := m[key]; ok {
			return v
		}
		return NullValue()
	}
	return NullValue()
}

// --- New instance expression ---

func (interp *Interpreter) VisitNewInstanceExpression(ctx *parser.NewInstanceExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	creator := ctx.Creator()
	if creator == nil {
		return NullValue()
	}
	return interp.visitNode(creator)
}

func (interp *Interpreter) VisitCreator(ctx *parser.CreatorContext) any {
	if ctx == nil {
		return NullValue()
	}

	typeName := ""
	if ctx.CreatedName() != nil {
		typeName = ctx.CreatedName().GetText()
	}
	lowerType := strings.ToLower(typeName)

	// List creation
	if strings.HasPrefix(lowerType, "list<") || ctx.ArrayCreatorRest() != nil {
		if ctx.ArrayCreatorRest() != nil {
			arrCtx, ok := ctx.ArrayCreatorRest().(*parser.ArrayCreatorRestContext)
			if ok && arrCtx != nil {
				if ai := arrCtx.ArrayInitializer(); ai != nil {
					aiCtx, ok := ai.(*parser.ArrayInitializerContext)
					if ok && aiCtx != nil {
						var elements []*Value
						for _, expr := range aiCtx.AllExpression() {
							elements = append(elements, safeValue(interp.visitNode(expr)))
						}
						return ListValue(elements)
					}
				}
			}
		}
		// The parser may use SetCreatorRest for List<T>{...} syntax
		if ctx.SetCreatorRest() != nil {
			setCtx, ok := ctx.SetCreatorRest().(*parser.SetCreatorRestContext)
			if ok && setCtx != nil {
				var elements []*Value
				for _, expr := range setCtx.AllExpression() {
					elements = append(elements, safeValue(interp.visitNode(expr)))
				}
				return ListValue(elements)
			}
		}
		return ListValue(nil)
	}

	// Map creation
	if strings.HasPrefix(lowerType, "map<") {
		if ctx.MapCreatorRest() != nil {
			mapCtx, ok := ctx.MapCreatorRest().(*parser.MapCreatorRestContext)
			if ok && mapCtx != nil {
				entries := make(map[string]*Value)
				for _, pair := range mapCtx.AllMapCreatorRestPair() {
					pairCtx, ok := pair.(*parser.MapCreatorRestPairContext)
					if ok && pairCtx != nil {
						exprs := pairCtx.AllExpression()
						if len(exprs) >= 2 {
							key := safeValue(interp.visitNode(exprs[0]))
							val := safeValue(interp.visitNode(exprs[1]))
							entries[key.ToString()] = val
						}
					}
				}
				return MapValue(entries)
			}
		}
		return MapValue(nil)
	}

	// Set creation
	if strings.HasPrefix(lowerType, "set<") {
		if ctx.SetCreatorRest() != nil {
			setCtx, ok := ctx.SetCreatorRest().(*parser.SetCreatorRestContext)
			if ok && setCtx != nil {
				entries := make(map[string]*Value)
				for _, expr := range setCtx.AllExpression() {
					val := safeValue(interp.visitNode(expr))
					entries[val.ToString()] = val
				}
				return SetValue(entries)
			}
		}
		return SetValue(nil)
	}

	// Class instantiation
	if classInfo, ok := interp.registry.Classes[lowerType]; ok {
		var args []*Value
		if ctx.ClassCreatorRest() != nil {
			crCtx, ok := ctx.ClassCreatorRest().(*parser.ClassCreatorRestContext)
			if ok && crCtx != nil {
				if argCtx := crCtx.Arguments(); argCtx != nil {
					argsCtx, ok := argCtx.(*parser.ArgumentsContext)
					if ok && argsCtx != nil {
						args = interp.evaluateExpressionList(argsCtx.ExpressionList())
					}
				}
			}
		}
		return interp.createInstance(classInfo, args)
	}

	// Generic SObject creation (e.g., new Account() or new Account(Name = 'Test'))
	if ctx.ClassCreatorRest() != nil {
		fields := make(map[string]*Value)
		crCtx, ok := ctx.ClassCreatorRest().(*parser.ClassCreatorRestContext)
		if ok && crCtx != nil {
			if argCtx := crCtx.Arguments(); argCtx != nil {
				argsCtx, ok := argCtx.(*parser.ArgumentsContext)
				if ok && argsCtx != nil && argsCtx.ExpressionList() != nil {
					elCtx, ok := argsCtx.ExpressionList().(*parser.ExpressionListContext)
					if ok && elCtx != nil {
						for _, expr := range elCtx.AllExpression() {
							// Each expression should be an assignment: Name = 'Test'
							if assignExpr, ok := expr.(*parser.AssignExpressionContext); ok {
								exprs := assignExpr.AllExpression()
								if len(exprs) >= 2 {
									fieldName := exprs[0].GetText()
									fieldVal := safeValue(interp.visitNode(exprs[1]))
									fields[fieldName] = fieldVal
								}
							}
						}
					}
				}
			}
		}
		return SObjectValue(typeName, fields)
	}

	return NullValue()
}

// --- Statements ---

func (interp *Interpreter) VisitBlock(ctx *parser.BlockContext) any {
	if ctx == nil {
		return NullValue()
	}
	prevEnv := interp.env
	interp.env = NewEnvironment(interp.env)

	var result any = NullValue()
	for _, stmt := range ctx.AllStatement() {
		result = interp.visitNode(stmt)
		// Check for control flow signals
		if _, ok := result.(*ReturnSignal); ok {
			interp.env = prevEnv
			return result
		}
		if _, ok := result.(*BreakSignal); ok {
			interp.env = prevEnv
			return result
		}
		if _, ok := result.(*ContinueSignal); ok {
			interp.env = prevEnv
			return result
		}
		if _, ok := result.(*ThrowSignal); ok {
			interp.env = prevEnv
			return result
		}
	}
	interp.env = prevEnv
	return result
}

func (interp *Interpreter) VisitStatement(ctx *parser.StatementContext) any {
	if ctx == nil {
		return NullValue()
	}

	// Trace line execution
	if interp.tracer.Enabled() && ctx.GetStart() != nil {
		className := ""
		if interp.currentClass != nil {
			className = interp.currentClass.Name
		}
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventLine,
			Timestamp: time.Now(),
			File:      interp.currentFile,
			Line:      ctx.GetStart().GetLine(),
			Class:     className,
		})
	}

	if s := ctx.Block(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.IfStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.ForStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.WhileStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.DoWhileStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.TryStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.ReturnStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.ThrowStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.BreakStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.ContinueStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.InsertStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.UpdateStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.DeleteStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.UndeleteStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.UpsertStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.LocalVariableDeclarationStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.ExpressionStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.SwitchStatement(); s != nil {
		return interp.visitNode(s)
	}
	if s := ctx.RunAsStatement(); s != nil {
		return interp.visitNode(s)
	}
	return NullValue()
}

func (interp *Interpreter) VisitLocalVariableDeclarationStatement(ctx *parser.LocalVariableDeclarationStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	return interp.visitNode(ctx.LocalVariableDeclaration())
}

func (interp *Interpreter) VisitLocalVariableDeclaration(ctx *parser.LocalVariableDeclarationContext) any {
	if ctx == nil {
		return NullValue()
	}
	// Determine if this is a non-list type declaration (for SOQL auto-unwrap)
	isListType := false
	if tr := ctx.TypeRef(); tr != nil {
		typeText := strings.ToLower(tr.GetText())
		if strings.HasPrefix(typeText, "list<") || strings.HasSuffix(typeText, "[]") {
			isListType = true
		}
	}

	vds := ctx.VariableDeclarators()
	if vds == nil {
		return NullValue()
	}
	result := safeValue(interp.visitNode(vds))

	// SOQL auto-unwrap: if assigning a List to a non-List variable, take the first element
	if !isListType && result.Type == TypeList {
		elems := result.Data.([]*Value)
		if len(elems) > 0 {
			// Re-define the variable with the unwrapped value
			vdsCtx, ok := vds.(*parser.VariableDeclaratorsContext)
			if ok {
				for _, vd := range vdsCtx.AllVariableDeclarator() {
					vdCtx, ok := vd.(*parser.VariableDeclaratorContext)
					if ok && vdCtx != nil {
						name := getIdText(vdCtx.Id())
						interp.env.Set(name, elems[0])
						return elems[0]
					}
				}
			}
		} else {
			return NullValue()
		}
	}

	return result
}

func (interp *Interpreter) VisitVariableDeclarators(ctx *parser.VariableDeclaratorsContext) any {
	if ctx == nil {
		return NullValue()
	}
	var lastVal *Value
	for _, vd := range ctx.AllVariableDeclarator() {
		lastVal = safeValue(interp.visitNode(vd))
	}
	if lastVal == nil {
		return NullValue()
	}
	return lastVal
}

func (interp *Interpreter) VisitVariableDeclarator(ctx *parser.VariableDeclaratorContext) any {
	if ctx == nil {
		return NullValue()
	}
	name := getIdText(ctx.Id())
	var val *Value
	if ctx.Expression() != nil {
		val = safeValue(interp.visitNode(ctx.Expression()))
	} else {
		val = NullValue()
	}
	interp.env.Define(name, val)
	return val
}

func (interp *Interpreter) VisitExpressionStatement(ctx *parser.ExpressionStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	return interp.visitNode(ctx.Expression())
}

// --- If statement ---

func (interp *Interpreter) VisitIfStatement(ctx *parser.IfStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	parExpr := ctx.ParExpression()
	if parExpr == nil {
		return NullValue()
	}
	cond := safeValue(interp.visitNode(parExpr))

	stmts := ctx.AllStatement()
	if cond.IsTruthy() {
		if len(stmts) > 0 {
			return interp.visitNode(stmts[0])
		}
	} else {
		if len(stmts) > 1 {
			return interp.visitNode(stmts[1])
		}
	}
	return NullValue()
}

func (interp *Interpreter) VisitParExpression(ctx *parser.ParExpressionContext) any {
	if ctx == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	return interp.visitNode(expr)
}

// --- For statement ---

func (interp *Interpreter) VisitForStatement(ctx *parser.ForStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	forCtrl := ctx.ForControl()
	if forCtrl == nil {
		return NullValue()
	}
	forCtrlCtx, ok := forCtrl.(*parser.ForControlContext)
	if !ok || forCtrlCtx == nil {
		return NullValue()
	}

	stmt := ctx.Statement()

	// Enhanced for loop: for (Type var : collection)
	if efc := forCtrlCtx.EnhancedForControl(); efc != nil {
		return interp.executeEnhancedFor(efc, stmt)
	}

	// Traditional for loop: for (init; cond; update)
	return interp.executeTraditionalFor(forCtrlCtx, stmt)
}

func (interp *Interpreter) executeEnhancedFor(efc parser.IEnhancedForControlContext, stmt parser.IStatementContext) any {
	efcCtx, ok := efc.(*parser.EnhancedForControlContext)
	if !ok || efcCtx == nil {
		return NullValue()
	}

	varName := getIdText(efcCtx.Id())
	collection := safeValue(interp.visitNode(efcCtx.Expression()))

	prevEnv := interp.env
	interp.env = NewEnvironment(interp.env)
	interp.env.Define(varName, NullValue())

	var elements []*Value
	if collection.Type == TypeList {
		elements = collection.Data.([]*Value)
	} else if collection.Type == TypeSet {
		m := collection.Data.(map[string]*Value)
		for _, v := range m {
			elements = append(elements, v)
		}
	}

	for _, elem := range elements {
		interp.env.Set(varName, elem)
		result := interp.visitNode(stmt)
		if _, ok := result.(*BreakSignal); ok {
			break
		}
		if _, ok := result.(*ContinueSignal); ok {
			continue
		}
		if _, ok := result.(*ReturnSignal); ok {
			interp.env = prevEnv
			return result
		}
	}

	interp.env = prevEnv
	return NullValue()
}

func (interp *Interpreter) executeTraditionalFor(forCtrl *parser.ForControlContext, stmt parser.IStatementContext) any {
	prevEnv := interp.env
	interp.env = NewEnvironment(interp.env)

	// Init
	if fi := forCtrl.ForInit(); fi != nil {
		interp.visitNode(fi)
	}

	// Loop
	for {
		// Condition
		if condExpr := forCtrl.Expression(); condExpr != nil {
			cond := safeValue(interp.visitNode(condExpr))
			if !cond.IsTruthy() {
				break
			}
		}

		// Body
		result := interp.visitNode(stmt)
		if _, ok := result.(*BreakSignal); ok {
			break
		}
		if _, ok := result.(*ContinueSignal); ok {
			// Continue to update
		}
		if _, ok := result.(*ReturnSignal); ok {
			interp.env = prevEnv
			return result
		}

		// Update
		if fu := forCtrl.ForUpdate(); fu != nil {
			interp.visitNode(fu)
		}
	}

	interp.env = prevEnv
	return NullValue()
}

func (interp *Interpreter) VisitForControl(ctx *parser.ForControlContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitForInit(ctx *parser.ForInitContext) any {
	if ctx == nil {
		return NullValue()
	}
	if lvd := ctx.LocalVariableDeclaration(); lvd != nil {
		return interp.visitNode(lvd)
	}
	if el := ctx.ExpressionList(); el != nil {
		return interp.visitNode(el)
	}
	return NullValue()
}

func (interp *Interpreter) VisitForUpdate(ctx *parser.ForUpdateContext) any {
	if ctx == nil {
		return NullValue()
	}
	if el := ctx.ExpressionList(); el != nil {
		return interp.visitNode(el)
	}
	return NullValue()
}

func (interp *Interpreter) VisitExpressionList(ctx *parser.ExpressionListContext) any {
	if ctx == nil {
		return NullValue()
	}
	var last any = NullValue()
	for _, expr := range ctx.AllExpression() {
		last = interp.visitNode(expr)
	}
	return last
}

func (interp *Interpreter) VisitEnhancedForControl(ctx *parser.EnhancedForControlContext) any {
	return NullValue()
}

// --- While statement ---

func (interp *Interpreter) VisitWhileStatement(ctx *parser.WhileStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	parExpr := ctx.ParExpression()
	stmt := ctx.Statement()
	if parExpr == nil {
		return NullValue()
	}

	for {
		cond := safeValue(interp.visitNode(parExpr))
		if !cond.IsTruthy() {
			break
		}
		result := interp.visitNode(stmt)
		if _, ok := result.(*BreakSignal); ok {
			break
		}
		if _, ok := result.(*ContinueSignal); ok {
			continue
		}
		if _, ok := result.(*ReturnSignal); ok {
			return result
		}
	}
	return NullValue()
}

// --- Do-while statement ---

func (interp *Interpreter) VisitDoWhileStatement(ctx *parser.DoWhileStatementContext) any {
	if ctx == nil {
		return NullValue()
	}
	parExpr := ctx.ParExpression()
	stmt := ctx.Statement()
	if parExpr == nil || stmt == nil {
		return NullValue()
	}

	for {
		result := interp.visitNode(stmt)
		if _, ok := result.(*BreakSignal); ok {
			break
		}
		if _, ok := result.(*ContinueSignal); ok {
			// continue to check condition
		}
		if _, ok := result.(*ReturnSignal); ok {
			return result
		}

		cond := safeValue(interp.visitNode(parExpr))
		if !cond.IsTruthy() {
			break
		}
	}
	return NullValue()
}

// --- Return statement ---

func (interp *Interpreter) VisitReturnStatement(ctx *parser.ReturnStatementContext) any {
	if ctx == nil {
		return &ReturnSignal{Value: NullValue()}
	}
	expr := ctx.Expression()
	if expr != nil {
		val := safeValue(interp.visitNode(expr))
		return &ReturnSignal{Value: val}
	}
	return &ReturnSignal{Value: NullValue()}
}

// --- Throw statement ---

func (interp *Interpreter) VisitThrowStatement(ctx *parser.ThrowStatementContext) any {
	if ctx == nil {
		return &ThrowSignal{Value: NullValue()}
	}
	expr := ctx.Expression()
	if expr != nil {
		val := safeValue(interp.visitNode(expr))
		return &ThrowSignal{Value: val}
	}
	return &ThrowSignal{Value: NullValue()}
}

// --- Break / Continue ---

func (interp *Interpreter) VisitBreakStatement(ctx *parser.BreakStatementContext) any {
	return &BreakSignal{}
}

func (interp *Interpreter) VisitContinueStatement(ctx *parser.ContinueStatementContext) any {
	return &ContinueSignal{}
}

// --- Try / Catch / Finally ---

func (interp *Interpreter) VisitTryStatement(ctx *parser.TryStatementContext) any {
	if ctx == nil {
		return NullValue()
	}

	var result any
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Handle caught exception
				for _, cc := range ctx.AllCatchClause() {
					ccCtx, ok := cc.(*parser.CatchClauseContext)
					if !ok || ccCtx == nil {
						continue
					}
					// Bind exception variable
					prevEnv := interp.env
					interp.env = NewEnvironment(interp.env)
					exName := getIdText(ccCtx.Id())
					if exName != "" {
						var exVal *Value
						if assertErr, ok := r.(*AssertException); ok {
							exVal = makeExceptionValue("System.AssertException", assertErr.Message)
						} else if ts, ok := r.(*ThrowSignal); ok {
							exVal = ts.Value
						} else if arithErr, ok := r.(*ArithmeticError); ok {
							exVal = makeExceptionValue("System.MathException", arithErr.Message)
						} else if npe, ok := r.(*NullPointerError); ok {
							exVal = makeExceptionValue("System.NullPointerException", npe.Message)
						} else {
							exVal = makeExceptionValue("System.Exception", fmt.Sprintf("%v", r))
						}
						interp.env.Define(exName, exVal)
					}
					result = interp.visitNode(ccCtx.Block())
					interp.env = prevEnv
					return
				}
				// Re-panic if not caught
				panic(r)
			}
		}()

		blockResult := interp.visitNode(ctx.Block())
		// Check if result is a ThrowSignal
		if ts, ok := blockResult.(*ThrowSignal); ok {
			panic(ts)
		}
		result = blockResult
	}()

	// Finally block
	if fb := ctx.FinallyBlock(); fb != nil {
		fbCtx, ok := fb.(*parser.FinallyBlockContext)
		if ok && fbCtx != nil && fbCtx.Block() != nil {
			interp.visitNode(fbCtx.Block())
		}
	}

	return result
}

// --- System.runAs ---

func (interp *Interpreter) VisitRunAsStatement(ctx *parser.RunAsStatementContext) any {
	if ctx == nil {
		return NullValue()
	}

	// Evaluate user argument
	var userVal *Value
	if el := ctx.ExpressionList(); el != nil {
		elCtx, ok := el.(*parser.ExpressionListContext)
		if ok && elCtx != nil {
			exprs := elCtx.AllExpression()
			if len(exprs) > 0 {
				userVal = safeValue(interp.visitNode(exprs[0]))
			}
		}
	}

	// Extract user fields from the SObject
	var newCtx *executionContext
	if userVal != nil && userVal.Type == TypeSObject {
		fields := userVal.Data.(map[string]*Value)
		userID := ""
		if idVal, ok := fields["Id"]; ok && idVal != nil {
			userID = idVal.ToString()
		}
		newCtx = &executionContext{
			userID:     userID,
			userFields: fields,
		}
	} else {
		// Fallback: no user object, just create empty context
		newCtx = &executionContext{
			userID:     "",
			userFields: make(map[string]*Value),
		}
	}

	// Save and set context
	prevCtx := interp.execCtx
	interp.execCtx = newCtx

	// Execute block
	var result any
	if blk := ctx.Block(); blk != nil {
		result = interp.visitNode(blk)
	}

	// Restore context
	interp.execCtx = prevCtx

	return result
}

// --- DML statements ---

func (interp *Interpreter) VisitInsertStatement(ctx *parser.InsertStatementContext) any {
	if ctx == nil || interp.engine == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	records := valueToRecords(val)
	typeName := extractSObjectType(val)

	// Check DML AS USER/SYSTEM
	if ctx.AS() != nil && ctx.USER() != nil {
		interp.checkCRUDPermission(typeName, "create")
	}

	// Auto-set OwnerId when inside runAs
	if interp.execCtx != nil && interp.execCtx.userID != "" {
		interp.setOwnerIdOnRecords(val)
	}

	interp.fireTriggers("BEFORE", "INSERT", typeName, val, nil)

	start := time.Now()
	records = valueToRecords(val) // re-extract in case before trigger modified fields
	if err := interp.engine.Insert(typeName, records); err != nil {
		panic(&ThrowSignal{Value: StringValue(fmt.Sprintf("DML INSERT error: %v", err))})
	}
	writeBackIds(val, records)
	interp.traceDML("INSERT", typeName, len(records), start)

	interp.fireTriggers("AFTER", "INSERT", typeName, val, nil)
	return NullValue()
}

// setOwnerIdOnRecords sets OwnerId on SObject values if not already set.
func (interp *Interpreter) setOwnerIdOnRecords(val *Value) {
	if interp.execCtx == nil || interp.execCtx.userID == "" {
		return
	}
	ownerID := interp.execCtx.userID
	setOwner := func(v *Value) {
		if v.Type == TypeSObject {
			fields := v.Data.(map[string]*Value)
			if _, ok := fields["OwnerId"]; !ok {
				fields["OwnerId"] = StringValue(ownerID)
			}
		}
	}
	if val.Type == TypeSObject {
		setOwner(val)
	} else if val.Type == TypeList {
		for _, item := range val.Data.([]*Value) {
			setOwner(item)
		}
	}
}

func (interp *Interpreter) VisitUpdateStatement(ctx *parser.UpdateStatementContext) any {
	if ctx == nil || interp.engine == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	typeName := extractSObjectType(val)

	// Check DML AS USER/SYSTEM
	if ctx.AS() != nil && ctx.USER() != nil {
		interp.checkCRUDPermission(typeName, "edit")
	}

	// For update, the "old" values are the current DB state (simplified: use same as new)
	interp.fireTriggers("BEFORE", "UPDATE", typeName, val, val)

	records := valueToRecords(val)
	start := time.Now()
	if err := interp.engine.Update(typeName, records); err != nil {
		panic(&ThrowSignal{Value: StringValue(fmt.Sprintf("DML UPDATE error: %v", err))})
	}
	interp.traceDML("UPDATE", typeName, len(records), start)

	interp.fireTriggers("AFTER", "UPDATE", typeName, val, val)
	return NullValue()
}

func (interp *Interpreter) VisitDeleteStatement(ctx *parser.DeleteStatementContext) any {
	if ctx == nil || interp.engine == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	typeName := extractSObjectType(val)

	// Check DML AS USER/SYSTEM
	if ctx.AS() != nil && ctx.USER() != nil {
		interp.checkCRUDPermission(typeName, "delete")
	}

	interp.fireTriggers("BEFORE", "DELETE", typeName, nil, val)

	records := valueToRecords(val)
	start := time.Now()
	if err := interp.engine.Delete(typeName, records); err != nil {
		panic(&ThrowSignal{Value: StringValue(fmt.Sprintf("DML DELETE error: %v", err))})
	}
	interp.traceDML("DELETE", typeName, len(records), start)

	interp.fireTriggers("AFTER", "DELETE", typeName, nil, val)
	return NullValue()
}

func (interp *Interpreter) VisitUndeleteStatement(ctx *parser.UndeleteStatementContext) any {
	return NullValue()
}

func (interp *Interpreter) VisitUpsertStatement(ctx *parser.UpsertStatementContext) any {
	if ctx == nil || interp.engine == nil {
		return NullValue()
	}
	expr := ctx.Expression()
	if expr == nil {
		return NullValue()
	}
	val := safeValue(interp.visitNode(expr))
	typeName := extractSObjectType(val)
	externalIdField := "Id"
	if qf := ctx.QualifiedName(); qf != nil {
		externalIdField = qf.GetText()
	}

	interp.fireTriggers("BEFORE", "INSERT", typeName, val, nil)

	records := valueToRecords(val)
	start := time.Now()
	if err := interp.engine.Upsert(typeName, records, externalIdField); err != nil {
		panic(&ThrowSignal{Value: StringValue(fmt.Sprintf("DML UPSERT error: %v", err))})
	}
	writeBackIds(val, records)
	interp.traceDML("UPSERT", typeName, len(records), start)

	interp.fireTriggers("AFTER", "INSERT", typeName, val, nil)
	return NullValue()
}

func valueToRecords(val *Value) []map[string]any {
	var records []map[string]any
	if val.Type == TypeSObject {
		rec := make(map[string]any)
		for k, v := range val.Data.(map[string]*Value) {
			rec[k] = v.ToGoValue()
		}
		records = append(records, rec)
	} else if val.Type == TypeList {
		for _, elem := range val.Data.([]*Value) {
			if elem.Type == TypeSObject {
				rec := make(map[string]any)
				for k, v := range elem.Data.(map[string]*Value) {
					rec[k] = v.ToGoValue()
				}
				records = append(records, rec)
			}
		}
	}
	return records
}

// writeBackIds copies generated IDs from engine records back to the original Apex values.
func writeBackIds(val *Value, records []map[string]any) {
	if val.Type == TypeSObject && len(records) == 1 {
		writeBackId(val.Data.(map[string]*Value), records[0])
	} else if val.Type == TypeList {
		elems := val.Data.([]*Value)
		for i, elem := range elems {
			if elem.Type == TypeSObject && i < len(records) {
				writeBackId(elem.Data.(map[string]*Value), records[i])
			}
		}
	}
}

func writeBackId(fields map[string]*Value, rec map[string]any) {
	id, ok := rec["Id"]
	if !ok {
		return
	}
	idStr := StringValue(fmt.Sprintf("%v", id))
	// Update existing Id key (case-insensitive) or create one
	for k := range fields {
		if strings.EqualFold(k, "Id") {
			fields[k] = idStr
			return
		}
	}
	fields["Id"] = idStr
}

// --- SOQL ---

func (interp *Interpreter) VisitSoqlLiteral(ctx *parser.SoqlLiteralContext) any {
	if ctx == nil {
		return NullValue()
	}
	queryCtx := ctx.Query()
	if queryCtx == nil {
		return ListValue(nil)
	}
	qCtx, ok := queryCtx.(*parser.QueryContext)
	if !ok || qCtx == nil {
		return ListValue(nil)
	}

	// Build a bind resolver that evaluates Apex expressions via the interpreter.
	resolver := func(be *parser.BoundExpressionContext) any {
		expr := be.Expression()
		if expr != nil {
			val := safeValue(interp.visitNode(expr))
			return val.ToGoValue()
		}
		return nil
	}
	params := extractQueryParams(qCtx, resolver)

	// Apply sharing filter: when "with sharing" and inside runAs, restrict to owned records.
	interp.applySharingFilter(params)

	if interp.engine == nil {
		return ListValue(nil)
	}

	queryText := ctx.GetText()
	// Strip surrounding [ ]
	if len(queryText) >= 2 {
		queryText = queryText[1 : len(queryText)-1]
	}

	start := time.Now()
	results, err := interp.engine.QueryWithFullParams(params)
	if err != nil {
		return ListValue(nil)
	}
	if interp.tracer.Enabled() {
		line := 0
		if ctx.GetStart() != nil {
			line = ctx.GetStart().GetLine()
		}
		interp.tracer.Record(tracer.TraceEvent{
			Type:      tracer.EventSOQL,
			Timestamp: time.Now(),
			File:      interp.currentFile,
			Line:      line,
			Detail:    queryText,
			Duration:  time.Since(start),
			RowCount:  len(results),
		})
	}

	sobjectType := params.SObject
	if params.IsAggregate {
		sobjectType = "AggregateResult"
	}

	// Apply field-level security after query
	interp.applyFieldSecurity(params, results)

	return queryResultsToList(results, sobjectType)
}

// applySharingFilter appends an OwnerId filter when sharing is enforced.
// Sharing is enforced when:
//   - The query uses WITH USER_MODE (always enforces sharing), OR
//   - The class uses "with sharing" (or "inherited sharing" resolves to "with")
//     AND the query does NOT use WITH SYSTEM_MODE
//
// SECURITY_ENFORCED does NOT enforce sharing (only CRUD+FLS).
func (interp *Interpreter) applySharingFilter(params *engine.QueryParams) {
	if interp.execCtx == nil || interp.execCtx.userID == "" {
		return
	}

	enforceSharing := false
	if params.AccessMode == "USER_MODE" {
		// USER_MODE always enforces sharing
		enforceSharing = true
	} else if params.AccessMode == "SYSTEM_MODE" {
		// SYSTEM_MODE never enforces sharing
		enforceSharing = false
	} else {
		// No explicit access mode or SECURITY_ENFORCED: use class-level sharing keyword
		// SECURITY_ENFORCED does not enforce sharing, only CRUD+FLS
		if params.AccessMode != "SECURITY_ENFORCED" {
			enforceSharing = interp.effectiveSharingMode() == "with"
		}
	}

	if !enforceSharing {
		return
	}
	if params.Where != "" {
		params.Where += ` AND "ownerid" = ?`
	} else {
		params.Where = `"ownerid" = ?`
	}
	params.WhereArgs = append(params.WhereArgs, interp.execCtx.userID)
}

// applyFieldSecurity enforces field-level security for USER_MODE and SECURITY_ENFORCED.
// Both modes throw on inaccessible fields (Salesforce docs). Stripping fields silently
// is the behavior of stripInaccessible(), not these access modes.
func (interp *Interpreter) applyFieldSecurity(params *engine.QueryParams, results []map[string]any) {
	if params.AccessMode == "" || params.AccessMode == "SYSTEM_MODE" {
		return
	}
	if interp.execCtx == nil {
		return
	}

	sobjectType := params.SObject
	for _, fieldName := range params.Fields {
		lower := strings.ToLower(fieldName)
		// Skip standard fields that are always accessible
		if lower == "id" || lower == "name" {
			continue
		}
		accessible := interp.checkFieldPermission(sobjectType, fieldName, "read")
		if !accessible {
			panic(&ThrowSignal{Value: StringValue(
				"Insufficient field privileges: " + fieldName + " on " + sobjectType)})
		}
	}
}

// bindResolver is a function that resolves a bound expression (:expr) to a Go value.
type bindResolver func(*parser.BoundExpressionContext) any

// extractQueryParams extracts fields, sobject, where, orderBy, limit, offset from a parsed SOQL query.
// The resolver is used to evaluate bound expressions; it may be nil if no binds are expected.
func extractQueryParams(qCtx *parser.QueryContext, resolver bindResolver) *engine.QueryParams {
	params := &engine.QueryParams{}

	// Extract fields from SELECT list.
	if sl := qCtx.SelectList(); sl != nil {
		slCtx, ok := sl.(*parser.SelectListContext)
		if ok && slCtx != nil {
			for _, se := range slCtx.AllSelectEntry() {
				seCtx, ok := se.(*parser.SelectEntryContext)
				if !ok || seCtx == nil {
					continue
				}

				// Check for TYPEOF polymorphic expression.
				if toCtx := seCtx.TypeOf(); toCtx != nil {
					toParsed, ok := toCtx.(*parser.TypeOfContext)
					if ok && toParsed != nil {
						tof := extractTypeOfField(toParsed)
						params.TypeOfFields = append(params.TypeOfFields, tof)
					}
					continue
				}

				// Check for child subquery: (SELECT ... FROM ...)
				if sq := seCtx.SubQuery(); sq != nil {
					sqCtx, ok := sq.(*parser.SubQueryContext)
					if ok && sqCtx != nil {
						childSQ := extractChildSubQuery(sqCtx, resolver)
						params.SubQueries = append(params.SubQueries, childSQ)
					}
					continue
				}

				// Check for aggregate function (COUNT, SUM, etc.).
				if sf := seCtx.SoqlFunction(); sf != nil {
					sfCtx, ok := sf.(*parser.SoqlFunctionContext)
					if ok && sfCtx != nil {
						funcSQL := soqlFunctionToSQL(sfCtx)
						alias := ""
						if sid := seCtx.SoqlId(); sid != nil {
							alias = strings.ToLower(sid.GetText())
						}
						if alias == "" {
							alias = fmt.Sprintf("expr%d", len(params.AggregateFields))
						}
						params.AggregateFields = append(params.AggregateFields, engine.AggregateField{
							FunctionSQL: funcSQL,
							Alias:       alias,
						})
						params.IsAggregate = true
						continue
					}
				}

				// Check for dotted field name (parent relationship).
				if fn := seCtx.FieldName(); fn != nil {
					fnCtx, ok := fn.(*parser.FieldNameContext)
					if ok && fnCtx != nil {
						ids := fnCtx.AllSoqlId()
						if len(ids) >= 2 {
							path := make([]string, len(ids)-1)
							for j := 0; j < len(ids)-1; j++ {
								path[j] = ids[j].GetText()
							}
							fieldName := ids[len(ids)-1].GetText()
							params.ParentFields = append(params.ParentFields, engine.ParentField{
								Path:      path,
								FieldName: fieldName,
							})
							continue
						}
					}
				}

				// Simple field.
				params.Fields = append(params.Fields, se.GetText())
			}
		}
	}

	// Extract FROM.
	if fnl := qCtx.FromNameList(); fnl != nil {
		params.SObject = fnl.GetText()
	}

	// Extract WHERE by walking the AST.
	if wc := qCtx.WhereClause(); wc != nil {
		wcCtx, ok := wc.(*parser.WhereClauseContext)
		if ok && wcCtx != nil {
			if le := wcCtx.LogicalExpression(); le != nil {
				leCtx, ok := le.(*parser.LogicalExpressionContext)
				if ok && leCtx != nil {
					var args []any
					params.Where = buildLogicalExpression(leCtx, resolver, &args)
					params.WhereArgs = args
				}
			}
		}
	}

	// Collect semi-join subquery table names from WHERE clause.
	if wc := qCtx.WhereClause(); wc != nil {
		params.SubQueryTables = collectSubQueryTables(wc)
	}

	// Extract ORDER BY by walking the AST.
	if oc := qCtx.OrderByClause(); oc != nil {
		ocCtx, ok := oc.(*parser.OrderByClauseContext)
		if ok && ocCtx != nil {
			params.OrderBy = buildOrderByClause(ocCtx)
		}
	}

	// Extract LIMIT.
	if lc := qCtx.LimitClause(); lc != nil {
		lcCtx, ok := lc.(*parser.LimitClauseContext)
		if ok && lcCtx != nil {
			if lit := lcCtx.IntegerLiteral(); lit != nil {
				if n, err := strconv.Atoi(lit.GetText()); err == nil {
					params.Limit = n
				}
			}
		}
	}

	// Extract OFFSET.
	if oc := qCtx.OffsetClause(); oc != nil {
		ocCtx, ok := oc.(*parser.OffsetClauseContext)
		if ok && ocCtx != nil {
			if lit := ocCtx.IntegerLiteral(); lit != nil {
				if n, err := strconv.Atoi(lit.GetText()); err == nil {
					params.Offset = n
				}
			}
		}
	}

	// Extract GROUP BY and HAVING.
	if gc := qCtx.GroupByClause(); gc != nil {
		gcCtx, ok := gc.(*parser.GroupByClauseContext)
		if ok && gcCtx != nil {
			if sl := gcCtx.SelectList(); sl != nil {
				slCtx, ok := sl.(*parser.SelectListContext)
				if ok && slCtx != nil {
					params.GroupBy = selectEntriesToSQL(slCtx)
				}
			}
			if gcCtx.HAVING() != nil {
				if le := gcCtx.LogicalExpression(); le != nil {
					leCtx, ok := le.(*parser.LogicalExpressionContext)
					if ok && leCtx != nil {
						var havingArgs []any
						params.Having = buildLogicalExpression(leCtx, resolver, &havingArgs)
						params.HavingArgs = havingArgs
					}
				}
			}
		}
	}

	// Extract WITH clause (USER_MODE / SYSTEM_MODE / SECURITY_ENFORCED).
	if wc := qCtx.WithClause(); wc != nil {
		wcCtx, ok := wc.(*parser.WithClauseContext)
		if ok && wcCtx != nil {
			if wcCtx.USER_MODE() != nil {
				params.AccessMode = "USER_MODE"
			} else if wcCtx.SYSTEM_MODE() != nil {
				params.AccessMode = "SYSTEM_MODE"
			} else if wcCtx.SECURITY_ENFORCED() != nil {
				params.AccessMode = "SECURITY_ENFORCED"
			}
		}
	}

	return params
}

// soqlFunctionToSQL converts a SoqlFunctionContext into valid SQL.
// COUNT() → COUNT(*), COUNT(field) → COUNT("field"), COUNT_DISTINCT(field) → COUNT(DISTINCT "field"),
// AVG/SUM/MIN/MAX(field) → AVG/SUM/MIN/MAX("field").
func soqlFunctionToSQL(ctx *parser.SoqlFunctionContext) string {
	if ctx.COUNT() != nil {
		if fn := ctx.FieldName(); fn != nil {
			fnCtx, ok := fn.(*parser.FieldNameContext)
			if ok && fnCtx != nil {
				return fmt.Sprintf("COUNT(%q)", strings.ToLower(fieldNameToSQL(fnCtx)))
			}
		}
		return "COUNT(*)"
	}
	if ctx.COUNT_DISTINCT() != nil {
		if fn := ctx.FieldName(); fn != nil {
			fnCtx, ok := fn.(*parser.FieldNameContext)
			if ok && fnCtx != nil {
				return fmt.Sprintf("COUNT(DISTINCT %q)", strings.ToLower(fieldNameToSQL(fnCtx)))
			}
		}
		return "COUNT(*)"
	}
	// AVG, SUM, MIN, MAX
	for _, name := range []string{"AVG", "SUM", "MIN", "MAX"} {
		var node antlr.TerminalNode
		switch name {
		case "AVG":
			node = ctx.AVG()
		case "SUM":
			node = ctx.SUM()
		case "MIN":
			node = ctx.MIN()
		case "MAX":
			node = ctx.MAX()
		}
		if node != nil {
			if fn := ctx.FieldName(); fn != nil {
				fnCtx, ok := fn.(*parser.FieldNameContext)
				if ok && fnCtx != nil {
					return fmt.Sprintf("%s(%q)", name, strings.ToLower(fieldNameToSQL(fnCtx)))
				}
			}
			return name + "(*)"
		}
	}
	// Fallback: use raw text.
	return ctx.GetText()
}

// selectEntryToSQL converts a single SelectEntryContext to a SQL expression.
// Used by both GROUP BY extraction and anywhere a field-or-function entry needs SQL.
func selectEntryToSQL(seCtx *parser.SelectEntryContext) string {
	if fn := seCtx.FieldName(); fn != nil {
		fnCtx, ok := fn.(*parser.FieldNameContext)
		if ok && fnCtx != nil {
			return fmt.Sprintf("%q", strings.ToLower(fieldNameToSQL(fnCtx)))
		}
	}
	if sf := seCtx.SoqlFunction(); sf != nil {
		sfCtx, ok := sf.(*parser.SoqlFunctionContext)
		if ok && sfCtx != nil {
			return soqlFunctionToSQL(sfCtx)
		}
	}
	return fmt.Sprintf("%q", strings.ToLower(seCtx.GetText()))
}

// selectEntriesToSQL converts all entries in a SelectListContext to a
// comma-separated SQL string. Shared by GROUP BY and other clause extraction.
func selectEntriesToSQL(slCtx *parser.SelectListContext) string {
	var parts []string
	for _, se := range slCtx.AllSelectEntry() {
		seCtx, ok := se.(*parser.SelectEntryContext)
		if !ok || seCtx == nil {
			continue
		}
		parts = append(parts, selectEntryToSQL(seCtx))
	}
	return strings.Join(parts, ", ")
}

// extractTypeOfField extracts a TypeOfField from a TypeOfContext.
// Grammar: TYPEOF fieldName whenClause+ elseClause? END
func extractTypeOfField(toCtx *parser.TypeOfContext) engine.TypeOfField {
	tof := engine.TypeOfField{}

	// The fieldName is the polymorphic relationship name (e.g. "What", "Who").
	if fn := toCtx.FieldName(); fn != nil {
		relName := fn.GetText()
		tof.FieldName = relName
		// Derive FK column: "What" -> "WhatId", "Who" -> "WhoId"
		tof.FKField = relName + "Id"
	}

	// Extract WHEN clauses.
	for _, wc := range toCtx.AllWhenClause() {
		wcCtx, ok := wc.(*parser.WhenClauseContext)
		if !ok || wcCtx == nil {
			continue
		}
		when := engine.TypeOfWhen{}
		if fn := wcCtx.FieldName(); fn != nil {
			when.SObjectType = fn.GetText()
		}
		if fnl := wcCtx.FieldNameList(); fnl != nil {
			fnlCtx, ok := fnl.(*parser.FieldNameListContext)
			if ok && fnlCtx != nil {
				for _, f := range fnlCtx.AllFieldName() {
					when.Fields = append(when.Fields, f.GetText())
				}
			}
		}
		tof.WhenClauses = append(tof.WhenClauses, when)
	}

	// Extract ELSE clause.
	if ec := toCtx.ElseClause(); ec != nil {
		ecCtx, ok := ec.(*parser.ElseClauseContext)
		if ok && ecCtx != nil {
			if fnl := ecCtx.FieldNameList(); fnl != nil {
				fnlCtx, ok := fnl.(*parser.FieldNameListContext)
				if ok && fnlCtx != nil {
					for _, f := range fnlCtx.AllFieldName() {
						tof.ElseFields = append(tof.ElseFields, f.GetText())
					}
				}
			}
		}
	}

	return tof
}

// extractChildSubQuery extracts a ChildSubQuery from a SubQueryContext.
func extractChildSubQuery(sqCtx *parser.SubQueryContext, resolver bindResolver) engine.ChildSubQuery {
	csq := engine.ChildSubQuery{}

	// Extract fields from SubFieldList.
	if sfl := sqCtx.SubFieldList(); sfl != nil {
		sflCtx, ok := sfl.(*parser.SubFieldListContext)
		if ok && sflCtx != nil {
			for _, sfe := range sflCtx.AllSubFieldEntry() {
				csq.Fields = append(csq.Fields, sfe.GetText())
			}
		}
	}

	// Extract FROM (the relationship name, e.g. "Contacts").
	if fnl := sqCtx.FromNameList(); fnl != nil {
		csq.RelationshipName = fnl.GetText()
	}

	// Extract WHERE by walking the AST.
	if wc := sqCtx.WhereClause(); wc != nil {
		wcCtx, ok := wc.(*parser.WhereClauseContext)
		if ok && wcCtx != nil {
			if le := wcCtx.LogicalExpression(); le != nil {
				leCtx, ok := le.(*parser.LogicalExpressionContext)
				if ok && leCtx != nil {
					var args []any
					csq.Where = buildLogicalExpression(leCtx, resolver, &args)
					csq.WhereArgs = args
				}
			}
		}
	}

	// Extract ORDER BY by walking the AST.
	if oc := sqCtx.OrderByClause(); oc != nil {
		ocCtx, ok := oc.(*parser.OrderByClauseContext)
		if ok && ocCtx != nil {
			csq.OrderBy = buildOrderByClause(ocCtx)
		}
	}

	// Extract LIMIT.
	if lc := sqCtx.LimitClause(); lc != nil {
		lcCtx, ok := lc.(*parser.LimitClauseContext)
		if ok && lcCtx != nil {
			if lit := lcCtx.IntegerLiteral(); lit != nil {
				if n, err := strconv.Atoi(lit.GetText()); err == nil {
					csq.Limit = n
				}
			}
		}
	}

	return csq
}

// buildLogicalExpression walks a LogicalExpressionContext and produces a SQL WHERE fragment.
func buildLogicalExpression(ctx *parser.LogicalExpressionContext, resolver bindResolver, args *[]any) string {
	conds := ctx.AllConditionalExpression()

	// NOT prefix.
	if ctx.NOT() != nil && len(conds) == 1 {
		inner := buildConditionalExpression(conds[0].(*parser.ConditionalExpressionContext), resolver, args)
		return "NOT (" + inner + ")"
	}

	// Determine the joining operator (AND or OR).
	joiner := " AND "
	if len(ctx.AllSOQLOR()) > 0 {
		joiner = " OR "
	}

	var parts []string
	for _, c := range conds {
		cCtx, ok := c.(*parser.ConditionalExpressionContext)
		if !ok || cCtx == nil {
			continue
		}
		parts = append(parts, buildConditionalExpression(cCtx, resolver, args))
	}
	return strings.Join(parts, joiner)
}

// buildConditionalExpression walks a ConditionalExpressionContext.
func buildConditionalExpression(ctx *parser.ConditionalExpressionContext, resolver bindResolver, args *[]any) string {
	// Parenthesized logical expression.
	if ctx.LPAREN() != nil {
		if le := ctx.LogicalExpression(); le != nil {
			leCtx, ok := le.(*parser.LogicalExpressionContext)
			if ok && leCtx != nil {
				return "(" + buildLogicalExpression(leCtx, resolver, args) + ")"
			}
		}
	}

	// Field expression: fieldName comparisonOperator value.
	if fe := ctx.FieldExpression(); fe != nil {
		feCtx, ok := fe.(*parser.FieldExpressionContext)
		if ok && feCtx != nil {
			return buildFieldExpression(feCtx, resolver, args)
		}
	}

	return ""
}

// buildFieldExpression walks a FieldExpressionContext and produces a SQL condition.
func buildFieldExpression(ctx *parser.FieldExpressionContext, resolver bindResolver, args *[]any) string {
	// Get the field name (left side).
	var fieldSQL string
	if fn := ctx.FieldName(); fn != nil {
		fnCtx, ok := fn.(*parser.FieldNameContext)
		if ok && fnCtx != nil {
			fieldSQL = fieldNameToSQL(fnCtx)
		}
	} else if sf := ctx.SoqlFunction(); sf != nil {
		sfCtx, ok := sf.(*parser.SoqlFunctionContext)
		if ok && sfCtx != nil {
			fieldSQL = soqlFunctionToSQL(sfCtx)
		} else {
			fieldSQL = sf.GetText()
		}
	}

	// Get the comparison operator.
	op := ""
	isNegated := false
	if co := ctx.ComparisonOperator(); co != nil {
		coCtx, ok := co.(*parser.ComparisonOperatorContext)
		if ok && coCtx != nil {
			op, isNegated = comparisonOpToSQL(coCtx)
		}
	}

	// Get the value (right side).
	if v := ctx.Value(); v != nil {
		return buildValueExpression(fieldSQL, op, isNegated, v, resolver, args)
	}

	return fieldSQL + " " + op
}

// fieldNameToSQL converts a FieldNameContext to a lowercase SQL identifier.
func fieldNameToSQL(ctx *parser.FieldNameContext) string {
	ids := ctx.AllSoqlId()
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strings.ToLower(id.GetText())
	}
	return strings.Join(parts, ".")
}

// comparisonOpToSQL converts a ComparisonOperatorContext to a SQL operator string.
// Returns the operator and whether NOT is present (for NOT IN).
func comparisonOpToSQL(ctx *parser.ComparisonOperatorContext) (string, bool) {
	if ctx.ASSIGN() != nil && ctx.GT() != nil {
		return ">=", false
	}
	if ctx.ASSIGN() != nil && ctx.LT() != nil {
		return "<=", false
	}
	if ctx.ASSIGN() != nil {
		return "=", false
	}
	if ctx.NOTEQUAL() != nil {
		return "!=", false
	}
	if ctx.LESSANDGREATER() != nil {
		return "<>", false
	}
	if ctx.LT() != nil {
		return "<", false
	}
	if ctx.GT() != nil {
		return ">", false
	}
	if ctx.LIKE() != nil {
		return "LIKE", false
	}
	if ctx.NOT() != nil && ctx.IN() != nil {
		return "NOT IN", true
	}
	if ctx.IN() != nil {
		return "IN", false
	}
	if ctx.INCLUDES() != nil {
		return "INCLUDES", false
	}
	if ctx.EXCLUDES() != nil {
		return "EXCLUDES", false
	}
	return "=", false
}

// buildValueExpression produces the SQL for a comparison with a value.
func buildValueExpression(field, op string, _ bool, v parser.IValueContext, resolver bindResolver, args *[]any) string {
	switch val := v.(type) {
	case *parser.NullValueContext:
		if op == "!=" || op == "<>" {
			return field + " IS NOT NULL"
		}
		return field + " IS NULL"

	case *parser.BoundExpressionValueContext:
		if resolver != nil {
			if be := val.BoundExpression(); be != nil {
				beCtx, ok := be.(*parser.BoundExpressionContext)
				if ok && beCtx != nil {
					resolved := resolver(beCtx)
					*args = append(*args, resolved)
					return field + " " + op + " ?"
				}
			}
		}
		// No resolver — keep the bind expression text as a placeholder.
		return field + " " + op + " " + v.GetText()

	case *parser.StringLiteralValueContext:
		text := val.GetText()
		// Strip surrounding quotes.
		if len(text) >= 2 {
			text = text[1 : len(text)-1]
		}
		*args = append(*args, text)
		return field + " " + op + " ?"

	case *parser.SignedNumberValueContext:
		text := val.GetText()
		if n, err := strconv.Atoi(text); err == nil {
			*args = append(*args, n)
		} else if f, err := strconv.ParseFloat(text, 64); err == nil {
			*args = append(*args, f)
		} else {
			*args = append(*args, text)
		}
		return field + " " + op + " ?"

	case *parser.BooleanLiteralValueContext:
		text := strings.ToLower(val.GetText())
		*args = append(*args, text == "true")
		return field + " " + op + " ?"

	case *parser.ValueListValueContext:
		if vl := val.ValueList(); vl != nil {
			vlCtx, ok := vl.(*parser.ValueListContext)
			if ok && vlCtx != nil {
				allVals := vlCtx.AllValue()
				placeholders := make([]string, len(allVals))
				for i, innerV := range allVals {
					placeholders[i] = "?"
					*args = append(*args, soqlValueToGo(innerV))
				}
				return field + " " + op + " (" + strings.Join(placeholders, ", ") + ")"
			}
		}
		return field + " " + op + " " + v.GetText()

	case *parser.SubQueryValueContext:
		if sq := val.SubQuery(); sq != nil {
			sqCtx, ok := sq.(*parser.SubQueryContext)
			if ok && sqCtx != nil {
				subSQL, subArgs := buildSemiJoinSQL(sqCtx, resolver)
				*args = append(*args, subArgs...)
				return field + " " + op + " (" + subSQL + ")"
			}
		}
		return field + " " + op + " " + v.GetText()

	default:
		// Date literals, date formulas, currency values, etc. — use text representation.
		*args = append(*args, v.GetText())
		return field + " " + op + " ?"
	}
}

// buildSemiJoinSQL constructs a SELECT SQL for a semi-join subquery.
func buildSemiJoinSQL(sqCtx *parser.SubQueryContext, resolver bindResolver) (string, []any) {
	var fields []string
	if sfl := sqCtx.SubFieldList(); sfl != nil {
		sflCtx, ok := sfl.(*parser.SubFieldListContext)
		if ok && sflCtx != nil {
			for _, sfe := range sflCtx.AllSubFieldEntry() {
				sfeCtx, ok := sfe.(*parser.SubFieldEntryContext)
				if !ok || sfeCtx == nil {
					continue
				}
				if fn := sfeCtx.FieldName(); fn != nil {
					fnCtx, ok := fn.(*parser.FieldNameContext)
					if ok && fnCtx != nil {
						fields = append(fields, fmt.Sprintf("%q", strings.ToLower(fieldNameToSQL(fnCtx))))
						continue
					}
				}
				fields = append(fields, fmt.Sprintf("%q", strings.ToLower(sfe.GetText())))
			}
		}
	}
	if len(fields) == 0 {
		fields = []string{`"id"`}
	}

	tableName := ""
	if fnl := sqCtx.FromNameList(); fnl != nil {
		tableName = strings.ToLower(fnl.GetText())
	}

	sql := fmt.Sprintf("SELECT %s FROM %q", strings.Join(fields, ", "), tableName)

	var args []any
	if wc := sqCtx.WhereClause(); wc != nil {
		wcCtx, ok := wc.(*parser.WhereClauseContext)
		if ok && wcCtx != nil {
			if le := wcCtx.LogicalExpression(); le != nil {
				leCtx, ok := le.(*parser.LogicalExpressionContext)
				if ok && leCtx != nil {
					where := buildLogicalExpression(leCtx, resolver, &args)
					sql += " WHERE " + where
				}
			}
		}
	}

	return sql, args
}

// collectSubQueryTables walks a WHERE clause tree and collects table names from semi-join subqueries.
func collectSubQueryTables(wc parser.IWhereClauseContext) []string {
	var tables []string
	collectSubQueryTablesRecursive(wc, &tables)
	return tables
}

func collectSubQueryTablesRecursive(node antlr.Tree, tables *[]string) {
	if sqv, ok := node.(*parser.SubQueryValueContext); ok {
		if sq := sqv.SubQuery(); sq != nil {
			sqCtx, ok := sq.(*parser.SubQueryContext)
			if ok && sqCtx != nil {
				if fnl := sqCtx.FromNameList(); fnl != nil {
					*tables = append(*tables, fnl.GetText())
				}
			}
		}
	}
	for i := 0; i < node.GetChildCount(); i++ {
		collectSubQueryTablesRecursive(node.GetChild(i), tables)
	}
}

// soqlValueToGo converts a SOQL value node to a Go value for use as a query argument.
func soqlValueToGo(v parser.IValueContext) any {
	switch val := v.(type) {
	case *parser.StringLiteralValueContext:
		text := val.GetText()
		if len(text) >= 2 {
			text = text[1 : len(text)-1]
		}
		return text
	case *parser.SignedNumberValueContext:
		text := val.GetText()
		if n, err := strconv.Atoi(text); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
		return text
	case *parser.BooleanLiteralValueContext:
		return strings.ToLower(val.GetText()) == "true"
	case *parser.NullValueContext:
		return nil
	default:
		return v.GetText()
	}
}

// buildOrderByClause walks an OrderByClauseContext and produces a SQL ORDER BY fragment.
func buildOrderByClause(ctx *parser.OrderByClauseContext) string {
	if fol := ctx.FieldOrderList(); fol != nil {
		folCtx, ok := fol.(*parser.FieldOrderListContext)
		if ok && folCtx != nil {
			var parts []string
			for _, fo := range folCtx.AllFieldOrder() {
				foCtx, ok := fo.(*parser.FieldOrderContext)
				if !ok || foCtx == nil {
					continue
				}
				parts = append(parts, buildFieldOrder(foCtx))
			}
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

// buildFieldOrder walks a FieldOrderContext and produces one ORDER BY term.
func buildFieldOrder(ctx *parser.FieldOrderContext) string {
	var sb strings.Builder

	if fn := ctx.FieldName(); fn != nil {
		fnCtx, ok := fn.(*parser.FieldNameContext)
		if ok && fnCtx != nil {
			sb.WriteString(fieldNameToSQL(fnCtx))
		}
	} else if sf := ctx.SoqlFunction(); sf != nil {
		sfCtx, ok := sf.(*parser.SoqlFunctionContext)
		if ok && sfCtx != nil {
			sb.WriteString(soqlFunctionToSQL(sfCtx))
		} else {
			sb.WriteString(sf.GetText())
		}
	}

	if ctx.ASC() != nil {
		sb.WriteString(" ASC")
	} else if ctx.DESC() != nil {
		sb.WriteString(" DESC")
	}

	if ctx.NULLS() != nil {
		if ctx.FIRST() != nil {
			sb.WriteString(" NULLS FIRST")
		} else if ctx.LAST() != nil {
			sb.WriteString(" NULLS LAST")
		}
	}

	return sb.String()
}

// --- Helper: evaluate expression list to args ---

func (interp *Interpreter) evaluateExpressionList(el parser.IExpressionListContext) []*Value {
	if el == nil {
		return nil
	}
	elCtx, ok := el.(*parser.ExpressionListContext)
	if !ok || elCtx == nil {
		return nil
	}
	var args []*Value
	for _, expr := range elCtx.AllExpression() {
		args = append(args, safeValue(interp.visitNode(expr)))
	}
	return args
}

// --- Switch statement ---

func (interp *Interpreter) VisitSwitchStatement(ctx *parser.SwitchStatementContext) any {
	if ctx == nil {
		return NullValue()
	}

	// Evaluate the switch expression.
	switchVal := safeValue(interp.visitNode(ctx.Expression()))

	for _, wc := range ctx.AllWhenControl() {
		wcCtx, ok := wc.(*parser.WhenControlContext)
		if !ok || wcCtx == nil {
			continue
		}
		wv := wcCtx.WhenValue()
		if wv == nil {
			continue
		}
		wvCtx, ok := wv.(*parser.WhenValueContext)
		if !ok || wvCtx == nil {
			continue
		}

		// ELSE branch — always matches.
		if wvCtx.ELSE() != nil {
			result := interp.visitNode(wcCtx.Block())
			if isControlFlow(result) {
				return result
			}
			return NullValue()
		}

		// Type-matching: when TypeName varName { ... }
		// Case 3 in grammar: Id Id (e.g. "Account acc")
		// Case 4 in grammar: TypeRef Id (e.g. "My.Ns.Type x")
		ids := wvCtx.AllId()
		if len(ids) == 2 && len(wvCtx.AllWhenLiteral()) == 0 {
			typeName := ids[0].GetText()
			if switchVal.Type == TypeSObject && strings.EqualFold(switchVal.SType, typeName) {
				prevEnv := interp.env
				interp.env = NewEnvironment(interp.env)
				interp.env.Define(ids[1].GetText(), switchVal)
				result := interp.visitNode(wcCtx.Block())
				interp.env = prevEnv
				if isControlFlow(result) {
					return result
				}
				return NullValue()
			}
			continue
		}
		if tr := wvCtx.TypeRef(); tr != nil {
			trIds := wvCtx.AllId()
			if len(trIds) > 0 && switchVal.Type == TypeSObject {
				typeName := tr.GetText()
				// The variable name is the last Id (after the TypeRef).
				varName := trIds[len(trIds)-1].GetText()
				if strings.EqualFold(switchVal.SType, typeName) {
					prevEnv := interp.env
					interp.env = NewEnvironment(interp.env)
					interp.env.Define(varName, switchVal)
					result := interp.visitNode(wcCtx.Block())
					interp.env = prevEnv
					if isControlFlow(result) {
						return result
					}
					return NullValue()
				}
			}
			continue
		}

		// Literal matching: when value1, value2, ... { ... }
		for _, wl := range wvCtx.AllWhenLiteral() {
			wlCtx, ok := wl.(*parser.WhenLiteralContext)
			if !ok || wlCtx == nil {
				continue
			}
			litVal := evaluateWhenLiteral(wlCtx)
			if switchValuesEqual(switchVal, litVal) {
				result := interp.visitNode(wcCtx.Block())
				if isControlFlow(result) {
					return result
				}
				return NullValue()
			}
		}
	}

	return NullValue()
}

// evaluateWhenLiteral converts a WhenLiteralContext to a *Value.
func evaluateWhenLiteral(ctx *parser.WhenLiteralContext) *Value {
	// Parenthesized literal: (whenLiteral)
	if ctx.LPAREN() != nil {
		if inner := ctx.WhenLiteral(); inner != nil {
			innerCtx, ok := inner.(*parser.WhenLiteralContext)
			if ok && innerCtx != nil {
				return evaluateWhenLiteral(innerCtx)
			}
		}
	}
	if ctx.NULL() != nil {
		return NullValue()
	}
	isNegative := ctx.SUB() != nil
	if lit := ctx.IntegerLiteral(); lit != nil {
		if n, err := strconv.Atoi(lit.GetText()); err == nil {
			if isNegative {
				n = -n
			}
			return IntegerValue(n)
		}
	}
	if lit := ctx.LongLiteral(); lit != nil {
		text := lit.GetText()
		// Strip trailing 'l' or 'L'
		text = strings.TrimRight(text, "lL")
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			if isNegative {
				n = -n
			}
			return &Value{Type: TypeLong, Data: n}
		}
	}
	if lit := ctx.StringLiteral(); lit != nil {
		text := lit.GetText()
		if len(text) >= 2 {
			text = text[1 : len(text)-1]
		}
		return StringValue(text)
	}
	// Enum value: Id node (e.g. ACTIVE, CLOSED)
	if id := ctx.Id(); id != nil {
		return StringValue(id.GetText())
	}
	return NullValue()
}

// switchValuesEqual compares a switch expression value to a when-literal value.
func switchValuesEqual(switchVal, litVal *Value) bool {
	if switchVal.Type == TypeNull && litVal.Type == TypeNull {
		return true
	}
	if switchVal.Type == TypeNull || litVal.Type == TypeNull {
		return false
	}
	// Integer comparison.
	if switchVal.Type == TypeInteger && litVal.Type == TypeInteger {
		return switchVal.Data.(int) == litVal.Data.(int)
	}
	// String comparison (case-insensitive for enum-like values).
	sStr, sOk := switchVal.Data.(string)
	lStr, lOk := litVal.Data.(string)
	if sOk && lOk {
		return strings.EqualFold(sStr, lStr)
	}
	// Long comparison.
	if switchVal.Type == TypeLong && litVal.Type == TypeLong {
		return switchVal.Data.(int64) == litVal.Data.(int64)
	}
	// Cross-type numeric.
	if switchVal.Type == TypeInteger && litVal.Type == TypeLong {
		return int64(switchVal.Data.(int)) == litVal.Data.(int64)
	}
	if switchVal.Type == TypeLong && litVal.Type == TypeInteger {
		return switchVal.Data.(int64) == int64(litVal.Data.(int))
	}
	return false
}

// isControlFlow returns true if the result is a control flow signal.
func isControlFlow(result any) bool {
	if result == nil {
		return false
	}
	switch result.(type) {
	case *ReturnSignal, *BreakSignal, *ContinueSignal, *ThrowSignal:
		return true
	}
	return false
}

// makeExceptionValue creates an SObject-like Value representing an Apex exception.
func makeExceptionValue(typeName, message string) *Value {
	fields := map[string]*Value{
		"Message": StringValue(message),
	}
	return &Value{Type: TypeSObject, Data: fields, SType: typeName}
}
