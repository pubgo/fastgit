package docscmd

import (
	"fmt"
	"strings"
)

const docsBacktick = "`"

func renderCommitMessagePromptTemplate() string {
	tpl := `---
name: commit-message
description: 基于当前代码改动生成提交信息并直接执行本地 git commit，默认继续 push 到当前分支
argument-hint: "可选：补充本次提交的核心意图、偏好的 type/scope；默认直接完成本地提交并推送远程"
agent: agent
---

你是当前仓库的 Git 提交信息助手。

## 目标

基于当前代码改动生成高质量的 git commit message，并**直接完成本地提交与远程推送**。

你的首要职责不是解释 diff，而是：

1. 判断本次应提交哪些改动；
2. 生成最合适的 commit message；
3. 执行本地 {{BT}}git commit{{BT}}；
4. 执行 {{BT}}git push{{BT}}；
5. 报告结果。

除非用户明确要求不要推送，否则**默认执行 {{BT}}git push{{BT}}**。

## 必读上下文

优先按以下顺序读取或获取：

- {{BT}}git diff --cached --stat{{BT}}
- {{BT}}git diff --cached --name-only{{BT}}
- 如有必要，再查看 {{BT}}git diff --cached{{BT}}

如果 staged diff 为空，不要立即结束；继续读取：

- {{BT}}git status --short{{BT}}
- {{BT}}git diff --stat{{BT}}
- {{BT}}git diff --name-only{{BT}}
- 如有必要，再查看 {{BT}}git diff{{BT}}

处理规则：

- 如果 **有 staged diff**：
  - 仅基于 staged diff 生成 commit message；
  - 仅提交 staged 内容；
  - 不要自动把 unstaged 改动加入提交。
- 如果 **没有 staged diff，但有 unstaged diff**：
  - 基于 working tree 改动生成 commit message；
  - 自动执行 {{BT}}git add -A{{BT}} 将当前改动加入暂存区；
  - 再执行本地提交。
- 如果 staged / unstaged 都为空：
  - 明确提示当前没有可用于生成提交信息的代码改动。
  - 此时不要杜撰任何 commit message，也不要执行提交。

## 执行规则

在确定 commit message 后：

1. 如果 staged diff 非空：直接执行 {{BT}}git commit -m "<best>"{{BT}}。
2. 如果 staged diff 为空但 unstaged diff 非空：先执行 {{BT}}git add -A{{BT}}，再执行 {{BT}}git commit -m "<best>"{{BT}}。
3. 提交成功后，默认继续执行 {{BT}}git push{{BT}}。
4. 优先推送当前分支的上游；如果没有上游，则推送当前分支到同名远程分支。
5. 如果 {{BT}}git commit{{BT}} 失败，应输出失败原因，而不是假装成功。
6. 如果 {{BT}}git push{{BT}} 失败，应输出真实失败原因。
7. 不要编造 commit hash；只能使用真实执行结果。

## 判定优先级

生成提交信息时，按以下优先级判断：

1. **用户可见新能力** 优先于内部重构细节。
  - 例如：新增命令、子命令、交互入口、脚手架、repo prompt、工作流能力，优先考虑 {{BT}}feat{{BT}}。
2. **真实行为修复** 优先于实现细节调整。
  - 例如：修复发布流程、修复输出错误、修复空发布，优先考虑 {{BT}}fix{{BT}}。
3. **纯结构调整且无新增用户能力** 才优先考虑 {{BT}}refactor{{BT}}。
4. **纯文档更新** 才优先考虑 {{BT}}docs{{BT}}。

如果一次改动同时包含“用户可见新能力”和“内部重构”，应优先围绕**最核心的用户可见变化**选择 {{BT}}type{{BT}}。

## 通用规则

1. 只基于可见改动生成提交信息，不杜撰。
2. 使用 **Conventional Commits** 规范：
  - {{BT}}feat{{BT}}
  - {{BT}}fix{{BT}}
  - {{BT}}docs{{BT}}
  - {{BT}}refactor{{BT}}
  - {{BT}}test{{BT}}
  - {{BT}}chore{{BT}}
  - {{BT}}perf{{BT}}
  - {{BT}}build{{BT}}
  - {{BT}}ci{{BT}}
3. 标题格式：
  - {{BT}}<type>(<scope>): <summary>{{BT}}
  - 如果 scope 不明确，可省略 scope，使用 {{BT}}<type>: <summary>{{BT}}。
4. {{BT}}summary{{BT}} 使用英文，简洁明确，尽量不超过 50 个字符。
5. 优先描述本次改动的**核心行为变化**，不要机械罗列所有文件。
6. 如果主要是新增能力，优先用 {{BT}}feat{{BT}}。
7. 如果主要是修复问题，优先用 {{BT}}fix{{BT}}。
8. 如果主要是无行为变化的结构调整，优先用 {{BT}}refactor{{BT}}。
9. 如果主要是文档、说明、注释更新，优先用 {{BT}}docs{{BT}}。
10. 如果同时存在代码和文档改动，优先按代码主行为决定 type。
11. 如果改动新增了命令、子命令、prompt、规则文件、脚手架或发布工作流，且这些内容对用户直接可见，优先考虑 {{BT}}feat{{BT}}，不要轻易降级成 {{BT}}refactor{{BT}}。
12. 提交信息最终只能选择 1 条最优结果用于实际提交。

## scope 选择建议

优先根据当前仓库模块推断 scope，例如：

- {{BT}}copilot{{BT}}
- {{BT}}ggc{{BT}}
- {{BT}}changelog{{BT}}
- {{BT}}agentline{{BT}}
- {{BT}}ssh{{BT}}
- {{BT}}skills{{BT}}
- {{BT}}push{{BT}}
- {{BT}}commit{{BT}}
- {{BT}}docs{{BT}}

如果这些都不合适，再根据实际改动模块自行推断。

## 输出要求

如果成功提交并推送，请输出：

- {{BT}}mode:{{BT}} 说明本次基于 {{BT}}staged{{BT}} 还是 {{BT}}unstaged-auto-stage{{BT}}
- {{BT}}commit:{{BT}} 实际执行的 commit message
- {{BT}}hash:{{BT}} 实际生成的 commit hash（短 hash 即可）
- {{BT}}push:{{BT}} 推送目标或推送结果摘要
- {{BT}}reason:{{BT}} 用中文简短说明为什么这条提交信息最合适（1~2 句）

输出时必须遵守：

1. 只输出最终结果，不要展示分析过程。
2. 不要加标题，不要加 Markdown 段落说明，不要加“已完成 X 个步骤”。
3. 顶层字段固定使用：
  - {{BT}}mode:{{BT}}
  - {{BT}}commit:{{BT}}
  - {{BT}}hash:{{BT}}
  - {{BT}}push:{{BT}}
  - {{BT}}reason:{{BT}}
4. {{BT}}commit:{{BT}} 必须是**单行** commit message。
5. {{BT}}hash:{{BT}} 必须来自真实 {{BT}}git commit{{BT}} 结果。
6. {{BT}}push:{{BT}} 必须来自真实 {{BT}}git push{{BT}} 结果摘要。
7. {{BT}}reason:{{BT}} 只写 1~2 句中文，简洁即可。

如果当前没有任何可用改动，请只输出一段简短提示，说明：

- 当前没有 staged diff
- 当前也没有 unstaged diff（如果确实为空）
- 请先修改代码或执行 {{BT}}git add{{BT}}

如果提交失败，请输出简短失败结果，包含：

- {{BT}}mode:{{BT}}
- {{BT}}commit:{{BT}}
- {{BT}}error:{{BT}}

其中 {{BT}}error:{{BT}} 必须是真实报错摘要。

如果提交成功但推送失败，请输出简短失败结果，包含：

- {{BT}}mode:{{BT}}
- {{BT}}commit:{{BT}}
- {{BT}}hash:{{BT}}
- {{BT}}error:{{BT}}

其中 {{BT}}error:{{BT}} 必须是真实 push 报错摘要。

输出格式示例：

mode: staged
commit: feat(copilot): add interactive session commands
hash: a1b2c3d
push: pushed to origin/current-branch
reason: 本次改动核心是新增 Copilot 交互与会话能力，使用 feat 更准确，scope 选 copilot 更能概括主行为。

当没有 staged diff、但存在 unstaged diff 并自动暂存提交时，输出格式示例：

mode: unstaged-auto-stage
commit: feat(changelog): add prompt-based release workflow
hash: d4e5f6g
push: pushed to origin/current-branch
reason: 当前改动虽然尚未暂存，但核心变化包含用户可见的 changelog 工作流与 prompt 脚手架，因此优先使用 feat，并已自动完成本地提交。
`

	return strings.TrimSpace(strings.ReplaceAll(tpl, "{{BT}}", docsBacktick)) + "\n"
}

