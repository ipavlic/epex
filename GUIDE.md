# epex User Guide

## Getting Started

### Prerequisites

- Go 1.21+
- Java 17+ (only needed if regenerating the parser)

### Installation

```bash
go install github.com/ipavlic/epex/cmd/epex@latest
```

Or build from source:

```bash
git clone https://github.com/ipavlic/epex.git
cd epex
go build -o epex ./cmd/epex/
```

## Project Setup

epex expects an SFDX project layout. If you already have one from `sf project generate`, you're most of the way there.

### Minimal project structure

```
my-project/
├── epex.json                          # Project manifest
├── force-app/main/default/
│   ├── classes/
│   │   ├── AccountService.cls
│   │   ├── AccountService.cls-meta.xml
│   │   ├── AccountServiceTest.cls
│   │   └── AccountServiceTest.cls-meta.xml
│   └── objects/
│       └── Account/
│           ├── Account.object-meta.xml
│           └── fields/
│               └── Rating__c.field-meta.xml
└── packages/                               # Mock packages (optional)
    └── ringdna.apkg
```

### The manifest file (epex.json)

Create this at your project root:

```json
{
  "defaultNamespace": "",
  "sourceDir": "force-app/main/default",
  "packages": []
}
```

**Fields:**

| Field | Description | Default |
|---|---|---|
| `defaultNamespace` | Your package namespace. Set if your org has a namespace prefix (e.g. `"RDNACadence"`). Leave empty for unmanaged code. | `""` |
| `sourceDir` | Path to your SFDX source directory, relative to project root. | `"force-app/main/default"` |
| `packages` | Array of managed package dependencies (see [Working with Packages](#working-with-packages)). | `[]` |

## Writing Tests

epex runs standard Apex test classes. Write them exactly as you would for Salesforce:

```apex
@isTest
private class AccountServiceTest {

    @isTest
    static void testCreateAccount() {
        Account acc = new Account(Name = 'Test Corp');
        insert acc;

        Account result = [SELECT Id, Name FROM Account WHERE Id = :acc.Id];
        Assert.areEqual('Test Corp', result.Name);
    }

    @isTest
    static void testBulkInsert() {
        List<Account> accounts = new List<Account>();
        for (Integer i = 0; i < 200; i++) {
            accounts.add(new Account(Name = 'Account ' + i));
        }
        insert accounts;

        List<Account> results = [SELECT Id FROM Account];
        Assert.areEqual(200, results.size());
    }
}
```

### Supported test patterns

- `@isTest` annotation on class and/or methods
- `testMethod` modifier (legacy, still supported)
- `System.assert()`, `System.assertEquals()`, `System.assertNotEquals()` (legacy)
- `Assert.areEqual()`, `Assert.areNotEqual()`, `Assert.isTrue()`, `Assert.isFalse()`, `Assert.isNull()`, `Assert.isNotNull()`, `Assert.fail()` (preferred)
- `System.runAs(user) { ... }` for testing sharing and user context

### Testing with user context and permissions

You can test sharing rules and permission enforcement using the same patterns as real Salesforce tests:

```apex
@isTest
private class PermissionTest {
    @isTest
    static void testWithSharing() {
        // Set up a restricted profile
        Profile p = new Profile(Name = 'Limited');
        insert p;

        ObjectPermissions op = new ObjectPermissions(
            ParentId = p.Id,
            SobjectType = 'Account',
            PermissionsCreate = false,
            PermissionsRead = true
        );
        insert op;

        User u = new User(Username = 'limited@test.com', ProfileId = p.Id);
        insert u;

        // Test CRUD enforcement
        System.runAs(u) {
            try {
                insert as user new Account(Name = 'Test');
                Assert.fail('Should have thrown');
            } catch (Exception e) {
                Assert.isTrue(e.getMessage().contains('Insufficient privileges'));
            }
        }
    }

    @isTest
    static void testStripInaccessible() {
        // ... set up FieldPermissions ...

        System.runAs(restrictedUser) {
            List<Account> accs = [SELECT Id, Name, Industry FROM Account];
            SObjectAccessDecision decision = Security.stripInaccessible(
                AccessType.READABLE, accs
            );
            List<Account> safe = decision.getRecords();
            // Inaccessible fields have been silently removed
        }
    }
}
```

Permissions follow the **"no row = full access"** rule: tests that don't insert ObjectPermissions or FieldPermissions records see no enforcement at all, preserving backwards compatibility.

### What works in test code

| Feature | Supported |
|---|---|
| Variable declarations, assignments | Yes |
| Arithmetic, string concat, comparisons | Yes |
| if/else, for, while, do-while, switch | Yes |
| try/catch/finally, throw | Yes |
| break, continue, return | Yes |
| List, Set, Map operations | Yes |
| SObject creation and field access | Yes |
| DML: insert, update, delete, upsert | Yes |
| Inline SOQL: `[SELECT ...]` | Yes |
| Class instantiation, constructors | Yes |
| Static and instance methods | Yes |
| String methods (length, contains, split, ...) | Yes |
| System.debug | Yes |
| Inner classes | Yes |
| Ternary expressions | Yes |
| Type casting | Yes |
| Triggers (before/after insert/update/delete) | Yes |
| System.runAs(user) | Yes |
| Sharing keywords (with/without/inherited sharing) | Yes |
| DML access modes (INSERT AS USER/SYSTEM) | Yes |
| SOQL access modes (WITH USER_MODE/SYSTEM_MODE/SECURITY_ENFORCED) | Yes |
| Security.stripInaccessible() | Yes |
| ObjectPermissions / FieldPermissions enforcement | Yes |

### What does not work (yet)

| Feature | Status |
|---|---|
| Interfaces (implementing methods) | Parsed, not interpreted |
| Abstract classes | Parsed, not interpreted |
| HTTP callouts | Not supported |
| Platform events | Not supported |
| Batch/Queueable/Schedulable | Stubs only (no async execution) |
| Custom metadata / custom settings queries | Not supported |
| Dynamic Apex (Type.forName, newInstance) | Supported (primitives, user classes, SObjects) |
| Governor limits | Not enforced |
| Workflow rules, flows, process builder | Not supported |
| SOSL | Not supported |
| Aggregate SOQL (COUNT, SUM, GROUP BY, HAVING) | Supported |
| Relationship SOQL (parent/child, semi-joins) | Supported |

## Running Tests

### Basic usage

```bash
# Run tests
epex run ./my-project

# Run with tracing
epex run ./my-project --trace

# Run with trace output to specific file
epex run ./my-project --trace --trace-file output.json

# Validate Apex syntax only
epex parse ./my-project/classes/
```

### Result formats

**Human-readable** (default, similar to `sf apex run test --result-format human`):

```
=== Test Results
 OUTCOME  TEST NAME                                       RUNTIME  MESSAGE
 ───────  ───────────────────────────────────────────  ──────────  ────────
  Pass    AccountServiceTest.testCreateAccount               12ms
  Pass    AccountServiceTest.testBulkInsert                  45ms
  Fail    ContactServiceTest.testInvalidEmail                 3ms  Assert.areEqual failed: ...

=== Failures
 ContactServiceTest.testInvalidEmail
   Message: Assert.areEqual failed: Expected: true, Actual: false

=== Test Summary
 Outcome:         Fail
 Tests Ran:       3
 Passing:         2
 Failing:         1
 Pass Rate:       66.7%
 Fail Rate:       33.3%
 Test Run Time:   60ms
 Command Time:    120ms
```

**JSON** (for CI/CD pipelines):

```json
{
  "summary": {
    "outcome": "Fail",
    "testsRan": 3,
    "passing": 2,
    "failing": 1,
    "skipped": 0,
    "passRate": "66.7%",
    "failRate": "33.3%",
    "testTotalTime": 60,
    "commandTime": 120
  },
  "tests": [
    {
      "ClassName": "AccountServiceTest",
      "MethodName": "testCreateAccount",
      "Outcome": "Pass",
      "RunTime": 12
    }
  ]
}
```

## Tracing and Performance Analysis

Enable tracing to record every line execution, method call, SOQL query, and DML operation.

### Enabling tracing

```go
tr := tracer.NewRecordingTracer()
interp.SetTracer(tr)

// ... run tests ...

events := tr.Events()
```

### Perfetto visualization

Export the trace as Chrome Trace Event JSON, then open it in [ui.perfetto.dev](https://ui.perfetto.dev):

```go
f, _ := os.Create("trace.json")
tracer.WritePerfetto(f, tr.Events(), tr.Epoch())
f.Close()
```

Then open https://ui.perfetto.dev and drag `trace.json` onto it. You'll see:

- **Method spans** — each method call as a horizontal bar showing its duration
- **SOQL/DML events** — nested inside method spans, showing query text and row counts
- **Assert markers** — instant events showing pass/fail at the exact point they executed

### Execution summary

After tracing, generate an aggregated summary:

```go
summary := tracer.BuildSummary(tr.Events(), 20) // top 20 hot lines
tracer.FormatSummaryHuman(os.Stdout, summary)
```

Output:

```
=== Method Performance
 METHOD                                    CALLS  TOTAL       AVG
 ──────────────────────────────────────  ──────  ──────────  ──────────
 AccountService.getHighValueAccounts          3    45.2ms    15.1ms
 AccountService.createAccount                 2    12.0ms     6.0ms

=== SOQL Queries
 QUERY                                     CALLS       ROWS      TIME
 ──────────────────────────────────────  ──────  ──────────  ──────────
 SELECT Id, Name FROM Account WHERE ...       3         15    30.5ms

=== DML Operations
 OPERATION   SOBJECT               CALLS       ROWS      TIME
 ──────────  ────────────────────  ──────  ──────────  ──────────
 INSERT      Account                   2           5    10.2ms

=== Hot Lines
 LOCATION                                  EXECUTIONS
 ──────────────────────────────────────  ──────────
 AccountService.cls:15                           300
 AccountService.cls:16                           300
 AccountServiceTest.cls:8                         50
```

The summary is also available in JSON format via `tracer.FormatSummaryJSON()`.

## Working with Packages

### When you need packages

You need the package system when your Apex code depends on managed packages. For example, if your code calls `RNDNA.SomeClass.someMethod()`, epex needs to know that `RNDNA` is a namespace and `SomeClass` exists with that method signature.

### Creating a mock package

A mock package captures the metadata (class signatures, SObject definitions) of a managed package so epex can resolve references to it. The classes contain no implementation — they're stubs that return default values.

**Step 1: Build the mock data**

Currently, mock packages are created programmatically. You define the class signatures and SObject schema that your tests reference:

```go
mockPkg := pkg.NewPackage("RingDNA", "RNDNA")

// Add class stubs
mockPkg.Classes["someclass"] = &interpreter.ClassInfo{
    Name:      "SomeClass",
    Modifiers: []string{"global"},
    Methods: map[string]*interpreter.MethodInfo{
        "somemethod": {
            Name:       "someMethod",
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

// Add SObject definitions
mockPkg.Schema.SObjects["RNDNACall__c"] = &schema.SObjectSchema{
    Name:  "RNDNACall__c",
    Label: "Call",
    Fields: map[string]*schema.SObjectField{
        "Id":       {FullName: "Id", Type: schema.FieldTypeId},
        "Name":     {FullName: "Name", Type: schema.FieldTypeText},
        "Status__c": {FullName: "Status__c", Type: schema.FieldTypePicklist},
    },
}
```

**Step 2: Save the mock**

```go
err := pkg.SaveMock("packages/ringdna.apkg", mockPkg)
```

This creates a JSON file containing all class signatures and SObject definitions.

**Step 3: Reference in manifest**

```json
{
  "defaultNamespace": "RDNACadence",
  "sourceDir": "force-app/main/default",
  "packages": [
    {
      "name": "RingDNA",
      "namespace": "RNDNA",
      "mock": "packages/ringdna.apkg"
    }
  ]
}
```

### How namespace resolution works

When the interpreter encounters a name like `RNDNA.SomeClass.someMethod()`:

1. It splits on the first dot: namespace candidate = `RNDNA`
2. Looks up `RNDNA` in loaded packages
3. Finds `SomeClass` in that package
4. Checks access: the class must be `global` or `@NamespaceAccessible`
5. Resolves `someMethod` on the class

For unqualified names like `MyHelper.doWork()`:

1. Look in the current namespace first
2. Then the default namespace
3. Then all packages (global classes only)

### Access rules

| Modifier | Same namespace | Other namespaces |
|---|---|---|
| `private` | Same class only | No |
| `protected` | Same class + subclasses | No |
| `public` | Yes | No |
| `global` | Yes | Yes |
| `@NamespaceAccessible` | Yes | Yes |

### Loading a project with packages

```go
result, err := pkg.LoadProject("/path/to/my-project")
if err != nil {
    log.Fatal(err)
}

// result.LocalPackage  — your parsed Apex code
// result.MockPackages  — stub managed package dependencies
// result.AllPackages   — all packages in load order
// result.Schema        — merged SObject schema from all packages
// result.Resolver      — resolves namespace-qualified class names

// Build a merged registry for the interpreter
registry := result.Resolver.BuildMergedRegistry()
```

The merged schema combines SObjects from all packages. If multiple packages define fields on the same SObject (e.g., both your code and a managed package add custom fields to Account), they're merged together.

## SObject Schema

### Standard objects (built-in)

34 standard Salesforce objects have built-in field definitions and don't need metadata files:

Account, Contact, Lead, Opportunity, OpportunityLineItem, Case, Task, Event, User, Profile, UserRole, Campaign, CampaignMember, Contract, Order, OrderItem, Product2, Pricebook2, PricebookEntry, Asset, ContentDocument, ContentVersion, Attachment, Note, FeedItem, EmailMessage, Organization, Group, Solution, OpportunityContactRole, AccountContactRelation, RecordType, ObjectPermissions, FieldPermissions

When you use a standard object in your Apex code (e.g., `new Account(Name = 'Test')`), the engine automatically creates the table with all standard fields. You only need `.object-meta.xml` and `.field-meta.xml` for custom objects or to override standard object metadata.

### Standard SFDX layout

epex reads the standard SFDX object metadata format. You only need metadata files for custom objects and custom fields. Standard objects and their standard fields are built-in (see above), so metadata files for them are optional:

```
objects/
├── Account/
│   └── fields/
│       └── Rating__c.field-meta.xml      # custom field on standard object
└── MyCustomObject__c/
    ├── MyCustomObject__c.object-meta.xml
    └── fields/
        └── Status__c.field-meta.xml
```

Custom fields use the `__c` suffix; standard fields (e.g., `Name`, `Industry`, `Phone`) do not.

### Object metadata format

```xml
<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>My Custom Object</label>
    <pluralLabel>My Custom Objects</pluralLabel>
    <deploymentStatus>Deployed</deploymentStatus>
    <sharingModel>ReadWrite</sharingModel>
    <nameField>
        <label>Name</label>
        <type>Text</type>
    </nameField>
</CustomObject>
```

### Field metadata format

```xml
<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Status__c</fullName>
    <label>Status</label>
    <type>Picklist</type>
    <required>false</required>
</CustomField>
```

### Supported field types

| Salesforce Type | Stored as |
|---|---|
| Text, Email, Phone, Url, Picklist | TEXT |
| Checkbox | INTEGER (0/1) |
| Number, Currency, Percent | REAL |
| AutoNumber | INTEGER |
| Date, DateTime, Time | TEXT (ISO format) |
| Lookup, MasterDetail | TEXT (Id reference) |
| LongTextArea, Html | TEXT |

### Standard fields

Every SObject automatically gets these universal fields — you don't need to define them:

- `Id` (Id)
- `Name` (Text)
- `CreatedDate` (DateTime)
- `LastModifiedDate` (DateTime)
- `CreatedById` (Lookup → User)
- `LastModifiedById` (Lookup → User)
- `OwnerId` (Lookup → User)
- `IsDeleted` (Checkbox)

Standard objects also get their object-specific standard fields automatically. For example, Account gets `Industry`, `AnnualRevenue`, `Phone`, `Website`, `BillingCity`, etc. — Contact gets `Email`, `Phone`, `Title`, `Department`, etc. These are all available without any metadata files.

## Programmatic API

All functionality is available as Go packages for embedding in your own tools.

### Parsing

```go
import "github.com/ipavlic/epex/apex"

// Parse a single file
result, err := apex.ParseFile("MyClass.cls")

// Parse a string
result, err := apex.ParseString("MyClass.cls", sourceCode)

// Parse all .cls files in a directory
results, err := apex.ParseDirectory("force-app/main/default/classes")
```

### Schema building

```go
import "github.com/ipavlic/epex/schema"

// From an SFDX project root
s, err := schema.BuildSchema("/path/to/project")

// From an objects directory directly
s, err := schema.BuildSchemaFromDir("force-app/main/default/objects")
```

### Interpreting

```go
import (
    "github.com/ipavlic/epex/apex"
    "github.com/ipavlic/epex/interpreter"
)

// Parse
result, _ := apex.ParseString("MyClass.cls", source)

// Register
reg := interpreter.NewRegistry()
reg.RegisterClass(result.Tree)

// Create interpreter (nil engine = no DML/SOQL support)
interp := interpreter.NewInterpreter(reg, nil)

// Execute a method
returnValue := interp.ExecuteMethod("MyClass", "myMethod", []*interpreter.Value{
    interpreter.StringValue("arg1"),
})
```

### Running tests

```go
results := interp.RunTests() // discovers @isTest classes automatically

for _, r := range results {
    fmt.Printf("%s.%s: %v (%v)\n", r.ClassName, r.MethodName, r.Passed, r.Duration)
}
```

### With DML/SOQL engine

```go
import "github.com/ipavlic/epex/engine"

eng, err := engine.NewEngine(mySchema)
defer eng.Close()

interp := interpreter.NewInterpreter(reg, eng)
// Now insert, update, delete, and SOQL queries work
```

### Formatting results

```go
import "github.com/ipavlic/epex/reporter"

testResults := []reporter.TestResult{...}
r := reporter.NewTestRunResult(testResults, commandDuration)

reporter.FormatHuman(os.Stdout, r)  // human-readable
reporter.FormatJSON(os.Stdout, r)   // JSON
```

## Tips

### Test isolation

Each `@isTest` method starts with a completely clean database — all tables are emptied and ID counters are reset. Records inserted in one test method are not visible in other test methods. This matches Salesforce's test isolation behavior.

### Debugging parse errors

If a `.cls` file fails to parse, the error includes the file, line, and column:

```
parse errors in MyClass.cls: MyClass.cls:15:4: mismatched input 'foo' expecting {<EOF>, ...}
```

Check that your Apex syntax is valid. The parser uses the same ANTLR4 grammar as apexfmt, so if `apexfmt` can format it, epex can parse it.

### Handling managed package dependencies

If your test code calls methods on managed package classes, you have two options:

1. **Mock the package** — Create `.apkg` stubs for the classes your tests reference. The stubs don't need implementation; they just need the right method signatures so the interpreter can resolve calls.

2. **Mock in your test code** — Use dependency injection or test doubles in your Apex code so tests don't call managed package methods directly. This is the same pattern you'd use for Salesforce unit tests.

### Performance

epex is fast because there's no network, no deployment, and no org overhead. Typical test execution:

- Parsing: ~1ms per class file
- Schema build: <1ms for typical projects
- Test execution: depends on complexity, but 10-100x faster than deploying to a scratch org
