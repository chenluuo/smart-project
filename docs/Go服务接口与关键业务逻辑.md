# 智慧农业 Go 服务接口与关键业务逻辑

本文面向前端、测试和业务人员，只描述 Go 服务已经提供的业务能力、关键规则、请求参数与响应数据。本文不介绍框架、数据库实现、消息模型或任何非 Go 服务内部实现。

## 1. 使用约定

### 1.1 地址与认证

- 业务接口统一以 `/api/v1` 开头。
- 除注册、登录和健康检查外，业务接口均需携带访问令牌。
- 请求头格式：`Authorization: Bearer <accessToken>`。
- 用户只能访问自己名下的地块、设备、命令、告警和会话。
- 账户停用后，旧令牌也会立即失效。

### 1.2 统一响应

成功响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {}
}
```

失败响应：

```json
{
  "code": 40001,
  "message": "参数错误",
  "data": null
}
```

常用业务错误码：

| code | 含义 |
| --- | --- |
| `40001` | 请求参数不合法 |
| `40101` | 未登录、令牌失效或账户已停用 |
| `40102` | 内部服务认证失败 |
| `40301` | 没有对应权限 |
| `40401` | 用户或地块不存在 |
| `40402` | 设备不存在 |
| `40403` | 控制命令不存在 |
| `40404` | 告警或会话不存在 |
| `40405` | 知识文档不存在 |
| `40901` | 数据重复或设备已绑定 |
| `40902` | 灌溉设备当前不可控制 |
| `40903` | 告警或会话状态冲突 |
| `40904` | 知识文档重复或状态冲突 |
| `41301` | 上传文件超过限制 |
| `50000` | 服务内部错误 |
| `50301` | 所需存储能力未启用 |

时间统一使用 ISO 8601，例如 `2026-08-22T08:23:00Z`。响应中的数值 ID 使用正整数，命令 ID 和会话 ID 使用字符串。

## 2. 认证与当前用户

### 2.1 注册

`POST /api/v1/auth/register`

关键规则：

- `username` 为 3～64 个非空白字符。
- `mobile` 为 6～15 位数字，可带前导 `+`。
- `password` 长度为 8～72 字节。
- 用户名或手机号不能重复。

请求：

```json
{
  "mobile": "13812345678",
  "username": "grower01",
  "password": "strong-password"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "user": {
      "id": 7,
      "username": "grower01",
      "mobile": "13812345678",
      "status": "ACTIVE",
      "interactionStyle": null,
      "knowledgeReliance": null,
      "createdAt": "2026-08-22T08:00:00Z",
      "updatedAt": "2026-08-22T08:00:00Z"
    }
  }
}
```

### 2.2 登录

`POST /api/v1/auth/login`

请求：

```json
{
  "username": "grower01",
  "password": "strong-password"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "accessToken": "<access-token>",
    "expiresIn": 7200,
    "user": {
      "id": 7,
      "name": "grower01",
      "role": "FARMER"
    }
  }
}
```

### 2.3 当前用户

`GET /api/v1/users/me`

请求体：无。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 7,
    "name": "grower01",
    "role": "FARMER",
    "interactionStyle": null,
    "knowledgeReliance": null
  }
}
```

## 3. 看板、地块与遥测

### 3.1 总览看板

`GET /api/v1/dashboard/overview`

关键逻辑：

- 批量读取并汇总当前用户全部地块的最新土壤湿度、温度和光照。
- 统计设备在线、离线和总数。
- `active` 统计 `ACTIVE` 告警。
- `pendingConfirm` 按当前接口契约统计 `CONFIRMED` 告警，并兼容旧的 `ACKNOWLEDGED` 数据。
- 没有遥测数据时，对应值返回 `null`。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "sampleTime": "2026-08-22T08:20:00Z",
    "avgSoilMoisture": {
      "value": 27.8,
      "unit": "%"
    },
    "avgTemperature": {
      "value": 26.3,
      "unit": "°C"
    },
    "avgLight": {
      "value": 920,
      "unit": "lx"
    },
    "deviceOnline": {
      "online": 12,
      "total": 13,
      "offline": 1
    },
    "alerts": {
      "active": 2,
      "pendingConfirm": 1
    },
    "plots": [
      {
        "id": 11,
        "code": "A1",
        "soilMoisture": 27.8,
        "temperature": 26.3,
        "light": 920,
        "status": "ACTIVE"
      }
    ]
  }
}
```

### 3.2 地块列表

`GET /api/v1/plots`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": [
    {
      "id": 11,
      "code": "A1",
      "name": "西侧棚",
      "status": "ACTIVE",
      "soilMoisture": null,
      "temperature": null,
      "deviceStatus": null,
      "alertCount": 0,
      "updatedAt": "2026-08-22T08:00:00Z"
    }
  ]
}
```

### 3.3 地块详情

`GET /api/v1/plots/{plotId}`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 11,
    "code": "A1",
    "name": "西侧棚",
    "cropName": "番茄",
    "plantingTime": "2026-08-10T00:00:00Z",
    "area": 12.5,
    "status": "ACTIVE",
    "createdAt": "2026-08-01T00:00:00Z"
  }
}
```

### 3.4 更新地块作物

`POST /api/v1/plots/{plotId}/crop`

请求体：

```json
{
  "cropName": "番茄"
}
```

关键逻辑：校验地块归属后，更新该地块当前种植的作物名称，并将种植时间自动置为本次更新的时间戳（无需前端传入）。当 `cropName` 为空或超过 64 个字符时返回 `40001`。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 11,
    "cropName": "番茄",
    "plantingTime": "2026-08-24T10:00:00Z"
  }
}
```

### 3.5 单个地块最新遥测

`GET /api/v1/plots/{plotId}/telemetry/latest`

关键逻辑：先校验地块归属，再返回最新指标及该地块绑定的来源设备。暂无指标时 `metrics` 为空对象。

