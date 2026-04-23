package tracer

import (
	"fmt"
	"io"
	"strings"
)

// FormatSummaryHuman writes the trace summary in human-readable table format.
func FormatSummaryHuman(w io.Writer, s Summary) {
	if len(s.Methods) > 0 {
		fmt.Fprintln(w, "=== Method Performance")

		// Calculate column widths
		nameWidth := len("METHOD")
		for _, m := range s.Methods {
			full := m.Class + "." + m.Method
			if len(full) > nameWidth {
				nameWidth = len(full)
			}
		}

		fmt.Fprintf(w, " %-*s  %6s  %10s  %10s\n", nameWidth, "METHOD", "CALLS", "TOTAL", "AVG")
		fmt.Fprintf(w, " %s  %s  %s  %s\n",
			strings.Repeat("─", nameWidth),
			strings.Repeat("─", 6),
			strings.Repeat("─", 10),
			strings.Repeat("─", 10))

		for _, m := range s.Methods {
			fmt.Fprintf(w, " %-*s  %6d  %8.1fms  %8.1fms\n",
				nameWidth, m.Class+"."+m.Method, m.Calls, m.TotalMs, m.AvgMs)
		}
		fmt.Fprintln(w)
	}

	if len(s.SOQL) > 0 {
		fmt.Fprintln(w, "=== SOQL Queries")

		queryWidth := len("QUERY")
		for _, q := range s.SOQL {
			display := truncate(q.Query, 60)
			if len(display) > queryWidth {
				queryWidth = len(display)
			}
		}

		fmt.Fprintf(w, " %-*s  %6s  %10s  %10s\n", queryWidth, "QUERY", "CALLS", "ROWS", "TIME")
		fmt.Fprintf(w, " %s  %s  %s  %s\n",
			strings.Repeat("─", queryWidth),
			strings.Repeat("─", 6),
			strings.Repeat("─", 10),
			strings.Repeat("─", 10))

		for _, q := range s.SOQL {
			fmt.Fprintf(w, " %-*s  %6d  %10d  %8.1fms\n",
				queryWidth, truncate(q.Query, 60), q.Calls, q.TotalRows, q.TotalMs)
		}
		fmt.Fprintln(w)
	}

	if len(s.DML) > 0 {
		fmt.Fprintln(w, "=== DML Operations")
		fmt.Fprintf(w, " %-10s  %-20s  %6s  %10s  %10s\n", "OPERATION", "SOBJECT", "CALLS", "ROWS", "TIME")
		fmt.Fprintf(w, " %s  %s  %s  %s  %s\n",
			strings.Repeat("─", 10),
			strings.Repeat("─", 20),
			strings.Repeat("─", 6),
			strings.Repeat("─", 10),
			strings.Repeat("─", 10))

		for _, d := range s.DML {
			fmt.Fprintf(w, " %-10s  %-20s  %6d  %10d  %8.1fms\n",
				d.Operation, d.SObject, d.Calls, d.TotalRows, d.TotalMs)
		}
		fmt.Fprintln(w)
	}

	if len(s.Lines) > 0 {
		fmt.Fprintln(w, "=== Hot Lines")

		locWidth := len("LOCATION")
		for _, l := range s.Lines {
			loc := fmt.Sprintf("%s:%d", l.File, l.Line)
			if len(loc) > locWidth {
				locWidth = len(loc)
			}
		}

		fmt.Fprintf(w, " %-*s  %10s\n", locWidth, "LOCATION", "EXECUTIONS")
		fmt.Fprintf(w, " %s  %s\n",
			strings.Repeat("─", locWidth),
			strings.Repeat("─", 10))

		for _, l := range s.Lines {
			fmt.Fprintf(w, " %-*s  %10d\n", locWidth, fmt.Sprintf("%s:%d", l.File, l.Line), l.Executions)
		}
		fmt.Fprintln(w)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
