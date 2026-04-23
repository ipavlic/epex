package reporter

import (
	"encoding/json"
	"fmt"
	"io"
)

// FormatJSON writes test results as formatted JSON.
func FormatJSON(w io.Writer, r TestRunResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// FormatJSONString returns the test results as a JSON string.
func FormatJSONString(r TestRunResult) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling test results: %w", err)
	}
	return string(data), nil
}
