# Copilot 融合开发 DCOS 落地清单

> DCOS 这里采用：**Design（设计）- Code（开发）- Operate（运行）- Scale（扩展）**。

## D - Design（设计）

1. 统一入口
   - 正式入口：`fastgit copilot`
   - 示例入口：`example/copilot-demo` 仅作为薄封装，直接调用 `cmds/copilotcmd.New()`。
2. 统一交互层
   - 默认 `copilot` 根命令进入 `agentline` 交互模式。
   - 子命令（`chat/resume/sessions/status/models`）支持直接 CLI 调用与 `/slash` 调用。
3. 统一会话运行时
   - 在 `cmds/copilotcmd` 内维护单进程 runtime：client 复用 + session 缓存。
4. 风险约束
   - 当前权限策略使用 `ApproveAll`（仅 MVP）。
   - 后续需要替换为策略化审批（allow/deny/ask）。

## C - Code（开发）

### 已完成（MVP）

- 新增 `cmds/copilotcmd/cmd.go`
  - 提供命令：`chat`、`resume`、`sessions`、`status`、`models`、`interactive-demo`
  - 集成 `agentlineapp.Run` 作为默认交互入口。
   - 已支持高级配置：`ConfigDir`、`SystemMessage`、`Skills`、`MCP`、`CustomAgents`、`CustomTools`。
- 修改 `bootstrap/boot.go`
  - 注册 `copilotcmd.New()`，接入主程序命令树。
- 修改 `example/copilot-demo/main.go`
  - 简化为复用 `cmds/copilotcmd.New()`，防止示例与正式实现分叉。

### 下一步（P1）

1. 事件管理完善
   - 补回 `events` 命令（timeline/summary/jsonl 导出）。
2. 权限策略中心化
   - 增加 `--permission-mode=ask|allow|deny`，并与 agentline `/permissions` 打通。
3. 配置体系对齐
   - 接入 `configs` + `dix`，减少命令参数重复传递。
4. 会话持久化
   - 追加本地存储（jsonl/sqlite）与恢复索引。

## O - Operate（运行）

1. 配置环境变量
   - 项目根目录 `.env` 维护 `GITHUB_TOKEN`。
2. 基础验证流程
   - `fastgit copilot status`
   - `fastgit copilot chat --prompt "hello"`
   - `fastgit copilot sessions`
3. 交互模式验证
   - `fastgit copilot`
   - 在交互中执行：`/chat --prompt "帮我总结当前仓库"`

## S - Scale（扩展）

1. 分层重构
   - `pkg/copilotruntime`: client/session 生命周期。
   - `pkg/copilotflow`: 事件渲染与导出。
2. 质量保障
   - 单测：runtime 生命周期、session 缓存失效恢复。
   - 集成测试：`chat -> resume -> sessions` 闭环。
3. 安全与可观测
   - 权限审计日志、事件追踪 ID、错误分类与退出码。
