---
name: Changelog 专项规范
description: 仅用于维护 .version/changelog，保证 Unreleased 与版本文件结构稳定、分类一致、条目可追溯
applyTo: ".version/changelog/*.md"
---

# Changelog 维护规范

本规则仅适用于 `.version/changelog/*.md`。

## 结构约束

- `Unreleased.md` 推荐分类：`新增` / `修复` / `变更` / `文档`。
- 若某分类暂无内容，写“暂无”。

## 内容约束

- 仅基于可见改动编写条目，不杜撰能力或影响。
- 单条应简洁、可读、可追溯，以动词开头。
- 重复事项需合并去重，避免同义重复。
- 非标准分类（如 `优化`、`重构`）必须归入标准四类（通常归 `变更`）。
- 不改写历史版本文件语义，不重排已发布版本。

## 落版约束（release）

- 版本号来源于 `.version/VERSION`。
- 落版文件：`.version/changelog/<VERSION>.md`。
- 文件头格式：`# [<VERSION>] - <YYYY-MM-DD>`。
- 落版前检查版本文件是否已存在，已存在时提示用户确认。
- 落版后重建 `.version/changelog/Unreleased.md` 模板（四个分类）。
- 落版后同步更新 `.version/changelog/README.md` 索引。

## 协同建议

- 建议通过 agent 提示词执行：`/changelog draft|release`。
