package prcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyReviewToBody(t *testing.T) {
	body := renderBody("- feat: add x", " main.go | 1 +", "main.go", RepoContext{Branch: "feature/x", BaseRef: "main"})
	review := `## Blockers

None

## Suggestions

- Add unit test for edge case

## Nits

None

## Test plan

- [ ] Run go test ./pkg/foo/...`

	merged := ApplyReviewToBody(body, review)
	require.Contains(t, merged, "### Local code review")
	require.Contains(t, merged, "Add unit test for edge case")
	require.Contains(t, merged, "Run go test ./pkg/foo/...")
	require.Contains(t, merged, "Run `fastgit check run`")
}

func TestApplyReviewToBodySkipsEmptyBlockers(t *testing.T) {
	body := "## Test plan\n\n- [ ] default\n\n## Rollback\n\nok"
	review := "## Blockers\n\nNone\n\n## Suggestions\n\nNone\n\n## Nits\n\nNone\n\n## Test plan\n\nNone"
	merged := ApplyReviewToBody(body, review)
	require.NotContains(t, merged, "**Blockers**")
}

func TestReviewTestPlanItems(t *testing.T) {
	items := reviewTestPlanItems("## Test plan\n\n- Run tests\n- [ ] Manual QA\n")
	require.Equal(t, []string{"Run tests", "Manual QA"}, items)
}
