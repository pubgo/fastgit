package chglogcmd

import (
	"fmt"
	"strings"
)

const draftPromptTemplate = `你是当前仓库的 Changelog 维护助手。

目标：根据当前仓库改动更新 .version/changelog/Unreleased.md。

必须遵守：
1. 只根据可见改动编写条目，不杜撰。
2. 分类只能使用：新增 / 修复 / 变更 / 文档。
3. 中文技术文风，单条以动词开头，简洁、可追溯。
4. 合并去重，避免同义重复。
5. 只允许修改 .version/changelog/Unreleased.md。
6. 若分类下暂无内容，写“暂无”。
7. 不要改动 README、VERSION 或任何历史版本文件。

归类规则：
- feat / 新增能力 -> 新增
- fix / bug 修正 -> 修复
- 重构、依赖迁移、行为调整、优化 -> 变更
- README、docs、注释更新 -> 文档

工作目录：%s
基线分支：%s
当前版本：%s

请先核对以下上下文，再直接修改目标文件：

--- .version/changelog/Unreleased.md ---
%s

--- .version/changelog/README.md ---
%s

--- git diff %s --stat ---
%s

--- git diff %s --name-only ---
%s

完成后请仅输出简短自检：
- 是否只改动了 Unreleased.md
- 是否统一为标准四类
- 是否完成去重`

type draftPromptData struct {
	RepoRoot          string
	BaseRef           string
	Version           string
	UnreleasedContent string
	ReadmeContent     string
	DiffStat          string
	DiffNames         string
}

func renderDraftPrompt(data draftPromptData) string {
	return strings.TrimSpace(fmt.Sprintf(
		draftPromptTemplate,
		data.RepoRoot,
		data.BaseRef,
		data.Version,
		data.UnreleasedContent,
		data.ReadmeContent,
		data.BaseRef,
		data.DiffStat,
		data.BaseRef,
		data.DiffNames,
	))
}

func renderRepoChangelogPromptTemplate() string {
	return strings.TrimSpace(`---
name: changelog
description: 维护 .version/changelog（更新 Unreleased 或执行版本落版）
argument-hint: "模式：draft（更新 Unreleased）或 release（按 .version/VERSION 落版）"
agent: agent
---

你是当前仓库的 Changelog 维护助手。

## 目标

- `+"`draft`"+` 模式：根据当前改动更新 `+"`.version/changelog/Unreleased.md`"+`。
- `+"`release`"+` 模式：将 `+"`Unreleased.md`"+` 落版为版本文件，并重建空模板。

## 必读上下文

在开始前先读取：

- `+"`.version/changelog/Unreleased.md`"+`
- `+"`.version/VERSION`"+`

## 通用规则

1. 只基于可见改动生成条目，不杜撰。
2. **标准分类**：`+"`新增`"+` / `+"`修复`"+` / `+"`变更`"+` / `+"`文档`"+`。
   - 非标准分类（如 `+"`优化`"+`、`+"`重构`"+`）必须归入上述四类（通常归 `+"`变更`"+`）。
3. 语言使用中文技术文风，单条以动词开头，简洁可追溯。
4. 去重：同类项合并，避免语义重复。
5. 不改写历史版本文件语义与顺序。

## draft 模式

1. 获取工作区 diff：运行 `+"`git diff <base> --stat`"+` 和 `+"`git diff <base> --name-only`"+` 确认改动范围。
2. 仅更新 `+"`.version/changelog/Unreleased.md`"+`。
3. 若缺少分类小节则补齐；无内容的小节写“暂无”。
4. 归类规则：
   - feat / 新增能力 → `+"`新增`"+`
   - fix / bug 修正 → `+"`修复`"+`
   - 重构、依赖迁移、行为调整、优化 → `+"`变更`"+`
   - README、docs、注释更新 → `+"`文档`"+`

## release 模式

1. 读取 `+"`.version/VERSION`"+` 获取目标版本号（如 `+"`v0.3.0`"+`）。
2. **版本冲突检查**：若 `+"`.version/changelog/<VERSION>.md`"+` 已存在，提示用户确认是否需要递增版本号，不自行覆盖。
3. 创建版本文件 `+"`.version/changelog/<VERSION>.md`"+`：
   - 标题格式：`+"`# [<VERSION>] - <YYYY-MM-DD>`"+`。
   - 将 `+"`Unreleased.md`"+` 的内容迁移过去（分类统一为标准四类）。
4. 重建 `+"`Unreleased.md`"+` 空模板（四个分类均写“暂无”）。
5. 更新 `+"`.version/changelog/README.md`"+` 索引：在列表顶部（`+"`Unreleased`"+` 之后）插入新版本链接。
6. 更新 `+"`.version/VERSION`"+` 为下一个预期版本号（**仅在用户确认后**，否则保持当前值）。

## 输出要求

- 直接给出文件修改结果。
- 末尾附一段简短自检：
  - 是否仅改动 `+"`.version/`"+` 范围内的文件；
  - 分类是否统一为标准四类，是否完成去重；
  - 历史版本文件是否未被修改。
`) + "\n"
}

