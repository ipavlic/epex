package tracer

import (
	"encoding/json"
	"io"
)

// FormatSummaryJSON writes the trace summary as formatted JSON.
func FormatSummaryJSON(w io.Writer, s Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}
