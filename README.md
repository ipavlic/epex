# epex

A local Apex interpreter and test runner written in Go. Executes Apex code on your workstation without deploying to Salesforce.

## Architecture

```
                     ┌─────────────────────────────────────────────────┐
                     │              Package System                     │
                     │  epex.json ──► Load manifest               │
                     │  .apkg mocks    ──► Load stub packages          │
                     │  SFDX source    ──► Create local package        │
                     │         ▼ namespace-qualified resolution         │
                     └─────────────────────────────────────────────────┘
                                          │
.cls files ──► ANTLR4 Parser ──► AST ──► Tree-Walking Interpreter ──► Tracer
                                              │            │              │
.object-meta.xml ──► Schema Builder ──► Schema │            │         Perfetto
                                              ▼            ▼          JSON
                                         SQLite DB    DML/SOQL Engine
```

### 1. Parse (ANTLR4)

Apex source files are parsed into ASTs using the ANTLR4 grammar from [octoberswimmer/apexfmt](https://github.com/octoberswimmer/apexfmt).

### 2. Schema Build

SObject metadata is read from SFDX-style `.object-meta.xml` and `.field-meta.xml` files to construct an in-memory schema. Standard fields (Id, Name, CreatedDate, etc.) are auto-added to every SObject. 34 standard Salesforce objects (Account, Contact, Lead, Opportunity, Case, ObjectPermissions, FieldPermissions, etc.) have complete built-in field definitions — the schema builder automatically merges these when building from SFDX metadata, so `.field-meta.xml` files are not needed for standard objects.

### 3. Interpret

A tree-walking interpreter visits AST nodes via `visitNode()`, which dispatches each parse tree node to a typed visitor method. Every expression returns a `*Value`.

**Expressions** evaluate bottom-up through the precedence hierarchy:
- Literals → primary (variable lookup, `this`, `new`) → unary → multiplicative (`*`, `/`) → additive (`+`, `-`, string concat) → comparison → equality (null-safe) → logical AND/OR (short-circuit) → ternary → null coalescing (`??`)

**Method dispatch** has two entry points:
- **Dot expressions** (`target.method(args)`): resolves the target, then tries static class dispatch (`System`, `Database`, `Math`, user classes), builtin instance methods, user-defined instance methods, and finally field access (SObject dot notation)
- **Unqualified calls** (`method(args)`): looks up the method in the current class

User-defined methods execute by pushing a new scope, binding parameters, running the body, and catching return values via `ReturnException` (panic/recover). `break`/`continue` use the same pattern for loop control.

**Assignment** handles three forms: simple variable (`x = v`), dot expression (`obj.field = v`, case-insensitive), and array index (`list[i] = v`).

**Scopes** are lexically stacked — each block/method pushes a new scope, and variable lookup walks up the chain.

### 4. DML/SOQL

DML statements (`insert acc;`) and `Database.*` static methods (`Database.insert(acc)`, `Database.update`, `Database.delete`, `Database.upsert`) write to an in-memory SQLite database. IDs are written back to SObject instances after insert/upsert. `Database.*` methods return result objects (`SaveResult`, `DeleteResult`, `UpsertResult`) with `Id`, `success`, and `errors` fields. DML supports access modes (`INSERT AS USER`, `INSERT AS SYSTEM`) — `AS USER` enforces CRUD permissions via ObjectPermissions records, `AS SYSTEM` bypasses checks.

SOQL is supported in two forms:
- **Inline SOQL** (`[SELECT Id FROM Account WHERE ...]`) — parsed at compile time, fields passed individually to the engine
- **Dynamic SOQL** (`Database.query('SELECT ...')`, `Database.queryWithBinds(...)`) — the SOQL string is parsed at runtime using the same ANTLR grammar, then the parse tree is walked to extract query parameters

SOQL supports access mode clauses: `WITH USER_MODE` (enforces CRUD + FLS + sharing, throws on inaccessible fields), `WITH SECURITY_ENFORCED` (enforces CRUD + FLS, throws on inaccessible fields, no sharing), and `WITH SYSTEM_MODE` (bypasses all checks). Sharing rules are also enforced by the class-level `with sharing` / `without sharing` keywords.

All field access is case-insensitive, matching Apex/SOQL semantics.

**Dynamic table creation:** When an SObject is used in DML or SOQL and no table exists yet, the engine automatically creates the table from the built-in standard object definition. This means tests can use standard objects (Account, Contact, etc.) without any metadata files.

**Database isolation:** Each `@isTest` method starts with a clean database — all tables are emptied and ID counters are reset between test methods. Records inserted in one test are not visible in another.

### 5. Tracing

With tracing enabled, the interpreter records every line execution, method entry/exit, SOQL query, DML operation, and assertion. Trace output is Perfetto-compatible JSON, viewable in [ui.perfetto.dev](https://ui.perfetto.dev). An aggregated summary shows per-method timing, SOQL/DML stats, and a line heat map.

### 6. Packages & Namespaces

Each package runs in its own namespace, mimicking Salesforce's managed package boundaries. Mock packages are snapshots of managed package metadata (class signatures, SObject definitions) — no implementation, just stubs. Cross-package access enforces `global`/`public` visibility rules.

## Project Structure

```
epex/
├── apex/                   # Step 1: Apex file parser (convenience API)
│   ├── parse.go            #   ParseFile, ParseString, ParseDirectory
│   └── parse_test.go
│
├── parser/                 # ANTLR4-generated Go parser (do not edit)
│   ├── apex_lexer.go       #   Generated lexer
│   ├── apex_parser.go      #   Generated parser (~1MB, all grammar rules)
│   ├── apexparser_visitor.go     # Visitor interface
│   ├── apexparser_base_visitor.go # Base visitor (no-op defaults)
│   ├── apexparser_listener.go    # Listener interface
│   └── apexparser_base_listener.go
│
├── grammar/                # ANTLR4 grammar source
│   ├── ApexLexer.g4        #   Lexer grammar (from apexfmt)
│   ├── ApexParser.g4       #   Parser grammar (from apexfmt)
│   ├── antlr-4.13.1-complete.jar
│   ├── generate.go         #   go:generate directive
│   └── generate.sh         #   Runs ANTLR to regenerate parser/
│
├── schema/                 # Step 2: SObject schema builder
│   ├── schema.go           #   SObjectSchema, SObjectField, FieldType types
│   ├── builder.go          #   Reads .object-meta.xml + .field-meta.xml
│   ├── standard_objects.go #   Built-in definitions for 34 Salesforce standard objects
│   └── builder_test.go
│
├── interpreter/            # Step 4: Tree-walking Apex interpreter
│   ├── value.go            #   Runtime value system (null, bool, int, string, SObject, List, Map, Set)
│   ├── env.go              #   Lexical scope / environment chain
│   ├── registry.go         #   Class & method registry (built from ASTs)
│   ├── interpreter.go      #   AST visitor: expressions, statements, DML, SOQL
│   ├── context.go          #   Execution context: runAs, sharing mode, permission checks
│   ├── builtins.go         #   Built-in class methods (System, Assert, String, List, Map, Set, Database, Security, ...)
│   ├── builtins_string.go  #   String instance methods
│   ├── builtins_math.go    #   Math class methods
│   ├── builtins_datetime.go #  Date/DateTime/Time methods
│   ├── builtins_decimal.go #   Decimal instance methods
│   ├── builtins_json.go    #   JSON serialization/deserialization
│   ├── builtins_pattern.go #   Pattern & Matcher methods
│   ├── builtins_url.go     #   URL/EncodingUtil methods
│   ├── builtins_limits.go  #   Limits class methods
│   ├── builtins_crypto.go  #   Crypto class methods
│   ├── triggers.go         #   Trigger execution engine
│   ├── trigger_test.go
│   ├── testrunner.go       #   @isTest discovery and execution
│   ├── interpreter_test.go
│   ├── database_test.go    #   Database DML/SOQL method tests
│   └── context_test.go     #   runAs, sharing, permissions, stripInaccessible tests
│
├── engine/                 # Step 5: SQLite-backed DML/SOQL engine
│   ├── db.go               #   SQLite setup, table creation from schema
│   ├── id.go               #   Salesforce-style 18-char ID generation
│   ├── dml.go              #   INSERT, UPDATE, DELETE, UPSERT operations
│   ├── soql.go             #   SOQL-to-SQL translation and query execution
│   ├── engine.go           #   Top-level engine wiring DB + IDGenerator + Schema
│   └── engine_test.go
│
├── tracer/                 # Step 6: Apex-level execution tracing
│   ├── tracer.go           #   TraceEvent, Tracer interface, NoopTracer, RecordingTracer
│   ├── perfetto.go         #   Chrome Trace Event JSON export for Perfetto
│   ├── summary.go          #   Aggregate stats: per-method, SOQL, DML, line heat map
│   ├── summary_human.go    #   Human-readable summary tables
│   ├── summary_json.go     #   JSON summary output
│   └── tracer_test.go
│
├── reporter/               # Test result formatting
│   ├── reporter.go         #   TestRunResult, Summary types
│   ├── human.go            #   Human-readable output (sf-style)
│   ├── json.go             #   JSON output
│   └── reporter_test.go
│
├── pkg/                    # Step 7: Package & namespace system
│   ├── package.go          #   Package model, access rules
│   ├── manifest.go         #   epex.json manifest parsing
│   ├── create.go           #   Scan SFDX source → Package
│   ├── mock.go             #   Save/load .apkg mock files
│   ├── loader.go           #   Load project: manifest + mocks + local source
│   ├── resolve.go          #   Namespace-qualified name resolution
│   └── pkg_test.go
│
├── cmd/epex/          # CLI entry point
│   └── main.go
│
├── testdata/               # Sample SFDX data for tests
│   ├── classes/            #   AccountService.cls, AccountServiceTest.cls
│   └── objects/            #   Account/, Contact/ with field definitions
│
├── PLAN.md                 # Detailed implementation plan
├── go.mod
└── go.sum
```

### parser/ vs apex/

These two packages both deal with parsing but serve different roles:

| | `parser/` | `apex/` |
|---|---|---|
| **Source** | Auto-generated by ANTLR4 from `.g4` grammars | Hand-written |
| **Editable** | No — regenerate via `go generate ./grammar/...` | Yes |
| **Contains** | Lexer, parser, all AST node types (`*Context`), visitor/listener interfaces | Convenience functions: `ParseFile`, `ParseString`, `ParseDirectory` |
| **Role** | The parsing machine | User-friendly API on top of it |
| **Size** | ~1.5MB of generated code | ~80 lines |

The interpreter imports both: `apex/` to load files, and `parser/` for the AST node types it walks.

## Usage

```bash
# Run tests from a directory
epex run ./my-project

# Run with tracing
epex run ./my-project --trace

# Validate Apex syntax only
epex parse ./my-project/classes/

# Run Go unit tests
go test ./...
```

### Project setup with packages

Create an `epex.json` in your project root:

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

Create a mock package from an existing local package:

```go
// Save a package's metadata as a mock .apkg file
pkg.SaveMock("packages/ringdna.apkg", ringdnaPkg)
```

Load the full project (manifest + mocks + local source):

```go
result, err := pkg.LoadProject(".")
// result.LocalPackage  — your code
// result.MockPackages  — stub dependencies
// result.Schema        — merged schema from all packages
// result.Resolver      — namespace-qualified class resolution
```

### Output example

```
=== Step 1: Parsing Apex Classes ===
  testdata/classes/AccountService.cls      OK
  testdata/classes/AccountServiceTest.cls  OK
Parsed 2 class(es)

=== Step 2: Building SObject Schema ===
  Account (Account)
    Id                             Id
    Name                           Text
    AnnualRevenue                  Currency
    Industry                       Picklist
    ...
  Contact (Contact)
    Id                             Id
    AccountId                      Lookup -> Account
    Email                          Email
    ...
Built schema for 2 SObject(s)
```

### Test result formats

Human format (like `sf apex run test --result-format human`):

```
=== Test Results
 OUTCOME  TEST NAME                                    RUNTIME  MESSAGE
 ───────  ────────────────────────────────────────  ──────────  ────────
  Pass    AccountServiceTest.testCreateAccount            12ms
  Fail    ContactServiceTest.testInvalidEmail              3ms  Assert.areEqual failed: ...

=== Test Summary
 Outcome:         Fail
 Tests Ran:       2
 Passing:         1
 Failing:         1
 Pass Rate:       50%
```

JSON format is also available for CI/CD integration.

### Trace output

With tracing enabled, generates Perfetto-compatible JSON viewable at [ui.perfetto.dev](https://ui.perfetto.dev), plus an aggregated summary:

```
=== Method Performance
 METHOD                                    CALLS  TOTAL       AVG
 AccountService.getHighValueAccounts           3    45.0ms    15.0ms

=== SOQL Queries
 QUERY                                     CALLS       ROWS      TIME
 SELECT Id, Name FROM Account WHERE ...        3         15    30.0ms

=== Hot Lines
 LOCATION                                  EXECUTIONS
 AccountService.cls:15                            300
```

## Regenerating the Parser

If you modify the `.g4` grammar files:

```bash
cd grammar
go generate ./...
```

Requires Java 17+ (for ANTLR 4.13.1).

## Dependencies

| Dependency | Purpose |
|---|---|
| `github.com/antlr4-go/antlr/v4` v4.13.0 | ANTLR4 Go runtime |
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO) |

## Limitations

This is an interpreter, not a full Salesforce runtime:

- `System.runAs(user)` supported with full user context (UserInfo, sharing, OwnerId)
- Sharing keywords (`with sharing`, `without sharing`, `inherited sharing`) enforced with call-chain inheritance
- DML access modes (`INSERT AS USER`, `INSERT AS SYSTEM`) with CRUD enforcement
- SOQL access modes (`WITH USER_MODE`, `WITH SYSTEM_MODE`, `WITH SECURITY_ENFORCED`) with FLS enforcement
- `Security.stripInaccessible()` for silent field stripping based on permissions
- Permission checks use ObjectPermissions/FieldPermissions SObjects ("no row = full access")
- Triggers supported: BEFORE/AFTER INSERT/UPDATE/DELETE with Trigger.new, Trigger.old, context variables
- 34 standard objects have built-in field definitions (Account, Contact, Lead, Opportunity, Case, ObjectPermissions, FieldPermissions, etc.)
- No governor limits enforcement
- No workflow rules or flows
- SOQL support is a subset (limited aggregates)
- No HTTP callouts or platform events
- Managed package mocking captures signatures only, not implementation
