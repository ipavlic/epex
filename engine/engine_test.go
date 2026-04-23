package engine

import (
	"fmt"
	"testing"

	"github.com/ipavlic/epex/schema"
)

// testSchema loads the schema from the testdata/objects directory.
func testSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.BuildSchemaFromDir("../testdata/objects")
	if err != nil {
		t.Fatalf("building schema: %v", err)
	}
	return s
}

func TestCreateTablesFromSchema(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Verify tables exist by querying them.
	for name := range s.SObjects {
		results, err := eng.Query(&QueryParams{
			Fields:  []string{"Id"},
			SObject: name,
		})
		if err != nil {
			t.Errorf("querying %s: %v", name, err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 rows in %s, got %d", name, len(results))
		}
	}
}

func TestInsertAndQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "Acme Corp"},
		{"Name": "Globex Inc"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Both records should have IDs assigned.
	for i, rec := range records {
		id, ok := rec["Id"]
		if !ok || id == nil || id == "" {
			t.Errorf("record %d missing Id after insert", i)
		}
	}

	// Query all accounts.
	results, err := eng.Query(&QueryParams{
		Fields:  []string{"Id", "Name"},
		SObject: "Account",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Query with WHERE.
	results, err = eng.Query(&QueryParams{
		Fields:    []string{"Id", "Name"},
		SObject:   "Account",
		Where:     "\"name\" = ?",
		WhereArgs: []any{"Acme Corp"},
	})
	if err != nil {
		t.Fatalf("Query with WHERE: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["name"] != "Acme Corp" {
		t.Errorf("expected name 'Acme Corp', got %v", results[0]["name"])
	}
}

func TestUpdate(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "OldName"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	id := records[0]["Id"]

	// Update the name.
	updateRecs := []map[string]any{
		{"Id": id, "Name": "NewName"},
	}
	if err := eng.Update("Account", updateRecs); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify.
	results, err := eng.Query(&QueryParams{
		Fields:    []string{"Name"},
		SObject:   "Account",
		Where:     "\"id\" = ?",
		WhereArgs: []any{id},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["name"] != "NewName" {
		t.Errorf("expected 'NewName', got %v", results[0]["name"])
	}
}

func TestDelete(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "ToDelete"},
		{"Name": "ToKeep"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Delete the first record.
	deleteRecs := []map[string]any{
		{"Id": records[0]["Id"]},
	}
	if err := eng.Delete("Account", deleteRecs); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify only one remains.
	results, err := eng.Query(&QueryParams{
		Fields:  []string{"Id", "Name"},
		SObject: "Account",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["name"] != "ToKeep" {
		t.Errorf("expected 'ToKeep', got %v", results[0]["name"])
	}
}

func TestUpsert(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert initial record.
	records := []map[string]any{
		{"Name": "UpsertMe"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	existingId := records[0]["Id"]

	// Upsert: update existing by Id, insert new.
	upsertRecs := []map[string]any{
		{"Id": existingId, "Name": "Updated"},
		{"Name": "BrandNew"},
	}
	if err := eng.Upsert("Account", upsertRecs, "Id"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Verify: should have 2 records total.
	results, err := eng.Query(&QueryParams{
		Fields:  []string{"Id", "Name"},
		SObject: "Account",
		OrderBy: "\"name\" ASC",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Check the existing one was updated.
	found := false
	for _, r := range results {
		if r["id"] == existingId {
			if r["name"] != "Updated" {
				t.Errorf("expected updated name 'Updated', got %v", r["name"])
			}
			found = true
		}
	}
	if !found {
		t.Error("original record not found after upsert")
	}
}

func TestQueryWithLimitAndOffset(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "A"},
		{"Name": "B"},
		{"Name": "C"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Query with limit.
	results, err := eng.Query(&QueryParams{
		Fields:  []string{"Name"},
		SObject: "Account",
		OrderBy: "\"name\" ASC",
		Limit:   2,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with limit, got %d", len(results))
	}

	// Query with limit and offset.
	results, err = eng.Query(&QueryParams{
		Fields:  []string{"Name"},
		SObject: "Account",
		OrderBy: "\"name\" ASC",
		Limit:   2,
		Offset:  1,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with offset, got %d", len(results))
	}
	if results[0]["name"] != "B" {
		t.Errorf("expected first result 'B', got %v", results[0]["name"])
	}
}

func TestIDGenerator(t *testing.T) {
	gen := NewIDGenerator()

	id1 := gen.Generate("Account")
	id2 := gen.Generate("Account")
	id3 := gen.Generate("Contact")

	if len(id1) != 18 {
		t.Errorf("expected 18-char ID, got %d: %s", len(id1), id1)
	}
	if id1 == id2 {
		t.Error("sequential IDs should be different")
	}
	if id1[:3] != "001" {
		t.Errorf("Account prefix should be 001, got %s", id1[:3])
	}
	if id3[:3] != "003" {
		t.Errorf("Contact prefix should be 003, got %s", id3[:3])
	}

	// Unknown SObject should get default prefix.
	id4 := gen.Generate("CustomObj__c")
	if id4[:3] != "a0C" {
		t.Errorf("custom object prefix should be a0C, got %s", id4[:3])
	}
}

func TestParentRelationshipQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert an Account.
	accounts := []map[string]any{
		{"Name": "Acme Corp"},
	}
	if err := eng.Insert("Account", accounts); err != nil {
		t.Fatalf("Insert Account: %v", err)
	}
	acctId := accounts[0]["Id"]

	// Insert a Contact linked to the Account.
	// The test schema has AccountId__c as the lookup field.
	contacts := []map[string]any{
		{"Name": "John Doe", "AccountId": acctId},
	}
	if err := eng.Insert("Contact", contacts); err != nil {
		t.Fatalf("Insert Contact: %v", err)
	}

	// Query Contact with parent field Account.Name.
	// The convention: relationship name "Account" resolves via AccountId.
	results, err := eng.QueryWithFullParams(&QueryParams{
		Fields: []string{"Id", "Name"},
		ParentFields: []ParentField{
			{Path: []string{"Account"}, FieldName: "Name"},
		},
		SObject: "Contact",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The parent field should be returned as "account.name".
	parentName, ok := results[0]["account.name"]
	if !ok {
		t.Fatalf("expected 'account.name' key in results, got keys: %v", results[0])
	}
	if parentName != "Acme Corp" {
		t.Errorf("expected parent name 'Acme Corp', got %v", parentName)
	}
}

func TestChildSubQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert Accounts.
	accounts := []map[string]any{
		{"Name": "Acme Corp"},
		{"Name": "Globex Inc"},
	}
	if err := eng.Insert("Account", accounts); err != nil {
		t.Fatalf("Insert Account: %v", err)
	}
	acctId1 := accounts[0]["Id"]
	acctId2 := accounts[1]["Id"]

	// Insert Contacts linked to Accounts.
	contacts := []map[string]any{
		{"Name": "John Doe", "AccountId": acctId1},
		{"Name": "Jane Smith", "AccountId": acctId1},
		{"Name": "Bob Jones", "AccountId": acctId2},
	}
	if err := eng.Insert("Contact", contacts); err != nil {
		t.Fatalf("Insert Contact: %v", err)
	}

	// Query Accounts with child subquery for Contacts.
	results, err := eng.QueryWithFullParams(&QueryParams{
		Fields: []string{"Id", "Name"},
		SubQueries: []ChildSubQuery{
			{
				RelationshipName: "Contacts",
				Fields:           []string{"Id", "Name"},
			},
		},
		SObject: "Account",
		OrderBy: "\"name\" ASC",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Acme Corp should have 2 contacts.
	acmeRow := results[0]
	if acmeRow["name"] != "Acme Corp" {
		t.Fatalf("expected first row 'Acme Corp', got %v", acmeRow["name"])
	}
	acmeContacts, ok := acmeRow["contacts"].([]map[string]any)
	if !ok {
		t.Fatalf("expected 'contacts' to be []map[string]any, got %T", acmeRow["contacts"])
	}
	if len(acmeContacts) != 2 {
		t.Errorf("expected 2 contacts for Acme, got %d", len(acmeContacts))
	}

	// Globex Inc should have 1 contact.
	globexRow := results[1]
	globexContacts, ok := globexRow["contacts"].([]map[string]any)
	if !ok {
		t.Fatalf("expected 'contacts' to be []map[string]any, got %T", globexRow["contacts"])
	}
	if len(globexContacts) != 1 {
		t.Errorf("expected 1 contact for Globex, got %d", len(globexContacts))
	}
}

func TestMultiLevelParentRelationshipQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert Account.
	accounts := []map[string]any{
		{"Name": "Acme Corp"},
	}
	if err := eng.Insert("Account", accounts); err != nil {
		t.Fatalf("Insert Account: %v", err)
	}
	acctId := accounts[0]["Id"]

	// Insert Opportunity linked to Account.
	opps := []map[string]any{
		{"Name": "Big Deal", "AccountId": acctId, "StageName": "Closed Won"},
	}
	if err := eng.Insert("Opportunity", opps); err != nil {
		t.Fatalf("Insert Opportunity: %v", err)
	}
	oppId := opps[0]["Id"]

	// Insert OpportunityLineItem linked to Opportunity.
	items := []map[string]any{
		{"Name": "Line Item 1", "OpportunityId": oppId, "Quantity": 10, "UnitPrice": 100},
	}
	if err := eng.Insert("OpportunityLineItem", items); err != nil {
		t.Fatalf("Insert OpportunityLineItem: %v", err)
	}

	// Query OpportunityLineItem with multi-level parent: Opportunity.Account.Name
	results, err := eng.QueryWithFullParams(&QueryParams{
		Fields: []string{"Id", "Name"},
		ParentFields: []ParentField{
			{Path: []string{"Opportunity", "Account"}, FieldName: "Name"},
		},
		SObject: "OpportunityLineItem",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// The multi-level parent field should be returned as "opportunity.account.name".
	parentName, ok := results[0]["opportunity.account.name"]
	if !ok {
		t.Fatalf("expected 'opportunity.account.name' key in results, got keys: %v", results[0])
	}
	if parentName != "Acme Corp" {
		t.Errorf("expected parent name 'Acme Corp', got %v", parentName)
	}
}

func TestTypeOfPolymorphicQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert Account and Opportunity.
	accounts := []map[string]any{
		{"Name": "Acme Corp", "Phone": "555-1234"},
	}
	if err := eng.Insert("Account", accounts); err != nil {
		t.Fatalf("Insert Account: %v", err)
	}
	acctId := accounts[0]["Id"]

	opps := []map[string]any{
		{"Name": "Big Deal", "StageName": "Closed Won", "AccountId": acctId},
	}
	if err := eng.Insert("Opportunity", opps); err != nil {
		t.Fatalf("Insert Opportunity: %v", err)
	}
	oppId := opps[0]["Id"]

	// Insert two Tasks: one pointing to Account, one to Opportunity.
	tasks := []map[string]any{
		{"Subject": "Call Acme", "WhatId": acctId},
	}
	if err := eng.Insert("Task", tasks); err != nil {
		t.Fatalf("Insert Task (account): %v", err)
	}
	tasks2 := []map[string]any{
		{"Subject": "Follow Up Deal", "WhatId": oppId},
	}
	if err := eng.Insert("Task", tasks2); err != nil {
		t.Fatalf("Insert Task (opportunity): %v", err)
	}

	// Query with TYPEOF-style params.
	results, err := eng.QueryWithFullParams(&QueryParams{
		Fields:  []string{"Id", "Subject"},
		SObject: "Task",
		TypeOfFields: []TypeOfField{
			{
				FieldName: "What",
				FKField:   "WhatId",
				WhenClauses: []TypeOfWhen{
					{SObjectType: "Account", Fields: []string{"Phone", "Name"}},
					{SObjectType: "Opportunity", Fields: []string{"StageName", "Name"}},
				},
				ElseFields: []string{"Name"},
			},
		},
		OrderBy: "\"subject\"",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result: "Call Acme" -> Account
	whatAcct, ok := results[0]["what"]
	if !ok {
		t.Fatalf("expected 'what' key in result[0], got keys: %v", results[0])
	}
	acctMap, ok := whatAcct.(map[string]any)
	if !ok {
		t.Fatalf("expected what to be map[string]any, got %T", whatAcct)
	}
	if acctMap["name"] != "Acme Corp" {
		t.Errorf("expected Account name 'Acme Corp', got %v", acctMap["name"])
	}
	if acctMap["phone"] != "555-1234" {
		t.Errorf("expected Account phone '555-1234', got %v", acctMap["phone"])
	}
	if acctMap["type"] != "account" {
		t.Errorf("expected type 'account', got %v", acctMap["type"])
	}

	// Second result: "Follow Up Deal" -> Opportunity
	whatOpp, ok := results[1]["what"]
	if !ok {
		t.Fatalf("expected 'what' key in result[1], got keys: %v", results[1])
	}
	oppMap, ok := whatOpp.(map[string]any)
	if !ok {
		t.Fatalf("expected what to be map[string]any, got %T", whatOpp)
	}
	if oppMap["name"] != "Big Deal" {
		t.Errorf("expected Opportunity name 'Big Deal', got %v", oppMap["name"])
	}
	if oppMap["stagename"] != "Closed Won" {
		t.Errorf("expected Opportunity stageName 'Closed Won', got %v", oppMap["stagename"])
	}
}

func TestAggregateCount(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "Acme"},
		{"Name": "Globex"},
		{"Name": "Initech"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := eng.Query(&QueryParams{
		AggregateFields: []AggregateField{
			{FunctionSQL: "COUNT(*)", Alias: "expr0"},
		},
		SObject:     "Account",
		IsAggregate: true,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	count, ok := results[0]["expr0"].(int64)
	if !ok {
		t.Fatalf("expected int64 count, got %T: %v", results[0]["expr0"], results[0]["expr0"])
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestAggregateSumGroupBy(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert accounts with duplicate names to test GROUP BY.
	records := []map[string]any{
		{"Name": "Acme"},
		{"Name": "Acme"},
		{"Name": "Globex"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := eng.Query(&QueryParams{
		Fields: []string{"Name"},
		AggregateFields: []AggregateField{
			{FunctionSQL: `COUNT("id")`, Alias: "cnt"},
		},
		SObject:     "Account",
		GroupBy:     `"name"`,
		OrderBy:     `"name" ASC`,
		IsAggregate: true,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(results))
	}
	// Acme should have count 2.
	if results[0]["name"] != "Acme" {
		t.Errorf("expected first group 'Acme', got %v", results[0]["name"])
	}
	if results[0]["cnt"].(int64) != 2 {
		t.Errorf("expected Acme count 2, got %v", results[0]["cnt"])
	}
}

func TestAggregateHaving(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	records := []map[string]any{
		{"Name": "Acme"},
		{"Name": "Acme"},
		{"Name": "Globex"},
	}
	if err := eng.Insert("Account", records); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := eng.Query(&QueryParams{
		Fields: []string{"Name"},
		AggregateFields: []AggregateField{
			{FunctionSQL: `COUNT("id")`, Alias: "cnt"},
		},
		SObject:    "Account",
		GroupBy:    `"name"`,
		Having:     `COUNT("id") > ?`,
		HavingArgs: []any{1},
		IsAggregate: true,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 group with count > 1, got %d", len(results))
	}
	if results[0]["name"] != "Acme" {
		t.Errorf("expected 'Acme', got %v", results[0]["name"])
	}
}

func TestSemiJoinSubQuery(t *testing.T) {
	s := testSchema(t)
	eng, err := NewEngine(s)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer eng.Close()

	// Insert accounts.
	accounts := []map[string]any{
		{"Name": "Acme"},
		{"Name": "Globex"},
	}
	if err := eng.Insert("Account", accounts); err != nil {
		t.Fatalf("Insert accounts: %v", err)
	}
	acmeId := accounts[0]["Id"]

	// Insert contacts linked to Acme.
	contacts := []map[string]any{
		{"LastName": "Smith", "AccountId": acmeId},
		{"LastName": "Jones", "AccountId": acmeId},
	}
	if err := eng.Insert("Contact", contacts); err != nil {
		t.Fatalf("Insert contacts: %v", err)
	}

	// Semi-join: SELECT Id, LastName FROM Contact WHERE AccountId IN (SELECT Id FROM Account WHERE Name = 'Acme')
	subSQL := `SELECT "id" FROM "account" WHERE "name" = ?`
	results, err := eng.Query(&QueryParams{
		Fields:    []string{"Id", "LastName"},
		SObject:   "Contact",
		Where:     fmt.Sprintf(`"accountid" IN (%s)`, subSQL),
		WhereArgs: []any{"Acme"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 contacts, got %d", len(results))
	}
}