func renderDocumentationPromptTemplate() string {
	return fmt.Sprintf(`---
name: documentation
description: 维护 README/docs/example 文档，确保中文技术文风、结构一致、变更可追溯
argument-hint: "可选：补充要更新的文档范围、主题或目标读者"
agent: agent
---

你是当前仓库的文档维护助手。

## 目标

根据当前仓库真实改动，维护与同步以下文档内容：

- %sREADME.md%s
- %sdocs/**%s
- %sexample/**/README.md%s
- 其他与当前改动直接相关的 Markdown 文档

## 必读上下文

开始前优先读取：

- 当前变更涉及的文档文件
- 与改动直接相关的实现代码
- 如涉及行为变化，读取 %s.version/changelog/Unreleased.md%s

如果改动涉及架构、流程或命令面变化，优先检查：

- %sREADME.md%s
- %sdocs/DESIGN.md%s（如果存在）
- %sdocs/**%s 下对应专题文档

## 通用规则

1. 只基于当前仓库真实实现写作，不杜撰未实现能力。
2. 默认使用中文技术文风，表达简洁、可执行、可复现。
3. 优先使用二级/三级标题和短列表，避免大段空泛描述。
4. 流程、架构、关系图优先使用 Mermaid。
5. 避免在多个文档中复制粘贴同一段说明；优先引用单一事实来源。
6. 若只是措辞润色，不要改动技术语义与行为结论。
7. 描述命令时，优先使用仓库中真实存在的命令或任务名。

## 仓库约定

1. 如果当前仓库存在 %sdocs/INDEX.md%s，新增文档时应同步更新索引关系。
2. 涉及架构或流程变化时，优先更新 %sdocs/DESIGN.md%s（如果存在），再补 README / 示例 / 其他说明文档。
3. 用户可见行为变化，应同步更新 README 或对应示例文档。
4. 行为变更通常应同步 %s.version/changelog/Unreleased.md%s，但本 prompt 默认只修改文档文件；如确需修改 changelog，应单独说明。

## 输出要求

- 直接给出文档修改结果。
- 末尾附简短自检：
  - 是否仅基于真实改动更新文档；
  - 是否保持中文技术文风与结构化表达；
  - 是否已同步最关键的入口文档（README / DESIGN / 示例）。
  `, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick)
}

