# fastgit
agentic git commit generate tool

## Command Overview
- `fastgit commit`: AI 提交流程（保留原行为）
- `fastgit commit ai`: AI 提交流程显式入口（新增）
- `fastgit changelog init`: 初始化 `.version/changelog` 模板，适配任意项目仓库
- `fastgit changelog draft`: 使用 Copilot 根据当前改动更新 `Unreleased.md`
- `fastgit changelog release`: 将 `Unreleased.md` 落版为版本文件，并可同步推进 `.version/VERSION`
- `fastgit pull`: 拉取当前分支（支持 `--all`）
- `fastgit push`: 推送当前分支（支持 `--all` / `--force`）
- `fastgit ggc list`: 查看统一命令面（ggc 风格）
- `fastgit ggc <command ...>`: 执行统一命令，例如 `fastgit ggc status short`
- `fastgit ggc` / `fastgit ggc interactive`: 进入交互模式（增量搜索 + workflow）
- `fastgit ggc path`: 查看当前 `ggc.yaml` 的实际路径（按 OS/XDG 规则）

## New ggc-style command surface (phase 1)

- `fastgit ggc status|status short`
- `fastgit ggc add <file|.>`
- `fastgit ggc commit <message>`
- `fastgit ggc log simple|graph`
- `fastgit ggc diff|diff staged|diff unstaged`
- `fastgit ggc branch current|list local|list remote|checkout <name>|checkout remote <name>|create <name>|delete <name>`
- `fastgit ggc fetch|fetch prune`
- `fastgit ggc pull current|pull rebase`
- `fastgit ggc push current|push force`
- `fastgit ggc rebase <upstream>|continue|abort|skip`
- `fastgit ggc tag list|show <tag>`
- `fastgit ggc remote list`

## Interactive Mode (phase 2 - MVP)

- 搜索模式（默认）
	- 输入字符：实时 fuzzy 过滤命令
	- `↑/↓` 或 `Ctrl+N/P`：移动选中
	- `Enter`：执行当前命令
	- `Tab`：将当前命令加入当前 workflow
	- `Ctrl+T`：切换到 workflow 模式
	- `Ctrl+C`：退出
- workflow 模式
	- `n`：新建 workflow
	- `d` / `Ctrl+D`：删除当前 workflow
	- `c`：清空当前 workflow
	- `Ctrl+N/P`：切换 workflow
	- `x` 或 `Enter`：执行当前 workflow
	- `Ctrl+T`：返回搜索模式

> 对于带占位参数的命令（如 `<name>`），执行时会自动提示输入参数。

## Phase 3: Workflow 持久化 + Alias

- workflow 会持久化到：`<XDG 配置目录>/fastgit/ggc.yaml`
	- macOS 常见为：`~/Library/Application Support/fastgit/ggc.yaml`
	- Linux 常见为：`~/.config/fastgit/ggc.yaml`
- 每次进入 `fastgit ggc` 交互模式会自动加载上次 workflow
- 在交互模式里对 workflow 的新增/删除/清空会自动保存

### Alias 配置

`ggc.yaml` 支持两种 alias：

- 单条 alias（字符串）
- 序列 alias（字符串数组，按顺序执行）

示例：

```yaml
aliases:
	st: "status short"
	ci: "commit {0}"
	quick:
		- "status"
		- "add ."
		- "commit {0}"
```

说明：

- `{0}`、`{1}`... 表示位置参数
- 例如：`fastgit ggc ci "fix typo"`
- 例如：`fastgit ggc quick "chore: update"`
- `fastgit ggc list` 会同时显示内置命令与 alias

## Refer
- https://github.com/Nutlope/aicommits
- https://chat.deepseek.com

## ENV
- OPENAI_API_KEY
- OPENAI_BASE_URL, default: https://api.deepseek.com/v1
- OPENAI_MODEL, default: deepseek-chat
