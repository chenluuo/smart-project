# MQTT 传感器 / 水泵模拟器（含简易 Web UI）

生成虚拟遥测数据，发布到 MQTT broker，供 Go 后端（`internal/mqttclient` → telemetry）联调使用。

## 运行

```powershell
# conda 环境（已装 paho-mqtt）
D:\DuanKou\tools\Anacondaenvs\agent\python.exe simulator.py [--port 8090] [--host 0.0.0.0]
```

浏览器打开 `http://127.0.0.1:8090`（`--host 0.0.0.0` 时局域网设备可访问）。

## 功能

- **MQTT 连接**：顶部配置 broker / prefix / ownerId / deviceSn / 发布间隔(ms)，点"启动"后按间隔发布
  - topic：`{prefix}/{ownerId}/{deviceSn}/telemetry`（默认 `agri/2/SN-BEARPI-001/telemetry`）
- **传感器（土壤湿度 / 温度 / 光照）**：每张卡可设置
  - 目标值：当前值每 tick 移动 `差距 × 变化系数`（差距越大变化越快，逐步逼近目标）
  - 变化系数：0.5%~50%
  - 扰动：每 tick 叠加 ±扰动的随机量
  - 告警范围 min~max：超出即 payload 对应 `xxxWarning=true`
- **水泵**：开关 + 每次增量 + 扰动；开启时每 tick 给**土壤湿度**增加 `增量 + 扰动`
  - 与湿度传感器的目标值形成对抗（泵加湿度、传感器向目标逼近），可模拟缺水灌溉场景
- 每张卡带最近 150 点曲线图；底部显示最近 payload 与发布计数

## 与 Go 端契约

payload 严格对齐 Go `telemetry.Payload`（DisallowUnknownFields + 全必填）：

```json
{"temperature":26.5,"soilMoisture":48,"light":920,
 "temperatureWarning":false,"soilMoistureWarning":false,"lightWarning":false}
```

## HTTP API（供 UI 调用，也可脚本控制）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/state` | 全量状态快照 |
| POST | `/api/start` / `/api/stop` | 启停发布循环 |
| POST | `/api/config` | `{broker, prefix, owner_id, device_sn, interval_ms}` |
| POST | `/api/sensor` | `{id, target?, factor?, noise?, min?, max?}` |
| POST | `/api/pump` | `{enabled?, delta?, noise?}` |
