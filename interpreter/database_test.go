package interpreter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/engine"
	"github.com/ipavlic/epex/schema"
)

// mockEngine implements EngineInterface for testing Database methods.
type mockEngine struct {
	records   map[string][]map[string]any // sobject -> records
	idCounter int
}

func newMockEngine() *mockEngine {
	return &mockEngine{
		records: make(map[string][]map[string]any),
	}
}

func (m *mockEngine) Insert(sobjectName string, records []map[string]any) error {
	table := strings.ToLower(sobjectName)
	for _, rec := range records {
		if _, ok := rec["Id"]; !ok {
			m.idCounter++
			rec["Id"] = fmt.Sprintf("001%015d", m.idCounter)
		}
		m.records[table] = append(m.records[table], rec)
	}
	return nil
}

func (m *mockEngine) Update(sobjectName string, records []map[string]any) error {
	table := strings.ToLower(sobjectName)
	for _, rec := range records {
		id := rec["Id"]
		for i, existing := range m.records[table] {
			if existing["Id"] == id {
				for k, v := range rec {
					m.records[table][i][k] = v
				}
			}
		}
	}
	return nil
}

func (m *mockEngine) Delete(sobjectName string, records []map[string]any) error {
	table := strings.ToLower(sobjectName)
	for _, rec := range records {
		id := rec["Id"]
		for i, existing := range m.records[table] {
			if existing["Id"] == id {
				m.records[table] = append(m.records[table][:i], m.records[table][i+1:]...)
				break
			}
		}
	}
	return nil
}

func (m *mockEngine) Upsert(sobjectName string, records []map[string]any, externalIdField string) error {
	table := strings.ToLower(sobjectName)
	for _, rec := range records {
		found := false
		extVal := rec[externalIdField]
		if extVal != nil {
			for i, existing := range m.records[table] {
				if existing[externalIdField] == extVal {
					for k, v := range rec {
						m.records[table][i][k] = v
					}
					found = true
					break
				}
			}
		}
		if !found {
			if _, ok := rec["Id"]; !ok {
				m.idCounter++
				rec["Id"] = fmt.Sprintf("001%015d", m.idCounter)
			}
			m.records[table] = append(m.records[table], rec)
		}
	}
	return nil
}

func (m *mockEngine) EnsureTable(sobjectName string) error {
	return nil
}

func (m *mockEngine) GetSchema() *schema.Schema {
	return nil
}

func (m *mockEngine) SObjectTypeForID(id string) string {
	return ""
}

func (m *mockEngine) ResetDatabase() error {
	m.records = make(map[string][]map[string]any)
	m.idCounter = 0
	return nil
}

func (m *mockEngine) QueryFields(fields []string, sobject, where string, whereArgs []any, orderBy string, limit, offset int) ([]map[string]any, error) {
	return m.QueryWithFullParams(&engine.QueryParams{
		Fields:    fields,
		SObject:   sobject,
		Where:     where,
		WhereArgs: whereArgs,
		OrderBy:   orderBy,
		Limit:     limit,
		Offset:    offset,
	})
}

func (m *mockEngine) QueryWithFullParams(params *engine.QueryParams) ([]map[string]any, error) {
	table := strings.ToLower(params.SObject)
	allRecords := m.records[table]
	var results []map[string]any
	for _, rec := range allRecords {
		row := make(map[string]any)
		for _, f := range params.Fields {
			fl := strings.ToLower(f)
			for k, v := range rec {
				if strings.ToLower(k) == fl {
					row[fl] = v
				}
			}
		}
		results = append(results, row)
	}
	if params.Limit > 0 && len(results) > params.Limit {
		results = results[:params.Limit]
	}
	return results, nil
}

