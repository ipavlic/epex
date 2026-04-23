package interpreter

import (
	"testing"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/tracer"
)

func TestTraceMethodEntryExit(t *testing.T) {
	source := `public class Foo {
		public static Integer add(Integer a, Integer b) {
			return a + b;
		}
	}`

	result, err := apex.ParseString("Foo.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree, "Foo.cls")

	tr := tracer.NewRecordingTracer()
	interp := NewInterpreter(reg, nil)
	interp.SetTracer(tr)

	interp.ExecuteMethod("Foo", "add", []*Value{IntegerValue(2), IntegerValue(3)})

	events := tr.Events()

	var entries, exits int
	for _, e := range events {
		switch e.Type {
		case tracer.EventMethodEntry:
			entries++
			if e.Class != "Foo" || e.Method != "add" {
				t.Errorf("unexpected entry: %s.%s", e.Class, e.Method)
			}
			if e.File != "Foo.cls" {
				t.Errorf("expected file Foo.cls, got %s", e.File)
			}
		case tracer.EventMethodExit:
			exits++
			if e.Duration <= 0 {
				t.Error("expected positive duration on method exit")
			}
		}
	}

	if entries != 1 {
		t.Errorf("expected 1 method entry, got %d", entries)
	}
	if exits != 1 {
		t.Errorf("expected 1 method exit, got %d", exits)
	}
}

func TestTraceLineEvents(t *testing.T) {
	source := `public class Bar {
		public static void doStuff() {
			Integer x = 1;
			Integer y = 2;
			Integer z = x + y;
		}
	}`

	result, err := apex.ParseString("Bar.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree)

	tr := tracer.NewRecordingTracer()
	interp := NewInterpreter(reg, nil)
	interp.SetTracer(tr)
	interp.SetCurrentFile("Bar.cls")

	interp.ExecuteMethod("Bar", "doStuff", nil)

	var lineEvents int
	for _, e := range tr.Events() {
		if e.Type == tracer.EventLine {
			lineEvents++
		}
	}

	if lineEvents < 3 {
		t.Errorf("expected at least 3 line events, got %d", lineEvents)
	}
}

func TestTraceAssertEvents(t *testing.T) {
	source := `@isTest
private class AssertTest {
	@isTest
	static void testPass() {
		System.assertEquals(1, 1);
		System.assert(true);
	}
}`

	result, err := apex.ParseString("AssertTest.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree)

	tr := tracer.NewRecordingTracer()
	interp := NewInterpreter(reg, nil)
	interp.SetTracer(tr)
	interp.SetCurrentFile("AssertTest.cls")

	interp.ExecuteMethod("AssertTest", "testPass", nil)

	var assertEvents int
	for _, e := range tr.Events() {
		if e.Type == tracer.EventAssert {
			assertEvents++
			if !e.Passed {
				t.Errorf("expected passing assert, got failed: %s", e.Detail)
			}
		}
	}

	if assertEvents != 2 {
		t.Errorf("expected 2 assert events, got %d", assertEvents)
	}
}

func TestTraceAssertFailure(t *testing.T) {
	source := `@isTest
private class FailTest {
	@isTest
	static void testFail() {
		System.assertEquals(1, 2);
	}
}`

	result, err := apex.ParseString("FailTest.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree)

	tr := tracer.NewRecordingTracer()
	interp := NewInterpreter(reg, nil)
	interp.SetTracer(tr)
	interp.SetCurrentFile("FailTest.cls")

	// Should panic due to assertion failure; catch it
	func() {
		defer func() { recover() }()
		interp.ExecuteMethod("FailTest", "testFail", nil)
	}()

	var failedAsserts int
	for _, e := range tr.Events() {
		if e.Type == tracer.EventAssert && !e.Passed {
			failedAsserts++
		}
	}

	if failedAsserts != 1 {
		t.Errorf("expected 1 failed assert event, got %d", failedAsserts)
	}
}

func TestTraceSummaryIntegration(t *testing.T) {
	source := `public class Calc {
		public static Integer compute(Integer n) {
			Integer result = 0;
			for (Integer i = 0; i < n; i++) {
				result = result + i;
			}
			return result;
		}
	}`

	result, err := apex.ParseString("Calc.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree)

	tr := tracer.NewRecordingTracer()
	interp := NewInterpreter(reg, nil)
	interp.SetTracer(tr)
	interp.SetCurrentFile("Calc.cls")

	interp.ExecuteMethod("Calc", "compute", []*Value{IntegerValue(5)})

	summary := tracer.BuildSummary(tr.Events(), 10)

	if len(summary.Methods) != 1 {
		t.Fatalf("expected 1 method in summary, got %d", len(summary.Methods))
	}
	if summary.Methods[0].Method != "compute" {
		t.Errorf("expected method compute, got %s", summary.Methods[0].Method)
	}
	if summary.Methods[0].Calls != 1 {
		t.Errorf("expected 1 call, got %d", summary.Methods[0].Calls)
	}

	if len(summary.Lines) == 0 {
		t.Error("expected line stats in summary")
	}

	// The loop body line should have multiple executions
	foundHotLine := false
	for _, l := range summary.Lines {
		if l.Executions > 1 {
			foundHotLine = true
			break
		}
	}
	if !foundHotLine {
		t.Error("expected at least one line with multiple executions from the loop")
	}
}

func TestNoopTracerNoOverhead(t *testing.T) {
	source := `public class Noop {
		public static void run() {
			Integer x = 1;
		}
	}`

	result, err := apex.ParseString("Noop.cls", source)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewRegistry()
	reg.RegisterClass(result.Tree)

	// Default interpreter uses NoopTracer
	interp := NewInterpreter(reg, nil)
	interp.ExecuteMethod("Noop", "run", nil)
	// Just verify it doesn't crash — noop tracer is the default
}
