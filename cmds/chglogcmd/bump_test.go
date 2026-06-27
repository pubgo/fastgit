package chglogcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBumpConsistencyPatchRejectsAdded(t *testing.T) {
	sections := map[string]string{
		"新增": "- 新功能",
		"修复": "暂无",
		"变更": "暂无",
		"文档": "暂无",
	}
	require.Error(t, ValidateBumpConsistency(sections, "patch"))
	require.NoError(t, ValidateBumpConsistency(sections, "minor"))
}

func TestValidateBumpConsistencyDocsOnly(t *testing.T) {
	sections := map[string]string{
		"新增": "暂无",
		"修复": "暂无",
		"变更": "暂无",
		"文档": "- 更新 README",
	}
	require.Error(t, ValidateBumpConsistency(sections, "minor"))
	require.NoError(t, ValidateBumpConsistency(sections, "patch"))
}

func TestValidateBumpConsistencyBreaking(t *testing.T) {
	sections := map[string]string{
		"新增": "暂无",
		"修复": "暂无",
		"变更": "- BREAKING CHANGE: remove legacy API",
		"文档": "暂无",
	}
	require.Error(t, ValidateBumpConsistency(sections, "patch"))
	require.Error(t, ValidateBumpConsistency(sections, "minor"))
	require.NoError(t, ValidateBumpConsistency(sections, "major"))
}
