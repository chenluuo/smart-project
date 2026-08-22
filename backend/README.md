# 智慧农业 Gin 后端骨架

该目录是依据《智慧农业系统架构设计（修订版）》建立的 Go 模块化单体。目前包含 Gin HTTP 服务、MySQL 领域模型、GORM Repository、嵌入式数据库迁移，以及基于 JWT 的注册登录功能。

## 技术基线

- Go 1.25
- Gin
- GORM
- MySQL 8.4 LTS
- Docker Compose

## 模块结构

```text
backend/
├─ cmd/api/                    # HTTP 服务入口与优雅停机
├─ internal/
│  ├─ http/                   # Gin 路由与 Handler
│  ├─ config/                 # 环境变量配置
│  ├─ platform/database/      # MySQL 连接与嵌入式 SQL 迁移
│  ├─ shared/persistence/     # 公共模型与泛型 Repository
│  ├─ identity/               # 用户与角色
│  ├─ plot/                   # 农户所属田块
│  ├─ device/                 # 设备与绑定关系
│  ├─ alert/                  # 告警规则与告警记录
│  ├─ control/                # 设备控制命令
│  ├─ notification/           # 告警通知
│  ├─ audit/                  # 操作审计
│  └─ outbox/                 # 可靠异步事件
└─ compose.yaml
```

项目不预留 Agent/RAG 模块。

## 已建立的 MySQL 表

- `users`、`roles`、`user_roles`
- `plots`（通过 `owner_id` 归属农户）
- `devices`、`device_bindings`
- `alert_rules`、`alerts`
- `device_commands`
- `notifications`
- `audit_logs`
- `outbox_events`

高频温度、湿度遥测不进入 MySQL；后续由 `telemetry` 模块批量写入 TDengine，并将设备最新状态写入 Redis。

## 本地启动

启动 MySQL：

```powershell
docker compose up -d mysql
```

安装依赖并执行测试：

```powershell
go mod tidy
go test ./...
```

启动应用：

```powershell
go run ./cmd/api
```

健康检查：

```text
GET http://localhost:8080/actuator/health
GET http://localhost:8080/actuator/health/liveness
GET http://localhost:8080/actuator/health/readiness
```

认证接口：

```text
POST /api/v1/auth/register  # mobile、username、password，无手机验证码
POST /api/v1/auth/login     # username、password
GET  /api/v1/users/me       # Authorization: Bearer <access_token>
```

地块接口：

```text
GET  /api/v1/plots          # 当前用户的地块列表
GET  /api/v1/plots/{plotId} # 当前用户的地块详情
```

两个地块接口都要求 Bearer Token，并按令牌中的用户 ID 隔离数据。实时遥测接入前，列表中的 `soilMoisture`、`temperature` 和 `deviceStatus` 返回 `null`。

## 配置

本地默认连接：

```text
mysql://localhost:3307/smart_agriculture
username: smart_agriculture
password: smart_agriculture
```

支持以下环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `SERVER_PORT` | `8080` | HTTP 端口 |
| `GIN_MODE` | `debug` | Gin 运行模式 |
| `DB_DSN` | 空 | 完整 Go MySQL DSN，设置后优先级最高 |
| `DB_URL` | 原 JDBC MySQL URL | 兼容原 Spring 配置 |
| `DB_USERNAME` | `smart_agriculture` | 数据库用户名 |
| `DB_PASSWORD` | `smart_agriculture` | 数据库密码 |
| `DB_POOL_MAX_SIZE` | `10` | 最大连接数 |
| `DB_POOL_MIN_IDLE` | `2` | 最大空闲连接数 |
| `DB_MIGRATE` | `true` | 启动时执行嵌入式 SQL 迁移 |
| `JWT_SECRET` | 仅供本地开发的默认密钥 | HS256 签名密钥，至少 32 个字符 |
| `JWT_ISSUER` | `smart-agriculture-api` | JWT 签发方 |
| `JWT_TTL` | `2h` | 访问令牌有效期（Go duration） |

已有 Flyway 数据库可直接升级：启动时会读取 `flyway_schema_history`，把成功版本导入新的 `schema_migrations`，不会重复建表。

生产环境不得使用示例密码，也不得将密钥提交到仓库。

## 下一步

1. 实现应用服务和事务边界。
2. 增加按农户隔离的田块数据权限。
3. 接入 EMQX、TDengine 和 Redis。
4. 增加 REST Handler、DTO 和接口契约。
