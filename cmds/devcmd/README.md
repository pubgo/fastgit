# 开发工具使用说明

## 创建多个服务

### 方法一：编辑配置文件

编辑项目根目录下的 `.dev.yaml` 文件，添加多个服务配置：

```yaml
web_port: 8080  # Web 管理界面端口

services:
  # 服务1: API 服务器
  - name: api-server
    enabled: true
    port: 3000  # 可选：服务运行端口
    watch_dirs:
      - ./api
      - ./internal
    watch_exts:
      - .go
    ignore_dirs:
      - .git
      - vendor
      - node_modules
      - tmp
    build_cmd: go build -o tmp/api-server ./api
    run_cmd: ./tmp/api-server
    run_args:
      - --port
      - "3000"
    delay: 500  # 重启延迟（毫秒）
    log_max_lines: 1000

  # 服务2: Web 服务器
  - name: web-server
    enabled: true
    port: 3001
    watch_dirs:
      - ./web
    watch_exts:
      - .go
    ignore_dirs:
      - .git
      - vendor
      - tmp
    build_cmd: go build -o tmp/web-server ./web
    run_cmd: ./tmp/web-server
    run_args:
      - --port
      - "3001"
    delay: 500
    log_max_lines: 1000

  # 服务3: 后台任务
  - name: worker
    enabled: true
    watch_dirs:
      - ./worker
    watch_exts:
      - .go
    ignore_dirs:
      - .git
      - vendor
      - tmp
    build_cmd: go build -o tmp/worker ./worker
    run_cmd: ./tmp/worker
    run_args: []
    delay: 500
    log_max_lines: 1000

  # 服务4: 前端开发服务器（如果需要）
  - name: frontend
    enabled: true
    port: 5173
    watch_dirs:
      - ./frontend/src
    watch_exts:
      - .ts
      - .tsx
      - .js
      - .jsx
    ignore_dirs:
      - node_modules
      - .git
      - dist
    build_cmd: ""  # 前端通常不需要构建命令
    run_cmd: npm run dev
    run_args: []
    delay: 1000
    log_max_lines: 500
```

### 方法二：通过 Web 界面

1. 启动开发工具：
   ```bash
   fastcommit dev
   ```

2. 打开浏览器访问 `http://localhost:8080`

3. 在 Web 界面中：
   - 选择不同的服务进行配置
   - 修改配置后点击"保存配置"
   - 注意：通过 Web 界面修改的配置**不会保存到 `.dev.yaml` 文件**，只会在运行时生效

### 配置字段说明

- `name`: 服务名称（必填，唯一标识）
- `enabled`: 是否启用该服务（true/false）
- `port`: 服务运行端口（可选，0 表示不使用端口）
- `watch_dirs`: 监控的目录列表
- `watch_exts`: 监控的文件扩展名列表（如 `.go`, `.ts`）
- `ignore_dirs`: 忽略的目录列表
- `build_cmd`: 构建命令（空字符串表示不构建）
- `run_cmd`: 运行命令
- `run_args`: 运行参数列表
- `delay`: 文件变更后重启的延迟时间（毫秒）
- `log_max_lines`: 日志最大保留行数

### 使用示例

#### 示例1：Go 微服务项目

```yaml
web_port: 8080

services:
  - name: user-service
    enabled: true
    port: 3001
    watch_dirs: ["./services/user"]
    watch_exts: [".go"]
    ignore_dirs: [".git", "vendor", "tmp"]
    build_cmd: go build -o tmp/user-service ./services/user
    run_cmd: ./tmp/user-service
    run_args: ["--port", "3001"]
    delay: 500
    log_max_lines: 1000

  - name: order-service
    enabled: true
    port: 3002
    watch_dirs: ["./services/order"]
    watch_exts: [".go"]
    ignore_dirs: [".git", "vendor", "tmp"]
    build_cmd: go build -o tmp/order-service ./services/order
    run_cmd: ./tmp/order-service
    run_args: ["--port", "3002"]
    delay: 500
    log_max_lines: 1000
```

#### 示例2：全栈项目（Go + Node.js）

```yaml
web_port: 8080

services:
  - name: backend
    enabled: true
    port: 3000
    watch_dirs: ["."]
    watch_exts: [".go"]
    ignore_dirs: [".git", "vendor", "node_modules", "tmp", "frontend"]
    build_cmd: go build -o tmp/backend .
    run_cmd: ./tmp/backend
    run_args: []
    delay: 500
    log_max_lines: 1000

  - name: frontend
    enabled: true
    port: 5173
    watch_dirs: ["./frontend"]
    watch_exts: [".ts", ".tsx", ".js", ".jsx", ".vue"]
    ignore_dirs: ["node_modules", ".git", "dist"]
    build_cmd: ""
    run_cmd: npm run dev
    run_args: []
    delay: 1000
    log_max_lines: 500
```

### 注意事项

1. **服务名称必须唯一**：每个服务的 `name` 字段必须不同
2. **端口冲突**：确保不同服务的运行端口不冲突
3. **构建输出路径**：不同服务的构建输出路径应该不同，避免互相覆盖
4. **启用状态**：只有 `enabled: true` 的服务才会启动
5. **配置文件位置**：配置文件默认在项目根目录的 `.dev.yaml`，可以通过 `--config` 参数指定

### 启动和停止

- 启动所有启用的服务：`fastcommit dev`
- 在 Web 界面中可以单独重启或停止某个服务
- 按 `Ctrl+C` 停止所有服务