> **单参数传感器**：一台设备可只上报自己的参数（如土壤传感器只报 `soilMoisture`），`metrics` 只包含实际上报的指标；同一地块多台不同参数设备上报时，latest 自动合并（未上报的指标沿用最近一次值）。缺失指标前端显示 `--`，不参与告警同步。
> **执行器心跳**：水泵/阀门等无传感器参数的设备可发送全空 payload（`{}`）仅用于保活，服务端只标记设备在线、不写 latest/告警/历史（详见 `10_BearPi-HM-Nano_MQTT硬件接入说明.md` §5）。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "plotId": 11,
    "sampleTime": "2026-08-22T08:20:00Z",
    "metrics": {
      "soilMoisture": {
        "value": 27.8,
        "unit": "%"
      },
      "temperature": {
        "value": 26.3,
        "unit": "°C"
      },
      "light": {
        "value": 920,
        "unit": "lx"
      }
    },
    "sourceDevices": [
      {
        "id": 31,
        "name": "A1 土壤传感器",
        "status": "ONLINE",
        "battery": null
      }
    ]
  }
}
```

### 3.6 多地块最新遥测

`GET /api/v1/telemetry/latest?plotId=11`

`plotId` 可选；不传时返回当前用户全部地块。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": [
    {
      "plotId": 11,
      "plotCode": "A1",
      "soilMoisture": 27.8,
      "temperature": 26.3,
      "light": 920,
      "status": "ALERT",
      "sampleTime": "2026-08-22T08:20:00Z"
    }
  ]
}
```

其中 `status` 为 `NORMAL` 或 `ALERT`，由该地块是否存在活动告警决定。

### 3.7 历史趋势

`GET /api/v1/telemetry/history?plotId=11&metric=soilMoisture&range=24h&interval=1h`

关键规则：

- `metric`：`soilMoisture`、`temperature` 或 `light`。
- 时间可使用 `range=1h|24h|7d|30d`，也可使用 `startTime` 和 `endTime`。
- `interval`：`5m`、`1h` 或 `1d`，默认 `1h`。
- 查询区间不能超过 30 天。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "plotId": 11,
    "metric": "soilMoisture",
    "unit": "%",
    "points": [
      {
        "time": "2026-08-22T07:00:00Z",
        "avg": 27.6,
        "min": 26.9,
        "max": 28.2
      }
    ]
  }
}
```

## 4. 设备管理

### 4.1 设备列表

`GET /api/v1/devices?plotId=11&status=ONLINE&type=SOIL_SENSOR&page=1&pageSize=20`

筛选参数均可选；`pageSize` 最大为 100。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 31,
        "deviceSn": "SN-20260822-001",
        "name": "A1 土壤传感器",
        "type": "SOIL_SENSOR",
        "plotId": 11,
        "status": "ONLINE",
        "battery": 86,
        "lastSeenAt": "2026-08-22T08:20:00Z",
        "firmwareVersion": "1.2.0"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

设备状态可为 `UNACTIVATED`、`ONLINE`、`OFFLINE`、`RECONNECTING`、`FAULT` 或 `DISABLED`。

### 4.2 绑定设备

`POST /api/v1/devices/bind`

关键逻辑：地块必须属于当前用户；一个设备同一时间只能存在一条有效绑定；已登记设备的类型不能被改成其他类型。

请求：

```json
{
  "deviceSn": "SN-20260822-001",
  "plotId": 11,
  "name": "A1 土壤传感器",
  "type": "SOIL_SENSOR"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 31,
    "deviceSn": "SN-20260822-001",
    "status": "OFFLINE"
  }
}
```

### 4.3 解绑设备

`DELETE /api/v1/devices/{deviceId}/binding`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": true
}
```

### 4.4 设备状态

`GET /api/v1/devices/{deviceId}/status`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "deviceId": 31,
    "status": "ONLINE",
    "battery": 86,
    "signal": 72,
    "lastSeenAt": "2026-08-22T08:20:00Z",
    "message": null
  }
}
```

## 5. 灌溉控制

### 5.1 查询当前灌溉状态

`GET /api/v1/plots/{plotId}/irrigation/status`

关键逻辑：状态仅由最近一条成功或已确认的命令计算，失败、超时和被拒绝的命令不会覆盖实际状态。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "plotId": 11,
    "valveDeviceId": 41,
    "state": "ON",
    "mode": "MANUAL",
    "remainingSeconds": 480,
    "maxSeconds": 1800,
    "lastCommandId": "cmd_0123456789abcdef"
  }
}
```

### 5.2 下发灌溉命令

`POST /api/v1/plots/{plotId}/irrigation/commands`

必须携带请求头：

```text
Idempotency-Key: irrigation-11-20260822-001
```

关键规则：

- 幂等键必填，长度不超过 64；相同用户使用相同幂等键重试时返回原命令。
- `action`：`OPEN` 或 `CLOSE`。
- `mode`：`MANUAL`、`AUTO` 或 `AI_SUGGESTED`。
- `OPEN` 的 `durationSeconds` 必须为 60～1800。
- `CLOSE` 的 `durationSeconds` 必须为 0 或省略。
- 地块必须绑定在线的灌溉阀门。

**权限校验（后端 SQL 查归属，不依赖请求/设备 owner）**：
服务端按 `FindIrrigationDevice(请求者JWT, plotId)` 校验——SQL 从 `plots`（`p.owner_id = 请求者`）→ `device_bindings` → `devices`（类型 `IRRIGATION_VALVE`）查出阀门设备。请求者必须是**该地块的当前归属用户**；校验通过后，MQTT 命令发布到 `{prefix}/{该地块ownerId}/{deviceSn}/command`（topic 的 ownerId 为 SQL 验证过的地块归属，而非请求体或设备上报中的 ownerId）。地块转移归属后，只有新归属用户能下发命令。

请求：

```json
{
  "action": "OPEN",
  "durationSeconds": 600,
  "mode": "MANUAL",
  "reason": "土壤湿度偏低"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "commandId": "cmd_0123456789abcdef",
    "plotId": 11,
    "action": "OPEN",
    "status": "SUCCESS",
    "createdAt": "2026-08-22T08:21:10Z"
  }
}
```

