# 04. API 契约冻结

## 说明

本文件冻结当前前端开发使用的后端 API。契约以 `backend/internal/http/*.go` 为准。

默认前缀：

```text
/api/v1
```

认证方式：

```text
Authorization: Bearer <accessToken>
```

统一响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {}
}
```

错误响应：

```json
{
  "code": 40001,
  "message": "错误说明",
  "data": null
}
```

时间字段使用 ISO 8601 / RFC3339 格式。

## 健康检查

### `GET /actuator/health`

用途：后端就绪检查。

无需认证。

响应：

```json
{
  "status": "UP"
}
```

### `GET /actuator/health/liveness`

用途：存活检查。

### `GET /actuator/health/readiness`

用途：就绪检查。

## 认证与用户

### `POST /api/v1/auth/register`

用途：注册用户。

请求：

```json
{
  "mobile": "13800000000",
  "username": "farmer",
  "password": "password"
}
```

响应 `data`：

```json
{
  "user": {
    "id": 1,
    "username": "farmer",
    "mobile": "13800000000",
    "status": "ACTIVE"
  }
}
```

### `POST /api/v1/auth/login`

用途：登录。

请求：

```json
{
  "username": "farmer",
  "password": "password"
}
```

响应 `data`：

```json
{
  "accessToken": "jwt-token",
  "expiresIn": 7200,
  "user": {
    "id": 1,
    "name": "farmer",
    "role": "FARMER"
  }
}
```

### `GET /api/v1/users/me`

用途：读取当前用户。

需要认证。

响应 `data`：

```json
{
  "id": 1,
  "name": "farmer",
  "role": "FARMER",
  "interactionStyle": "plain",
  "knowledgeReliance": "data"
}
```

## Dashboard

### `GET /api/v1/dashboard/overview`

用途：移动端 A3 看板聚合数据。

需要认证。

响应 `data`：

```json
{
  "sampleTime": "2026-08-22T08:21:00+08:00",
  "avgSoilMoisture": {
    "value": 28.6,
    "unit": "%"
  },
  "avgTemperature": {
    "value": 26.4,
    "unit": "°C"
  },
  "deviceOnline": {
    "online": 12,
    "total": 13,
    "offline": 1
  },
  "alerts": {
    "active": 1,
    "pendingConfirm": 0
  },
  "plots": [
    {
      "id": 1,
      "code": "A3",
      "soilMoisture": 27.8,
      "temperature": 26.4,
      "status": "ACTIVE"
    }
  ]
}
```

注意：

- 遥测尚未接入时，`sampleTime`、`avgSoilMoisture`、`avgTemperature` 可能为 `null`。
- 单个地块的 `soilMoisture`、`temperature` 也可能为 `null`。

## 地块

### `GET /api/v1/plots`

用途：当前用户地块列表。

需要认证。

响应 `data`：

```json
[
  {
    "id": 1,
    "code": "A3",
    "name": "A3 地块",
    "status": "ACTIVE",
    "soilMoisture": null,
    "temperature": null,
    "deviceStatus": null,
    "alertCount": 0,
    "updatedAt": "2026-08-22T08:21:00+08:00"
  }
]
```

### `GET /api/v1/plots/{plotId}`

用途：地块详情。

需要认证。

响应 `data`：

```json
{
  "id": 1,
  "code": "A3",
  "name": "A3 地块",
  "cropName": "番茄",
  "area": 12.5,
  "status": "ACTIVE",
  "createdAt": "2026-08-22T08:21:00+08:00"
}
```

## 遥测

### `GET /api/v1/plots/{plotId}/telemetry/latest`

用途：读取单个地块最新遥测与来源设备。

需要认证。

响应 `data`：

```json
{
  "plotId": 1,
  "sampleTime": "2026-08-22T08:21:00+08:00",
  "metrics": {
    "soilMoisture": {
      "value": 28.6,
      "unit": "%"
    },
    "temperature": {
      "value": 26.4,
      "unit": "°C"
    }
  },
  "sourceDevices": [
    {
      "id": 10,
      "name": "A3 土壤传感器",
      "status": "ONLINE",
      "battery": 90
    }
  ]
}
```

注意：

- 遥测尚未接入时，`metrics` 可能为空对象。
- `sourceDevices` 始终返回数组。

## 设备

### `GET /api/v1/devices`

用途：设备列表。

需要认证。

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `plotId` | 否 | 地块 ID |
| `status` | 否 | 设备状态 |
| `type` | 否 | 设备类型 |
| `page` | 否 | 默认 1 |
| `pageSize` | 否 | 默认 20，最大 100 |

设备状态：

- `UNACTIVATED`
- `ONLINE`
- `OFFLINE`
- `RECONNECTING`
- `FAULT`
- `DISABLED`

响应 `data`：

```json
{
  "items": [
    {
      "id": 10,
      "deviceSn": "SN-001",
      "name": "A3 土壤传感器",
      "type": "SOIL_MOISTURE_SENSOR",
      "plotId": 1,
      "status": "ONLINE",
      "battery": 90,
      "lastSeenAt": "2026-08-22T08:21:00+08:00",
      "firmwareVersion": "1.0.0"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### `POST /api/v1/devices/bind`

用途：绑定设备到地块。

需要认证。

请求：

```json
{
  "deviceSn": "SN-001",
  "plotId": 1,
  "name": "A3 土壤传感器",
  "type": "SOIL_MOISTURE_SENSOR"
}
```

响应 `data`：

```json
{
  "id": 10,
  "deviceSn": "SN-001",
  "status": "ONLINE"
}
```

### `DELETE /api/v1/devices/{deviceId}/binding`

用途：解绑设备。

需要认证。

响应 `data`：

```json
true
```

### `GET /api/v1/devices/{deviceId}/status`

用途：读取设备状态详情。

需要认证。

响应 `data`：

```json
{
  "deviceId": 10,
  "status": "ONLINE",
  "battery": 90,
  "signal": 80,
  "lastSeenAt": "2026-08-22T08:21:00+08:00",
  "message": "正常"
}
```

## 灌溉控制与命令

### `GET /api/v1/plots/{plotId}/irrigation/status`

用途：读取地块灌溉阀门状态。

需要认证。

响应 `data`：

```json
{
  "plotId": 1,
  "valveDeviceId": 20,
  "state": "OFF",
  "mode": "MANUAL",
  "remainingSeconds": 0,
  "maxSeconds": 1800,
  "lastCommandId": "cmd_xxx"
}
```

### `POST /api/v1/plots/{plotId}/irrigation/commands`

用途：下发灌溉控制命令。

需要认证。

必须请求头：

```text
Idempotency-Key: <最长64字符>
```

请求开启：

```json
{
  "action": "OPEN",
  "durationSeconds": 600,
  "mode": "MANUAL",
  "reason": "手动开启灌溉"
}
```

请求关闭：

```json
{
  "action": "CLOSE",
  "durationSeconds": 0,
  "mode": "MANUAL",
  "reason": "手动关闭灌溉"
}
```

字段限制：

- `action`：`OPEN` / `CLOSE`。
- `mode`：`MANUAL` / `AUTO` / `AI_SUGGESTED`。
- `OPEN.durationSeconds`：60 到 1800。
- `CLOSE.durationSeconds`：必须为 0。
- `reason`：最长 500 字符。

响应 `data`：

```json
{
  "commandId": "cmd_xxx",
  "plotId": 1,
  "action": "OPEN",
  "status": "SUCCESS",
  "createdAt": "2026-08-22T08:21:00+08:00"
}
```

### `GET /api/v1/commands`

用途：命令列表。

需要认证。

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `plotId` | 否 | 地块 ID |
| `status` | 否 | 命令状态 |
| `page` | 否 | 默认 1 |
| `pageSize` | 否 | 默认 20，最大 100 |

命令状态：

- `PENDING`
- `REJECTED`
- `SENT`
- `ACKNOWLEDGED`
- `SUCCEEDED`
- `FAILED`
- `TIMEOUT`
- `EXPIRED`

响应 `data`：

```json
{
  "items": [
    {
      "id": "cmd_xxx",
      "plotCode": "A3",
      "action": "OPEN",
      "durationSeconds": 600,
      "status": "SUCCEEDED",
      "operatorName": "farmer",
      "createdAt": "2026-08-22T08:21:00+08:00"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### `GET /api/v1/commands/{commandId}`

用途：命令详情。

需要认证。

响应 `data`：

```json
{
  "id": "cmd_xxx",
  "plotId": 1,
  "deviceId": 20,
  "action": "OPEN",
  "status": "SUCCEEDED",
  "requestPayload": {
    "durationSeconds": 600,
    "mode": "MANUAL",
    "reason": "手动开启灌溉"
  },
  "ackPayload": {
    "state": "ON",
    "remainingSeconds": 600
  },
  "createdAt": "2026-08-22T08:21:00+08:00",
  "ackAt": "2026-08-22T08:21:01+08:00"
}
```

## 告警与阈值

### `GET /api/v1/plots/{plotId}/thresholds`

用途：读取地块阈值规则。

需要认证。

响应 `data`：

```json
[
  {
    "id": 100,
    "plotId": 1,
    "metric": "soilMoisture",
    "operator": "LT",
    "value": 30,
    "unit": "%",
    "durationSeconds": 300,
    "enabled": true,
    "level": "MEDIUM"
  }
]
```

### `PUT /api/v1/plots/{plotId}/thresholds/{thresholdId}`

用途：更新阈值规则，并在同一事务中创建版本化机器配置下发任务。HTTP 成功不代表机器已应用。

需要认证。

请求：

```json
{
  "metric": "soilMoisture",
  "operator": "LT",
  "value": 30,
  "durationSeconds": 300,
  "level": "MEDIUM",
  "enabled": true
}
```

响应 `data`：

```json
{
  "id": 100,
  "updatedAt": "2026-08-22T08:21:00+08:00",
  "configVersion": 7,
  "syncStatus": "PENDING",
  "targetCount": 2
}
```

### `GET /api/v1/plots/{plotId}/thresholds/{thresholdId}/sync`

用途：查询当前阈值配置版本以及每台绑定机器的下发、ACK 或超时状态。状态为 `PENDING`、`SENT`、`APPLIED`、`FAILED` 或 `TIMEOUT`。

需要认证。

响应 `data`：

```json
{
  "ruleId": 100,
  "configVersion": 7,
  "status": "SENT",
  "targetCount": 2,
  "devices": [
    {
      "deviceId": 3,
      "deviceSn": "BEARPI-001",
      "messageId": "thr_01K3...",
      "status": "APPLIED",
      "sentAt": "2026-08-22T08:21:01+08:00",
      "acknowledgedAt": "2026-08-22T08:21:03+08:00",
      "expiresAt": "2026-08-22T08:23:00+08:00"
    },
    {
      "deviceId": 4,
      "deviceSn": "BEARPI-002",
      "messageId": "thr_01K4...",
      "status": "SENT",
      "sentAt": "2026-08-22T08:21:01+08:00",
      "expiresAt": "2026-08-22T08:23:00+08:00"
    }
  ]
}
```

前端应轮询本接口展示最终设备结果；不要把 PUT 返回的 `PENDING` 当作失败，也不要仅凭 MQTT 已发送就展示为 `APPLIED`。

### `GET /api/v1/alerts`

用途：告警列表。

需要认证。

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `plotId` | 否 | 地块 ID |
| `status` | 否 | `ACTIVE` / `CONFIRMED` / `RESOLVED` / `CLOSED` |
| `page` | 否 | 默认 1 |
| `pageSize` | 否 | 默认 20，最大 100 |

响应 `data`：

```json
{
  "items": [
    {
      "id": 200,
      "plotId": 1,
      "plotCode": "A3",
      "metric": "soilMoisture",
      "level": "MEDIUM",
      "status": "ACTIVE",
      "title": "A3 地块湿度偏低",
      "content": "28.6% 触发持续阈值告警",
      "currentValue": 28.6,
      "thresholdValue": 30,
      "startedAt": "2026-08-22T08:21:00+08:00",
      "confirmedAt": null,
      "confirmRemark": null,
      "recoveredAt": null
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### `GET /api/v1/alerts/logs`

用途：告警日志列表。

需要认证。

查询参数：

- 支持 `plotId`、`status`、`startTime`、`endTime`、`page`、`pageSize`。
- `startTime` / `endTime` 必须为 ISO 8601 时间。

响应结构同 `GET /api/v1/alerts`。

### `POST /api/v1/alerts/{alertId}/confirm`

用途：确认活动告警。

需要认证。

请求：

```json
{
  "remark": "已查看并安排处理"
}
```

响应 `data`：

```json
{
  "id": 200,
  "status": "CONFIRMED",
  "confirmedAt": "2026-08-22T08:21:00+08:00"
}
```

## AI 会话

### `POST /api/v1/ai/sessions`

用途：创建当前用户 AI 会话。

需要认证。

请求：

```json
{
  "plotId": 1
}
```

`plotId` 可省略。

响应 `data`：

```json
{
  "id": "session_xxx",
  "userId": 1,
  "plotId": 1,
  "status": "ACTIVE",
  "summary": null,
  "lastMessageAt": null,
  "closedAt": null,
  "createdAt": "2026-08-22T08:21:00+08:00",
  "updatedAt": "2026-08-22T08:21:00+08:00"
}
```

### `GET /api/v1/ai/sessions/{sessionId}/messages`

用途：读取会话消息。

需要认证。

查询参数：

- `page`：默认 1。
- `pageSize`：默认 20，最大 100。

响应 `data`：

```json
{
  "items": [
    {
      "id": 1,
      "sessionId": "session_xxx",
      "role": "ASSISTANT",
      "content": "当前湿度偏低，建议灌溉。",
      "citations": null,
      "plotId": 1,
      "modelVersion": "v1",
      "traceId": "trace_xxx",
      "createdAt": "2026-08-22T08:21:00+08:00"
    }
  ],
  "page": 1,
  "pageSize": 20,
  "total": 1
}
```

### `POST /api/v1/ai/sessions/{sessionId}/close`

用途：关闭会话。

需要认证。

响应 `data`：

```json
{
  "sessionId": "session_xxx",
  "status": "CLOSED"
}
```

## 知识库

### `GET /api/v1/knowledge/docs`

用途：读取已发布知识文档。

需要认证。

查询参数：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `category` | 否 | 文档分类 |

响应 `data`：

```json
[
  {
    "id": 1,
    "title": "番茄灌溉指南",
    "category": "irrigation",
    "version": 1,
    "source": "manual",
    "downloadUrl": "https://signed-url",
    "publishedAt": "2026-08-22T08:21:00+08:00",
    "updatedAt": "2026-08-22T08:21:00+08:00"
  }
]
```

### `POST /api/v1/knowledge/docs`

用途：上传知识文档。

需要认证，且必须是 `SYSTEM_ADMIN`。

请求类型：

```text
multipart/form-data
```

字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `file` | 是 | 文件 |
| `title` | 是 | 标题 |
| `category` | 是 | 分类 |
| `source` | 否 | 来源 |
| `version` | 否 | 正整数 |

响应 `data`：`knowledge_documents` 文档对象。

### `POST /api/v1/knowledge/docs/{docId}/approve`

用途：审核文档，`DRAFT -> APPROVED`。

需要 `SYSTEM_ADMIN`。

### `POST /api/v1/knowledge/docs/{docId}/publish`

用途：发布文档，`APPROVED -> ACTIVE`。

需要 `SYSTEM_ADMIN`。

### `POST /api/v1/knowledge/docs/{docId}/archive`

用途：归档文档，`ACTIVE -> ARCHIVED`。

需要 `SYSTEM_ADMIN`。

## 实时事件

### `GET /api/v1/events/stream`

用途：SSE 实时事件。

需要认证。

请求头：

```text
Authorization: Bearer <accessToken>
Last-Event-ID: <可选>
```

响应类型：

```text
text/event-stream; charset=utf-8
```

服务端会发送心跳：

```text
: heartbeat 1234567890
```

事件格式：

```text
id: <eventId>
event: <eventType>
data: {"eventTime":"...","resourceId":"..."}
```

支持事件：

| event | data 主要字段 |
| --- | --- |
| `telemetry.updated` | `plotId`、`plotCode`、`soilMoisture`、`temperature`、`sampleTime` |
| `alert.created` | `alertId`、`plotId`、`level`、`title`、`createdAt` |
| `alert.recovered` | `alertId`、`plotId`、`recoveredAt` |
| `device.status.changed` | `deviceId`、`status`、`lastSeenAt` |
| `command.result` | `commandId`、`status`、`plotId`、`ackAt` |

## 内部接口

以下接口不是普通前端页面使用的接口：

- `POST /internal/alerts/trigger`
- `POST /internal/agent/sessions/{sessionId}/messages`

这些接口使用 `X-Internal-Service-Key`，供后端内部或智能体服务调用。

