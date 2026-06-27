package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecommendUsesDefaults(t *testing.T) {
	m := &Memory{data: defaultState()}
	recs := m.Recommend("commit")
	require.Contains(t, recs, "push")
}

func TestRecordAndRecommend(t *testing.T) {
	dir := t.TempDir()
	m := &Memory{
		path: dir + "/workflow.yaml",
		data: defaultState(),
	}
	require.NoError(t, m.Record("commit"))
	require.NoError(t, m.Record("push"))
	require.NoError(t, m.Record("commit"))
	require.NoError(t, m.Record("push"))

	recs := m.Recommend("commit")
	require.NotEmpty(t, recs)
	require.Equal(t, "push", recs[0])
}

func TestNormalizeCommand(t *testing.T) {
	require.Equal(t, "commit", normalizeCommand("fastgit commit"))
	require.Equal(t, "pr", normalizeCommand("pr create"))
}
