package chglogcmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeriveReleaseMeta(t *testing.T) {
	meta := DeriveReleaseMeta("cmds/prcmd/cmd.go\npkg/aiprovider/openai.go\n")
	require.Contains(t, meta.Impact, "CLI 命令")
	require.Contains(t, meta.Validation, "fastgit check run")
	require.Contains(t, meta.Rollback, "revert")
}

func TestMergeMetaSections(t *testing.T) {
	content := renderUnreleasedTemplate()
	meta := ReleaseMeta{
		Impact:     "- 影响 CLI",
		Validation: "- 跑测试",
		Rollback:   "- revert",
	}
	updated, changed := mergeMetaSections(content, meta)
	require.True(t, changed)
	require.Contains(t, updated, "## 影响范围")
	require.Contains(t, updated, "影响 CLI")
}

func TestValidateReleaseReadinessRequiresMeta(t *testing.T) {
	content := `# [Unreleased]

## 新增

- item

## 修复

暂无

## 变更

暂无

## 文档

暂无

## 影响范围

暂无

## 验证建议

- ok

## 回滚建议

- ok
`
	require.Error(t, ValidateReleaseReadiness(content))
}
