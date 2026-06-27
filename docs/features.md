# fastgit 功能文档

## 文档目标

本文档聚焦“能做什么、怎么用、适合什么场景”，用于：

- 快速了解 `fastgit` 的命令能力边界
- 按实际开发场景选择命令组合
- 减少阅读源码前的摸索成本

> 架构细节请配合阅读：`docs/architecture.md`

---

## 1. 命令总览（按能力域）

| 能力域       | 命令组                 | 主要用途                                         |
| ------------ | ---------------------- | ------------------------------------------------ |
| 基础信息     | `version`              | 查看构建版本、commit、构建时间、设备标识         |
| 配置初始化   | `init`                 | 初始化全局配置、环境模板、仓库本地 env           |
| 配置管理     | `config`               | 编辑/查看 `config`、`env`、`local env`           |
| AI 提交      | `commit` / `commit ai` | 基于 diff 生成提交信息并辅助提交                 |
| 质量门禁     | `check`                | fmt/vet/test/lint/secret 一键检查，支持 hook     |
| PR 流程      | `pr`                   | create/status/sync/merge，依赖 gh CLI            |
| 冲突处理     | `conflict`             | 冲突分组摘要、列表、打开文件                     |
| 团队治理     | `team`                 | 初始化/校验 `.fastgit` 仓库规则                  |
| 本地评审     | `review`               | staged diff 结构化 review（AI + fallback）       |
| 变更记录     | `changelog`            | 初始化模板、草拟 Unreleased、发布落版            |
| 文档模板     | `docs init`            | 初始化文档 prompt/instruction 模板               |
| 同步拉取     | `pull`                 | 拉取当前分支，支持 `--all`、`--hard`             |
| 推送发布     | `push`                 | 推送当前分支；保护分支策略阻断；`--override-policy` |
| 标签发布     | `tag`                  | 生成并推送 tag，支持列表与交互选择               |
| 工作树       | `worktree`             | 创建/删除/查看多工作树并行开发                   |
| 统一命令面   | `ggc`                  | 统一 git 子命令 + 交互 workflow + alias          |
| Copilot 集成 | `copilot`              | 会话聊天、恢复、诊断、模型/skills 管理           |
| 自升级       | `upgrade`              | 查询并下载匹配当前 OS/ARCH 的发布版本            |
| 其他工具     | `ssh-login`、`history` | SSH 二次认证登录、历史命令交互处理               |

---

## 2. 高频功能说明

### 2.1 AI 提交流程（`fastgit commit`）

典型用途：

- 想快速生成规范化提交信息
- 希望 commit message 保持 conventional 风格

关键特性：

- 提示词由 `utils.GeneratePrompt()` 统一生成
- 默认限制提交信息风格与长度
- 支持 `--amend`、`--fast`、`--candidates`、`--skip-check`、`--skip-policy`、`--override-policy`
- `.fastgit/commit.yaml` 可设 `candidates_default: true` 默认三选一
- 提交前默认运行 `check run --staged-only`（可用 `--skip-check` 跳过）
- `.fastgit/policy.yaml` 中 `enforce: true` 时，分支名/commit message 违规将阻断提交
- 读取 `.fastgit/commit.yaml`（locale、max_length、require_scope）
- push 前校验 `.fastgit/policy.yaml` 保护分支
- 完成后推荐下一步（如 `push` → `pr create`）

---

### 2.2 质量门禁（`fastgit check`）

子命令：

- `run`：执行 fmt / vet / test / lint / secrets 流水线
- `run --dry-run`：预览将执行的步骤，不改动仓库
- `run --staged-only`：仅检查 staged 文件（fmt 限定到 staged `.go`）
- `run --fix`：对可修复项先修复（如 `gofmt -w`）
- `config`：展示当前门禁步骤
- `hook install|uninstall`：安装/卸载 pre-commit（staged check）与 pre-push（全量 check）
- 配置：`.fastgit/check.yaml` 自定义 steps；`team init` 会生成模板
- 非 fastgit 管理的已有钩子需 `--force` 覆盖；与 lefthook 等请只保留一套 pre-commit

适用场景：

- commit 前本地自检
- 与 CI 对齐的本地门禁

---

### 2.3 Pull Request 流程（`fastgit pr`）

子命令：

- `create`：从 git log/diff 生成 PR 标题与正文（Summary / Risk / Test plan / Rollback）
- `create --dry-run`：只预览，不调用 `gh`
- `create --ai`：用 AI 润色标题与正文（失败时保留规则版）
- `create --ai-provider=auto|openai|copilot`：选择 AI 提供方
- `create --review`：将 `base..HEAD` 本地 review 摘要写入 Test plan
- `status`：查看当前分支 PR 状态（需 `gh`）
- `sync`：rebase 到 base 并 `push --force-with-lease`
- `sync --update-body`：sync 后重新生成并更新 PR 正文
- `sync --update-body --ai`：更新时用 AI 润色
- `sync --update-body --review`：更新时合并本地 review 到 Test plan
- `merge`：合并 PR（默认 squash，交互确认，`--yes` 跳过）

依赖：`gh` CLI 已安装并登录；分支需有 upstream。

---

### 2.4 冲突助手（`fastgit conflict`）

子命令：

- `summary`（默认）：按模块分组输出冲突文件与处理建议
- `summary --ai`：AI 分析冲突原因（失败时保留启发式建议）
- `list`：列出冲突文件
- `open`：在 `$EDITOR` 中打开全部冲突文件

`pull` / `commit` 检测到冲突时也会自动输出摘要。

---

### 2.5 团队治理（`fastgit team`）

仓库级配置目录：`.fastgit/`

