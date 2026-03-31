---
name: generic-sample
description: "Use when: 需要通用问题处理模板（分析、实现、验证）时。"
"argument-hint": "输入任务目标、约束、涉及模块与预期输出。"
metadata:
  summary: "通用执行技能模板，强调最小变更、可验证与明确输出。"
  version: "1.0.0"
  tags: ["template", "execution", "copilot"]
  use_when:
    - "任务尚未有专用 skill"
    - "需要统一的分析-实现-验证流程"
  tools: ["semantic_search", "grep_search", "read_file", "get_errors", "runTests"]
---

# Generic Execution Template

## 目标
- 将自然语言需求转为可执行、可验证的改动。

## 执行步骤
1. 明确目标与边界（输入/输出/约束/非目标）。
2. 收集代码证据（入口、实现点、测试点）。
3. 制定最小改动计划并分步实施。
4. 每步改动后执行检查与测试。
5. 输出结果、风险和后续建议。

## 约束
- 不编造事实，不省略关键假设。
- 避免一次性大改；优先小步可回归。
- 涉及接口变更时说明兼容策略。

## 输出契约
- 变更摘要（文件 + 目的）
- 验证结果（测试/检查）
- 已知风险与后续动作
