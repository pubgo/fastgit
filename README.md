# fastgit
agentic git commit generate tool

## Documentation

- 文档索引：`docs/INDEX.md`
- 架构文档：`docs/architecture.md`
- 功能文档：`docs/features.md`
- 路线图文档：`docs/roadmap.md`
- Copilot DCOS：`docs/copilot-dcos.md`

## Desktop Client (Wails v3)

- 桌面端代码位于：`desktop/`
- 该子项目不会影响现有 CLI（`main.go` / `bootstrap.Main()` 保持不变）

快速启动：

```bash
cd desktop
go run github.com/wailsapp/wails/v3/cmd/wails3@latest generate bindings -clean=true -ts -i
cd frontend && npm install && npm run build
cd ..
go run .
```

如果本机已安装 `wails3`，可直接在 `desktop/` 下执行：

```bash
wails3 dev
```

## Command Overview
- `fastgit commit`: AI 提交流程（保留原行为）
- `fastgit commit ai`: AI 提交流程显式入口（新增）
- `fastgit changelog init`: 初始化 `.version/changelog` 模板，适配任意项目仓库
- `fastgit changelog draft`: 使用 Copilot 根据当前改动更新 `Unreleased.md`
- `fastgit changelog release`: 将 `Unreleased.md` 落版为版本文件，并可同步推进 `.version/VERSION`
- `fastgit docs init`: 初始化文档维护用的 prompt / instruction 模板
- `fastgit pull`: 拉取当前分支（支持 `--all`）
- `fastgit pull --hard`: 强制与远端同步（`fetch + reset --hard`）
- `fastgit push`: 推送当前分支（支持 `--all` / `--force`）
- `fastgit worktree`: 列出当前仓库 worktree
- `fastgit worktree create <issue|branch> [--base <branch>]`: 创建 worktree
- `fastgit worktree remove <issue|branch>`: 删除 worktree
- `fastgit worktree remove --path <worktree-path>`: 按路径删除 worktree
- `fastgit ggc list`: 查看统一命令面（ggc 风格）
- `fastgit ggc <command ...>`: 执行统一命令，例如 `fastgit ggc status short`
- `fastgit ggc` / `fastgit ggc interactive`: 进入交互模式（增量搜索 + workflow）
- `fastgit ggc path`: 查看当前 `ggc.yaml` 的实际路径（按 OS/XDG 规则）

## Repo Prompt Templates

`fastgit` 现在支持为仓库初始化一组可直接复用的 Copilot prompt / instruction 模板，适合把常用工作流沉淀到项目里，而不是只存在聊天上下文中。

### Changelog 模板

- 初始化：`fastgit changelog init`
- 生成后会创建：
	- `.version/changelog/*`
	- `.github/prompts/changelog.prompt.md`
	- `.github/instructions/changelog.instructions.md`
	- `.github/instructions/release.instructions.md`

适合场景：

- 维护 `Unreleased.md`
- 准备版本落版
- 统一 changelog 分类与发布流程

### Documentation 模板

- 初始化：`fastgit docs init`
- 生成后会创建：
	- `.github/prompts/documentation.prompt.md`
	- `.github/prompts/commit-message.prompt.md`
	- `.github/instructions/documentation.instructions.md`

适合场景：

- 更新 `README.md`
- 同步 `docs/**`
- 维护 `example/**/README.md`
- 生成或沉淀提交信息 / 提交流程 prompt 模板
- 让文档写作遵循统一中文技术文风与结构规范

### Commit Prompt 模板

仓库中还可以维护提交辅助 prompt，例如：

- `.github/prompts/commit-message.prompt.md`

当前这类 prompt 可用于：

- 基于 staged / working tree 改动生成提交信息
- 按模板约束执行本地提交
- 按模板约束继续推送到远程（如果 prompt 明确要求）

> 建议：把“生成建议”和“执行提交”区分成不同 prompt，便于在不同风险场景下选择更稳妥的工作流。

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