func renderDocumentationInstructionTemplate() string {
	return fmt.Sprintf(`---
name: 文档专项规范
description: 适用于仓库文档写作与维护（README/docs/example/internal），确保中文技术文风、结构一致、变更可追溯
applyTo: "**/*.md"
---

# 仓库文档专项规范

仅在“项目文档内容”场景生效（如 %sREADME.md%s、%sdocs/**%s、%sexample/**/README.md%s、%sinternal/**/README.md%s）。

## 基本要求

- 默认使用中文技术文风，表达简洁、可执行、可复现。
- 结构化写作：优先使用二级/三级标题与短列表，避免大段空泛描述。
- 流程、架构、关系图优先使用 Mermaid。
- 避免复制粘贴同一段说明到多个文档；优先“引用索引文档”或“链接到单一事实来源”。

## 仓库约定（必须遵循）

- 如果当前仓库存在 %sdocs/INDEX.md%s，新增文档时需补充索引关系（如适用）。
- 涉及架构或流程变化时，先更新 %sdocs/DESIGN.md%s（如存在），再补示例/说明文档。
- 行为变更需同步 %s.version/changelog/Unreleased.md%s；必要时同步其他评估或使用文档。
- 术语需与当前仓库现有命名保持一致，不擅自发明新概念。

## 写作与更新策略

- 面向“当前仓库真实实现”写作，不杜撰未实现能力。
- 描述命令时优先使用仓库中已存在的命令名与任务名。
- 变更文档时说明“改了什么、为什么改、影响范围”。
- 若仅做措辞润色，不应改动技术语义与行为结论。

## Changelog 联动

- 如涉及行为变化，建议同步更新 %s.version/changelog/Unreleased.md%s。
- 建议通过相关 prompt 执行 changelog 维护。
  `, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick, docsBacktick)
}
