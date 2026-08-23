# 07. 前后端联调说明

## 当前状态

- 前端目录：`frontend/`
- 前端技术栈：Vite + React + TypeScript
- 前端本地地址：`http://127.0.0.1:5173/`
- 后端代理目标：`http://localhost:8080`
- 后端目录：`backend/`
- 后端依赖：Go、MySQL、MinIO

## 启动顺序

### 1. 启动 MySQL 和 MinIO

在 `backend/` 目录执行：

```powershell
docker compose up -d mysql minio
```

MySQL 连接信息：

```text
host: localhost
port: 3307
database: smart_agriculture
username: smart_agriculture
password: smart_agriculture
```

### 2. 启动 Go 后端

在 `backend/` 目录执行：

```powershell
go run ./cmd/api
```

后端默认端口：

```text
http://localhost:8080
```

健康检查：

```text
GET http://localhost:8080/actuator/health
```

期望返回：

```json
{
  "status": "UP"
}
```

### 3. 启动前端

在 `frontend/` 目录执行：

```powershell
npm.cmd run dev -- --host 127.0.0.1
```

浏览器打开：

```text
http://127.0.0.1:5173/
```

## 联调检查点

1. 注册账号：`POST /api/v1/auth/register`
2. 登录账号：`POST /api/v1/auth/login`
3. 读取当前用户：`GET /api/v1/users/me`
4. 读取地块：`GET /api/v1/plots`
5. 读取 Dashboard：`GET /api/v1/dashboard/overview`
6. 读取设备：`GET /api/v1/devices`
7. 读取告警：`GET /api/v1/alerts`
8. 读取命令：`GET /api/v1/commands`
9. 读取知识库：`GET /api/v1/knowledge/docs`

## 常见问题

### 页面提示后端服务不可达

说明前端能运行，但 Vite 代理访问不到 Go 后端。检查：

- `go run ./cmd/api` 是否已启动。
- `http://localhost:8080/actuator/health` 是否返回 `UP`。
- 后端端口是否仍是 `8080`。

### 后端启动失败

优先检查：

- Docker 是否已启动。
- MySQL 容器是否运行。
- `backend/.env` 是否存在，或环境变量是否正确。
- 3307 端口是否被占用。

### 注册或登录失败

优先看后端终端日志。前端请求已经通过 `/api` 代理到 `localhost:8080`，如果返回业务错误，页面会显示后端返回的 `message`。
