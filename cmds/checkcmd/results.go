package checkcmd

import (
	"fmt"
	"strings"
)

// FailedStepName returns the first step that failed (not skipped).
func FailedStepName(results []StepResult) string {
	for _, r := range results {
		if r.Err != nil && !r.Skipped {
			return r.Step.Name
		}
	}
	return ""
}

// FormatFailureSummary builds a readable failure summary for CLI output.
func FormatFailureSummary(results []StepResult, runErr error) string {
	step := FailedStepName(results)
	if step == "" && runErr != nil {
		return runErr.Error()
	}

	var b strings.Builder
	if step != "" {
		fmt.Fprintf(&b, "check failed at step %q", step)
	} else {
		b.WriteString("check failed")
	}
	if runErr != nil {
		fmt.Fprintf(&b, ": %v", runErr)
	}
	for _, r := range results {
		if r.Err == nil || r.Skipped {
			continue
		}
		if strings.TrimSpace(r.Output) != "" {
			fmt.Fprintf(&b, "\n--- %s output ---\n%s", r.Step.Name, strings.TrimSpace(r.Output))
		}
	}
	return b.String()
}
