package reporter

import (
	"fmt"
	"time"
)

// TestResult mirrors interpreter.TestResult but is decoupled for the reporter layer.
type TestResult struct {
	ClassName  string        `json:"ClassName"`
	MethodName string        `json:"MethodName"`
	Outcome    Outcome       `json:"Outcome"`
	Message    string        `json:"Message,omitempty"`
	Duration   time.Duration `json:"-"`
	DurationMs float64       `json:"RunTime"`
}

// Outcome represents the result of a test method.
type Outcome string

const (
	OutcomePass Outcome = "Pass"
	OutcomeFail Outcome = "Fail"
	OutcomeSkip Outcome = "Skip"
)

// Summary holds aggregate test run statistics.
type Summary struct {
	Outcome   Outcome `json:"outcome"`
	TestsRan  int     `json:"testsRan"`
	Passing   int     `json:"passing"`
	Failing   int     `json:"failing"`
	Skipped   int     `json:"skipped"`
	PassRate  string  `json:"passRate"`
	FailRate  string  `json:"failRate"`
	TestRunMs float64 `json:"testTotalTime"`
	CommandMs float64 `json:"commandTime"`
}

// TestRunResult is the top-level result object containing all test data.
type TestRunResult struct {
	Summary Summary      `json:"summary"`
	Tests   []TestResult `json:"tests"`
}

// NewTestRunResult builds a TestRunResult from a slice of TestResults and total command duration.
func NewTestRunResult(results []TestResult, commandDuration time.Duration) TestRunResult {
	r := TestRunResult{
		Tests: results,
	}

	var totalTestMs float64
	for i := range r.Tests {
		r.Tests[i].DurationMs = float64(r.Tests[i].Duration.Milliseconds())
		totalTestMs += r.Tests[i].DurationMs
		switch r.Tests[i].Outcome {
		case OutcomePass:
			r.Summary.Passing++
		case OutcomeFail:
			r.Summary.Failing++
		case OutcomeSkip:
			r.Summary.Skipped++
		}
	}

	r.Summary.TestsRan = r.Summary.Passing + r.Summary.Failing
	r.Summary.TestRunMs = totalTestMs
	r.Summary.CommandMs = float64(commandDuration.Milliseconds())

	total := r.Summary.Passing + r.Summary.Failing
	if total > 0 {
		r.Summary.PassRate = formatPercent(r.Summary.Passing, total)
		r.Summary.FailRate = formatPercent(r.Summary.Failing, total)
	} else {
		r.Summary.PassRate = "0%"
		r.Summary.FailRate = "0%"
	}

	if r.Summary.Failing > 0 {
		r.Summary.Outcome = OutcomeFail
	} else {
		r.Summary.Outcome = OutcomePass
	}

	return r
}

func formatPercent(n, total int) string {
	pct := float64(n) / float64(total) * 100
	if pct == float64(int(pct)) {
		return fmt.Sprintf("%d%%", int(pct))
	}
	return fmt.Sprintf("%.1f%%", pct)
}