- `team init`：生成 `policy.yaml` + `commit.yaml` 模板
- `team validate --branch` / `--message`：校验分支名与 commit message

`policy.yaml` 控制：分支命名、保护分支、conventional commit、敏感路径。  
`commit.yaml` 控制：AI commit 的 locale、长度、scope 要求。

`check` / `commit` / `pr create` 会读取这些规则并给出 warning。  
`push` 与 `commit` 对 `protected_branches`（如 main/master）硬阻断，可用 `--override-policy` 跳过。

---

### 2.6 本地代码评审（`fastgit review`）

- `review staged`：对 staged diff 输出 Blockers / Suggestions / Nits / Test plan
- `review staged --dry-run`：预览将评审的内容
- `review staged --ai-provider=auto|openai|copilot`：选择 AI 提供方（不可用时规则 fallback）

适用场景：PR 创建前自检、敏感改动二次确认。

---

### 2.7 Changelog 流程（`fastgit changelog`）

子命令：

- `init`：初始化 `.version/changelog` 及仓库级模板
- `draft`：Copilot 更新 Unreleased.md
- `draft --enrich`：规则引擎预填「影响范围 / 验证建议 / 回滚建议」
- `release`：落版并重建 Unreleased 模板
- `release --skip-validate`：跳过 meta 小节完整性校验
- `release --skip-bump-check`：跳过 bump 与变更类型一致性校验

适用场景：

- 发布前整理变更记录
- 团队统一 changelog 分类（新增/修复/变更/文档）

---

### 2.8 Copilot 会话（`fastgit copilot`）

支持能力：

- `chat`：创建/复用会话并发送 prompt
- `resume`：恢复历史会话继续对话
- `sessions`：列会话并可 hydrate 摘要
- `status`、`models`：连接与模型可用性检查
- `doctor`、`inspect`：配置诊断与生效摘要
- `skills`：技能发现、读取、创建、加载

特点：

- 默认进入 `agentline` 交互模式
- 会话 runtime 在进程内复用，减少重复初始化开销

---

### 2.9 统一 Git 命令面（`fastgit ggc`）

`ggc` 提供统一命令入口与交互检索：

- `ggc list`：查看命令面
- `ggc interactive`：fuzzy 选择 + workflow，底部展示 `Next:` 推荐链
- `ggc path`：查看状态文件位置

可将多步 Git 操作沉淀成 workflow/alias，适合高频重复动作。

---

### 2.10 工作树并行开发（`fastgit worktree`）

子命令：

- `worktree list`
- `worktree create <issue|branch> [--base main]`
- `worktree remove <issue|branch>`
- `worktree remove --path <worktree-path>`

适用场景：

- 多需求并行、隔离开发上下文
- 同仓库多分支同时调试

---

## 3. 典型场景工作流

### 场景 A：日常提交流程

1. `fastgit pull`
2. 本地修改并暂存
3. `fastgit commit`（或 `fastgit commit ai`）
4. `fastgit push`

### 场景 B：发布前 changelog 落版

1. `fastgit changelog draft`
2. 人工检查 `Unreleased.md`
3. `fastgit changelog release --bump patch`
4. `fastgit tag`（按团队流程执行）

### 场景 C：并行开发

1. `fastgit worktree create 123 --base main`
2. 在新 worktree 开发与提交
3. `fastgit worktree list` 检查状态
4. 完成后 `fastgit worktree remove 123`

### 场景 D：本地改动到 PR 闭环

1. `fastgit team init`（首次，下发 `.fastgit/` 规则）
2. `fastgit check run --staged-only`（提交前自检）
3. `fastgit review staged`（可选，AI 自检）
4. `fastgit commit`（默认跑 check + 策略校验）
5. `fastgit pr create --review`（review 摘要写入 Test plan）
6. `fastgit pr merge`（交互确认后合并）

### 场景 E：质量门禁常驻

1. `fastgit check hook install`（安装 pre-commit + pre-push）
2. 日常 `git commit` 自动触发 staged check
3. `git push` 自动触发全量 check
4. 与 lefthook/husky 共存时，避免重复 pre-commit；非 fastgit 钩子用 `--force` 覆盖

---

## 4. 配置与环境要点

### 配置文件

- 全局配置：`~/.config/fastgit/config.yaml`
- 全局环境模板：`~/.config/fastgit/env.yaml`
- 仓库本地覆盖：`<repo>/.git/fastgit.env`
- 团队规则：`<repo>/.fastgit/policy.yaml`、`commit.yaml`、`check.yaml`（`fastgit team init` 生成）

合并优先级：CLI flag > 仓库 `.fastgit/` > 本地 env > 全局配置 > 内置默认。

### 常见环境变量

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `OPENAI_MODEL`
- `GITHUB_TOKEN`
- `FASTGIT_AI_CACHE`：设为 `1` 启用 diff 摘要缓存（`~/.config/fastgit/ai-cache/`）

---

## 5. 功能边界与注意事项

- 大部分 Git 行为依赖系统 `git` 命令，请确保本机可用。
- `copilot` 相关功能依赖 Copilot SDK/CLI 运行环境。
- `pr` 命令族依赖 `gh` CLI 已安装并登录，且分支需有 upstream。
- AI 能力不可用时，`commit`/`pr`/`review`/`conflict` 自动降级为规则版输出。
- `upgrade` 按当前 `GOOS/GOARCH` 过滤资产，不会跨平台安装。
- 部分命令是交互式设计（例如 `tag`、`ggc interactive`、`history`、`pr merge`），在非 TTY 下不可用。

---

## 6. 你可以从这里继续

- 想理解内部执行链路：看 `docs/architecture.md`
- 想立刻上手命令：看仓库根 `README.md`
- 想维护文档模板：执行 `fastgit docs init`