关闭请求：

```json
{
  "action": "CLOSE",
  "mode": "MANUAL",
  "reason": "灌溉完成"
}
```

### 5.3 查询命令结果

`GET /api/v1/commands/{commandId}`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": "cmd_0123456789abcdef",
    "plotId": 11,
    "deviceId": 41,
    "action": "OPEN",
    "status": "SUCCEEDED",
    "requestPayload": {
      "durationSeconds": 600,
      "mode": "MANUAL",
      "reason": "土壤湿度偏低"
    },
    "ackPayload": {
      "state": "ON",
      "remainingSeconds": 600
    },
    "createdAt": "2026-08-22T08:21:10Z",
    "ackAt": "2026-08-22T08:21:10Z"
  }
}
```

命令状态包括 `PENDING`、`REJECTED`、`SENT`、`ACKNOWLEDGED`、`SUCCEEDED`、`FAILED`、`TIMEOUT` 和 `EXPIRED`。

### 5.4 命令列表

`GET /api/v1/commands?plotId=11&status=SUCCEEDED&page=1&pageSize=20`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": "cmd_0123456789abcdef",
        "plotCode": "A1",
        "action": "OPEN",
        "durationSeconds": 600,
        "status": "SUCCEEDED",
        "operatorName": "grower01",
        "createdAt": "2026-08-22T08:21:10Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

## 6. 阈值与告警

### 6.1 查询阈值规则

`GET /api/v1/plots/{plotId}/thresholds`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": [
    {
      "id": 2,
      "plotId": 11,
      "metric": "soilMoisture",
      "operator": "LT",
      "value": 28,
      "hysteresis": 2,
      "unit": "%",
      "durationSeconds": 300,
      "enabled": true,
      "level": "MEDIUM"
    }
  ]
}
```

### 6.2 阈值规则：新建与更新并下发配置

#### 新建规则（POST）

`POST /api/v1/plots/{plotId}/thresholds`

无 thresholdId，服务端自动创建规则并触发版本化下发。请求体：

```json
{
  "metric": "soilMoisture",
  "operator": "LT",
  "value": 28,
  "hysteresis": 2,
  "enabled": true
}
```

- `level` 不传，服务端默认 `MEDIUM`；`durationSeconds` 不传，服务端默认 `60`（这两个字段对硬件判断不参与——设备端只按 `min/max` 判定告警，系统保留字段用于告警记录与配置快照）。
- 响应 `201`，返回体与 PUT 相同（`id`/`configVersion`/`syncStatus`/`targetCount`）。
- 错误：`40001` 字段不合法（**逐字段返回可选项提示**，如 `metric 必须为 soilMoisture、temperature 或 light`、`operator 必须为 LT、LTE、GT 或 GTE`、`value 超出该指标允许范围`）；`40401` 地块不存在。

#### 更新规则（PUT）

`PUT /api/v1/plots/{plotId}/thresholds/{thresholdId}`

关键规则：

- `operator`：`LT`、`LTE`、`GT` 或 `GTE`。
- `level`：`LOW`、`MEDIUM` 或 `HIGH`。
- `metric`：`temperature`、`soilMoisture` 或 `light`；取值范围依次为 `-50～100`、`0～100` 和 `0～200000`。
- `durationSeconds` 范围为 0～86400。
- `hysteresis` 必须大于等于 0；更新时不传该字段会保留已有回差值，传入时才覆盖。
- 路径中的 `thresholdId` 不存在时创建该 ID 的规则；已存在时只能在其原地块内更新。

请求：

```json
{
  "metric": "soilMoisture",
  "operator": "LT",
  "value": 28,
  "hysteresis": 2,
  "durationSeconds": 300,
  "level": "MEDIUM",
  "enabled": true
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 2,
    "updatedAt": "2026-08-22T08:22:00Z",
    "configVersion": 7,
    "syncStatus": "PENDING",
    "targetCount": 2
  }
}
```

HTTP `200` 只表示规则和下发任务已经可靠落库，不表示机器已经应用。服务端在一个 MySQL 事务中完成：

1. 校验地块归属并锁定地块版本。
2. 新增或更新规则，同时保存操作审计。
3. 将地块级 `configVersion` 单调加一。
4. 读取该地块全部阈值规则，生成完整配置快照。
5. 为每台当前绑定机器写入一条 `PENDING` 投递记录。
6. 为每台目标机器写入 `THRESHOLD_CONFIG_REQUESTED` Outbox 事件。

事务回滚时以上数据全部回滚。存在目标机器时，初始 `syncStatus` 为 `PENDING`；没有绑定机器时，`targetCount` 为 `0`，聚合状态为 `APPLIED`，表示当前没有待同步目标。

### 6.3 查询阈值同步状态

