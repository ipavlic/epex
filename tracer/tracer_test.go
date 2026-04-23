package tracer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func epoch() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func sampleEvents() []TraceEvent {
	t0 := epoch()
	return []TraceEvent{
		// Method entry
		{Type: EventMethodEntry, Timestamp: t0, File: "AccountService.cls", Line: 2, Class: "AccountService", Method: "getAccounts"},
		// Line events
		{Type: EventLine, Timestamp: t0.Add(100 * time.Microsecond), File: "AccountService.cls", Line: 3, Class: "AccountService", Method: "getAccounts"},
		{Type: EventLine, Timestamp: t0.Add(200 * time.Microsecond), File: "AccountService.cls", Line: 4, Class: "AccountService", Method: "getAccounts"},
		{Type: EventLine, Timestamp: t0.Add(300 * time.Microsecond), File: "AccountService.cls", Line: 3, Class: "AccountService", Method: "getAccounts"},
		// SOQL
		{Type: EventSOQL, Timestamp: t0.Add(5 * time.Millisecond), File: "AccountService.cls", Line: 3, Detail: "SELECT Id, Name FROM Account", Duration: 4 * time.Millisecond, RowCount: 10},
		// DML
		{Type: EventDML, Timestamp: t0.Add(8 * time.Millisecond), File: "AccountService.cls", Line: 4, Detail: "INSERT", Class: "Account", Duration: 2 * time.Millisecond, RowCount: 5},
		// Assert
		{Type: EventAssert, Timestamp: t0.Add(9 * time.Millisecond), File: "AccountServiceTest.cls", Line: 10, Detail: "Expected 10, got 10", Passed: true},
		{Type: EventAssert, Timestamp: t0.Add(10 * time.Millisecond), File: "AccountServiceTest.cls", Line: 12, Detail: "Expected true, got false", Passed: false},
		// Method exit
		{Type: EventMethodExit, Timestamp: t0.Add(10 * time.Millisecond), Class: "AccountService", Method: "getAccounts", Duration: 10 * time.Millisecond},
		// Second call to same method
		{Type: EventMethodEntry, Timestamp: t0.Add(11 * time.Millisecond), Class: "AccountService", Method: "getAccounts"},
		{Type: EventSOQL, Timestamp: t0.Add(14 * time.Millisecond), Detail: "SELECT Id, Name FROM Account", Duration: 2 * time.Millisecond, RowCount: 3},
		{Type: EventMethodExit, Timestamp: t0.Add(15 * time.Millisecond), Class: "AccountService", Method: "getAccounts", Duration: 4 * time.Millisecond},
	}
}

func TestNoopTracer(t *testing.T) {
	tr := &NoopTracer{}
	if tr.Enabled() {
		t.Error("noop tracer should not be enabled")
	}
	tr.Record(TraceEvent{Type: EventLine})
	if len(tr.Events()) != 0 {
		t.Error("noop tracer should return no events")
	}
}

func TestRecordingTracer(t *testing.T) {
	tr := NewRecordingTracer()
	if !tr.Enabled() {
		t.Error("recording tracer should be enabled")
	}

	tr.Record(TraceEvent{Type: EventLine, File: "test.cls", Line: 1})
	tr.Record(TraceEvent{Type: EventLine, File: "test.cls", Line: 2})

	events := tr.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Line != 1 || events[1].Line != 2 {
		t.Error("events not recorded correctly")
	}

	// Verify timestamps are auto-set
	if events[0].Timestamp.IsZero() {
		t.Error("expected auto-set timestamp")
	}
}

func TestRecordingTracerEventsCopy(t *testing.T) {
	tr := NewRecordingTracer()
	tr.Record(TraceEvent{Type: EventLine})

	events := tr.Events()
	events = append(events, TraceEvent{Type: EventDML})

	// Original should be unmodified
	if len(tr.Events()) != 1 {
		t.Error("Events() should return a copy")
	}
}

