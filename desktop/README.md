# fastgit desktop (Wails v3)

该桌面端当前采用“SDK/方法调用”模型，不再通过输入命令字符串再解析执行。
前端采用 React + feature 分层组织（参考 colanode 的 app/layout/features 结构）。

## 当前开发状态

最新进度见：

- `../docs/desktop-status.md`

当前桌面端已覆盖模块：

- repo / remote / branch / worktree / issue / pr / tag

并支持：

- 全局 project/repo 命名空间切换
- 项目默认值（base branch / remote）
- 列表页搜索、过滤、排序、批量操作
- 高风险动作 `RESET` 二次确认（force sync 类）

## Quick Tasks (from repo root)

```bash
task desktop:frontend:build
task desktop:test
task desktop:verify
task desktop:run
task desktop:dev
```

`task desktop:dev` 当前是本地开发运行（会先构建前端，然后 `go run .`），不依赖 `build/config.yml`。

## Run in dev mode

```bash
cd desktop/frontend
npm install
npm run build
cd ..
go run .
```

> 如果你本机安装了 `wails3`，也可使用 `wails3 dev`。

## Build desktop app

```bash
wails3 build
```

## 最小验证建议

1. 启动后切换不同模块，确认会自动进入对应列表页。
2. 在列表页执行搜索/过滤/排序，确认计数与详情联动。
3. 对 `强制对齐` 类动作，确认未输入 `RESET` 无法执行。
4. 配置会话 GitHub Token 后，Issue/PR 列表可正常读取。