`GET /api/v1/plots/{plotId}/thresholds/{thresholdId}/sync`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "ruleId": 2,
    "configVersion": 7,
    "status": "SENT",
    "targetCount": 2,
    "devices": [
      {
        "deviceId": 3,
        "deviceSn": "BEARPI-001",
        "messageId": "thr_01K3...",
        "status": "APPLIED",
        "sentAt": "2026-08-22T08:22:01Z",
        "acknowledgedAt": "2026-08-22T08:22:03Z",
        "expiresAt": "2026-08-22T08:24:00Z"
      },
      {
        "deviceId": 4,
        "deviceSn": "BEARPI-002",
        "messageId": "thr_01K4...",
        "status": "SENT",
        "sentAt": "2026-08-22T08:22:01Z",
        "expiresAt": "2026-08-22T08:24:00Z"
      }
    ]
  }
}
```

设备状态含义：

| 状态 | 含义 |
| --- | --- |
| `PENDING` | 已落库，等待 MQTT 发布 |
| `SENT` | Broker 已接受 QoS 1 发布，等待机器持久化 ACK |
| `APPLIED` | 机器已原子持久化并启用该版本 |
| `FAILED` | 机器拒绝或持久化失败，`lastError` 给出原因 |
| `TIMEOUT` | 在 `MQTT_THRESHOLD_ACK_TIMEOUT` 内未收到有效 ACK |

聚合状态优先级为 `FAILED` → `TIMEOUT` → 全部 `APPLIED` → 存在 `SENT` → `PENDING`。查询接口始终校验当前用户对地块和规则的归属。

阈值更新和同步查询的主要错误语义：

| HTTP | 业务码 | 场景 |
| --- | --- | --- |
| `400` | `40001` | 路径 ID 非正整数、请求体格式错误或规则字段越界 |
| `401` | `40101` | 未登录或访问令牌无效 |
| `404` | `40401` | 地块不属于当前用户，或阈值规则不存在/不属于该地块 |
| `500` | `50000` | 数据库事务或同步状态查询失败 |

### 6.4 MQTT 阈值配置与 ACK 契约

服务端异步发布完整快照，Topic 为：

```text
agri/{ownerId}/{deviceSn}/config/thresholds/v/{configVersion}
```

发布使用 QoS 1 和 retained 消息。Payload：

```json
{
  "messageId": "thr_01K3...",
  "plotId": 11,
  "configVersion": 7,
  "rules": [
    {
      "id": 2,
      "metric": "soilMoisture",
      "operator": "LT",
      "value": 28,
      "hysteresis": 2,
      "durationSeconds": 300,
      "level": "MEDIUM",
      "enabled": true
    }
  ],
  "issuedAt": "2026-08-22T08:22:00Z",
  "expiresAt": "2026-08-22T08:24:00Z"
}
```

机器只应用高于本地版本的完整快照，必须先原子持久化 `configVersion + rules`，成功后再向下列 Topic ACK：

```text
agri/{ownerId}/{deviceSn}/config/thresholds/ack
```

成功与失败 Payload：

```json
{"messageId":"thr_01K3...","configVersion":7,"status":"APPLIED"}
```

```json
{"messageId":"thr_01K3...","configVersion":7,"status":"FAILED","reason":"flash write failed"}
```

ACK 只接受 `APPLIED` 或 `FAILED`；`FAILED` 必须携带 `reason`。服务端同时核对 Topic 中的 owner、设备序列号、`messageId` 和版本。重复的相同终态 ACK 幂等；不同终态、错误版本、错误设备以及终态后的迟到 ACK 不会覆盖已有结果。

告警是否触发由机器根据已持久化规则在本地判定。遥测中的三类 warning 布尔值是机器判定结果，服务端只接收、去重并维护告警生命周期，不再按阈值重新计算。

### 6.5 告警列表

`GET /api/v1/alerts?plotId=11&status=ACTIVE&page=1&pageSize=20`

告警状态包括 `ACTIVE`、`CONFIRMED`、`RESOLVED` 和 `CLOSED`；旧的 `ACKNOWLEDGED` 数据对外按 `CONFIRMED` 返回。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 101,
        "plotId": 11,
        "plotCode": "A1",
        "metric": "soilMoisture",
        "level": "MEDIUM",
        "status": "ACTIVE",
        "title": "A1 地块湿度偏低",
        "content": "26.5% 触发持续阈值告警",
        "currentValue": 26.5,
        "thresholdValue": 28,
        "startedAt": "2026-08-22T08:20:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

### 6.6 告警日志

`GET /api/v1/alerts/logs?plotId=11&startTime=2026-08-01T00:00:00Z&endTime=2026-08-22T23:59:59Z&page=1&pageSize=20`

响应结构与告警列表一致，可附加时间范围和状态筛选。

### 6.7 确认告警

`POST /api/v1/alerts/{alertId}/confirm`

关键逻辑：确认操作幂等。客户端因超时而重试时仍返回成功，并保持第一次确认的时间和备注。

请求：

```json
{
  "remark": "已开启灌溉，继续观察"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 101,
    "status": "CONFIRMED",
    "confirmedAt": "2026-08-22T08:23:00Z"
  }
}
```

## 7. 实时事件

`GET /api/v1/events/stream`

该接口返回事件流，不使用统一 JSON 响应外壳。断线重连时可携带 `Last-Event-ID` 请求头。

事件示例：

```text
id: 18
event: command.result
data: {"commandId":"cmd_0123456789abcdef","status":"SUCCEEDED","plotId":11,"ackAt":"2026-08-22T08:21:10Z","eventTime":"2026-08-22T08:21:10Z","resourceId":"cmd_0123456789abcdef"}
```

Go 服务定义的事件类型：

- `telemetry.updated`
- `alert.created`
- `alert.recovered`
- `device.status.changed`
- `command.result`

事件按当前用户隔离，不会把其他用户的数据推送到本连接。

## 8. 会话数据管理（Go 侧）

本节只描述 Go 服务对会话和消息记录的管理，不涉及回答生成过程。

### 8.1 创建会话

`POST /api/v1/ai/sessions`

请求中的 `plotId` 可选：

```json
{
  "plotId": 11
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": "chat_0123456789abcdef",
    "userId": 7,
    "plotId": 11,
    "status": "ACTIVE",
    "createdAt": "2026-08-22T08:30:00Z",
    "updatedAt": "2026-08-22T08:30:00Z"
  }
}
```

### 8.2 查询会话消息

`GET /api/v1/ai/sessions/{sessionId}/messages?page=1&pageSize=20`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 501,
        "sessionId": "chat_0123456789abcdef",
        "role": "ASSISTANT",
        "content": "建议继续观察当前湿度变化。",
        "citations": [],
        "plotId": 11,
        "modelVersion": "model-v1",
        "traceId": "trace-001",
        "createdAt": "2026-08-22T08:31:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

### 8.3 关闭会话

`POST /api/v1/ai/sessions/{sessionId}/close`

关闭操作幂等。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "sessionId": "chat_0123456789abcdef",
    "status": "CLOSED"
  }
}
```

### 8.4 Python 智能体回写会话消息

`POST /api/v1/agent/sessions/{sessionId}/messages`

