package apex

import (
	"testing"
)

func TestParseString_SimpleClass(t *testing.T) {
	source := `public class HelloWorld {
		public static String greet(String name) {
			return 'Hello, ' + name + '!';
		}
	}`

	result, err := ParseString("HelloWorld.cls", source)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result.Tree == nil {
		t.Fatal("expected non-nil parse tree")
	}
	if len(result.Errors) > 0 {
		t.Fatalf("expected no errors, got: %v", result.Errors)
	}
}

func TestParseString_TestClass(t *testing.T) {
	source := `@isTest
private class MyTest {
	@isTest
	static void testSomething() {
		System.assertEquals(1, 1);
	}
}`

	result, err := ParseString("MyTest.cls", source)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result.Tree == nil {
		t.Fatal("expected non-nil parse tree")
	}
}

func TestParseString_SOQLQuery(t *testing.T) {
	source := `public class QueryExample {
	public static List<Account> getAccounts() {
		return [SELECT Id, Name FROM Account WHERE Name != null];
	}
}`

	result, err := ParseString("QueryExample.cls", source)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result.Tree == nil {
		t.Fatal("expected non-nil parse tree")
	}
}

func TestParseString_InvalidSyntax(t *testing.T) {
	source := `public class Broken {
		this is not valid apex
	}`

	result, err := ParseString("Broken.cls", source)
	if err == nil {
		t.Fatal("expected parse error for invalid syntax")
	}
	if result == nil {
		t.Fatal("expected non-nil result even with errors")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors in result")
	}
}

func TestParseFile(t *testing.T) {
	result, err := ParseFile("../testdata/classes/AccountService.cls")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if result.Tree == nil {
		t.Fatal("expected non-nil parse tree")
	}
}

func TestParseDirectory(t *testing.T) {
	results, err := ParseDirectory("../testdata/classes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
}
