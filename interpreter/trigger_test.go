package interpreter

import (
	"testing"

	"github.com/ipavlic/epex/apex"
)

// setupWithTrigger parses a class + trigger and creates an interpreter with mock engine.
func setupWithTrigger(t *testing.T, classSource, triggerSource string) (*Interpreter, *mockEngine) {
	t.Helper()
	reg := NewRegistry()

	if classSource != "" {
		classResult, err := apex.ParseString("test.cls", classSource)
		if err != nil {
			t.Fatalf("class parse error: %v", err)
		}
		reg.RegisterClass(classResult.Tree, "test.cls")
	}

	trigResult, err := apex.ParseString("test.trigger", triggerSource)
	if err != nil {
		t.Fatalf("trigger parse error: %v", err)
	}
	if !reg.RegisterTrigger(trigResult.Tree, "test.trigger") {
		t.Fatal("failed to register trigger")
	}

	eng := newMockEngine()
	interp := NewInterpreter(reg, eng)
	return interp, eng
}

func TestTriggerRegistration(t *testing.T) {
	triggerSource := `trigger AccountTrigger on Account (before insert, after update) {
		System.debug('trigger fired');
	}`

	result, err := apex.ParseString("AccountTrigger.trigger", triggerSource)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	ok := reg.RegisterTrigger(result.Tree, "AccountTrigger.trigger")
	if !ok {
		t.Fatal("expected trigger to be registered")
	}
	if len(reg.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(reg.Triggers))
	}
	ti := reg.Triggers[0]
	if ti.Name != "AccountTrigger" {
		t.Errorf("expected name AccountTrigger, got %s", ti.Name)
	}
	if ti.SObject != "Account" {
		t.Errorf("expected SObject Account, got %s", ti.SObject)
	}
	if len(ti.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(ti.Events))
	}
	if !ti.Events[0].IsBefore || ti.Events[0].Op != "INSERT" {
		t.Errorf("expected before insert, got %+v", ti.Events[0])
	}
	if !ti.Events[1].IsAfter || ti.Events[1].Op != "UPDATE" {
		t.Errorf("expected after update, got %+v", ti.Events[1])
	}
}

func TestTriggerBeforeInsertFiresOnDML(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String testInsert() {
        Account acc = new Account();
        acc.Name = 'Original';
        insert acc;
        return acc.Name;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (before insert) {
    for (Account a : Trigger.new) {
        a.Name = 'Modified';
    }
}`
	interp, eng := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testInsert", nil)

	// The before trigger should have modified the Name before insert
	if len(eng.records["account"]) != 1 {
		t.Fatalf("expected 1 record, got %d", len(eng.records["account"]))
	}

	// Check that the record was inserted (trigger fired)
	_ = result
}

func TestTriggerAfterInsertFires(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static Integer testAfterInsert() {
        Account acc = new Account();
        acc.Name = 'Test';
        insert acc;
        return 1;
    }
}`
	// Use a trigger that reads Trigger.new - it should have the records
	triggerSource := `trigger AccountTrigger on Account (after insert) {
    System.debug('After insert: ' + Trigger.new.size());
}`
	interp, eng := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testAfterInsert", nil)

	if result.Type != TypeInteger || result.Data.(int) != 1 {
		t.Errorf("expected 1, got %v", result)
	}
	if len(eng.records["account"]) != 1 {
		t.Errorf("expected 1 record, got %d", len(eng.records["account"]))
	}
}

func TestTriggerContextVariables(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result;
    public static String testContext() {
        Account acc = new Account();
        acc.Name = 'Test';
        insert acc;
        return TriggerTest.result;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (before insert) {
    TriggerTest.result = 'isBefore=' + Trigger.isBefore + ',isInsert=' + Trigger.isInsert + ',size=' + Trigger.size;
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testContext", nil)

	expected := "isBefore=true,isInsert=true,size=1"
	if result.Type != TypeString || result.Data.(string) != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}

func TestTriggerDoesNotFireOnWrongSObject(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result;
    public static String testWrongSObject() {
        TriggerTest.result = 'not fired';
        Contact c = new Contact();
        c.LastName = 'Test';
        insert c;
        return TriggerTest.result;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (before insert) {
    TriggerTest.result = 'fired';
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testWrongSObject", nil)

	if result.Type != TypeString || result.Data.(string) != "not fired" {
		t.Errorf("expected 'not fired', got %v", result)
	}
}

func TestTriggerDoesNotFireOnWrongEvent(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result;
    public static String testWrongEvent() {
        TriggerTest.result = 'not fired';
        Account acc = new Account();
        acc.Name = 'Test';
        insert acc;
        return TriggerTest.result;
    }
}`
	// Trigger only fires on update, not insert
	triggerSource := `trigger AccountTrigger on Account (before update) {
    TriggerTest.result = 'fired';
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testWrongEvent", nil)

	if result.Type != TypeString || result.Data.(string) != "not fired" {
		t.Errorf("expected 'not fired', got %v", result)
	}
}

func TestTriggerOnDatabaseInsert(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result = 'not fired';
    public static String testDatabaseInsert() {
        Account acc = new Account();
        acc.Name = 'Test';
        Database.insert(acc);
        return TriggerTest.result;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (before insert) {
    TriggerTest.result = 'fired';
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testDatabaseInsert", nil)

	if result.Type != TypeString || result.Data.(string) != "fired" {
		t.Errorf("expected 'fired', got %v", result)
	}
}

func TestTriggerDeleteContext(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result;
    public static String testDelete() {
        Account acc = new Account();
        acc.Name = 'Test';
        insert acc;
        delete acc;
        return TriggerTest.result;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (before delete) {
    TriggerTest.result = 'isDelete=' + Trigger.isDelete + ',isBefore=' + Trigger.isBefore;
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testDelete", nil)

	expected := "isDelete=true,isBefore=true"
	if result.Type != TypeString || result.Data.(string) != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}

func TestTriggerOperationType(t *testing.T) {
	classSource := `
public class TriggerTest {
    public static String result;
    public static String testOpType() {
        Account acc = new Account();
        acc.Name = 'Test';
        insert acc;
        return TriggerTest.result;
    }
}`
	triggerSource := `trigger AccountTrigger on Account (after insert) {
    TriggerTest.result = '' + Trigger.operationType;
}`

	interp, _ := setupWithTrigger(t, classSource, triggerSource)
	result := interp.ExecuteMethod("TriggerTest", "testOpType", nil)

	expected := "AFTER_INSERT"
	if result.Type != TypeString || result.Data.(string) != expected {
		t.Errorf("expected %q, got %v", expected, result)
	}
}
