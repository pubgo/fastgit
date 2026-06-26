# Project Skills

这个目录用于存放项目级 Skills。

建议结构：

- `skills/<skill-name>/SKILL.md`

示例：

- `skills/repo-context/SKILL.md`

推荐 frontmatter（兼容 schema）：

- `name`
- `description`
- `argument-hint`
- `metadata`（将扩展字段放到这里，如 `summary/version/tags/use_when/tools`）

推荐正文结构：

1. `# <Skill 标题>`
2. `## 目标`
3. `## 执行步骤`
4. `## 约束`
5. `## 输出契约`

实践建议：

- 任务相关的“何时使用”写在 `description` 与 `metadata.use_when`。
- 工具边界写在 `metadata.tools`，避免技能行为漂移。
- 保持最小可执行流程，减少抽象描述。
