package reporter

import (
	"fmt"
	"io"
	"strings"
)

// FormatHuman writes test results in a human-readable table format
// similar to `sf apex run test --result-format human`.
func FormatHuman(w io.Writer, r TestRunResult) {
	// Test results table
	if len(r.Tests) > 0 {
		fmt.Fprintln(w, "=== Test Results")

		// Calculate column widths
		nameWidth := len("TEST NAME")
		for _, t := range r.Tests {
			full := t.ClassName + "." + t.MethodName
			if len(full) > nameWidth {
				nameWidth = len(full)
			}
		}

		// Header
		fmt.Fprintf(w, " %-7s  %-*s  %10s  %s\n", "OUTCOME", nameWidth, "TEST NAME", "RUNTIME", "MESSAGE")
		fmt.Fprintf(w, " %s  %s  %s  %s\n",
			strings.Repeat("─", 7),
			strings.Repeat("─", nameWidth),
			strings.Repeat("─", 10),
			strings.Repeat("─", 40))

		for _, t := range r.Tests {
			full := t.ClassName + "." + t.MethodName
			marker := outcomeMarker(t.Outcome)
			msg := t.Message
			if len(msg) > 60 {
				msg = msg[:57] + "..."
			}
			fmt.Fprintf(w, " %s  %-*s  %8dms  %s\n",
				marker, nameWidth, full, int(t.DurationMs), msg)
		}
		fmt.Fprintln(w)
	}

	// Failures detail
	var failures []TestResult
	for _, t := range r.Tests {
		if t.Outcome == OutcomeFail {
			failures = append(failures, t)
		}
	}
	if len(failures) > 0 {
		fmt.Fprintln(w, "=== Failures")
		for _, t := range failures {
			fmt.Fprintf(w, " %s.%s\n", t.ClassName, t.MethodName)
			fmt.Fprintf(w, "   Message: %s\n", t.Message)
			fmt.Fprintln(w)
		}
	}

	// Summary
	fmt.Fprintln(w, "=== Test Summary")
	fmt.Fprintf(w, " %-16s %s\n", "Outcome:", string(r.Summary.Outcome))
	fmt.Fprintf(w, " %-16s %d\n", "Tests Ran:", r.Summary.TestsRan)
	fmt.Fprintf(w, " %-16s %d\n", "Passing:", r.Summary.Passing)
	fmt.Fprintf(w, " %-16s %d\n", "Failing:", r.Summary.Failing)
	if r.Summary.Skipped > 0 {
		fmt.Fprintf(w, " %-16s %d\n", "Skipped:", r.Summary.Skipped)
	}
	fmt.Fprintf(w, " %-16s %s\n", "Pass Rate:", r.Summary.PassRate)
	fmt.Fprintf(w, " %-16s %s\n", "Fail Rate:", r.Summary.FailRate)
	fmt.Fprintf(w, " %-16s %.0fms\n", "Test Run Time:", r.Summary.TestRunMs)
	fmt.Fprintf(w, " %-16s %.0fms\n", "Command Time:", r.Summary.CommandMs)
}

func outcomeMarker(o Outcome) string {
	switch o {
	case OutcomePass:
		return " Pass  "
	case OutcomeFail:
		return " Fail  "
	case OutcomeSkip:
		return " Skip  "
	default:
		return " ???   "
	}
}
