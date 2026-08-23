# 03. 数据库契约冻结

## 说明

本文件只冻结前端开发需要理解的数据模型。前端不直接访问数据库，所有读写必须通过后端 API 完成。

数据库结构以 `backend/internal/platform/database/migrations/*.sql` 和后端 model 为准。

## 技术边界

- 数据库：MySQL。
- ORM：GORM。
- 前端访问方式：仅通过 `/api/v1` HTTP API。
- 遥测高频历史数据当前不在 MySQL 表内；前端第一阶段只消费后端暴露的最新遥测 API。

## 用户与权限

### `users`

用途：登录用户、农户、系统管理员。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 用户 ID |
| `name` | 登录用户名，唯一 |
| `mobile` | 手机号，唯一 |
| `password_hash` | 密码哈希，前端不可见 |
| `status` | 用户状态：`ACTIVE` / `DISABLED` |
| `interaction_style` | AI 交互风格偏好 |
| `knowledge_reliance` | AI 决策依据偏好 |
| `created_at` / `updated_at` | 创建与更新时间 |

### `roles`

用途：角色字典。

当前种子角色：

| role_code | role_name |
| --- | --- |
| `FARMER` | 农户 |
| `SYSTEM_ADMIN` | 系统管理员 |

历史迁移中曾有 `FARM_ADMIN`，当前已移除。

### `user_roles`

用途：用户与角色多对多关系。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `user_id` | 用户 ID |
| `role_id` | 角色 ID |

## 地块

### `plots`

用途：农户拥有的地块。当前数据隔离以 `owner_id` 为主，不再通过 `farms`。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 地块 ID |
| `owner_id` | 所属用户 ID |
| `code` | 地块编号，如 A3 / P1 |
| `name` | 地块名称 |
| `crop_type` | 作物类型 |
| `growth_stage` | 生长阶段 |
| `area` | 面积 |
| `location` | 位置描述 |
| `status` | `ACTIVE` / `DISABLED` |
| `created_at` / `updated_at` | 创建与更新时间 |

约束：

- 同一用户下 `code` 唯一：`uk_plots_owner_code(owner_id, code)`。

## 设备

### `devices`

用途：设备主数据。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 设备 ID |
| `device_code` | 设备编码，唯一 |
| `serial_no` | 设备序列号，唯一 |
| `name` | 设备名称 |
| `device_type` | 设备类型 |
| `model` | 型号 |
| `status` | 设备状态 |
| `battery` | 电量 |
| `signal` | 信号强度 |
| `firmware_version` | 固件版本 |
| `status_message` | 状态说明 |
| `credential_status` | 设备凭据状态 |
| `activated_at` | 激活时间 |
| `last_seen_at` | 最后心跳时间 |

设备状态：

- `UNACTIVATED`
- `ONLINE`
- `OFFLINE`
- `RECONNECTING`
- `FAULT`
- `DISABLED`

凭据状态：

- `PENDING`
- `ACTIVE`
- `REVOKED`

### `device_bindings`

用途：设备与地块绑定关系，支持软解绑。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 绑定 ID |
| `device_id` | 设备 ID |
| `plot_id` | 地块 ID |
| `bound_by` | 绑定操作人 |
| `bound_at` | 绑定时间 |
| `unbound_at` | 解绑时间，空值表示当前有效 |

## 告警

### `alert_rules`

用途：地块阈值规则。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 规则 ID |
| `plot_id` | 地块 ID |
| `name` | 规则名称 |
| `metric` | 指标，如 `soilMoisture` / `temperature` |
| `comparison_operator` | 比较符 |
| `threshold` | 阈值 |
| `duration_seconds` | 持续时间 |
| `hysteresis` | 回差 |
| `level` | 告警级别 |
| `enabled` | 是否启用 |

比较符：

- `LT`
- `LTE`
- `GT`
- `GTE`

告警级别：

- `LOW`
- `MEDIUM`
- `HIGH`

### `alerts`

用途：告警记录。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 告警 ID |
| `rule_id` | 规则 ID |
| `device_id` | 触发设备，可为空 |
| `acknowledged_by` | 确认人 |
| `level` | 告警级别 |
| `status` | 告警状态 |
| `trigger_value` | 触发值 |
| `triggered_at` | 触发时间 |
| `acknowledged_at` | 确认时间 |
| `confirmation_remark` | 确认说明 |
| `resolved_at` | 恢复时间 |

前端状态：

- `ACTIVE`
- `CONFIRMED`
- `RESOLVED`
- `CLOSED`

兼容状态：

- `ACKNOWLEDGED`：后端会在列表中归一为 `CONFIRMED`。

## 控制命令

### `device_commands`

用途：灌溉等设备控制命令记录。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 自增 ID |
| `command_id` | 对外命令 ID |
| `device_id` | 设备 ID |
| `plot_id` | 地块 ID |
| `issued_by` | 发起用户 |
| `action` | 内部动作 |
| `parameters_json` | 命令参数 |
| `idempotency_key` | 幂等键，唯一 |
| `status` | 命令状态 |
| `error_code` | 错误码 |
| `error_message` | 错误信息 |
| `issued_at` | 下发时间 |
| `expires_at` | 过期时间 |
| `executed_at` | 执行时间 |

内部动作：

- `IRRIGATION_ON`
- `IRRIGATION_OFF`

前端请求动作：

- `OPEN`
- `CLOSE`

命令状态：

- `PENDING`
- `REJECTED`
- `SENT`
- `ACKNOWLEDGED`
- `SUCCEEDED`
- `FAILED`
- `TIMEOUT`
- `EXPIRED`

## 通知、审计、Outbox

### `notifications`

用途：站内通知和告警通知记录。第一阶段前端不直接展示通知中心。

### `audit_logs`

用途：操作审计。第一阶段前端不直接展示审计日志。

### `outbox_events`

用途：可靠异步事件投递。第一阶段前端不直接展示。

## AI 会话

### `chat_sessions`

用途：AI 会话。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 会话 ID |
| `user_id` | 用户 ID |
| `plot_id` | 绑定地块，可为空 |
| `status` | `ACTIVE` / `CLOSED` |
| `summary` | 会话摘要 |
| `last_message_at` | 最后一条消息时间 |
| `closed_at` | 关闭时间 |

### `chat_messages`

用途：AI 消息事实源。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 消息 ID |
| `session_id` | 会话 ID |
| `role` | 消息角色 |
| `content` | 消息内容 |
| `citations_json` | 引用 |
| `plot_id` | 关联地块 |
| `model_version` | 模型版本 |
| `trace_id` | 链路 ID |
| `created_at` | 创建时间 |

消息角色：

- `USER`
- `ASSISTANT`
- `SYSTEM`
- `TOOL`

### `ai_suggestions`

用途：AI 建议。第一阶段前端只预留，不实现采纳流程。

核心状态：

- `PENDING`
- `ACCEPTED`
- `REJECTED`
- `EXPIRED`

## 知识库

### `knowledge_documents`

用途：知识库文档元数据。

核心字段：

| 字段 | 含义 |
| --- | --- |
| `id` | 文档 ID |
| `title` | 标题 |
| `category` | 分类 |
| `object_key` | MinIO 对象键 |
| `file_hash` | SHA-256 |
| `source` | 来源 |
| `status` | 文档状态 |
| `version` | 版本 |
| `uploaded_by` | 上传人 |
| `approved_by` | 审核人 |
| `published_at` | 发布时间 |
| `archived_at` | 归档时间 |

文档状态：

- `DRAFT`
- `APPROVED`
- `ACTIVE`
- `ARCHIVED`

前端普通用户只读取 `ACTIVE` 文档。

