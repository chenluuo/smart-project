# 智慧农业后端骨架

该目录是依据《智慧农业系统架构设计（修订版）》建立的 Spring Boot 模块化单体骨架。目前只包含工程基础设施、MySQL 领域模型、Repository 和数据库迁移，不包含 Controller 或业务接口实现。

## 技术基线

- Java 21
- Spring Boot 4.1.1
- Maven Wrapper
- Spring Data JPA
- Flyway
- MySQL 8.4 LTS
- Docker Compose

## 模块结构

```text
src/main/java/com/smartagriculture/
├─ identity/                 # 用户与角色
│  ├─ controller/            # HTTP 接口层（暂为空）
│  ├─ service/               # 应用服务与事务（暂为空）
│  ├─ dto/                   # 请求与响应对象（暂为空）
│  ├─ domain/                # 实体、枚举和领域规则
│  └─ repository/            # 数据访问
├─ farm/                     # 农场、成员和地块
├─ device/                   # 设备与绑定关系
├─ telemetry/                # 遥测，后续接入 EMQX/TDengine/Redis
├─ alert/                    # 告警规则与告警记录
├─ control/                  # 设备控制命令
├─ notification/             # 告警通知
├─ audit/                    # 操作审计
├─ outbox/                   # 可靠异步事件
├─ agent/                    # 智能问答，后续接入 RAG
└─ shared/
   ├─ config/                # 公共配置（暂为空）
   ├─ exception/             # 统一异常（暂为空）
   └─ persistence/           # 公共持久化基础类
```

其他业务模块与 `identity` 采用相同的 `controller / service / dto / domain / repository` 分层；暂未实现的目录使用 `.gitkeep` 让 Git 保留。

## 已建立的 MySQL 表

- `users`、`roles`、`user_roles`
- `farms`、`farm_users`、`plots`
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

执行测试：

```powershell
.\mvnw.cmd test
```

启动应用：

```powershell
.\mvnw.cmd spring-boot:run
```

健康检查：

```text
GET http://localhost:8080/actuator/health
```

## 配置

本地默认连接：

```text
jdbc:mysql://localhost:3307/smart_agriculture
username: smart_agriculture
password: smart_agriculture
```

可通过环境变量 `DB_URL`、`DB_USERNAME`、`DB_PASSWORD` 覆盖。生产环境不得使用示例密码，也不得将密钥提交到仓库。

## 下一步

1. 实现应用服务和事务边界。
2. 增加用户认证与农场/地块数据权限。
3. 接入 EMQX、TDengine 和 Redis。
4. 最后再增加 REST Controller、DTO 和接口契约。
