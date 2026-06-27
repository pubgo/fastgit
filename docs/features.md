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
| 变更记录     | `changelog`            | 初始化模板、草拟 Unreleased、发布落版            |
| 文档模板     | `docs init`            | 初始化文档 prompt/instruction 模板               |
| 同步拉取     | `pull`                 | 拉取当前分支，支持 `--all`、`--hard`             |
| 推送发布     | `push`                 | 推送当前分支，支持 `--all`、`--force-with-lease` |
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
- 支持 `--amend`、`--fast` 等模式

---

### 2.2 Changelog 流程（`fastgit changelog`）

子命令：

- `init`：初始化 `.version/changelog` 及仓库级模板
- `draft`：基于改动生成草稿，更新 `Unreleased.md`
- `release`：将 Unreleased 落版为版本文件并重建模板

适用场景：

- 发布前整理变更记录
- 团队统一 changelog 分类（新增/修复/变更/文档）

---

### 2.3 Copilot 会话（`fastgit copilot`）

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

### 2.4 统一 Git 命令面（`fastgit ggc`）

`ggc` 提供统一命令入口与交互检索：

- `ggc list`：查看命令面
- `ggc interactive`：fuzzy 选择 + workflow
- `ggc path`：查看状态文件位置

可将多步 Git 操作沉淀成 workflow/alias，适合高频重复动作。

---

### 2.5 工作树并行开发（`fastgit worktree`）

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

---

## 4. 配置与环境要点

### 配置文件

- 全局配置：`~/.config/fastgit/config.yaml`
- 全局环境模板：`~/.config/fastgit/env.yaml`
- 仓库本地覆盖：`<repo>/.git/fastgit.env`

### 常见环境变量

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `OPENAI_MODEL`
- `GITHUB_TOKEN`

---

## 5. 功能边界与注意事项

- 大部分 Git 行为依赖系统 `git` 命令，请确保本机可用。
- `copilot` 相关功能依赖 Copilot SDK/CLI 运行环境。
- `upgrade` 按当前 `GOOS/GOARCH` 过滤资产，不会跨平台安装。
- 部分命令是交互式设计（例如 `tag`、`ggc interactive`、`history`），在非 TTY 下不可用。

---

## 6. 你可以从这里继续

- 想理解内部执行链路：看 `docs/architecture.md`
- 想立刻上手命令：看仓库根 `README.md`
- 想维护文档模板：执行 `fastgit docs init`
