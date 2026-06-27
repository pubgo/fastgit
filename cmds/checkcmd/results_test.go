package checkcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFailedStepName(t *testing.T) {
	require.Empty(t, FailedStepName(nil))
	require.Equal(t, "vet", FailedStepName([]StepResult{
		{Step: Step{Name: "fmt"}},
		{Step: Step{Name: "vet"}, Err: errTest("vet failed")},
	}))
}

func TestFormatFailureSummary(t *testing.T) {
	summary := FormatFailureSummary([]StepResult{
		{Step: Step{Name: "fmt"}, Output: "main.go", Err: errTest("fmt failed: unformatted files")},
	}, errTest("fmt failed"))
	require.Contains(t, summary, `check failed at step "fmt"`)
	require.Contains(t, summary, "main.go")
}

type errTest string

func (e errTest) Error() string { return string(e) }
