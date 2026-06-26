---
name: 发布前变更核对约束
description: "Use when preparing a release or completing behavior-impacting changes, including changelog updates and release regression checks."
---

# 发布前核对规则

用于“准备发布”或“完成具备行为影响的改动”时的统一核对。

## 发布前检查清单

- 变更说明已写入 `.version/changelog/Unreleased.md`，分类正确（新增/修复/变更/文档）。
- 用户可见行为变化，已同步示例或说明文档。

## 质量门槛

- 执行完整回归测试并确认通过。
- 仅基于真实改动与真实测试结果编写发布说明，不杜撰。

## 落版流程

- 首选通过 `fastgit changelog release` 或等效 agent prompt 执行。
- 版本号来源于 `.version/VERSION`；若版本文件已存在，需确认是否递增。
- 落版后重建 `Unreleased` 模板并更新 changelog 索引。
- changelog 结构与落版细节以 `.github/instructions/changelog.instructions.md` 为准。
