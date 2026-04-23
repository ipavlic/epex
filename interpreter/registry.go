package interpreter

import (
	"strings"

	"github.com/ipavlic/epex/parser"
)

// ClassInfo holds metadata about a parsed Apex class.
type ClassInfo struct {
	Name         string
	SourceFile   string
	Modifiers    []string
	Annotations  []string
	Methods      map[string]*MethodInfo
	Fields       map[string]*FieldInfo
	Constructors []*ConstructorInfo
	InnerClasses map[string]*ClassInfo
	SuperClass   string
	Interfaces   []string
	Node         parser.IClassDeclarationContext
}

// MethodInfo holds metadata about a method.
type MethodInfo struct {
	Name       string
	ReturnType string
	Params     []ParamInfo
	IsStatic   bool
	IsTest     bool
	Modifiers  []string
	Node       parser.IMethodDeclarationContext
}

// ParamInfo holds metadata about a parameter.
type ParamInfo struct {
	Name string
	Type string
}

// FieldInfo holds metadata about a field.
type FieldInfo struct {
	Name      string
	Type      string
	IsStatic  bool
	Modifiers []string
	Node      parser.IFieldDeclarationContext
}

// ConstructorInfo holds metadata about a constructor.
type ConstructorInfo struct {
	Params    []ParamInfo
	Modifiers []string
	Node      parser.IConstructorDeclarationContext
}

// TriggerEvent represents when a trigger fires.
type TriggerEvent struct {
	IsBefore bool
	IsAfter  bool
	Op       string // "INSERT", "UPDATE", "DELETE", "UNDELETE"
}

// TriggerInfo holds metadata about a parsed Apex trigger.
type TriggerInfo struct {
	Name       string
	SObject    string
	SourceFile string
	Events     []TriggerEvent
	Node       parser.ITriggerUnitContext
}

// Registry holds all registered classes and triggers.
type Registry struct {
	Classes  map[string]*ClassInfo
	Triggers []*TriggerInfo
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		Classes: make(map[string]*ClassInfo),
	}
}

// RegisterTrigger walks a CompilationUnit AST and extracts trigger info.
func (r *Registry) RegisterTrigger(tree parser.ICompilationUnitContext, sourceFile ...string) bool {
	cu, ok := tree.(*parser.CompilationUnitContext)
	if !ok || cu == nil {
		return false
	}
	tu := cu.TriggerUnit()
	if tu == nil {
		return false
	}
	tuCtx, ok := tu.(*parser.TriggerUnitContext)
	if !ok || tuCtx == nil {
		return false
	}

	ids := tuCtx.AllId()
	if len(ids) < 2 {
		return false
	}
	triggerName := ids[0].GetText()
	sobjectName := ids[1].GetText()

	var events []TriggerEvent
	for _, tc := range tuCtx.AllTriggerCase() {
		tcCtx, ok := tc.(*parser.TriggerCaseContext)
		if !ok || tcCtx == nil {
			continue
		}
		ev := TriggerEvent{
			IsBefore: tcCtx.BEFORE() != nil,
			IsAfter:  tcCtx.AFTER() != nil,
		}
		switch {
		case tcCtx.INSERT() != nil:
			ev.Op = "INSERT"
		case tcCtx.UPDATE() != nil:
			ev.Op = "UPDATE"
		case tcCtx.DELETE() != nil:
			ev.Op = "DELETE"
		case tcCtx.UNDELETE() != nil:
			ev.Op = "UNDELETE"
		}
		events = append(events, ev)
	}

	info := &TriggerInfo{
		Name:    triggerName,
		SObject: sobjectName,
		Events:  events,
		Node:    tuCtx,
	}
	if len(sourceFile) > 0 {
		info.SourceFile = sourceFile[0]
	}
	r.Triggers = append(r.Triggers, info)
	return true
}