func TestWritePerfetto(t *testing.T) {
	events := sampleEvents()
	var buf bytes.Buffer
	err := WritePerfetto(&buf, events, epoch())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify valid JSON array
	var parsed []perfettoEvent
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	if len(parsed) != len(events) {
		t.Errorf("expected %d perfetto events, got %d", len(events), len(parsed))
	}

	// Check first event is method entry (B)
	if parsed[0].Ph != "B" {
		t.Errorf("expected first event ph=B, got %s", parsed[0].Ph)
	}
	if parsed[0].Cat != "apex" {
		t.Errorf("expected cat=apex, got %s", parsed[0].Cat)
	}
	if !strings.Contains(parsed[0].Name, "AccountService.getAccounts") {
		t.Errorf("expected name to contain AccountService.getAccounts, got %s", parsed[0].Name)
	}

	// Check SOQL event is X with duration
	soqlIdx := 4
	if parsed[soqlIdx].Ph != "X" {
		t.Errorf("expected SOQL event ph=X, got %s", parsed[soqlIdx].Ph)
	}
	if parsed[soqlIdx].Cat != "soql" {
		t.Errorf("expected cat=soql, got %s", parsed[soqlIdx].Cat)
	}
	if parsed[soqlIdx].Dur == 0 {
		t.Error("expected SOQL event to have duration")
	}

	// Check assert event is instant (i)
	assertIdx := 6
	if parsed[assertIdx].Ph != "i" {
		t.Errorf("expected assert event ph=i, got %s", parsed[assertIdx].Ph)
	}
	if parsed[assertIdx].Cat != "assert" {
		t.Errorf("expected cat=assert, got %s", parsed[assertIdx].Cat)
	}
}

func TestBuildSummary(t *testing.T) {
	events := sampleEvents()
	s := BuildSummary(events, 10)

	// Methods
	if len(s.Methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(s.Methods))
	}
	m := s.Methods[0]
	if m.Class != "AccountService" || m.Method != "getAccounts" {
		t.Errorf("unexpected method: %s.%s", m.Class, m.Method)
	}
	if m.Calls != 2 {
		t.Errorf("expected 2 calls, got %d", m.Calls)
	}
	if m.TotalMs <= 0 {
		t.Error("expected positive total time")
	}

	// SOQL
	if len(s.SOQL) != 1 {
		t.Fatalf("expected 1 SOQL pattern, got %d", len(s.SOQL))
	}
	q := s.SOQL[0]
	if q.Calls != 2 {
		t.Errorf("expected 2 SOQL calls, got %d", q.Calls)
	}
	if q.TotalRows != 13 {
		t.Errorf("expected 13 total rows, got %d", q.TotalRows)
	}

	// DML
	if len(s.DML) != 1 {
		t.Fatalf("expected 1 DML pattern, got %d", len(s.DML))
	}
	d := s.DML[0]
	if d.Operation != "INSERT" || d.SObject != "Account" {
		t.Errorf("unexpected DML: %s %s", d.Operation, d.SObject)
	}
	if d.Calls != 1 || d.TotalRows != 5 {
		t.Errorf("unexpected DML stats: calls=%d rows=%d", d.Calls, d.TotalRows)
	}

	// Lines
	if len(s.Lines) == 0 {
		t.Fatal("expected line stats")
	}
	// Line 3 executed twice
	found := false
	for _, l := range s.Lines {
		if l.File == "AccountService.cls" && l.Line == 3 && l.Executions == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected AccountService.cls:3 with 2 executions")
	}
}

func TestBuildSummaryTopLines(t *testing.T) {
	events := sampleEvents()
	s := BuildSummary(events, 1)
	if len(s.Lines) != 1 {
		t.Errorf("expected 1 line (top 1), got %d", len(s.Lines))
	}
}

func TestFormatSummaryHuman(t *testing.T) {
	events := sampleEvents()
	s := BuildSummary(events, 10)

	var buf bytes.Buffer
	FormatSummaryHuman(&buf, s)
	output := buf.String()

	if !strings.Contains(output, "=== Method Performance") {
		t.Error("missing Method Performance section")
	}
	if !strings.Contains(output, "AccountService.getAccounts") {
		t.Error("missing method name")
	}
	if !strings.Contains(output, "=== SOQL Queries") {
		t.Error("missing SOQL section")
	}
	if !strings.Contains(output, "SELECT Id, Name FROM Account") {
		t.Error("missing SOQL query")
	}
	if !strings.Contains(output, "=== DML Operations") {
		t.Error("missing DML section")
	}
	if !strings.Contains(output, "INSERT") {
		t.Error("missing DML operation")
	}
	if !strings.Contains(output, "=== Hot Lines") {
		t.Error("missing Hot Lines section")
	}
}

func TestFormatSummaryJSON(t *testing.T) {
	events := sampleEvents()
	s := BuildSummary(events, 10)

	var buf bytes.Buffer
	err := FormatSummaryJSON(&buf, s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed Summary
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(parsed.Methods))
	}
	if len(parsed.SOQL) != 1 {
		t.Errorf("expected 1 SOQL, got %d", len(parsed.SOQL))
	}
}
