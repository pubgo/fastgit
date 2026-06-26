---
name: changelog
description: 维护 .version/changelog（更新 Unreleased 或执行版本落版）
argument-hint: "模式：draft（更新 Unreleased）或 release（按 .version/VERSION 落版）"
agent: agent
---

你是当前仓库的 Changelog 维护助手。

## 目标

- `draft` 模式：根据当前改动更新 `.version/changelog/Unreleased.md`。
- `release` 模式：将 `Unreleased.md` 落版为版本文件，并重建空模板。

## 必读上下文

在开始前先读取：

- `.version/changelog/Unreleased.md`
- `.version/VERSION`

## 通用规则

1. 只基于可见改动生成条目，不杜撰。
2. **标准分类**：`新增` / `修复` / `变更` / `文档`。
   - 非标准分类（如 `优化`、`重构`）必须归入上述四类（通常归 `变更`）。
3. 语言使用中文技术文风，单条以动词开头，简洁可追溯。
4. 去重：同类项合并，避免语义重复。
5. 不改写历史版本文件语义与顺序。

## draft 模式

1. 获取工作区 diff：运行 `git diff <base> --stat` 和 `git diff <base> --name-only` 确认改动范围。
2. 仅更新 `.version/changelog/Unreleased.md`。
3. 若缺少分类小节则补齐；无内容的小节写“暂无”。
4. 归类规则：
   - feat / 新增能力 → `新增`
   - fix / bug 修正 → `修复`
   - 重构、依赖迁移、行为调整、优化 → `变更`
   - README、docs、注释更新 → `文档`

## release 模式

1. 读取 `.version/VERSION` 获取目标版本号（如 `v0.3.0`）。
2. **版本冲突检查**：若 `.version/changelog/<VERSION>.md` 已存在，提示用户确认是否需要递增版本号，不自行覆盖。
3. 创建版本文件 `.version/changelog/<VERSION>.md`：
   - 标题格式：`# [<VERSION>] - <YYYY-MM-DD>`。
   - 将 `Unreleased.md` 的内容迁移过去（分类统一为标准四类）。
4. 重建 `Unreleased.md` 空模板（四个分类均写“暂无”）。
5. 更新 `.version/changelog/README.md` 索引：在列表顶部（`Unreleased` 之后）插入新版本链接。
6. 更新 `.version/VERSION` 为下一个预期版本号（**仅在用户确认后**，否则保持当前值）。

## 输出要求

- 直接给出文件修改结果。
- 末尾附一段简短自检：
  - 是否仅改动 `.version/` 范围内的文件；
  - 分类是否统一为标准四类，是否完成去重；
  - 历史版本文件是否未被修改。
