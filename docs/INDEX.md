# fastgit 文档索引

## 阅读入口

- `architecture.md`：项目架构、启动流程、分层职责、关键数据流
- `features.md`：功能总览、命令分组、场景化使用指南
- `roadmap.md`：功能路线图、Issue 拆分建议、验收标准（DoD）
- `desktop-status.md`：桌面端（Wails v3）开发状态、进度快照与回归清单
- `copilot-dcos.md`：Copilot 融合开发的 DCOS 落地清单

团队仓库规则见 `fastgit team init` 生成的 `.fastgit/*.yaml`（`policy` / `commit` / `check`），功能说明在 `features.md` §2.5；配置合并优先级与 `FASTGIT_AI_CACHE` 等环境变量见 `features.md` §4 与 `architecture.md` §4。

本地改动到 PR 的闭环、质量门禁常驻见 `features.md` §3 场景 D/E。

## 推荐阅读顺序

1. `architecture.md`
2. `features.md`
3. `roadmap.md`
4. `desktop-status.md`
5. `README.md`
6. `copilot-dcos.md`（按需）

## 维护说明

- 架构变化优先更新 `architecture.md`
- 功能变化优先更新 `features.md` 与 `README.md`
- 避免在多个文件复制相同事实，优先在索引中建立跳转
