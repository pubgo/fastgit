# Local Copilot Skills

这个目录用于存放仅本仓库使用的 Copilot Skills。

建议每个技能单独目录，文件名使用 `SKILL.md`。

建议模板：

- frontmatter：`name`、`description`、`argument-hint`、`metadata`
- 正文：`目标`、`执行步骤`、`约束`、`输出契约`

说明：

- `metadata` 中可扩展 `summary/version/tags/use_when/tools`。
- 顶层 frontmatter 请使用受支持字段，避免解析报错。
