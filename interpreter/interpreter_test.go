package interpreter

import (
	"strings"
	"testing"

	"github.com/ipavlic/epex/apex"
)

// helper: parse apex source and create an interpreter with the class registered
func setupInterpreter(t *testing.T, source string) (*Interpreter, *apex.ParseResult) {
	t.Helper()
	result, err := apex.ParseString("test.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	reg := NewRegistry()
	reg.RegisterClass(result.Tree)
	interp := NewInterpreter(reg, nil)
	return interp, result
}

func TestArithmeticExpressions(t *testing.T) {
	source := `
public class MathTest {
    public static Integer testAdd() {
        return 2 + 3;
    }
    public static Integer testSub() {
        return 10 - 3;
    }
    public static Integer testMul() {
        return 4 * 5;
    }
    public static Integer testDiv() {
        return 20 / 4;
    }
    public static Integer testMod() {
        return 10 % 3;
    }
    public static Integer testComplex() {
        return (2 + 3) * 4;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	tests := []struct {
		method   string
		expected int
	}{
		{"testAdd", 5},
		{"testSub", 7},
		{"testMul", 20},
		{"testDiv", 5},
		{"testMod", 1},
		{"testComplex", 20},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := interp.ExecuteMethod("MathTest", tt.method, nil)
			if result.Type != TypeInteger {
				t.Fatalf("expected TypeInteger, got %v", result.Type)
			}
			if result.Data.(int) != tt.expected {
				t.Fatalf("expected %d, got %d", tt.expected, result.Data.(int))
			}
		})
	}
}

func TestStringConcatenation(t *testing.T) {
	source := `
public class StringTest {
    public static String testConcat() {
        return 'Hello' + ' ' + 'World';
    }
    public static String testConcatNum() {
        return 'Value: ' + 42;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("StringTest", "testConcat", nil)
	if result.Type != TypeString || result.Data.(string) != "Hello World" {
		t.Fatalf("expected 'Hello World', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StringTest", "testConcatNum", nil)
	if result.Type != TypeString || result.Data.(string) != "Value: 42" {
		t.Fatalf("expected 'Value: 42', got '%v'", result.Data)
	}
}

func TestComparisonExpressions(t *testing.T) {
	source := `
public class CmpTest {
    public static Boolean testLT() { return 1 < 2; }
    public static Boolean testGT() { return 2 > 1; }
    public static Boolean testLE() { return 2 <= 2; }
    public static Boolean testGE() { return 3 >= 2; }
    public static Boolean testEQ() { return 1 == 1; }
    public static Boolean testNE() { return 1 != 2; }
}
`
	interp, _ := setupInterpreter(t, source)

	tests := []struct {
		method   string
		expected bool
	}{
		{"testLT", true},
		{"testGT", true},
		{"testLE", true},
		{"testGE", true},
		{"testEQ", true},
		{"testNE", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := interp.ExecuteMethod("CmpTest", tt.method, nil)
			if result.Type != TypeBoolean {
				t.Fatalf("expected TypeBoolean, got %v", result.Type)
			}
			if result.Data.(bool) != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result.Data.(bool))
			}
		})
	}
}

func TestVariableDeclarationAndAssignment(t *testing.T) {
	source := `
public class VarTest {
    public static Integer testVar() {
        Integer x = 10;
        x = x + 5;
        return x;
    }
    public static Integer testMultiVar() {
        Integer a = 1;
        Integer b = 2;
        Integer c = a + b;
        return c;
    }
    public static Integer testCompoundAssign() {
        Integer x = 10;
        x += 5;
        return x;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("VarTest", "testVar", nil)
	if result.Data.(int) != 15 {
		t.Fatalf("testVar: expected 15, got %v", result.Data)
	}

	result = interp.ExecuteMethod("VarTest", "testMultiVar", nil)
	if result.Data.(int) != 3 {
		t.Fatalf("testMultiVar: expected 3, got %v", result.Data)
	}

	result = interp.ExecuteMethod("VarTest", "testCompoundAssign", nil)
	if result.Data.(int) != 15 {
		t.Fatalf("testCompoundAssign: expected 15, got %v", result.Data)
	}
}

func TestIfElse(t *testing.T) {
	source := `
public class IfTest {
    public static String testIfTrue() {
        if (true) {
            return 'yes';
        }
        return 'no';
    }
    public static String testIfFalse() {
        if (false) {
            return 'yes';
        }
        return 'no';
    }
    public static String testIfElse() {
        Integer x = 5;
        if (x > 3) {
            return 'greater';
        } else {
            return 'not greater';
        }
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("IfTest", "testIfTrue", nil)
	if result.Data.(string) != "yes" {
		t.Fatalf("testIfTrue: expected 'yes', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("IfTest", "testIfFalse", nil)
	if result.Data.(string) != "no" {
		t.Fatalf("testIfFalse: expected 'no', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("IfTest", "testIfElse", nil)
	if result.Data.(string) != "greater" {
		t.Fatalf("testIfElse: expected 'greater', got '%v'", result.Data)
	}
}

func TestTraditionalForLoop(t *testing.T) {
	source := `
public class ForTest {
    public static Integer testFor() {
        Integer sum = 0;
        for (Integer i = 0; i < 5; i++) {
            sum += i;
        }
        return sum;
    }
    public static Integer testForBreak() {
        Integer sum = 0;
        for (Integer i = 0; i < 10; i++) {
            if (i == 3) {
                break;
            }
            sum += i;
        }
        return sum;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("ForTest", "testFor", nil)
	if result.Data.(int) != 10 {
		t.Fatalf("testFor: expected 10, got %v", result.Data)
	}

	result = interp.ExecuteMethod("ForTest", "testForBreak", nil)
	if result.Data.(int) != 3 {
		t.Fatalf("testForBreak: expected 3, got %v", result.Data)
	}
}

func TestEnhancedForLoop(t *testing.T) {
	source := `
public class EnhForTest {
    public static Integer testEnhancedFor() {
        List<Integer> nums = new List<Integer>{1, 2, 3, 4, 5};
        Integer sum = 0;
        for (Integer n : nums) {
            sum += n;
        }
        return sum;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("EnhForTest", "testEnhancedFor", nil)
	if result.Type != TypeInteger || result.Data.(int) != 15 {
		t.Fatalf("testEnhancedFor: expected 15, got %v", result.Data)
	}
}

func TestWhileLoop(t *testing.T) {
	source := `
public class WhileTest {
    public static Integer testWhile() {
        Integer i = 0;
        Integer sum = 0;
        while (i < 5) {
            sum += i;
            i++;
        }
        return sum;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("WhileTest", "testWhile", nil)
	if result.Data.(int) != 10 {
		t.Fatalf("testWhile: expected 10, got %v", result.Data)
	}
}

func TestStringMethods(t *testing.T) {
	source := `
public class StrMethodTest {
    public static Integer testLength() {
        String s = 'hello';
        return s.length();
    }
    public static String testSubstring() {
        String s = 'hello world';
        return s.substring(0, 5);
    }
    public static Boolean testContains() {
        String s = 'hello world';
        return s.contains('world');
    }
    public static String testToLower() {
        String s = 'HELLO';
        return s.toLowerCase();
    }
    public static String testToUpper() {
        String s = 'hello';
        return s.toUpperCase();
    }
    public static String testTrim() {
        String s = '  hello  ';
        return s.trim();
    }
    public static String testReplace() {
        String s = 'hello world';
        return s.replace('world', 'apex');
    }
    public static Boolean testStartsWith() {
        String s = 'hello world';
        return s.startsWith('hello');
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("StrMethodTest", "testLength", nil)
	if result.Data.(int) != 5 {
		t.Fatalf("testLength: expected 5, got %v", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testSubstring", nil)
	if result.Data.(string) != "hello" {
		t.Fatalf("testSubstring: expected 'hello', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testContains", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testContains: expected true")
	}

	result = interp.ExecuteMethod("StrMethodTest", "testToLower", nil)
	if result.Data.(string) != "hello" {
		t.Fatalf("testToLower: expected 'hello', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testToUpper", nil)
	if result.Data.(string) != "HELLO" {
		t.Fatalf("testToUpper: expected 'HELLO', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testTrim", nil)
	if result.Data.(string) != "hello" {
		t.Fatalf("testTrim: expected 'hello', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testReplace", nil)
	if result.Data.(string) != "hello apex" {
		t.Fatalf("testReplace: expected 'hello apex', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("StrMethodTest", "testStartsWith", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testStartsWith: expected true")
	}
}

func TestClassInstantiationAndFieldAccess(t *testing.T) {
	source := `
public class PersonTest {
    public String name;
    public Integer age;

    public PersonTest(String n, Integer a) {
        this.name = n;
        this.age = a;
    }

    public String getName() {
        return this.name;
    }

    public static String testCreate() {
        PersonTest p = new PersonTest('Alice', 30);
        return p.getName();
    }

    public static Integer testFieldAccess() {
        PersonTest p = new PersonTest('Bob', 25);
        return p.age;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("PersonTest", "testCreate", nil)
	if result.Type != TypeString || result.Data.(string) != "Alice" {
		t.Fatalf("testCreate: expected 'Alice', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("PersonTest", "testFieldAccess", nil)
	if result.Type != TypeInteger || result.Data.(int) != 25 {
		t.Fatalf("testFieldAccess: expected 25, got '%v'", result.Data)
	}
}

func TestListOperations(t *testing.T) {
	source := `
public class ListTest {
    public static Integer testListAddSize() {
        List<String> items = new List<String>();
        items.add('one');
        items.add('two');
        items.add('three');
        return items.size();
    }
    public static String testListGet() {
        List<String> items = new List<String>();
        items.add('first');
        items.add('second');
        return items.get(1);
    }
    public static Boolean testListContains() {
        List<String> items = new List<String>();
        items.add('hello');
        return items.contains('hello');
    }
    public static Boolean testListIsEmpty() {
        List<String> items = new List<String>();
        return items.isEmpty();
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("ListTest", "testListAddSize", nil)
	if result.Data.(int) != 3 {
		t.Fatalf("testListAddSize: expected 3, got %v", result.Data)
	}

	result = interp.ExecuteMethod("ListTest", "testListGet", nil)
	if result.Data.(string) != "second" {
		t.Fatalf("testListGet: expected 'second', got '%v'", result.Data)
	}

	result = interp.ExecuteMethod("ListTest", "testListContains", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testListContains: expected true")
	}

	result = interp.ExecuteMethod("ListTest", "testListIsEmpty", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testListIsEmpty: expected true")
	}
}

func TestMapOperations(t *testing.T) {
	source := `
public class MapTest {
    public static Integer testMapPutSize() {
        Map<String, Integer> m = new Map<String, Integer>();
        m.put('a', 1);
        m.put('b', 2);
        return m.size();
    }
    public static Integer testMapGet() {
        Map<String, Integer> m = new Map<String, Integer>();
        m.put('key', 42);
        return m.get('key');
    }
    public static Boolean testMapContainsKey() {
        Map<String, Integer> m = new Map<String, Integer>();
        m.put('x', 1);
        return m.containsKey('x');
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("MapTest", "testMapPutSize", nil)
	if result.Data.(int) != 2 {
		t.Fatalf("testMapPutSize: expected 2, got %v", result.Data)
	}

	result = interp.ExecuteMethod("MapTest", "testMapGet", nil)
	if result.Data.(int) != 42 {
		t.Fatalf("testMapGet: expected 42, got %v", result.Data)
	}

	result = interp.ExecuteMethod("MapTest", "testMapContainsKey", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testMapContainsKey: expected true")
	}
}

func TestSetOperations(t *testing.T) {
	source := `
public class SetTest {
    public static Integer testSetAddSize() {
        Set<String> s = new Set<String>();
        s.add('one');
        s.add('two');
        return s.size();
    }
    public static Boolean testSetContains() {
        Set<String> s = new Set<String>();
        s.add('hello');
        return s.contains('hello');
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("SetTest", "testSetAddSize", nil)
	if result.Data.(int) != 2 {
		t.Fatalf("testSetAddSize: expected 2, got %v", result.Data)
	}

	result = interp.ExecuteMethod("SetTest", "testSetContains", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testSetContains: expected true")
	}
}

func TestTernaryExpression(t *testing.T) {
	source := `
public class TernaryTest {
    public static String testTernary() {
        Integer x = 5;
        return x > 3 ? 'big' : 'small';
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("TernaryTest", "testTernary", nil)
	if result.Data.(string) != "big" {
		t.Fatalf("testTernary: expected 'big', got '%v'", result.Data)
	}
}

func TestLogicalOperators(t *testing.T) {
	source := `
public class LogTest {
    public static Boolean testAnd() {
        return true && true;
    }
    public static Boolean testAndFalse() {
        return true && false;
    }
    public static Boolean testOr() {
        return false || true;
    }
    public static Boolean testNot() {
        return !false;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	tests := []struct {
		method   string
		expected bool
	}{
		{"testAnd", true},
		{"testAndFalse", false},
		{"testOr", true},
		{"testNot", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := interp.ExecuteMethod("LogTest", tt.method, nil)
			if result.Data.(bool) != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, result.Data)
			}
		})
	}
}

func TestDoWhileLoop(t *testing.T) {
	source := `
public class DoWhileTest {
    public static Integer testDoWhile() {
        Integer i = 0;
        Integer sum = 0;
        do {
            sum += i;
            i++;
        } while (i < 5);
        return sum;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("DoWhileTest", "testDoWhile", nil)
	if result.Data.(int) != 10 {
		t.Fatalf("testDoWhile: expected 10, got %v", result.Data)
	}
}

func TestNullHandling(t *testing.T) {
	source := `
public class NullTest {
    public static Boolean testNullEquality() {
        String s = null;
        return s == null;
    }
    public static Boolean testNullInequality() {
        String s = 'hello';
        return s != null;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("NullTest", "testNullEquality", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testNullEquality: expected true")
	}

	result = interp.ExecuteMethod("NullTest", "testNullInequality", nil)
	if result.Data.(bool) != true {
		t.Fatalf("testNullInequality: expected true")
	}
}

func TestTestRunner(t *testing.T) {
	source := `
@isTest
public class MyTestClass {
    @isTest
    static void testPassing() {
        System.assertEquals(1, 1);
    }
    @isTest
    static void testFailing() {
        System.assertEquals(1, 2);
    }
}
`
	interp, _ := setupInterpreter(t, source)

	results := interp.RunTests()
	if len(results) != 2 {
		t.Fatalf("expected 2 test results, got %d", len(results))
	}

	var passing, failing *TestResult
	for _, r := range results {
		if strings.Contains(r.MethodName, "assing") {
			passing = r
		} else {
			failing = r
		}
	}

	if passing != nil && !passing.Passed {
		t.Fatalf("testPassing should have passed: %s", passing.Error)
	}
	if failing != nil && failing.Passed {
		t.Fatalf("testFailing should have failed")
	}
}

func TestMethodWithParameters(t *testing.T) {
	source := `
public class ParamTest {
    public static Integer add(Integer a, Integer b) {
        return a + b;
    }
    public static Integer testAdd() {
        return add(3, 4);
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("ParamTest", "testAdd", nil)
	if result.Type != TypeInteger || result.Data.(int) != 7 {
		t.Fatalf("testAdd: expected 7, got %v", result.Data)
	}
}

func TestPreAndPostIncrement(t *testing.T) {
	source := `
public class IncTest {
    public static Integer testPreInc() {
        Integer x = 5;
        Integer y = ++x;
        return y;
    }
    public static Integer testPostInc() {
        Integer x = 5;
        Integer y = x++;
        return y;
    }
    public static Integer testPostIncResult() {
        Integer x = 5;
        x++;
        return x;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("IncTest", "testPreInc", nil)
	if result.Data.(int) != 6 {
		t.Fatalf("testPreInc: expected 6, got %v", result.Data)
	}

	result = interp.ExecuteMethod("IncTest", "testPostInc", nil)
	if result.Data.(int) != 5 {
		t.Fatalf("testPostInc: expected 5, got %v", result.Data)
	}

	result = interp.ExecuteMethod("IncTest", "testPostIncResult", nil)
	if result.Data.(int) != 6 {
		t.Fatalf("testPostIncResult: expected 6, got %v", result.Data)
	}
}

func TestUnaryMinus(t *testing.T) {
	source := `
public class UnaryTest {
    public static Integer testNeg() {
        return -5;
    }
    public static Integer testNegExpr() {
        Integer x = 10;
        return -x;
    }
}
`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("UnaryTest", "testNeg", nil)
	if result.Data.(int) != -5 {
		t.Fatalf("testNeg: expected -5, got %v", result.Data)
	}

	result = interp.ExecuteMethod("UnaryTest", "testNegExpr", nil)
	if result.Data.(int) != -10 {
		t.Fatalf("testNegExpr: expected -10, got %v", result.Data)
	}
}

func TestAssertAreEqual(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.areEqual(34, 17 * 2);
    }
    public static void testFail() {
        Assert.areEqual(1, 2);
    }
    public static void testWithMessage() {
        Assert.areEqual(1, 2, 'custom message');
    }
}`
	interp, _ := setupInterpreter(t, source)

	// Should not panic
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	// Should panic with AssertException
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected Assert.areEqual to fail")
			}
			ae, ok := r.(*AssertException)
			if !ok {
				t.Errorf("expected AssertException, got %T", r)
			}
			if !strings.Contains(ae.Message, "Expected") {
				t.Errorf("expected default message, got: %s", ae.Message)
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()

	// Should panic with custom message
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected Assert.areEqual to fail")
			}
			ae, ok := r.(*AssertException)
			if !ok {
				t.Errorf("expected AssertException, got %T", r)
			}
			if ae.Message != "custom message" {
				t.Errorf("expected 'custom message', got: %s", ae.Message)
			}
		}()
		interp.ExecuteMethod("AssertTest", "testWithMessage", nil)
	}()
}

func TestAssertAreNotEqual(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.areNotEqual(1, 2);
    }
    public static void testFail() {
        Assert.areNotEqual(5, 5);
    }
}`
	interp, _ := setupInterpreter(t, source)
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected Assert.areNotEqual to fail")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()
}

func TestAssertIsTrue(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.isTrue(1 == 1);
    }
    public static void testFail() {
        Assert.isTrue(false);
    }
}`
	interp, _ := setupInterpreter(t, source)
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected Assert.isTrue to fail")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()
}

func TestAssertIsFalse(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.isFalse(1 == 2);
    }
    public static void testFail() {
        Assert.isFalse(true);
    }
}`
	interp, _ := setupInterpreter(t, source)
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected Assert.isFalse to fail")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()
}

func TestAssertIsNull(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.isNull(null);
    }
    public static void testFail() {
        Assert.isNull('not null');
    }
}`
	interp, _ := setupInterpreter(t, source)
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected Assert.isNull to fail")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()
}

func TestAssertIsNotNull(t *testing.T) {
	source := `
public class AssertTest {
    public static void testPass() {
        Assert.isNotNull('value');
    }
    public static void testFail() {
        Assert.isNotNull(null);
    }
}`
	interp, _ := setupInterpreter(t, source)
	interp.ExecuteMethod("AssertTest", "testPass", nil)

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected Assert.isNotNull to fail")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()
}

func TestAssertFail(t *testing.T) {
	source := `
public class AssertTest {
    public static void testFail() {
        Assert.fail();
    }
    public static void testFailMsg() {
        Assert.fail('boom');
    }
}`
	interp, _ := setupInterpreter(t, source)

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected Assert.fail to panic")
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFail", nil)
	}()

	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Error("expected Assert.fail to panic")
			}
			ae := r.(*AssertException)
			if ae.Message != "boom" {
				t.Errorf("expected 'boom', got: %s", ae.Message)
			}
		}()
		interp.ExecuteMethod("AssertTest", "testFailMsg", nil)
	}()
}

func TestNullPointerExceptionOnArithmetic(t *testing.T) {
	source := `
public class NpeTest {
    public static Object testAddNull() {
        Integer a = null;
        return a + 1;
    }
    public static Object testSubNull() {
        Integer a = null;
        return a - 1;
    }
    public static Object testMulNull() {
        Integer a = null;
        return a * 2;
    }
    public static Object testDivNull() {
        Integer a = null;
        return a / 2;
    }
    public static Object testStringConcatNull() {
        String s = null;
        return s + ' world';
    }
}`
	interp, _ := setupInterpreter(t, source)

	// String concatenation with null should work (not throw)
	result := interp.ExecuteMethod("NpeTest", "testStringConcatNull", nil)
	if result.Type != TypeString || result.Data.(string) != "null world" {
		t.Errorf("expected 'null world', got %v", result)
	}

	// Numeric operations with null should throw NullPointerException
	for _, method := range []string{"testAddNull", "testSubNull", "testMulNull", "testDivNull"} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Errorf("%s: expected NullPointerException", method)
					return
				}
				npe, ok := r.(*NullPointerError)
				if !ok {
					t.Errorf("%s: expected NullPointerError, got %T: %v", method, r, r)
					return
				}
				if !strings.Contains(npe.Error(), "NullPointerException") {
					t.Errorf("%s: unexpected message: %s", method, npe.Error())
				}
			}()
			interp.ExecuteMethod("NpeTest", method, nil)
		}()
	}
}

func TestIntegerOverflowWraparound(t *testing.T) {
	source := `
public class OverflowTest {
    public static Object testOverflow() {
        Integer a = 2147483647;
        return a + 1;
    }
    public static Object testUnderflow() {
        Integer a = -2147483648;
        return a - 1;
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("OverflowTest", "testOverflow", nil)
	if result.Type != TypeInteger || result.Data.(int) != -2147483648 {
		t.Errorf("expected -2147483648 (wraparound), got %v", result.Data)
	}

	result = interp.ExecuteMethod("OverflowTest", "testUnderflow", nil)
	if result.Type != TypeInteger || result.Data.(int) != 2147483647 {
		t.Errorf("expected 2147483647 (wraparound), got %v", result.Data)
	}
}

func TestEscapeSequences(t *testing.T) {
	source := `
public class EscapeTest {
    public static Object testFormFeed() {
        return 'a\fb';
    }
    public static Object testBackspace() {
        return 'a\bb';
    }
    public static Object testCarriageReturn() {
        return 'a\rb';
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("EscapeTest", "testFormFeed", nil)
	if result.Data.(string) != "a\fb" {
		t.Errorf("expected form feed, got %q", result.Data)
	}
	result = interp.ExecuteMethod("EscapeTest", "testBackspace", nil)
	if result.Data.(string) != "a\bb" {
		t.Errorf("expected backspace, got %q", result.Data)
	}
	result = interp.ExecuteMethod("EscapeTest", "testCarriageReturn", nil)
	if result.Data.(string) != "a\rb" {
		t.Errorf("expected carriage return, got %q", result.Data)
	}
}

func TestSwitchOnInteger(t *testing.T) {
	source := `
public class SwitchTest {
    public static String testSwitch(Integer val) {
        String result;
        switch on val {
            when 1 {
                result = 'one';
            }
            when 2, 3 {
                result = 'two or three';
            }
            when null {
                result = 'null';
            }
            when else {
                result = 'other';
            }
        }
        return result;
    }
}`
	interp, _ := setupInterpreter(t, source)

	tests := []struct {
		input    *Value
		expected string
	}{
		{IntegerValue(1), "one"},
		{IntegerValue(2), "two or three"},
		{IntegerValue(3), "two or three"},
		{IntegerValue(99), "other"},
		{NullValue(), "null"},
	}
	for _, tt := range tests {
		result := interp.ExecuteMethod("SwitchTest", "testSwitch", []*Value{tt.input})
		if result.Data.(string) != tt.expected {
			t.Errorf("switch on %v: expected %q, got %v", tt.input, tt.expected, result.Data)
		}
	}
}

func TestSwitchOnString(t *testing.T) {
	source := `
public class SwitchTest {
    public static Integer testSwitch(String val) {
        switch on val {
            when 'hello' {
                return 1;
            }
            when 'world' {
                return 2;
            }
            when else {
                return 0;
            }
        }
    }
}`
	interp, _ := setupInterpreter(t, source)

	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 1},
		{"world", 2},
		{"other", 0},
	}
	for _, tt := range tests {
		result := interp.ExecuteMethod("SwitchTest", "testSwitch", []*Value{StringValue(tt.input)})
		if result.Data.(int) != tt.expected {
			t.Errorf("switch on %q: expected %d, got %v", tt.input, tt.expected, result.Data)
		}
	}
}

func TestSwitchOnSObjectType(t *testing.T) {
	source := `
public class SwitchTest {
    public static String testSwitch(SObject obj) {
        switch on obj {
            when Account acc {
                return 'account: ' + acc.Name;
            }
            when Contact con {
                return 'contact: ' + con.LastName;
            }
            when else {
                return 'unknown';
            }
        }
    }
}`
	interp, _ := setupInterpreter(t, source)

	accFields := map[string]*Value{"Name": StringValue("Acme")}
	acc := &Value{Type: TypeSObject, Data: accFields, SType: "Account"}
	result := interp.ExecuteMethod("SwitchTest", "testSwitch", []*Value{acc})
	if result.Data.(string) != "account: Acme" {
		t.Errorf("expected 'account: Acme', got %v", result.Data)
	}

	conFields := map[string]*Value{"LastName": StringValue("Doe")}
	con := &Value{Type: TypeSObject, Data: conFields, SType: "Contact"}
	result = interp.ExecuteMethod("SwitchTest", "testSwitch", []*Value{con})
	if result.Data.(string) != "contact: Doe" {
		t.Errorf("expected 'contact: Doe', got %v", result.Data)
	}
}

func TestSwitchReturnPropagation(t *testing.T) {
	source := `
public class SwitchTest {
    public static Integer testReturn() {
        Integer x = 2;
        switch on x {
            when 1 {
                return 10;
            }
            when 2 {
                return 20;
            }
        }
        return 0;
    }
}`
	interp, _ := setupInterpreter(t, source)
	result := interp.ExecuteMethod("SwitchTest", "testReturn", nil)
	if result.Data.(int) != 20 {
		t.Errorf("expected 20, got %v", result.Data)
	}
}

func TestDivisionByZeroThrows(t *testing.T) {
	source := `
public class DivTest {
    public static String testDivByZero() {
        try {
            Integer x = 10 / 0;
            return 'no error';
        } catch (Exception e) {
            return 'caught: ' + e.getMessage();
        }
    }
    public static String testDoubleDivByZero() {
        try {
            Double x = 10.0 / 0;
            return 'no error';
        } catch (Exception e) {
            return 'caught';
        }
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("DivTest", "testDivByZero", nil)
	got := result.Data.(string)
	if got != "caught: Divide by 0" {
		t.Errorf("expected 'caught: Divide by 0', got %q", got)
	}

	result = interp.ExecuteMethod("DivTest", "testDoubleDivByZero", nil)
	got = result.Data.(string)
	if got != "caught" {
		t.Errorf("expected 'caught', got %q", got)
	}
}

func TestLongDivisionReturnsLong(t *testing.T) {
	// Test Long division at the Value level.
	a := LongValue(10)
	b := LongValue(3)
	result := a.Divide(b)
	if result.Type != TypeLong {
		t.Errorf("expected TypeLong, got %v", result.Type)
	}
	if result.Data.(int64) != 3 {
		t.Errorf("expected 3, got %v", result.Data)
	}

	// Test Long/Integer mixed division returns Long.
	c := IntegerValue(7)
	result = a.Divide(c)
	if result.Type != TypeLong {
		t.Errorf("Long/Integer: expected TypeLong, got %v", result.Type)
	}
}

func TestModuloByZeroThrows(t *testing.T) {
	source := `
public class ModTest {
    public static String testModByZero() {
        try {
            Integer x = 10;
            Integer y = x % 0;
            return 'no error';
        } catch (Exception e) {
            return 'caught: ' + e.getMessage();
        }
    }
}`
	interp, _ := setupInterpreter(t, source)
	result := interp.ExecuteMethod("ModTest", "testModByZero", nil)
	got := result.Data.(string)
	if got != "caught: Divide by 0" {
		t.Errorf("expected 'caught: Divide by 0', got %q", got)
	}
}

func TestTypeForNameAndNewInstance(t *testing.T) {
	source := `
public class TypeTest {
    public static Object testForName() {
        Type t = Type.forName('Account');
        Object obj = t.newInstance();
        return obj;
    }
    public static String testGetName() {
        Type t = Type.forName('Account');
        return t.getName();
    }
    public static Object testForNameNamespace() {
        Type t = Type.forName('myns', 'MyClass');
        return t.getName();
    }
    public static Object testNewInstanceUserClass() {
        Type t = Type.forName('TypeTest');
        return t.newInstance();
    }
}`
	interp, _ := setupInterpreter(t, source)

	// forName + newInstance creates an empty SObject
	result := interp.ExecuteMethod("TypeTest", "testForName", nil)
	if result.Type != TypeSObject || result.SType != "Account" {
		t.Errorf("expected SObject Account, got type=%v stype=%v", result.Type, result.SType)
	}

	// getName returns the type name
	result = interp.ExecuteMethod("TypeTest", "testGetName", nil)
	if result.Data.(string) != "Account" {
		t.Errorf("expected 'Account', got %v", result.Data)
	}

	// forName with namespace ignores namespace
	result = interp.ExecuteMethod("TypeTest", "testForNameNamespace", nil)
	if result.Data.(string) != "MyClass" {
		t.Errorf("expected 'MyClass', got %v", result.Data)
	}
}

func TestIdValueOfAndGetSobjectType(t *testing.T) {
	source := `
public class IdTest {
    public static Object testValueOf() {
        Id myId = Id.valueOf('001000000000001');
        return myId;
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("IdTest", "testValueOf", nil)
	if result.Data.(string) != "001000000000001" {
		t.Errorf("expected '001000000000001', got %v", result.Data)
	}
}

func TestAssertIsNotInstanceOfType(t *testing.T) {
	// Test the method directly since .class syntax requires special parser support
	reg := NewRegistry()
	interp := NewInterpreter(reg, nil)

	// Pass: String is not Integer
	result := interp.callAssertMethod("isNotInstanceOfType", []*Value{
		StringValue("hello"),
		StringValue("Integer"),
	})
	if result.Type != TypeNull {
		t.Errorf("expected null return, got %v", result.Type)
	}

	// Fail: String is String — should panic
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("expected assertion failure for isNotInstanceOfType with matching type")
			}
		}()
		interp.callAssertMethod("isNotInstanceOfType", []*Value{
			StringValue("hello"),
			StringValue("String"),
		})
	}()

	// Pass with message: Integer is not String
	result = interp.callAssertMethod("isNotInstanceOfType", []*Value{
		IntegerValue(42),
		StringValue("String"),
		StringValue("should not be String"),
	})
	if result.Type != TypeNull {
		t.Errorf("expected null return, got %v", result.Type)
	}
}

func TestSystemAbortJob(t *testing.T) {
	source := `
public class AbortJobTest {
    public static Object testAbortJob() {
        System.abortJob('7071000000000001');
        return true;
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("AbortJobTest", "testAbortJob", nil)
	if result.Type != TypeBoolean || result.Data.(bool) != true {
		t.Errorf("expected true, got %v", result.Data)
	}
}

func TestSystemEnqueueJob(t *testing.T) {
	source := `
public class EnqueueJobTest {
    public static String testEnqueueJob() {
        Id jobId = System.enqueueJob(null);
        return jobId;
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("EnqueueJobTest", "testEnqueueJob", nil)
	if result.Type != TypeString || result.Data.(string) != "7071000000000001" {
		t.Errorf("expected '7071000000000001', got %v", result.Data)
	}
}

func TestTestStartStopTest(t *testing.T) {
	source := `
public class StartStopTest {
    public static Boolean testStartStop() {
        Test.startTest();
        Test.stopTest();
        return true;
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("StartStopTest", "testStartStop", nil)
	if result.Type != TypeBoolean || result.Data.(bool) != true {
		t.Errorf("expected true, got %v", result.Data)
	}
}

func TestTestIsRunningTest(t *testing.T) {
	source := `
public class IsRunningTest {
    public static Boolean testIsRunning() {
        return Test.isRunningTest();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("IsRunningTest", "testIsRunning", nil)
	if result.Type != TypeBoolean || result.Data.(bool) != true {
		t.Errorf("expected true, got %v", result.Data)
	}
}

func TestTestGetStandardPricebookId(t *testing.T) {
	source := `
public class PricebookTest {
    public static String testGetPricebookId() {
        return Test.getStandardPricebookId();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("PricebookTest", "testGetPricebookId", nil)
	if result.Type != TypeString || result.Data.(string) != "01s000000000001" {
		t.Errorf("expected '01s000000000001', got %v", result.Data)
	}
}

func TestTestLoadData(t *testing.T) {
	source := `
public class LoadDataTest {
    public static Integer testLoadData() {
        List<SObject> records = Test.loadData(Account.SObjectType, 'testdata');
        return records.size();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("LoadDataTest", "testLoadData", nil)
	if result.Type != TypeInteger || result.Data.(int) != 0 {
		t.Errorf("expected 0, got %v", result.Data)
	}
}

func TestUserInfoGetUserId(t *testing.T) {
	source := `
public class UserInfoTest {
    public static String testGetUserId() {
        return UserInfo.getUserId();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("UserInfoTest", "testGetUserId", nil)
	if result.Type != TypeString || result.Data.(string) != "005000000000001" {
		t.Errorf("expected '005000000000001', got %v", result.Data)
	}
}

func TestUserInfoGetUserName(t *testing.T) {
	source := `
public class UserNameTest {
    public static String testGetUserName() {
        return UserInfo.getUserName();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("UserNameTest", "testGetUserName", nil)
	if result.Type != TypeString || result.Data.(string) != "test@example.com" {
		t.Errorf("expected 'test@example.com', got %v", result.Data)
	}
}

func TestUserInfoGetProfileId(t *testing.T) {
	source := `
public class ProfileIdTest {
    public static String testGetProfileId() {
        return UserInfo.getProfileId();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("ProfileIdTest", "testGetProfileId", nil)
	if result.Type != TypeString || result.Data.(string) != "00e000000000001" {
		t.Errorf("expected '00e000000000001', got %v", result.Data)
	}
}

func TestUserInfoGetOrganizationId(t *testing.T) {
	source := `
public class OrgIdTest {
    public static String testGetOrgId() {
        return UserInfo.getOrganizationId();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("OrgIdTest", "testGetOrgId", nil)
	if result.Type != TypeString || result.Data.(string) != "00D000000000001" {
		t.Errorf("expected '00D000000000001', got %v", result.Data)
	}
}

func TestUserInfoIsMultiCurrencyOrganization(t *testing.T) {
	source := `
public class MultiCurrTest {
    public static Boolean testIsMultiCurrency() {
        return UserInfo.isMultiCurrencyOrganization();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("MultiCurrTest", "testIsMultiCurrency", nil)
	if result.Type != TypeBoolean || result.Data.(bool) != false {
		t.Errorf("expected false, got %v", result.Data)
	}
}

func TestUserInfoGetDefaultCurrency(t *testing.T) {
	source := `
public class DefaultCurrTest {
    public static String testGetDefaultCurrency() {
        return UserInfo.getDefaultCurrency();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("DefaultCurrTest", "testGetDefaultCurrency", nil)
	if result.Type != TypeString || result.Data.(string) != "USD" {
		t.Errorf("expected 'USD', got %v", result.Data)
	}
}

func TestUserInfoGetLocale(t *testing.T) {
	source := `
public class LocaleTest {
    public static String testGetLocale() {
        return UserInfo.getLocale();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("LocaleTest", "testGetLocale", nil)
	if result.Type != TypeString || result.Data.(string) != "en_US" {
		t.Errorf("expected 'en_US', got %v", result.Data)
	}
}

func TestUserInfoGetTimeZone(t *testing.T) {
	source := `
public class TimeZoneTest {
    public static String testGetTimeZone() {
        TimeZone tz = UserInfo.getTimeZone();
        return tz.getId();
    }
}`
	interp, _ := setupInterpreter(t, source)

	result := interp.ExecuteMethod("TimeZoneTest", "testGetTimeZone", nil)
	if result.Type != TypeString || result.Data.(string) != "America/Los_Angeles" {
		t.Errorf("expected 'America/Los_Angeles', got %v", result.Data)
	}
}
