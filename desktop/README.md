# fastgit desktop (Wails v3)

## Quick Tasks (from repo root)

```bash
task desktop:frontend:build
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
