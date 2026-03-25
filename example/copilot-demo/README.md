# copilot-demo 示例

用于演示如何在 `redant` 中通过 `github/copilot-sdk/go` 复用 Copilot CLI 能力，并与 `agentline` 交互模式打通。

## 功能覆盖

- 连接与健康检查：`status`（包含 `Ping`、`GetStatus`、`GetAuthStatus`）
- 新会话对话：`chat`
- 恢复会话：`resume`
- 只读查看会话事件：`events`
- 会话管理：`sessions`、`last-session`、`delete-session`
- 模型发现：`models`
- 可视化控制台：`web`（集成 `cmds/webcmd`）
- ACP 权限回合演示：`acp-turn`（支持 allow/deny/cancel 三种决策）
- 双向交互协议演示：`interactive-demo`（通过 interaction bridge 执行 Emit + Ask）
- SDK 能力演示：
  - `OnPermissionRequest`（默认 `ApproveAll`）
  - `OnUserInputRequest`（自动回答）
  - `SessionHooks`（start/end）
  - 自定义 Tool（`demo_echo`）
  - 流式输出（`--stream`）

## 快速运行

- 查看状态：
  - `go run example/copilot-demo/main.go status`
- 新建会话并对话：
  - `go run example/copilot-demo/main.go chat --prompt "解释一下 redant 的命令分发"`
- 使用 `chat` 继续指定会话（提供 `session-id` 时自动进入 resume 模式）：
  - `go run example/copilot-demo/main.go chat --session-id <SESSION_ID> --prompt "继续"`
  - `go run example/copilot-demo/main.go chat --session-id <SESSION_ID> --prompt "继续" --dump-events`
- 恢复会话继续对话：
  - `go run example/copilot-demo/main.go resume --session-id <SESSION_ID> --prompt "继续"`
  - `go run example/copilot-demo/main.go resume --session-id <SESSION_ID> --prompt "继续" --dump-events`
  - `go run example/copilot-demo/main.go resume --session-id <SESSION_ID> --prompt "继续" --dump-events --events-limit 200 --events-raw`
- 只读查看会话事件（不发送 prompt）：
  - `go run example/copilot-demo/main.go events --session-id <SESSION_ID>`
  - `go run example/copilot-demo/main.go events --session-id <SESSION_ID> --events-limit 200 --events-raw`
  - `go run example/copilot-demo/main.go events --session-id <SESSION_ID> --events-out data.jsonl --events-view timeline`
  - `go run example/copilot-demo/main.go events --session-id <SESSION_ID> --events-out data.jsonl --events-view summary`

> 说明：`--events-out` 默认为 `data.jsonl`（JSONL 逐行记录），便于后续 grep/jq 分析；`--events-view` 用于控制终端展示样式：`timeline`（默认，适合回放全过程）/`summary`（仅统计）/`none`（只导出文件）。
- 列出会话：
  - `go run example/copilot-demo/main.go sessions`
- 补全会话摘要（恢复会话并抓最近消息）：
  - `go run example/copilot-demo/main.go sessions --hydrate`
  - `go run example/copilot-demo/main.go sessions --hydrate --hydrate-timeout 6s --hydrate-max-events 120`

> 说明：若你看到会话列表大多只有 `id`，通常是 Copilot CLI 当前仅返回了 `sessionId`，未返回 `summary/start/modified/context`。示例会在这种情况下显示 `meta=empty` 并给出提示信息。

启用 `--hydrate` 后，示例会尝试短时恢复每个会话并读取最近事件，从而补充 `hydrate.assistant`（最近一条 assistant 消息摘要）与 `hydrate.messages`（事件总数）。
- 模型列表：
  - `go run example/copilot-demo/main.go models`
- 启动可视化页面（默认会自动打开浏览器）：
  - `go run example/copilot-demo/main.go web`
- 指定地址并禁用自动打开浏览器：
  - `go run example/copilot-demo/main.go web --addr 127.0.0.1:18080 --open=false`
- 运行一次 ACP 权限回合（显式命令入口）：
  - `go run example/copilot-demo/main.go acp-turn --prompt "请修改一个文件"`
  - `go run example/copilot-demo/main.go acp-turn --permission-decision deny`
  - `go run example/copilot-demo/main.go acp-turn --permission-decision cancel`
- 在 agentline 中演示双向交互协议（推荐）：
  - `go run example/copilot-demo/main.go agentline`
  - 在交互里执行：`/interactive-demo --question "是否继续执行下一步？"`
  - 查看并回复待答问题：`/questions`，然后 `/reply 1 继续`（或 `/skip 1`）

## 与 agentline 联动

- 启动交互：
  - `go run example/copilot-demo/main.go agentline`
- 启动交互并自动恢复会话：
  - `go run example/copilot-demo/main.go agentline --resume-session-id <SESSION_ID>`
  - `go run example/copilot-demo/main.go agentline --resume-session-id <SESSION_ID> --resume-prompt "继续修复这个问题"`
- 在交互中执行 slash 命令：
  - `/chat --prompt "给我一个 Go CLI 设计建议"`
  - `/resume --session-id <SESSION_ID> --prompt "继续"`
  - `/interactive-demo --question "是否继续执行下一步？"`
  - `/acp-demo --prompt "请执行一次需要权限确认的操作"`
  - `/permissions`（查看待审批请求）
  - `/allow 1` / `/deny 1`（对指定请求决策）
  - `/questions`、`/reply 1 <answer>`、`/skip 1`（处理运行中问答）

`chat` 与 `resume` 已标记 `agent.command=true`，可作为 slash command 使用。

`sessions` 也已标记 `agent.command=true`，可在交互中使用：`/sessions`、`/sessions --hydrate`。

## 常用参数

- `--copilot-token`：显式传入 GitHub Token（也支持环境变量 `GITHUB_TOKEN`）
- `--copilot-cli-path`：指定 Copilot CLI 可执行路径
- `--model`：指定模型（默认 `gpt-5`）
- `--reasoning-effort`：推理强度（`low/medium/high/xhigh`）
- `--system-message`：追加系统提示词
- `--stream`：启用流式输出