// RegisterClass walks a CompilationUnit AST and extracts class info.
// The optional sourceFile parameter sets the source filename for tracing.
func (r *Registry) RegisterClass(tree parser.ICompilationUnitContext, sourceFile ...string) {
	cu, ok := tree.(*parser.CompilationUnitContext)
	if !ok || cu == nil {
		return
	}
	td := cu.TypeDeclaration()
	if td == nil {
		return
	}
	tdCtx, ok := td.(*parser.TypeDeclarationContext)
	if !ok || tdCtx == nil {
		return
	}

	classDecl := tdCtx.ClassDeclaration()
	if classDecl == nil {
		return
	}
	classDeclCtx, ok := classDecl.(*parser.ClassDeclarationContext)
	if !ok || classDeclCtx == nil {
		return
	}

	// Extract top-level modifiers and annotations
	modifiers, annotations := extractModifiersAndAnnotations(tdCtx.AllModifier())

	info := r.buildClassInfo(classDeclCtx, modifiers, annotations)
	if len(sourceFile) > 0 {
		info.SourceFile = sourceFile[0]
	}
	r.Classes[strings.ToLower(info.Name)] = info
}

func (r *Registry) buildClassInfo(ctx *parser.ClassDeclarationContext, modifiers, annotations []string) *ClassInfo {
	info := &ClassInfo{
		Name:         getIdText(ctx.Id()),
		Modifiers:    modifiers,
		Annotations:  annotations,
		Methods:      make(map[string]*MethodInfo),
		Fields:       make(map[string]*FieldInfo),
		Constructors: []*ConstructorInfo{},
		InnerClasses: make(map[string]*ClassInfo),
		Node:         ctx,
	}

	// Extract superclass
	if ctx.EXTENDS() != nil && ctx.TypeRef() != nil {
		info.SuperClass = ctx.TypeRef().GetText()
	}

	// Extract interfaces
	if ctx.IMPLEMENTS() != nil && ctx.TypeList() != nil {
		info.Interfaces = []string{ctx.TypeList().GetText()}
	}

	// Walk class body
	body := ctx.ClassBody()
	if body == nil {
		return info
	}
	bodyCtx, ok := body.(*parser.ClassBodyContext)
	if !ok || bodyCtx == nil {
		return info
	}

	for _, cbd := range bodyCtx.AllClassBodyDeclaration() {
		cbdCtx, ok := cbd.(*parser.ClassBodyDeclarationContext)
		if !ok || cbdCtx == nil {
			continue
		}

		memberDecl := cbdCtx.MemberDeclaration()
		if memberDecl == nil {
			continue
		}
		memberCtx, ok := memberDecl.(*parser.MemberDeclarationContext)
		if !ok || memberCtx == nil {
			continue
		}

		mods, annots := extractModifiersAndAnnotations(cbdCtx.AllModifier())

		// Method
		if md := memberCtx.MethodDeclaration(); md != nil {
			mdCtx, ok := md.(*parser.MethodDeclarationContext)
			if !ok || mdCtx == nil {
				continue
			}
			mi := buildMethodInfo(mdCtx, mods, annots)
			info.Methods[strings.ToLower(mi.Name)] = mi
		}

		// Field
		if fd := memberCtx.FieldDeclaration(); fd != nil {
			fdCtx, ok := fd.(*parser.FieldDeclarationContext)
			if !ok || fdCtx == nil {
				continue
			}
			fields := buildFieldInfos(fdCtx, mods)
			for _, fi := range fields {
				info.Fields[strings.ToLower(fi.Name)] = fi
			}
		}

		// Constructor
		if cd := memberCtx.ConstructorDeclaration(); cd != nil {
			cdCtx, ok := cd.(*parser.ConstructorDeclarationContext)
			if !ok || cdCtx == nil {
				continue
			}
			ci := buildConstructorInfo(cdCtx, mods)
			info.Constructors = append(info.Constructors, ci)
		}

		// Inner class
		if innerClass := memberCtx.ClassDeclaration(); innerClass != nil {
			innerCtx, ok := innerClass.(*parser.ClassDeclarationContext)
			if !ok || innerCtx == nil {
				continue
			}
			innerInfo := r.buildClassInfo(innerCtx, mods, annots)
			info.InnerClasses[strings.ToLower(innerInfo.Name)] = innerInfo
		}
	}

	return info
}

