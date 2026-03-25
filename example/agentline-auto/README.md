# agentline-auto 示例

用于验证“command 既可作为普通 CLI 命令，也可在交互模式中作为 slash command 执行”的双模式行为。

## 运行方式

- 直接运行普通命令（不会进入 agentline）：
  - `go run example/agentline-auto/main.go status`
- 运行带 `agent.command=true` 的命令（保持普通 CLI 行为）：
  - `go run example/agentline-auto/main.go commit --message "feat: normal"`
- 进入交互模式并把命令当作 slash command 执行：
  - `go run example/agentline-auto/main.go agentline`
  - 在交互里先输入 `/`（或按 `Tab`）可看到包含 `/commit` 的候选列表
  - 在交互里输入：`/commit --message "feat: from slash"`

## 验证点

- `commit` 命令带有 metadata：`agent.command=true`，用于交互模式下的 agent command 识别。
- 普通 CLI 调用 `commit` 时会直接执行，不强制进入 `agentline`。
- 在 agentline 中可直接使用 `/<command ...>`（例如 `/commit --message hi`）执行命令。
