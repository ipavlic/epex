package tracer

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Chrome Trace Event format for Perfetto visualization.
// See: https://docs.google.com/document/d/1CvAClvFfyA5R-PhYUmn5OOQtYMH4h6I0nSsKchNAySU

// perfettoEvent is a single event in Chrome Trace Event format.
type perfettoEvent struct {
	Name string         `json:"name"`
	Cat  string         `json:"cat"`
	Ph   string         `json:"ph"`            // B=begin, E=end, X=complete, i=instant
	Ts   float64        `json:"ts"`            // timestamp in microseconds
	Dur  float64        `json:"dur,omitempty"` // duration in microseconds (for X events)
	Pid  int            `json:"pid"`
	Tid  int            `json:"tid"`
	Args map[string]any `json:"args,omitempty"`
}

// WritePerfetto writes trace events in Chrome Trace Event JSON format.
func WritePerfetto(w io.Writer, events []TraceEvent, epoch time.Time) error {
	var perfEvents []perfettoEvent

	for _, e := range events {
		tsUs := float64(e.Timestamp.Sub(epoch).Microseconds())

		switch e.Type {
		case EventMethodEntry:
			perfEvents = append(perfEvents, perfettoEvent{
				Name: fmt.Sprintf("%s.%s", e.Class, e.Method),
				Cat:  "apex",
				Ph:   "B",
				Ts:   tsUs,
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"file": e.File,
					"line": e.Line,
				}),
			})

		case EventMethodExit:
			perfEvents = append(perfEvents, perfettoEvent{
				Name: fmt.Sprintf("%s.%s", e.Class, e.Method),
				Cat:  "apex",
				Ph:   "E",
				Ts:   tsUs,
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"duration_ms": float64(e.Duration.Microseconds()) / 1000.0,
				}),
			})

		case EventSOQL:
			perfEvents = append(perfEvents, perfettoEvent{
				Name: "SOQL",
				Cat:  "soql",
				Ph:   "X",
				Ts:   tsUs - float64(e.Duration.Microseconds()),
				Dur:  float64(e.Duration.Microseconds()),
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"query": e.Detail,
					"rows":  e.RowCount,
					"file":  e.File,
					"line":  e.Line,
				}),
			})

		case EventDML:
			perfEvents = append(perfEvents, perfettoEvent{
				Name: fmt.Sprintf("DML %s", e.Detail),
				Cat:  "dml",
				Ph:   "X",
				Ts:   tsUs - float64(e.Duration.Microseconds()),
				Dur:  float64(e.Duration.Microseconds()),
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"operation": e.Detail,
					"sobject":   e.Class,
					"rows":      e.RowCount,
					"file":      e.File,
					"line":      e.Line,
				}),
			})

		case EventAssert:
			result := "Pass"
			if !e.Passed {
				result = "Fail"
			}
			perfEvents = append(perfEvents, perfettoEvent{
				Name: fmt.Sprintf("Assert (%s)", result),
				Cat:  "assert",
				Ph:   "i",
				Ts:   tsUs,
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"result":  result,
					"message": e.Detail,
					"file":    e.File,
					"line":    e.Line,
				}),
			})

		case EventLine:
			perfEvents = append(perfEvents, perfettoEvent{
				Name: fmt.Sprintf("%s:%d", e.File, e.Line),
				Cat:  "line",
				Ph:   "i",
				Ts:   tsUs,
				Pid:  1,
				Tid:  1,
				Args: nonEmptyArgs(map[string]any{
					"file":   e.File,
					"line":   e.Line,
					"class":  e.Class,
					"method": e.Method,
				}),
			})
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(perfEvents)
}

func nonEmptyArgs(args map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range args {
		switch val := v.(type) {
		case string:
			if val != "" {
				out[k] = val
			}
		case int:
			if val != 0 {
				out[k] = val
			}
		default:
			if v != nil {
				out[k] = v
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
