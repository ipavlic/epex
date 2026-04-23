package pkg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipavlic/epex/interpreter"
	"github.com/ipavlic/epex/schema"
)

// setupTestProject creates a temporary project directory with manifest,
// source, and mock data for testing.
func setupTestProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Create manifest
	manifest := &Manifest{
		DefaultNamespace: "MyNS",
		SourceDir:        "src",
		Packages: []PackageDependency{
			{Name: "DepPkg", Namespace: "DEP", Mock: "packages/dep.apkg"},
		},
	}
	if err := SaveManifest(filepath.Join(dir, "epex.json"), manifest); err != nil {
		t.Fatal(err)
	}

	// Create source directories
	srcClasses := filepath.Join(dir, "src", "classes")
	srcObjects := filepath.Join(dir, "src", "objects", "MyObj__c")
	srcFields := filepath.Join(dir, "src", "objects", "MyObj__c", "fields")
	for _, d := range []string{srcClasses, srcObjects, srcFields} {
		os.MkdirAll(d, 0755)
	}

	// Write a sample class
	os.WriteFile(filepath.Join(srcClasses, "MyService.cls"), []byte(`
public class MyService {
	public static String greet(String name) {
		return 'Hello, ' + name;
	}
}
`), 0644)

	// Write a test class
	os.WriteFile(filepath.Join(srcClasses, "MyServiceTest.cls"), []byte(`
@isTest
private class MyServiceTest {
	@isTest
	static void testGreet() {
		System.assertEquals('Hello, World', MyService.greet('World'));
	}
}
`), 0644)

	// Write an object
	os.WriteFile(filepath.Join(srcObjects, "MyObj__c.object-meta.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>My Object</label>
    <pluralLabel>My Objects</pluralLabel>
    <deploymentStatus>Deployed</deploymentStatus>
    <sharingModel>ReadWrite</sharingModel>
    <nameField>
        <label>Name</label>
        <type>Text</type>
    </nameField>
</CustomObject>
`), 0644)

	os.WriteFile(filepath.Join(srcFields, "Status__c.field-meta.xml"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Status__c</fullName>
    <label>Status</label>
    <type>Picklist</type>
    <required>false</required>
</CustomField>
`), 0644)

	// Create mock package
	mockPkg := NewPackage("DepPkg", "DEP")
	mockPkg.Classes[strings.ToLower("DepHelper")] = &interpreter.ClassInfo{
		Name:      "DepHelper",
		Modifiers: []string{"global"},
		Methods: map[string]*interpreter.MethodInfo{
			"dowork": {
				Name:       "doWork",
				ReturnType: "String",
				Params:     []interpreter.ParamInfo{{Name: "input", Type: "String"}},
				IsStatic:   true,
				Modifiers:  []string{"global", "static"},
			},
		},
		Fields:       make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}
	mockPkg.Classes[strings.ToLower("PrivateHelper")] = &interpreter.ClassInfo{
		Name:      "PrivateHelper",
		Modifiers: []string{"public"},
		Methods:   make(map[string]*interpreter.MethodInfo),
		Fields:    make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}
	mockPkg.Schema.SObjects["DepObj__c"] = &schema.SObjectSchema{
		Name:  "DepObj__c",
		Label: "Dep Object",
		Fields: map[string]*schema.SObjectField{
			"Id":   {FullName: "Id", Type: schema.FieldTypeId},
			"Name": {FullName: "Name", Type: schema.FieldTypeText},
		},
	}

	pkgDir := filepath.Join(dir, "packages")
	os.MkdirAll(pkgDir, 0755)
	if err := SaveMock(filepath.Join(pkgDir, "dep.apkg"), mockPkg); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestLoadManifest(t *testing.T) {
	dir := setupTestProject(t)
	m, err := LoadManifest(filepath.Join(dir, "epex.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.DefaultNamespace != "MyNS" {
		t.Errorf("expected namespace MyNS, got %s", m.DefaultNamespace)
	}
	if m.SourceDir != "src" {
		t.Errorf("expected sourceDir src, got %s", m.SourceDir)
	}
	if len(m.Packages) != 1 {
		t.Fatalf("expected 1 package dep, got %d", len(m.Packages))
	}
	if m.Packages[0].Namespace != "DEP" {
		t.Errorf("expected dep namespace DEP, got %s", m.Packages[0].Namespace)
	}
}

func TestCreateFromSource(t *testing.T) {
	dir := setupTestProject(t)
	pkg, err := CreateFromSource("local", "MyNS", filepath.Join(dir, "src"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Namespace != "MyNS" {
		t.Errorf("expected namespace MyNS, got %s", pkg.Namespace)
	}
	if len(pkg.Classes) != 2 {
		t.Errorf("expected 2 classes, got %d", len(pkg.Classes))
	}
	if _, ok := pkg.Classes["myservice"]; !ok {
		t.Error("expected MyService class")
	}
	if _, ok := pkg.Classes["myservicetest"]; !ok {
		t.Error("expected MyServiceTest class")
	}
	if len(pkg.Schema.SObjects) != 1 {
		t.Errorf("expected 1 SObject, got %d", len(pkg.Schema.SObjects))
	}
	if _, ok := pkg.Schema.SObjects["MyObj__c"]; !ok {
		t.Error("expected MyObj__c SObject")
	}
}

func TestSaveMockAndLoadMock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.apkg")

	original := NewPackage("TestPkg", "TST")
	original.Classes["helper"] = &interpreter.ClassInfo{
		Name:      "Helper",
		Modifiers: []string{"global"},
		Methods: map[string]*interpreter.MethodInfo{
			"run": {
				Name:       "run",
				ReturnType: "void",
				Params:     []interpreter.ParamInfo{{Name: "x", Type: "Integer"}},
				IsStatic:   true,
				Modifiers:  []string{"global", "static"},
			},
		},
		Fields:       make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}
	original.Schema.SObjects["TstObj__c"] = &schema.SObjectSchema{
		Name:  "TstObj__c",
		Label: "Test Object",
		Fields: map[string]*schema.SObjectField{
			"Id": {FullName: "Id", Type: schema.FieldTypeId},
		},
	}

	if err := SaveMock(path, original); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadMock(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if loaded.Name != "TestPkg" {
		t.Errorf("expected name TestPkg, got %s", loaded.Name)
	}
	if loaded.Namespace != "TST" {
		t.Errorf("expected namespace TST, got %s", loaded.Namespace)
	}
	if !loaded.IsMock {
		t.Error("expected IsMock to be true")
	}

	ci, ok := loaded.Classes["helper"]
	if !ok {
		t.Fatal("expected Helper class")
	}
	if ci.Name != "Helper" {
		t.Errorf("expected class name Helper, got %s", ci.Name)
	}
	mi, ok := ci.Methods["run"]
	if !ok {
		t.Fatal("expected run method")
	}
	if mi.ReturnType != "void" {
		t.Errorf("expected return type void, got %s", mi.ReturnType)
	}
	if len(mi.Params) != 1 || mi.Params[0].Name != "x" {
		t.Error("unexpected params")
	}

	if _, ok := loaded.Schema.SObjects["TstObj__c"]; !ok {
		t.Error("expected TstObj__c in schema")
	}
}

func TestResolveClassSameNamespace(t *testing.T) {
	pkg1 := NewPackage("local", "MyNS")
	pkg1.Classes["myclass"] = &interpreter.ClassInfo{
		Name:      "MyClass",
		Modifiers: []string{"public"},
		Methods:   make(map[string]*interpreter.MethodInfo),
		Fields:    make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}

	resolver := NewResolver([]*Package{pkg1}, "MyNS")

	ci, p, found := resolver.ResolveClass("MyClass", "MyNS")
	if !found {
		t.Fatal("expected to find MyClass")
	}
	if ci.Name != "MyClass" {
		t.Errorf("expected MyClass, got %s", ci.Name)
	}
	if p.Namespace != "MyNS" {
		t.Errorf("expected package MyNS, got %s", p.Namespace)
	}
}

func TestResolveClassQualifiedName(t *testing.T) {
	pkg1 := NewPackage("dep", "DEP")
	pkg1.Classes["helper"] = &interpreter.ClassInfo{
		Name:      "Helper",
		Modifiers: []string{"global"},
		Methods:   make(map[string]*interpreter.MethodInfo),
		Fields:    make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}

	resolver := NewResolver([]*Package{pkg1}, "MyNS")

	ci, _, found := resolver.ResolveClass("DEP.Helper", "MyNS")
	if !found {
		t.Fatal("expected to find DEP.Helper")
	}
	if ci.Name != "Helper" {
		t.Errorf("expected Helper, got %s", ci.Name)
	}
}

func TestResolveClassCrossNamespaceGlobalOnly(t *testing.T) {
	pkg1 := NewPackage("dep", "DEP")
	pkg1.Classes["globalclass"] = &interpreter.ClassInfo{
		Name:      "GlobalClass",
		Modifiers: []string{"global"},
		Methods:   make(map[string]*interpreter.MethodInfo),
		Fields:    make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}
	pkg1.Classes["publicclass"] = &interpreter.ClassInfo{
		Name:      "PublicClass",
		Modifiers: []string{"public"},
		Methods:   make(map[string]*interpreter.MethodInfo),
		Fields:    make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}

	resolver := NewResolver([]*Package{pkg1}, "MyNS")

	// Global class should be accessible cross-namespace
	_, _, found := resolver.ResolveClass("GlobalClass", "MyNS")
	if !found {
		t.Error("expected to find GlobalClass cross-namespace")
	}

	// Public class should NOT be accessible cross-namespace via unqualified name
	_, _, found = resolver.ResolveClass("PublicClass", "MyNS")
	if found {
		t.Error("public class should not be accessible cross-namespace")
	}

	// But it IS accessible via qualified name... well, actually no.
	// The access check should block it.
	_, _, found = resolver.ResolveClass("DEP.PublicClass", "MyNS")
	if found {
		t.Error("public class should not be accessible cross-namespace even with qualifier")
	}
}

func TestBuildMergedRegistry(t *testing.T) {
	pkg1 := NewPackage("local", "MyNS")
	pkg1.Classes["localclass"] = &interpreter.ClassInfo{
		Name:    "LocalClass",
		Methods: make(map[string]*interpreter.MethodInfo),
		Fields:  make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}

	pkg2 := NewPackage("dep", "DEP")
	pkg2.Classes["depclass"] = &interpreter.ClassInfo{
		Name:    "DepClass",
		Methods: make(map[string]*interpreter.MethodInfo),
		Fields:  make(map[string]*interpreter.FieldInfo),
		Constructors: []*interpreter.ConstructorInfo{},
		InnerClasses: make(map[string]*interpreter.ClassInfo),
	}

	resolver := NewResolver([]*Package{pkg1, pkg2}, "MyNS")
	reg := resolver.BuildMergedRegistry()

	if _, ok := reg.Classes["localclass"]; !ok {
		t.Error("expected localclass in merged registry")
	}
	if _, ok := reg.Classes["myns.localclass"]; !ok {
		t.Error("expected myns.localclass in merged registry")
	}
	if _, ok := reg.Classes["depclass"]; !ok {
		t.Error("expected depclass in merged registry")
	}
	if _, ok := reg.Classes["dep.depclass"]; !ok {
		t.Error("expected dep.depclass in merged registry")
	}
}

func TestLoadProject(t *testing.T) {
	dir := setupTestProject(t)
	result, err := LoadProject(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Local package
	if result.LocalPackage == nil {
		t.Fatal("expected local package")
	}
	if len(result.LocalPackage.Classes) != 2 {
		t.Errorf("expected 2 local classes, got %d", len(result.LocalPackage.Classes))
	}

	// Mock packages
	if len(result.MockPackages) != 1 {
		t.Fatalf("expected 1 mock package, got %d", len(result.MockPackages))
	}
	if result.MockPackages[0].Namespace != "DEP" {
		t.Errorf("expected DEP namespace, got %s", result.MockPackages[0].Namespace)
	}

	// Merged schema should have objects from both
	if _, ok := result.Schema.SObjects["MyObj__c"]; !ok {
		t.Error("expected MyObj__c in merged schema")
	}
	if _, ok := result.Schema.SObjects["DepObj__c"]; !ok {
		t.Error("expected DepObj__c in merged schema")
	}

	// Resolver should find classes across namespaces
	_, _, found := result.Resolver.ResolveClass("MyService", "MyNS")
	if !found {
		t.Error("expected to resolve MyService")
	}
	_, _, found = result.Resolver.ResolveClass("DEP.DepHelper", "MyNS")
	if !found {
		t.Error("expected to resolve DEP.DepHelper")
	}
}

func TestMergeSchemas(t *testing.T) {
	pkg1 := NewPackage("a", "A")
	pkg1.Schema.SObjects["Account"] = &schema.SObjectSchema{
		Name:  "Account",
		Label: "Account",
		Fields: map[string]*schema.SObjectField{
			"Id":   {FullName: "Id", Type: schema.FieldTypeId},
			"Name": {FullName: "Name", Type: schema.FieldTypeText},
		},
	}

	pkg2 := NewPackage("b", "B")
	pkg2.Schema.SObjects["Account"] = &schema.SObjectSchema{
		Name: "Account",
		Fields: map[string]*schema.SObjectField{
			"Custom__c": {FullName: "Custom__c", Type: schema.FieldTypeText},
		},
	}
	pkg2.Schema.SObjects["Contact"] = &schema.SObjectSchema{
		Name:  "Contact",
		Label: "Contact",
		Fields: map[string]*schema.SObjectField{
			"Id": {FullName: "Id", Type: schema.FieldTypeId},
		},
	}

	merged := mergeSchemas([]*Package{pkg1, pkg2})

	// Account should have fields from both packages
	acc, ok := merged.SObjects["Account"]
	if !ok {
		t.Fatal("expected Account")
	}
	if _, ok := acc.Fields["Id"]; !ok {
		t.Error("expected Id field")
	}
	if _, ok := acc.Fields["Name"]; !ok {
		t.Error("expected Name field")
	}
	if _, ok := acc.Fields["Custom__c"]; !ok {
		t.Error("expected Custom__c field from pkg2")
	}
	// Label should stay from pkg1 since pkg2 has empty label
	if acc.Label != "Account" {
		t.Errorf("expected label Account, got %s", acc.Label)
	}

	// Contact should exist from pkg2
	if _, ok := merged.SObjects["Contact"]; !ok {
		t.Error("expected Contact")
	}
}
