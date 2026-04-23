package schema

import (
	"testing"
)

func TestBuildSchemaFromDir(t *testing.T) {
	schema, err := BuildSchemaFromDir("../testdata/objects")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(schema.SObjects) != 2 {
		t.Fatalf("expected 2 SObjects, got %d", len(schema.SObjects))
	}

	// Check Account
	account, ok := schema.SObjects["Account"]
	if !ok {
		t.Fatal("expected Account SObject")
	}
	if account.Label != "Account" {
		t.Errorf("expected label 'Account', got '%s'", account.Label)
	}
	if account.NameFieldType != "Text" {
		t.Errorf("expected name field type 'Text', got '%s'", account.NameFieldType)
	}

	// Check Account has standard fields + custom fields
	if _, ok := account.Fields["Id"]; !ok {
		t.Error("expected standard field 'Id'")
	}
	if _, ok := account.Fields["Name"]; !ok {
		t.Error("expected standard field 'Name'")
	}
	if f, ok := account.Fields["Industry"]; !ok {
		t.Error("expected field 'Industry'")
	} else if f.Type != FieldTypePicklist {
		t.Errorf("expected Industry type Picklist, got %s", f.Type)
	}
	if f, ok := account.Fields["AnnualRevenue"]; !ok {
		t.Error("expected field 'AnnualRevenue'")
	} else {
		if f.Type != FieldTypeCurrency {
			t.Errorf("expected AnnualRevenue type Currency, got %s", f.Type)
		}
	}

	// Check Contact
	contact, ok := schema.SObjects["Contact"]
	if !ok {
		t.Fatal("expected Contact SObject")
	}
	if contact.Label != "Contact" {
		t.Errorf("expected label 'Contact', got '%s'", contact.Label)
	}

	// Check Lookup field
	if f, ok := contact.Fields["AccountId__c"]; !ok {
		t.Error("expected custom field 'AccountId__c'")
	} else {
		if f.Type != FieldTypeLookup {
			t.Errorf("expected type Lookup, got %s", f.Type)
		}
		if f.ReferenceTo != "Account" {
			t.Errorf("expected referenceTo 'Account', got '%s'", f.ReferenceTo)
		}
		if f.ChildRelationshipName != "CustomContacts" {
			t.Errorf("expected childRelationshipName 'CustomContacts', got '%s'", f.ChildRelationshipName)
		}
	}

	// Check Email field
	if f, ok := contact.Fields["Email__c"]; !ok {
		t.Error("expected custom field 'Email__c'")
	} else {
		if f.Type != FieldTypeEmail {
			t.Errorf("expected type Email, got %s", f.Type)
		}
		if !f.Unique {
			t.Error("expected Email__c to be unique")
		}
	}
}
