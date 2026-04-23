package reporter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleResults() []TestResult {
	return []TestResult{
		{
			ClassName:  "AccountServiceTest",
			MethodName: "testCreateAccount",
			Outcome:    OutcomePass,
			Duration:   12 * time.Millisecond,
		},
		{
			ClassName:  "AccountServiceTest",
			MethodName: "testGetHighValueAccounts",
			Outcome:    OutcomePass,
			Duration:   8 * time.Millisecond,
		},
		{
			ClassName:  "ContactServiceTest",
			MethodName: "testInvalidEmail",
			Outcome:    OutcomeFail,
			Duration:   3 * time.Millisecond,
			Message:    "Expected: true, Actual: false",
		},
	}
}

func TestNewTestRunResult(t *testing.T) {
	r := NewTestRunResult(sampleResults(), 50*time.Millisecond)

	if r.Summary.TestsRan != 3 {
		t.Errorf("expected 3 tests ran, got %d", r.Summary.TestsRan)
	}
	if r.Summary.Passing != 2 {
		t.Errorf("expected 2 passing, got %d", r.Summary.Passing)
	}
	if r.Summary.Failing != 1 {
		t.Errorf("expected 1 failing, got %d", r.Summary.Failing)
	}
	if r.Summary.Outcome != OutcomeFail {
		t.Errorf("expected outcome Fail, got %s", r.Summary.Outcome)
	}
	if r.Summary.CommandMs != 50 {
		t.Errorf("expected command time 50ms, got %.0f", r.Summary.CommandMs)
	}
}

func TestFormatHuman(t *testing.T) {
	r := NewTestRunResult(sampleResults(), 50*time.Millisecond)

	var buf bytes.Buffer
	FormatHuman(&buf, r)
	output := buf.String()

	// Check key sections exist
	if !strings.Contains(output, "=== Test Results") {
		t.Error("missing Test Results header")
	}
	if !strings.Contains(output, "=== Failures") {
		t.Error("missing Failures section")
	}
	if !strings.Contains(output, "=== Test Summary") {
		t.Error("missing Test Summary section")
	}

	// Check test names appear
	if !strings.Contains(output, "AccountServiceTest.testCreateAccount") {
		t.Error("missing test name AccountServiceTest.testCreateAccount")
	}
	if !strings.Contains(output, "ContactServiceTest.testInvalidEmail") {
		t.Error("missing test name ContactServiceTest.testInvalidEmail")
	}

	// Check summary values
	if !strings.Contains(output, "Tests Ran:") {
		t.Error("missing Tests Ran in summary")
	}
	if !strings.Contains(output, "Passing:") {
		t.Error("missing Passing in summary")
	}
	if !strings.Contains(output, "Failing:") {
		t.Error("missing Failing in summary")
	}

	// Check failure detail
	if !strings.Contains(output, "Expected: true, Actual: false") {
		t.Error("missing failure message in Failures section")
	}
}

func TestFormatHumanAllPassing(t *testing.T) {
	results := []TestResult{
		{ClassName: "MyTest", MethodName: "test1", Outcome: OutcomePass, Duration: 5 * time.Millisecond},
	}
	r := NewTestRunResult(results, 10*time.Millisecond)

	var buf bytes.Buffer
	FormatHuman(&buf, r)
	output := buf.String()

	if strings.Contains(output, "=== Failures") {
		t.Error("should not have Failures section when all tests pass")
	}
	if !strings.Contains(output, "Outcome:         Pass") {
		t.Error("expected outcome Pass")
	}
}

func TestFormatJSON(t *testing.T) {
	r := NewTestRunResult(sampleResults(), 50*time.Millisecond)

	var buf bytes.Buffer
	err := FormatJSON(&buf, r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON
	var parsed TestRunResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.Summary.TestsRan != 3 {
		t.Errorf("expected 3 tests ran, got %d", parsed.Summary.TestsRan)
	}
	if parsed.Summary.Passing != 2 {
		t.Errorf("expected 2 passing, got %d", parsed.Summary.Passing)
	}
	if parsed.Summary.Failing != 1 {
		t.Errorf("expected 1 failing, got %d", parsed.Summary.Failing)
	}
	if len(parsed.Tests) != 3 {
		t.Errorf("expected 3 tests, got %d", len(parsed.Tests))
	}
	if parsed.Tests[2].Message != "Expected: true, Actual: false" {
		t.Errorf("unexpected message: %s", parsed.Tests[2].Message)
	}
}

func TestFormatJSONString(t *testing.T) {
	r := NewTestRunResult(sampleResults(), 50*time.Millisecond)

	s, err := FormatJSONString(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(s, `"outcome": "Fail"`) {
		t.Error("JSON should contain outcome Fail")
	}
	if !strings.Contains(s, `"testsRan": 3`) {
		t.Error("JSON should contain testsRan 3")
	}
}

func TestPassRate(t *testing.T) {
	tests := []struct {
		results  []TestResult
		passRate string
		failRate string
	}{
		{
			results: []TestResult{
				{Outcome: OutcomePass, Duration: time.Millisecond},
				{Outcome: OutcomePass, Duration: time.Millisecond},
			},
			passRate: "100%",
			failRate: "0%",
		},
		{
			results: []TestResult{
				{Outcome: OutcomePass, Duration: time.Millisecond},
				{Outcome: OutcomeFail, Duration: time.Millisecond},
				{Outcome: OutcomeFail, Duration: time.Millisecond},
			},
			passRate: "33.3%",
			failRate: "66.7%",
		},
	}

	for _, tc := range tests {
		r := NewTestRunResult(tc.results, time.Millisecond)
		if r.Summary.PassRate != tc.passRate {
			t.Errorf("expected pass rate %s, got %s", tc.passRate, r.Summary.PassRate)
		}
		if r.Summary.FailRate != tc.failRate {
			t.Errorf("expected fail rate %s, got %s", tc.failRate, r.Summary.FailRate)
		}
	}
}