func buildMethodInfo(ctx *parser.MethodDeclarationContext, modifiers, annotations []string) *MethodInfo {
	name := ""
	if mid := ctx.MethodId(); mid != nil {
		name = mid.GetText()
	}

	returnType := "void"
	if ctx.TypeRef() != nil {
		returnType = ctx.TypeRef().GetText()
	} else if ctx.VOID() != nil {
		returnType = "void"
	}

	isStatic := containsIgnoreCase(modifiers, "static")
	isTest := containsIgnoreCase(annotations, "isTest") || containsIgnoreCase(modifiers, "testMethod")

	params := extractParams(ctx.FormalParameters())

	return &MethodInfo{
		Name:       name,
		ReturnType: returnType,
		Params:     params,
		IsStatic:   isStatic,
		IsTest:     isTest,
		Modifiers:  modifiers,
		Node:       ctx,
	}
}

func buildFieldInfos(ctx *parser.FieldDeclarationContext, modifiers []string) []*FieldInfo {
	var fields []*FieldInfo
	typeName := ""
	if ctx.TypeRef() != nil {
		typeName = ctx.TypeRef().GetText()
	}
	isStatic := containsIgnoreCase(modifiers, "static")

	vds := ctx.VariableDeclarators()
	if vds == nil {
		return fields
	}
	vdsCtx, ok := vds.(*parser.VariableDeclaratorsContext)
	if !ok || vdsCtx == nil {
		return fields
	}

	for _, vd := range vdsCtx.AllVariableDeclarator() {
		vdCtx, ok := vd.(*parser.VariableDeclaratorContext)
		if !ok || vdCtx == nil {
			continue
		}
		name := getIdText(vdCtx.Id())
		fields = append(fields, &FieldInfo{
			Name:      name,
			Type:      typeName,
			IsStatic:  isStatic,
			Modifiers: modifiers,
			Node:      ctx,
		})
	}
	return fields
}

func buildConstructorInfo(ctx *parser.ConstructorDeclarationContext, modifiers []string) *ConstructorInfo {
	params := extractParams(ctx.FormalParameters())
	return &ConstructorInfo{
		Params:    params,
		Modifiers: modifiers,
		Node:      ctx,
	}
}

func extractParams(fp parser.IFormalParametersContext) []ParamInfo {
	if fp == nil {
		return nil
	}
	fpCtx, ok := fp.(*parser.FormalParametersContext)
	if !ok || fpCtx == nil {
		return nil
	}
	fpl := fpCtx.FormalParameterList()
	if fpl == nil {
		return nil
	}
	fplCtx, ok := fpl.(*parser.FormalParameterListContext)
	if !ok || fplCtx == nil {
		return nil
	}

	var params []ParamInfo
	for _, param := range fplCtx.AllFormalParameter() {
		pCtx, ok := param.(*parser.FormalParameterContext)
		if !ok || pCtx == nil {
			continue
		}
		typeName := ""
		if pCtx.TypeRef() != nil {
			typeName = pCtx.TypeRef().GetText()
		}
		name := getIdText(pCtx.Id())
		params = append(params, ParamInfo{Name: name, Type: typeName})
	}
	return params
}

func extractModifiersAndAnnotations(mods []parser.IModifierContext) ([]string, []string) {
	var modifiers, annotations []string
	for _, m := range mods {
		mCtx, ok := m.(*parser.ModifierContext)
		if !ok || mCtx == nil {
			continue
		}
		if ann := mCtx.Annotation(); ann != nil {
			annCtx, ok := ann.(*parser.AnnotationContext)
			if ok && annCtx != nil && annCtx.QualifiedName() != nil {
				annotations = append(annotations, annCtx.QualifiedName().GetText())
			}
		} else {
			text := mCtx.GetText()
			if text != "" {
				modifiers = append(modifiers, text)
			}
		}
	}
	return modifiers, annotations
}

func getIdText(id parser.IIdContext) string {
	if id == nil {
		return ""
	}
	return id.GetText()
}

func containsIgnoreCase(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}