// helper: parse apex source and create interpreter with mock engine
func setupWithEngine(t *testing.T, source string) (*Interpreter, *mockEngine) {
	t.Helper()
	result, err := apex.ParseString("test.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	reg := NewRegistry()
	reg.RegisterClass(result.Tree)
	eng := newMockEngine()
	interp := NewInterpreter(reg, eng)
	return interp, eng
}

func TestDatabaseInsert(t *testing.T) {
	source := `
public class DbTest {
    public static Object testInsert() {
        Account acc = new Account();
        acc.Name = 'Test Corp';
        List<Database.SaveResult> results = Database.insert(acc);
        return results.size();
    }
}`
	interp, eng := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testInsert", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 SaveResult, got %v", result)
	}
	if len(eng.records["account"]) != 1 {
		t.Errorf("expected 1 record inserted, got %d", len(eng.records["account"]))
	}
}

func TestDatabaseUpdate(t *testing.T) {
	source := `
public class DbTest {
    public static Object testUpdate() {
        Account acc = new Account();
        acc.Name = 'Old Name';
        Database.insert(acc);
        acc.Name = 'New Name';
        List<Database.SaveResult> results = Database.update(acc);
        return results.size();
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testUpdate", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 SaveResult, got %v", result)
	}
}

func TestDatabaseDelete(t *testing.T) {
	source := `
public class DbTest {
    public static Object testDelete() {
        Account acc = new Account();
        acc.Name = 'Delete Me';
        Database.insert(acc);
        List<Database.DeleteResult> results = Database.delete(acc);
        return results.size();
    }
}`
	interp, eng := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testDelete", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 DeleteResult, got %v", result)
	}
	if len(eng.records["account"]) != 0 {
		t.Errorf("expected 0 records after delete, got %d", len(eng.records["account"]))
	}
}

func TestDatabaseUpsert(t *testing.T) {
	source := `
public class DbTest {
    public static Object testUpsert() {
        Account acc = new Account();
        acc.Name = 'Upsert Me';
        List<Database.UpsertResult> results = Database.upsert(acc);
        return results.size();
    }
}`
	interp, eng := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testUpsert", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 UpsertResult, got %v", result)
	}
	if len(eng.records["account"]) != 1 {
		t.Errorf("expected 1 record after upsert, got %d", len(eng.records["account"]))
	}
}

func TestDatabaseQuery(t *testing.T) {
	source := `
public class DbTest {
    public static Object testQuery() {
        Account acc = new Account();
        acc.Name = 'Query Me';
        Database.insert(acc);
        List<Account> results = Database.query('SELECT Id, Name FROM Account');
        return results.size();
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testQuery", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 query result, got %v", result)
	}
}