该接口供 Python `agent-service` 在完成一轮问答后，将用户问题或模型回答写入 Go 服务。Python 透传当前用户的访问令牌，Go 根据令牌校验会话归属；会话不存在、不属于当前用户或已经关闭时，不允许写入。

请求头：

```text
Authorization: Bearer <accessToken>
Content-Type: application/json
X-Trace-Id: trace-001
```

其中 `X-Trace-Id` 可选；请求体未提供 `trace_id` 时，Go 使用该请求头记录链路 ID。

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `sessionId` | string | 是 | 已由 `POST /api/v1/ai/sessions` 创建的会话 ID，最长 64 个字符 |

请求 JSON：

```json
{
  "role": "assistant",
  "content": "根据当前土壤湿度，建议继续观察，暂不灌溉。",
  "citations": [
    {
      "docId": "61",
      "title": "番茄灌溉规范",
      "version": 1
    }
  ],
  "plot_id": "11",
  "model_version": "deepseek-v4-flash",
  "trace_id": "trace-001"
}
```

请求字段：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `role` | string | 是 | 消息角色，不区分大小写；允许 `USER`、`ASSISTANT`、`SYSTEM`、`TOOL` |
| `content` | string | 是 | 消息正文；去除首尾空白后不能为空，最大 100000 个字符 |
| `citations` | array/object/null | 否 | 模型回答引用的结构化 JSON，最大 64 KiB |
| `plot_id` | string/null | 否 | Python 使用的 snake_case 字段；值必须是十进制正整数形式，如 `"11"`，并且必须与会话绑定地块一致 |
| `model_version` | string | 否 | Python 使用的 snake_case 字段；模型名称或版本，最长 64 个字符 |
| `trace_id` | string | 否 | Python 使用的 snake_case 字段；链路 ID，最长 64 个字符；优先级高于 `X-Trace-Id` 请求头 |
| `prompt_tokens` | integer | 否 | 本轮问答消耗的 LLM 输入 token 数（agent 记录，非负整数，缺省 0） |
| `completion_tokens` | integer | 否 | 本轮问答消耗的 LLM 输出 token 数（agent 记录，非负整数，缺省 0） |

成功响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "messageId": 501,
    "createdAt": "2026-08-22T08:31:00Z"
  }
}
```

失败响应示例：

```json
{
  "code": 40001,
  "message": "参数错误：plot_id 必须是正整数",
  "data": null
}
```

可能返回的业务错误：

| HTTP 状态 | code | 场景 |
| --- | --- | --- |
| `400` | `40001` | JSON 无法解析、缺少 `role/content`、字段超长、引用 JSON 无效、地块 ID 非法或与会话不一致 |
| `401` | `40101` | 未携带用户访问令牌、令牌无效或账户已停用 |
| `404` | `40404` | 会话不存在，或者会话不属于当前用户 |
| `409` | `40903` | 会话已经关闭，禁止继续写入消息 |

### 8.5 查询自己的 token 消耗

`GET /api/v1/users/me/token-usage`

当前登录用户查询自己的 LLM token 消耗，数据来自 `chat_messages` 中 `ASSISTANT` 消息聚合（agent 每次问答随消息落库 `prompt_tokens`/`completion_tokens`）。

请求体：无。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "todayPromptTokens": 15371,
    "todayCompletionTokens": 807,
    "todayTotal": 16178,
    "totalPromptTokens": 15371,
    "totalCompletionTokens": 807,
    "total": 16178
  }
}
```

- `today*`：按数据库时间（UTC）当日 00:00 起累计；`total*`：全部历史累计。
- 历史消息没有 token 数据时各值为 `0`，新问答产生后开始累计。

## 9. 知识文档管理（Go 侧）

### 9.1 查询已发布文档

`GET /api/v1/knowledge/docs?category=irrigation`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": [
    {
      "id": 61,
      "title": "番茄灌溉规范",
      "category": "irrigation",
      "version": 1,
      "source": "农业技术手册",
      "downloadUrl": "<temporary-download-url>",
      "publishedAt": "2026-08-22T09:00:00Z",
      "updatedAt": "2026-08-22T09:00:00Z"
    }
  ]
}
```

### 9.2 上传文档

`POST /api/v1/knowledge/docs`

仅 `SYSTEM_ADMIN` 可调用。该请求实际使用文件表单；业务字段等价表示如下：

```json
{
  "file": "<binary-file>",
  "title": "番茄灌溉规范",
  "category": "irrigation",
  "source": "农业技术手册",
  "version": 1
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 61,
    "title": "番茄灌溉规范",
    "category": "irrigation",
    "objectKey": "knowledge/20687/abcdef-manual.pdf",
    "fileHash": "<sha256>",
    "source": "农业技术手册",
    "status": "DRAFT",
    "version": 1,
    "uploadedBy": 1,
    "createdAt": "2026-08-22T08:50:00Z",
    "updatedAt": "2026-08-22T08:50:00Z"
  }
}
```

### 9.3 审批、发布与归档

仅 `SYSTEM_ADMIN` 可调用，请求体均为空：

- `POST /api/v1/knowledge/docs/{docId}/approve`：`DRAFT` → `APPROVED`
- `POST /api/v1/knowledge/docs/{docId}/publish`：`APPROVED` → `ACTIVE`
- `POST /api/v1/knowledge/docs/{docId}/archive`：`ACTIVE` → `ARCHIVED`

同标题的新版本发布后，旧的活动版本会自动归档。

响应示例：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 61,
    "title": "番茄灌溉规范",
    "category": "irrigation",
    "objectKey": "knowledge/20687/abcdef-manual.pdf",
    "fileHash": "<sha256>",
    "source": "农业技术手册",
    "status": "ACTIVE",
    "version": 1,
    "uploadedBy": 1,
    "approvedBy": 1,
    "publishedAt": "2026-08-22T09:00:00Z",
    "createdAt": "2026-08-22T08:50:00Z",
    "updatedAt": "2026-08-22T09:00:00Z"
  }
}
```

## 10. Go 服务内部接口

内部接口不面向浏览器或普通用户，使用 `X-Internal-Service-Key` 请求头。

