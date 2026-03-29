# auto-agentline-input

这个示例提供一个**独立 PTY 封装**，用于自动控制交互命令（例如 copilot）。

它不依赖 `copilotcmd` 包，而是直接启动子进程并通过 PTY 发送输入。

## 运行

默认执行：

- 命令：`go run . copilot`
- 不自动发送脚本
- 默认不进入 raw 接管（可控模式）
- 你的输入会按行转发给子进程

在仓库根目录执行：

`go run ./example/auto-agentline-input`

## 常用参数

- `-cmd`：要封装的交互命令
- `-script`：自动输入脚本，使用 `\n` 分隔（默认空）
- `-stdin`：是否直接进入 raw 接管（默认 false）
- `-line-input`：非 raw 模式下按行转发输入（默认 true）
- `-expect`：先等待输出出现某文本再发送脚本（可选）
- `-step-delay`：每步输入间隔
- `-timeout`：整体超时（默认 0，不超时）

示例：

`go run ./example/auto-agentline-input -cmd "go run . copilot"`

若你确实想启动后立刻接管为完整交互（raw 模式）：

`go run ./example/auto-agentline-input -cmd "go run . copilot" -stdin=true`

自动脚本示例：

`go run ./example/auto-agentline-input -cmd "go run . copilot" -expect "copilot>" -script "/help\n/quit"`