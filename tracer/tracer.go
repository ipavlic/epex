package tracer

import (
	"sync"
	"time"
)

// EventType classifies a trace event.
type EventType int

const (
	EventLine        EventType = iota // An Apex source line was executed
	EventMethodEntry                  // Entering a method
	EventMethodExit                   // Leaving a method
	EventSOQL                         // A SOQL query was executed
	EventDML                          // A DML operation was executed
	EventAssert                       // An assertion was evaluated
)

func (e EventType) String() string {
	switch e {
	case EventLine:
		return "Line"
	case EventMethodEntry:
		return "MethodEntry"
	case EventMethodExit:
		return "MethodExit"
	case EventSOQL:
		return "SOQL"
	case EventDML:
		return "DML"
	case EventAssert:
		return "Assert"
	default:
		return "Unknown"
	}
}

// TraceEvent records a single event during Apex execution.
type TraceEvent struct {
	Type      EventType
	Timestamp time.Time
	File      string
	Line      int
	Class     string
	Method    string
	Detail    string        // SOQL query text, DML operation, assert message, etc.
	Duration  time.Duration // For method exit, SOQL, DML
	RowCount  int           // For SOQL/DML
	Passed    bool          // For Assert events
}

// Tracer is the interface for recording trace events.
type Tracer interface {
	Enabled() bool
	Record(event TraceEvent)
	Events() []TraceEvent
}

// NoopTracer discards all events with zero overhead.
type NoopTracer struct{}

func (n *NoopTracer) Enabled() bool        { return false }
func (n *NoopTracer) Record(_ TraceEvent)  {}
func (n *NoopTracer) Events() []TraceEvent { return nil }

// RecordingTracer collects all trace events in memory.
type RecordingTracer struct {
	mu     sync.Mutex
	events []TraceEvent
	epoch  time.Time
}

// NewRecordingTracer creates a tracer that records all events.
func NewRecordingTracer() *RecordingTracer {
	return &RecordingTracer{
		epoch: time.Now(),
	}
}

func (r *RecordingTracer) Enabled() bool { return true }

func (r *RecordingTracer) Record(event TraceEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *RecordingTracer) Events() []TraceEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]TraceEvent, len(r.events))
	copy(out, r.events)
	return out
}

// Epoch returns the start time of this tracer for relative timestamp calculation.
func (r *RecordingTracer) Epoch() time.Time {
	return r.epoch
}