### 10.1 写入会话消息

`POST /internal/agent/sessions/{sessionId}/messages`

请求：

```json
{
  "role": "ASSISTANT",
  "content": "建议继续观察当前湿度变化。",
  "citations": [],
  "plotId": 11,
  "modelVersion": "model-v1",
  "traceId": "trace-001"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "messageId": 501,
    "createdAt": "2026-08-22T08:31:00Z"
  }
}
```

### 10.2 触发告警

`POST /internal/alerts/trigger`

关键逻辑：同一规则已有未结束告警时不会重复创建；首次创建会同时生成站内通知，并记录待后续处理的可靠事件。

请求：

```json
{
  "ruleId": 2,
  "deviceId": 31,
  "triggerValue": 26.5,
  "triggeredAt": "2026-08-22T08:20:00Z",
  "traceId": "trace-alert-001"
}
```

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": 101,
    "status": "ACTIVE",
    "created": true,
    "triggeredAt": "2026-08-22T08:20:00Z"
  }
}
```

重复触发时 `created` 为 `false`，并返回已有告警 ID。

## 11. 健康检查

无需认证：

- `GET /actuator/health/liveness`：进程是否存活。
- `GET /actuator/health/readiness`：服务是否可以接收业务请求。
- `GET /actuator/health`：与 readiness 相同。

正常响应：

```json
{
  "status": "UP"
}
```

不可用响应：

```json
{
  "status": "DOWN"
}
```

## 12. 管理后台接口（SYSTEM_ADMIN）

管理后台路由统一挂 `/api/v1/admin/*`，**全部要求 `SYSTEM_ADMIN` 角色**（复用 `requireSystemAdmin`，非管理员返回 `40301`）。与农户视角接口不同，这些接口不做 JWT 归属过滤，展示全量数据、执行管理动作。配套前端为独立 PC 管理后台（`#/admin`，见 `docs/管理后台接口设计.md`）。

### 12.1 用户列表

`GET /api/v1/admin/users`（SYSTEM_ADMIN）

查询参数：`page`（默认1）、`pageSize`（默认20，上限100）、`keyword`（用户名/手机号模糊）、`role`（FARMER/SYSTEM_ADMIN）、`status`（ACTIVE/DISABLED）

响应 `{code, message, data}`：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 2,
        "username": "farmer2",
        "mobile": "13800000002",
        "role": "FARMER",
        "status": "ACTIVE",
        "plotCount": 3,
        "createdAt": "2026-08-01T08:00:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 4
  }
}
```

- `role` 取该用户主角色（多角色时优先 `SYSTEM_ADMIN`，与登录 JWT 的 `primaryRole` 一致）。
- `plotCount` 为该用户名下地块数（`count(plots where owner_id = user.id)`）。

### 12.2 地块列表（全量）

`GET /api/v1/admin/plots`（SYSTEM_ADMIN）

查询参数：`page`、`pageSize`（上限100）、`keyword`（编码/名称模糊）、`ownerId`（0=未分配）、`status`（ACTIVE/DISABLED）

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 3,
        "code": "A3",
        "name": "A3 番茄试验田",
        "area": 12.5,
        "location": "北区",
        "status": "ACTIVE",
        "ownerId": 2,
        "ownerName": "farmer2",
        "deviceCount": 2,
        "createdAt": "2026-08-01T08:00:00Z",
        "updatedAt": "2026-08-02T08:00:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 3
  }
}
```

- `ownerName` 联 `users` 表；`deviceCount` 为有效绑定数（`device_bindings` 中 `unbound_at IS NULL` 计数）。

### 12.3 创建地块

`POST /api/v1/admin/plots`（SYSTEM_ADMIN）

请求体：

```json
{
  "code": "B2",
  "name": "B2 西瓜试验田",
  "area": 8.6,
  "location": "南区",
  "ownerId": 2
}
```

- `code`、`name`、`ownerId` 必填；`area` 非负；`location` 可选。
- **`ownerId` 必填且必须为有效用户**：`plots.owner_id` 有外键 `fk_plots_owner → users(id)`，不存在"未分配"（owner_id 0/NULL）状态，绑定语义即"地块必须归属某用户"。
- 错误：`40001` 参数不合法或归属用户不存在（外键 1452）；`40904` 该用户下编码重复（唯一索引 `uk_plots_owner_code` 冲突）。
- 响应 `201`，返回完整地块对象（含 `ownerId`）。

### 12.4 分配/转移地块归属

`PUT /api/v1/admin/plots/{plotId}/owner`（SYSTEM_ADMIN）

请求体：

```json
{ "ownerId": 4 }
```

- `ownerId` 必填（>0），将该地块归属转移给指定用户；用户登录后即可见该地块。
- 错误：`40401` 地块不存在；`40001` 归属用户不存在；`40904` 目标用户下已存在同编码地块。

### 12.5 知识文档全状态列表（含待审批队列）

`GET /api/v1/admin/knowledge/docs`（SYSTEM_ADMIN）

查询参数：`page`、`pageSize`（上限100）、`status`（DRAFT/APPROVED/ACTIVE/ARCHIVED，不传=全部）、`category`、`keyword`（标题模糊）

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 7,
        "title": "番茄种植手册",
        "category": "planting",
        "status": "DRAFT",
        "version": 4,
        "source": "农技站",
        "uploaderName": "codex08231008",
        "downloadUrl": "http://minio:9000/knowledge/...（预签名）",
        "createdAt": "2026-08-01T08:00:00Z",
        "updatedAt": "2026-08-02T08:00:00Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 12
  }
}
```

- 与农户视角的 `GET /api/v1/knowledge/docs`（仅 ACTIVE）不同，此接口返回**任意状态**文档，供管理员查看 DRAFT/APPROVED 待审批队列；`downloadUrl` 为 MinIO 预签名地址，可直接用于**文本预览**（前端 `fetch` 原文渲染）。

### 12.6 物理删除知识文档

`DELETE /api/v1/admin/knowledge/docs/{docId}`（SYSTEM_ADMIN）

无请求体。任意状态（DRAFT/APPROVED/ACTIVE/ARCHIVED）均可删除；错误：`40405` 文档不存在。

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": { "id": 7, "deleted": true, "indexCleanup": "queued" }
}
```

**删除链路（物理删除 + 清向量索引）**：

```
DELETE /api/v1/admin/knowledge/docs/{docId}
  → 事务：删 knowledge_documents 行 + 审计日志 + outbox KNOWLEDGE_DOCUMENT_DELETED
  → MinIO Remove(objectKey)（失败仅告警日志，不阻断）
  → knowledge dispatcher → agent-service /internal/knowledge/notify（event=DELETED）
  → Redis XADD queue:doc.process {doc_id, event:"DELETED"}
  → ingest 消费：文档不在 ACTIVE 清单 → Milvus knowledge collection 按 doc_id 删除向量（幂等）
```

- 审批/发布复用现有接口：`POST /api/v1/knowledge/docs/{docId}/approve`、`/publish`、`/archive`（见 §9.3）。
- 向量清理为异步最终一致：删除响应返回后极短时间内旧向量可能仍可被检索到。

### 12.7 设备列表（全量，管理后台）

`GET /api/v1/admin/devices`（SYSTEM_ADMIN）

与农户视角的 `GET /api/v1/devices`（按 JWT 归属过滤）不同，此接口返回**全部设备**（含未绑定设备），并附当前有效绑定的地块与归属用户。

查询参数：`page`、`pageSize`（上限100）、`plotId`、`status`、`type`

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 1,
        "deviceSn": "SN-001",
        "name": "A1 土壤传感器",
        "type": "SOIL_SENSOR",
        "status": "ONLINE",
        "plotId": 1,
        "plotCode": "A1",
        "plotName": "A1 番茄地",
        "ownerName": "testfarmer",
        "firmwareVersion": null,
        "lastSeenAt": null
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 6
  }
}
```

- `plotId = 0` 表示未绑定（`plotCode`/`plotName`/`ownerName` 为 null）。

### 12.8 绑定设备（任意地块，管理后台）

`POST /api/v1/admin/devices/bind`（SYSTEM_ADMIN）

请求体与农户版 `POST /api/v1/devices/bind` 一致（`deviceSn`/`plotId`/`name`/`type`），但**不做地块归属校验**——管理员可将设备绑定到任意农户的地块（农户版受 JWT 归属限制）。

- **`plotId` 传 0（或缺省）时只添加设备、不绑定地块**：序列号不存在则自动创建设备记录（生成 device_code，status=OFFLINE）；已存在则仅更新名称、保留现有绑定。
- 错误：`40001` 参数不合法；`40401` 地块不存在；`40901` 设备已绑定到其他地块（提示"请先解绑"）或设备类型与已登记信息不一致。
- 响应：`{ "id", "deviceSn", "status" }`。

### 12.9 解绑设备（任意，管理后台）

`DELETE /api/v1/admin/devices/{deviceId}/binding`（SYSTEM_ADMIN）

解绑任意设备（农户版受 JWT 归属限制）。错误：`40402` 设备不存在或未绑定。

### 12.10 删除设备（管理后台）

`DELETE /api/v1/admin/devices/{deviceId}`（SYSTEM_ADMIN）

物理删除设备：删除该设备全部绑定记录（含历史）、告警解除设备引用（保留告警历史）、删除设备行。

- **保护**：设备存在命令记录（`device_commands`）时返回 `40901` 拒绝删除（保留操作历史）。
- 错误：`40402` 设备不存在；`40901` 设备存在命令记录，无法删除。

### 12.11 告警记录（全量，管理后台）

`GET /api/v1/admin/alerts?plotId=11&status=ACTIVE&startTime=2026-08-01T00:00:00Z&endTime=2026-08-22T23:59:59Z&page=1&pageSize=50`（SYSTEM_ADMIN）

查询**全部用户**的告警记录，不做 JWT 归属过滤（管理员可跨地块查看）。筛选参数与农户版告警列表一致：

- `plotId`：按地块过滤（可选）
- `status`：按状态过滤（可选；`CONFIRMED` 兼容旧的 `ACKNOWLEDGED`）
- `startTime` / `endTime`：触发时间范围（ISO 8601，可选）
- `page` / `pageSize`：分页（默认 `page=1&pageSize=20`，`pageSize` 上限 100）

响应结构与 6.5 告警列表一致：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 339,
        "plotId": 3,
        "plotCode": "C1",
        "metric": "soilMoisture",
        "level": "MEDIUM",
        "status": "RESOLVED",
        "title": "C1 地块湿度警告",
        "content": "设备上报湿度警告，当前值 59.85%",
        "currentValue": 59.85,
        "thresholdValue": null,
        "startedAt": "2026-08-26T01:26:44.400718Z",
        "recoveredAt": "2026-08-26T01:27:46.620668Z"
      }
    ],
    "page": 1,
    "pageSize": 50,
    "total": 19
  }
}
```

- 实现说明：复用告警列表的查询与转换逻辑，仅去掉 `p.owner_id` 归属过滤；查询**不经过 Redis 缓存**，直接读库保证实时性。
- 权限：非 SYSTEM_ADMIN 返回 `40301` 需要系统管理员权限。

### 12.12 命令记录（全量，管理后台）

`GET /api/v1/admin/commands?plotId=2&status=SUCCEEDED&page=1&pageSize=20`（SYSTEM_ADMIN）

查询**全部用户**的灌溉命令记录（复用 `device_commands` 表，不做 JWT 归属过滤），供管理员面板"命令记录"页使用。

查询参数：

- `plotId`：按地块过滤（可选）
- `status`：按状态过滤（可选；`PENDING`/`SENT`/`ACKNOWLEDGED`/`SUCCEEDED`/`FAILED`/`TIMEOUT`/`EXPIRED`/`REJECTED`）
- `page` / `pageSize`：分页（默认 `page=1&pageSize=20`，`pageSize` 上限 100）

响应：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": 332,
        "commandId": "cmd_d99f534c54fd3441fe3f6fb9b5b780de",
        "plotId": 2,
        "plotCode": "B1",
        "plotName": "B1 黄瓜地",
        "deviceId": 3,
        "issuedBy": 4,
        "operatorName": "h268",
        "action": "IRRIGATION_OFF",
        "parameters": { "mode": "MANUAL", "reason": "达到目标湿度 65.0%（当前 69.3%）" },
        "status": "SUCCEEDED",
        "errorCode": null,
        "errorMessage": null,
        "issuedAt": "2026-08-30T07:14:49Z",
        "expiresAt": "2026-08-30T07:15:49Z",
        "executedAt": "2026-08-30T07:14:49Z",
        "createdAt": "2026-08-30T07:14:49Z"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 313
  }
}
```

- `parameters` 为 `device_commands.parameters_json` 原样透传（含 `mode`/`reason`/`durationSeconds` 等）。
- `plotCode`/`plotName` 联 `plots` 表，`operatorName` 联 `users` 表（`issued_by`）。
- 与农户视角的 `GET /api/v1/commands`（按 JWT 归属过滤）不同，此接口返回全部命令。
- 权限：非 SYSTEM_ADMIN 返回 `40301` 需要系统管理员权限。

## 13. 仓储与意向订单

仓储（物料/仓库/库存/流水/收获入库）与采购意向订单接口。全链路**无价格字段**；数量类型 `DECIMAL(18,3)`，单位取 `materials.unit`。

### 13.1 物料与仓库主数据

`GET/POST/PUT/DELETE /api/v1/materials`、`GET/POST/PUT/DELETE /api/v1/warehouses`

- 权限：`WAREHOUSE_MANAGER` / `SYSTEM_ADMIN`（读写）；列表支持 `keyword`/`status`/分页。
- 物料字段：`name`、`category`、`unit`、`spec`（可空）、`status`（ACTIVE/DISABLED）；删除为软删除（`DELETED`），物料/仓库存在非零库存时拒绝删除（`40901`）。

### 13.2 库存与出入库流水

`GET /api/v1/stocks?warehouseId=&materialId=&page=&pageSize=`

- 权限：`WAREHOUSE_MANAGER` / `SYSTEM_ADMIN`。
- 返回每行含 `totalQuantity`（总量）、`reservedQuantity`（已占用，`TRADING` 状态意向占用）、`availableQuantity`（可售 = 总量 − 占用）：

```json
{
  "code": 0, "message": "OK",
  "data": {
    "items": [{
      "stockId": 1, "warehouseId": 1, "warehouseName": "成品仓",
      "materialId": 1, "materialName": "番茄", "unit": "kg",
      "totalQuantity": "500.000", "reservedQuantity": "200.000", "availableQuantity": "300.000"
    }],
    "page": 1, "pageSize": 20, "total": 1
  }
}
```

`GET /api/v1/stock-records?warehouseId=&materialId=&type=IN|OUT&plotId=&startAt=&endAt=&page=&pageSize=`

- 权限：`WAREHOUSE_MANAGER` / `SYSTEM_ADMIN`；返回流水（`type`/`quantity`/`refType`（HARVEST/ORDER/ADJUSTMENT）/`refId`/`plotId`/`operatorName` 等），倒序分页。

### 13.3 收获入库

`POST /api/v1/stocks/inbound`

- 权限：**`FARMER` / `WAREHOUSE_MANAGER` / `SYSTEM_ADMIN`**（FARMER 可登记收获入库）。
- **必须携带 `Idempotency-Key` 请求头**（≤128 字符，同键同内容重试返回原结果）；`plotId` 必填（来源地块）。
- 请求体：

```json
{
  "warehouseId": 1,
  "materialId": 1,
  "quantity": "100",
  "plotId": 2,
  "remark": "3 号地块番茄收获"
}
```

- 响应 `201`：`{ "recordId": 1, "stockQuantity": "600.000" }`（流水 `ref_type=HARVEST`，`ref_id` 为幂等键）。
- 错误：`40001` 参数/幂等键不合法；`40401` 仓库、物料或地块不存在；`40901` 内容不一致或主数据停用。

### 13.4 采购意向订单（只读）

`GET /api/v1/orders?status=&page=&pageSize=`

- 权限：登录即可；**`FARMER` / `WAREHOUSE_MANAGER` / `SYSTEM_ADMIN` 可见全部**，**`CUSTOMER` 仅见自己**发起的意向。
- `status` 可选：`PENDING`/`APPROVED`/`TRADING`/`CONFIRMED`/`CLOSED`/`REJECTED`。

响应（`data.items[]`，倒序分页）：

```json
{
  "id": 1, "orderNo": "INT-20260830-001", "status": "APPROVED",
  "customerId": 26, "customerName": "customer1",
  "expectedTime": "2026-09-15T00:00:00Z", "remark": "番茄采购意向",
  "createdAt": "2026-08-30T08:00:00Z",
  "items": [{
    "materialId": 1, "materialName": "番茄", "unit": "kg",
    "quantity": "300.000", "availableQuantity": "300.000"
  }]
}
```

- 每单明细的 `availableQuantity` 为该物料可售数量（库存总量 − TRADING 占用，与 `GET /stocks` 同源）。
- 意向单的创建/审批/成交等写路径由订单模块后续补充；当前表结构、占用计算与只读查询已就绪。

`GET /api/v1/orders/{id}`

- 权限同上；`CUSTOMER` 访问他人意向返回 `40401`。
- 返回结构与列表单条一致。

### 13.5 Agent 工具（tool-service）

| 工具 | 数据源 | 说明 |
|---|---|---|
| `get_order_intents` | `GET /orders?status=APPROVED` | 查审批通过的意向（**固定 APPROVED，无入参**），返回物料/数量/期望时间，供种植建议 |
| `harvest_inbound` | `POST /stocks/inbound` | 收获入库登记（内部生成 `Idempotency-Key` 幂等） |