func TestDatabaseQueryWithBinds(t *testing.T) {
	source := `
public class DbTest {
    public static Object testQueryWithBinds() {
        Account acc = new Account();
        acc.Name = 'Bind Test';
        Database.insert(acc);
        Map<String, Object> binds = new Map<String, Object>();
        binds.put('name', 'Bind Test');
        List<Account> results = Database.queryWithBinds(
            'SELECT Id, Name FROM Account WHERE Name = :name',
            binds,
            null
        );
        return results.size();
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testQueryWithBinds", nil)
	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1 query result, got %v", result)
	}
}

func TestDatabaseInsertSetsId(t *testing.T) {
	source := `
public class DbTest {
    public static Object testInsertSetsId() {
        Account acc = new Account();
        acc.Name = 'Test';
        Database.insert(acc);
        return acc.Id;
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testInsertSetsId", nil)
	if result.Type == TypeNull || result.Data == nil {
		t.Error("expected Id to be set after insert")
	}
}

func TestDatabaseInsertWithAllOrNone(t *testing.T) {
	source := `
public class DbTest {
    public static Object testInsertAllOrNone() {
        Account acc = new Account();
        acc.Name = 'Test';
        List<Database.SaveResult> results = Database.insert(acc, false);
        return results.get(0).success;
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testInsertAllOrNone", nil)

	// The result should be a field access on the SObject-typed SaveResult
	// Since success is a boolean field, check it
	if result.Type != TypeBoolean || result.Data.(bool) != true {
		t.Errorf("expected success=true, got %v (%v)", result.Data, result.Type)
	}
}

func TestUpdateThenQueryWithBind(t *testing.T) {
	source := `
public class DbTest {
    public static Object testUpdateAndQuery() {
        Account acc = new Account();
        acc.Name = 'Test Corp';
        acc.Industry = 'Technology';
        insert acc;

        acc.Industry = 'Finance';
        update acc;

        Account updatedAcc = [SELECT Industry FROM Account WHERE Id = :acc.Id];
        return updatedAcc.Industry;
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testUpdateAndQuery", nil)
	if result.Type != TypeString || result.Data.(string) != "Finance" {
		t.Errorf("expected 'Finance', got %v", result)
	}
}

func TestDatabaseSaveResultFields(t *testing.T) {
	source := `
public class DbTest {
    public static Object testSaveResult() {
        Account acc = new Account();
        acc.Name = 'Test';
        List<Database.SaveResult> results = Database.insert(acc);
        Database.SaveResult sr = results.get(0);
        return sr.Id;
    }
}`
	interp, _ := setupWithEngine(t, source)
	result := interp.ExecuteMethod("DbTest", "testSaveResult", nil)
	if result.Type == TypeNull || result.Data == nil {
		t.Error("expected SaveResult.Id to be set")
	}
}

func TestDatabaseIsolationBetweenTests(t *testing.T) {
	// Two test methods: first inserts a record, second queries and expects 0 results.
	// This verifies that database state is reset between test runs.
	source := `
@isTest
public class IsolationTest {
    @isTest
    static void testInsertRecord() {
        Account acc = new Account();
        acc.Name = 'Leaked?';
        Database.insert(acc);
        List<Account> results = Database.query('SELECT Id, Name FROM Account');
        System.assertEquals(1, results.size());
    }
    @isTest
    static void testEmptyDatabase() {
        List<Account> results = Database.query('SELECT Id, Name FROM Account');
        System.assertEquals(0, results.size());
    }
}`
	interp, eng := setupWithEngine(t, source)
	_ = eng
	results := interp.RunTests()
	for _, r := range results {
		if !r.Passed {
			t.Errorf("test %s.%s failed: %s", r.ClassName, r.MethodName, r.Error)
		}
	}
}

// setupWithRealEngine creates an interpreter with a real SQLite-backed engine.
func setupWithRealEngine(t *testing.T, source string) *Interpreter {
	t.Helper()
	result, err := apex.ParseString("test.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	s, err := schema.BuildSchemaFromDir("../testdata/objects")
	if err != nil {
		t.Fatalf("building schema: %v", err)
	}
	eng, err := engine.NewEngine(s)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	reg := NewRegistry()
	reg.RegisterClass(result.Tree)
	return NewInterpreter(reg, eng)
}

func TestParentRelationshipSOQL(t *testing.T) {
	source := `
public class RelTest {
    public static Object testParentField() {
        Account acc = new Account();
        acc.Name = 'Acme Corp';
        insert acc;

        Contact con = new Contact();
        con.LastName = 'Doe';
        con.AccountId = acc.Id;
        insert con;

        Contact result = [SELECT Id, Account.Name FROM Contact WHERE Id = :con.Id];
        return result.Account.Name;
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("RelTest", "testParentField", nil)
	if result.Type != TypeString || result.Data.(string) != "Acme Corp" {
		t.Errorf("expected 'Acme Corp', got %v (type=%v)", result.Data, result.Type)
	}
}

func TestChildSubquerySOQL(t *testing.T) {
	source := `
public class RelTest {
    public static Object testChildSubquery() {
        Account acc = new Account();
        acc.Name = 'Acme Corp';
        insert acc;

        Contact con1 = new Contact();
        con1.LastName = 'Doe';
        con1.AccountId = acc.Id;
        insert con1;

        Contact con2 = new Contact();
        con2.LastName = 'Smith';
        con2.AccountId = acc.Id;
        insert con2;

        Account result = [SELECT Id, Name, (SELECT Id, LastName FROM Contacts) FROM Account WHERE Id = :acc.Id];
        return result.Contacts.size();
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("RelTest", "testChildSubquery", nil)
	if result.Type != TypeInteger || result.Data.(int) != 2 {
		t.Errorf("expected 2, got %v (type=%v)", result.Data, result.Type)
	}
}

func TestMultiLevelParentRelationshipSOQL(t *testing.T) {
	source := `
public class RelTest {
    public static Object testMultiLevel() {
        Account acc = new Account();
        acc.Name = 'Acme Corp';
        insert acc;

        Opportunity opp = new Opportunity();
        opp.Name = 'Big Deal';
        opp.AccountId = acc.Id;
        opp.StageName = 'Closed Won';
        insert opp;

        OpportunityLineItem oli = new OpportunityLineItem();
        oli.Name = 'Line 1';
        oli.OpportunityId = opp.Id;
        oli.Quantity = 10;
        oli.UnitPrice = 100;
        insert oli;

        OpportunityLineItem result = [SELECT Id, Opportunity.Account.Name FROM OpportunityLineItem WHERE Id = :oli.Id];
        return result.Opportunity.Account.Name;
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("RelTest", "testMultiLevel", nil)
	if result.Type != TypeString || result.Data.(string) != "Acme Corp" {
		t.Errorf("expected 'Acme Corp', got %v (type=%v)", result.Data, result.Type)
	}
}

func TestTypeOfPolymorphicSOQL(t *testing.T) {
	source := `
public class TypeOfTest {
    public static Object testTypeOf() {
        Account acc = new Account();
        acc.Name = 'Acme Corp';
        acc.Phone = '555-1234';
        insert acc;

        Task tsk = new Task();
        tsk.Subject = 'Call Acme';
        tsk.WhatId = acc.Id;
        insert tsk;

        List<Task> tasks = [SELECT Id, Subject, TYPEOF What WHEN Account THEN Phone, Name WHEN Opportunity THEN StageName, Name ELSE Name END FROM Task];
        Task result = tasks[0];
        return result.What.Name;
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("TypeOfTest", "testTypeOf", nil)
	if result.Type != TypeString || result.Data.(string) != "Acme Corp" {
		t.Errorf("expected 'Acme Corp', got %v (type=%v)", result.Data, result.Type)
	}
}

func TestSOQLAggregateCountApex(t *testing.T) {
	source := `
public class AggTest {
    public static Object testCount() {
        insert new Account(Name = 'A');
        insert new Account(Name = 'B');
        insert new Account(Name = 'C');

        List<AggregateResult> results = [SELECT COUNT() cnt FROM Account];
        return results[0].get('cnt');
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("AggTest", "testCount", nil)
	got := result.ToGoValue()
	// COUNT returns int64 from SQLite.
	if got != 3 {
		t.Errorf("expected 3, got %v (type %T)", got, got)
	}
}

func TestSOQLGroupByApex(t *testing.T) {
	source := `
public class GBTest {
    public static Object testGroupBy() {
        insert new Account(Name = 'Acme');
        insert new Account(Name = 'Acme');
        insert new Account(Name = 'Globex');

        List<AggregateResult> results = [SELECT Name, COUNT(Id) cnt FROM Account GROUP BY Name ORDER BY Name];
        // First group is Acme with count 2.
        return results[0].get('cnt');
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("GBTest", "testGroupBy", nil)
	got := result.ToGoValue()
	if got != 2 {
		t.Errorf("expected 2, got %v (type %T)", got, got)
	}
}

func TestSOQLHavingApex(t *testing.T) {
	source := `
public class HavTest {
    public static Object testHaving() {
        insert new Account(Name = 'Acme');
        insert new Account(Name = 'Acme');
        insert new Account(Name = 'Globex');

        List<AggregateResult> results = [SELECT Name, COUNT(Id) cnt FROM Account GROUP BY Name HAVING COUNT(Id) > 1];
        return results[0].get('Name');
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("HavTest", "testHaving", nil)
	got := result.ToGoValue()
	if got != "Acme" {
		t.Errorf("expected 'Acme', got %v", got)
	}
}

func TestSOQLSemiJoinApex(t *testing.T) {
	source := `
public class SemiTest {
    public static Object testSemiJoin() {
        Account acc = new Account(Name = 'Acme');
        insert acc;
        Account acc2 = new Account(Name = 'Other');
        insert acc2;

        Contact con1 = new Contact(LastName = 'Smith', AccountId = acc.Id);
        insert con1;
        Contact con2 = new Contact(LastName = 'Jones', AccountId = acc2.Id);
        insert con2;

        List<Contact> results = [SELECT Id, LastName FROM Contact WHERE AccountId IN (SELECT Id FROM Account WHERE Name = 'Acme')];
        return results.size();
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("SemiTest", "testSemiJoin", nil)
	got := result.ToGoValue()
	if got != 1 {
		t.Errorf("expected 1, got %v (type %T)", got, got)
	}
}

func TestIdGetSobjectType(t *testing.T) {
	source := `
public class IdSTypeTest {
    public static Object testGetSobjectType() {
        Account acc = new Account(Name = 'Test');
        insert acc;
        Id accId = acc.Id;
        Schema.SObjectType stype = accId.getSobjectType();
        Schema.DescribeSObjectResult describe = stype.getDescribe();
        return describe.getName();
    }
}`
	interp := setupWithRealEngine(t, source)
	result := interp.ExecuteMethod("IdSTypeTest", "testGetSobjectType", nil)
	if result.Type != TypeString || result.Data.(string) != "Account" {
		t.Errorf("expected 'Account', got %v (type=%v)", result.Data, result.Type)
	}
}

func TestSchemaGetGlobalDescribe(t *testing.T) {
	source := `
public class SchemaTest {
    public static Object testGlobalDescribe() {
        Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
        return gd.size();
    }
    public static Object testDescribeFields() {
        Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
        Schema.SObjectType accType = gd.get('account');
        Schema.DescribeSObjectResult describe = accType.getDescribe();
        Map<String, Schema.SObjectField> fieldMap = describe.fields.getMap();
        return fieldMap.size();
    }
    public static Object testFieldDescribe() {
        Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
        Schema.SObjectType accType = gd.get('account');
        Schema.DescribeSObjectResult describe = accType.getDescribe();
        Map<String, Schema.SObjectField> fieldMap = describe.fields.getMap();
        Schema.DescribeFieldResult nameField = fieldMap.get('name');
        return nameField.getName();
    }
}`
	interp := setupWithRealEngine(t, source)

	// getGlobalDescribe returns a map with at least Account and Contact
	result := interp.ExecuteMethod("SchemaTest", "testGlobalDescribe", nil)
	size, ok := result.Data.(int)
	if !ok || size < 2 {
		t.Errorf("expected at least 2 SObjects in global describe, got %v (type %T)", result.Data, result.Data)
	}

	// describe.fields.getMap() returns field map
	result = interp.ExecuteMethod("SchemaTest", "testDescribeFields", nil)
	fieldSize, ok := result.Data.(int)
	if !ok || fieldSize < 1 {
		t.Errorf("expected at least 1 field in Account describe, got %v", result.Data)
	}

	// Individual field describe
	result = interp.ExecuteMethod("SchemaTest", "testFieldDescribe", nil)
	if result.Type != TypeString || result.Data.(string) != "Name" {
		t.Errorf("expected 'Name', got %v", result.Data)
	}
}