func renderRepoChangelogRulesTemplate() string {
	return strings.TrimSpace(`---
name: Changelog 专项规范
description: 仅用于维护 .version/changelog，保证 Unreleased 与版本文件结构稳定、分类一致、条目可追溯
applyTo: ".version/changelog/*.md"
---

# Changelog 维护规范

本规则仅适用于 `+"`.version/changelog/*.md`"+`。

## 结构约束

- `+"`Unreleased.md`"+` 推荐分类：`+"`新增`"+` / `+"`修复`"+` / `+"`变更`"+` / `+"`文档`"+`。
- 若某分类暂无内容，写“暂无”。

## 内容约束

- 仅基于可见改动编写条目，不杜撰能力或影响。
- 单条应简洁、可读、可追溯，以动词开头。
- 重复事项需合并去重，避免同义重复。
- 非标准分类（如 `+"`优化`"+`、`+"`重构`"+`）必须归入标准四类（通常归 `+"`变更`"+`）。
- 不改写历史版本文件语义，不重排已发布版本。

## 落版约束（release）

- 版本号来源于 `+"`.version/VERSION`"+`。
- 落版文件：`+"`.version/changelog/<VERSION>.md`"+`。
- 文件头格式：`+"`# [<VERSION>] - <YYYY-MM-DD>`"+`。
- 落版前检查版本文件是否已存在，已存在时提示用户确认。
- 落版后重建 `+"`.version/changelog/Unreleased.md`"+` 模板（四个分类）。
- 落版后同步更新 `+"`.version/changelog/README.md`"+` 索引。

## 协同建议

- 建议通过 agent 提示词执行：`+"`/changelog draft|release`"+`。
`) + "\n"
}

func renderRepoReleaseRulesTemplate() string {
	return strings.TrimSpace(`---
name: 发布前变更核对约束
description: "Use when preparing a release or completing behavior-impacting changes, including changelog updates and release regression checks."
---

# 发布前核对规则

用于“准备发布”或“完成具备行为影响的改动”时的统一核对。

## 发布前检查清单

- 变更说明已写入 `+"`.version/changelog/Unreleased.md`"+`，分类正确（新增/修复/变更/文档）。
- 用户可见行为变化，已同步示例或说明文档。

## 质量门槛

- 执行完整回归测试并确认通过。
- 仅基于真实改动与真实测试结果编写发布说明，不杜撰。

## 落版流程

- 首选通过 `+"`fastgit changelog release`"+` 或等效 agent prompt 执行。
- 版本号来源于 `+"`.version/VERSION`"+`；若版本文件已存在，需确认是否递增。
- 落版后重建 `+"`Unreleased`"+` 模板并更新 changelog 索引。
- changelog 结构与落版细节以 `+"`.github/instructions/changelog.instructions.md`"+` 为准。
`) + "\n"
}
