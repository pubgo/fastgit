# fastgit Copilot 指令

## 项目概览
- CLI 入口在 `main.go`，实际命令注册在 `bootstrap/boot.go`，使用 `redant` 命令树 + `dix` 依赖注入。
- 运行前会 `initConfig()`：配置写入 `~/.config/fastgit/config.yaml`，环境模板写入 `~/.config/fastgit/env.yaml`，本地覆盖写入 `.git/fastgit.env`（见 `configs/config.go`）。
- AI 生成提交信息：`cmds/fastcommitcmd/cmd.go` 调用 `utils.GeneratePrompt()`（`utils/prompts.go`），通过 `go-openai` 发起 ChatCompletion。

## 关键流程/数据流
- `bootstrap/boot.go` 中间件：检测交互式终端 → 初始化配置 → DI 注入 `OpenaiClient` 和配置。
- `commit` 命令默认生成 **conventional** 格式（`<type>(<scope>): <message>`）且长度 ≤ 50（见 `utils/prompts.go`）。
- `tag` 命令支持自动计算 semver（`utils/util.go`）并可更新 `.version/VERSION`。

## 工作流（常用命令）
- 运行：`go install .` 或 `go run .`（入口 `main.go`）。
- 初始化配置：`fastgit init`（生成 `~/.config/fastgit/*` 与 `.git/fastgit.env`）。
- 测试/质量：`task test`、`task vet`、`task lint`（见 `Taskfile.yml`）。
- 本地配置编辑：`fastgit config edit [config|env|local]`。

## 代码约定与模式
- 命令实现放在 `cmds/*/cmd.go`，返回 `*redant.Command`；新增命令时在 `bootstrap/Main()` 注册。
- git 操作优先走 `utils/*`（例如 `GetStagedDiff`, `GitPull`, `GitPush`），保持日志/错误处理一致。
- 需要 OpenAI 配置时从 `OpenaiConfig` 注入（`utils/openai.go`），不要在命令内直接读环境变量。

## 外部依赖与集成点
- OpenAI 兼容接口（默认 DeepSeek）：`OPENAI_API_KEY/OPENAI_BASE_URL/OPENAI_MODEL`（见 `configs/env.yaml`）。
- GitHub Release 升级逻辑在 `cmds/upgradecmd`，使用 `utils/githubclient` 获取资源。
