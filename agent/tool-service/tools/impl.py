"""tool-service：19 个工具实现（集中管理）+ JSON Schema 定义。

工具清单：
  1. get_user_plots        田块查询入口（Go /plots，JWT 权限）
  2. get_latest_telemetry  地块最新遥测（Go /plots/{id}/telemetry/latest）
  3. get_telemetry_history 历史趋势（Go /telemetry/history）
  4. get_farm_overview     总览（Go /dashboard/overview）
  5. get_active_alerts     活动告警（Go /alerts）
  6. get_alert_rules       阈值规则（Go /plots/{id}/thresholds）
  7. get_device_status     设备状态（Go /devices）
  8. search_knowledge      知识检索（Milvus 直连，ACTIVE 由 Go 保证）
  9. get_irrigation_status 灌溉状态（Go /plots/{id}/irrigation/status）
 10. get_command_result    命令执行结果（Go /commands/{id}）
 11. set_crop              设置地块种植作物（Go POST /plots/{id}/crop，"未种植"=清除）
 12. update_alert_rule     修改告警阈值规则（Go PUT /plots/{id}/thresholds/{tid}）
 13. create_alert_rule     新建告警阈值规则（Go POST /plots/{id}/thresholds，创建即下发）
 14. irrigate_to_target_humidity 目标湿度灌溉（阈值闭环，内部经 Go 灌溉命令）
 15. create_scheduled_task 创建定时任务（interval/cron/once，到点提醒）
 16. list_scheduled_tasks  查询我的定时任务
 17. cancel_scheduled_task 取消定时任务
 18. get_order_intents     查询审批通过的采购意向（Go GET /orders，供种植建议）
 19. harvest_inbound       收获入库（Go POST /stocks/inbound，幂等）

注：
- 按时间控泵的 send_irrigation_command 已不对 LLM 暴露（保留函数仅供
  irrigate_to_target_humidity 内部发 OPEN/CLOSE 使用），灌溉统一按阈值驱动。
- get_document_content（文档原文工具）已移除：RAG 检索自带 title/来源，
  问答引用文章名即可，无需整篇原文工具（历史实现见 git 提交 227da55，可按需找回）。

mock_go=true 时（config.yaml tool.mock_go），现场/排查工具返回契约示例数据，
便于 Go 未就绪时本地联调。
"""
from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from shared.config import get_config  # noqa: E402
from shared.go_client import get_go_client  # noqa: E402

# ============================================================
# mock 开关与示例数据
# ============================================================

_MOCK_PLOTS = [
    {"id": "plot_a1", "code": "A1", "name": "西侧棚", "cropName": "番茄",
     "area": 300.0, "soilMoisture": 29.1, "temperature": 26.1, "deviceStatus": "ONLINE"},
    {"id": "plot_a3", "code": "A3", "name": "东侧棚", "cropName": "番茄",
     "area": 320.5, "soilMoisture": 27.8, "temperature": 26.8, "deviceStatus": "ONLINE"},
]


def _mock_enabled() -> bool:
    return bool(get_config("tool").get("mock_go", False))


def _plots_full_threshold() -> int:
    return int(get_config("tool").get("plots_full_threshold", 5))


def _top_k() -> int:
    return int(get_config("tool").get("top_k", 5))


# ============================================================
# 1. get_user_plots：田块查询入口
# ============================================================

GET_USER_PLOTS_SCHEMA = {
    "farm_id": {"type": "string", "description": "按农场过滤（可选）"},
    "crop_type": {"type": "string", "description": "按作物过滤（可选）"},
    "keyword": {"type": "string", "description": "田块名称关键词（可选）"},
    "limit": {"type": "integer", "description": "返回条数上限"},
    "offset": {"type": "integer"},
}


def get_user_plots(authorization: str, args: dict) -> dict:
    """返回用户权限范围内的田块列表；田块少（≤ 阈值）全量返回，多则分页+total。"""
    if _mock_enabled():
        return _mock_plots(args)

    query = {
        k: v for k, v in args.items()
        if k in ("farm_id", "crop_type", "keyword", "limit", "offset") and v is not None
    }
    plots = get_go_client().get_plots(authorization, **query)
    if not isinstance(plots, list):
        return {"ok": False, "error": "Go 返回格式异常"}
    return {"ok": True, "data": {"total": len(plots), "plots": plots}}


def _mock_plots(args: dict) -> dict:
    plots = _MOCK_PLOTS
    keyword = args.get("keyword")
    if keyword:
        plots = [p for p in plots if keyword in p["name"]]
    return {"ok": True, "data": {"total": len(plots), "plots": plots}}


# ============================================================
# 2. get_latest_telemetry：地块最新遥测
# ============================================================

GET_LATEST_TELEMETRY_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "metrics": {"type": "array", "items": {"type": "string"},
                "description": "指标列表，如 soilMoisture/temperature；空=全部"},
}


def get_latest_telemetry(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "plotId": args.get("plot_id"),
            "sampleTime": "2026-08-22T08:21:00+08:00",
            "metrics": {"soilMoisture": {"value": 27.8, "unit": "%"},
                        "temperature": {"value": 26.8, "unit": "°C"}},
        }}
    metrics = ",".join(args.get("metrics") or [])
    data = get_go_client().get_plot_telemetry_latest(
        authorization, args["plot_id"], metrics=metrics
    )
    return {"ok": True, "data": data}


# ============================================================
# 3. get_telemetry_history：历史趋势
# ============================================================

GET_TELEMETRY_HISTORY_SCHEMA = {
    "plot_id": {"type": "string"},
    "metric": {"type": "string", "enum": ["soilMoisture", "temperature", "light"],
               "description": "指标：soilMoisture 土壤湿度 / temperature 温度 / light 光照"},
    "range": {"type": "string", "enum": ["1h", "24h", "7d", "30d"]},
    "interval": {"type": "string", "description": "聚合粒度，如 5m/1h/1d"},
}


def get_telemetry_history(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "plotId": args.get("plot_id"), "metric": args.get("metric"), "unit": "%",
            "points": [{"time": "2026-08-16T00:00:00+08:00", "avg": 34.2, "min": 28.5, "max": 39.1}],
        }}
    # agent 参数 snake_case → Go 接口 camelCase（plot_id → plotId 等），否则 Go 读不到参数报 400
    query = dict(args)
    key_map = {"plot_id": "plotId", "start_time": "startTime", "end_time": "endTime"}
    for k, v in key_map.items():
        if k in query:
            query[v] = query.pop(k)
    data = get_go_client().get_telemetry_history(authorization, **query)
    return {"ok": True, "data": data}


# ============================================================
# 4. get_farm_overview：总览
# ============================================================

GET_FARM_OVERVIEW_SCHEMA = {
    "farm_id": {"type": "string"},
}


def get_farm_overview(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "farmId": args.get("farm_id"), "farmName": "张家湾温室群",
            "avgSoilMoisture": {"value": 28.9, "unit": "%"},
            "deviceOnline": {"online": 12, "total": 13, "offline": 1},
            "alerts": {"active": 2},
        }}
    data = get_go_client().get_dashboard_overview(authorization, farmId=args.get("farm_id"))
    return {"ok": True, "data": data}


# ============================================================
# 5. get_active_alerts：活动告警
# ============================================================

GET_ACTIVE_ALERTS_SCHEMA = {
    "plot_id": {"type": "string", "description": "按地块过滤（可选）"},
    "status": {"type": "string", "description": "ACTIVE/CONFIRMED 等（可选）"},
}


def get_active_alerts(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": [{
            "id": "alert_001", "plotId": args.get("plot_id") or "plot_a3", "plotCode": "A3",
            "metric": "soilMoisture", "level": "MEDIUM", "status": "ACTIVE",
            "title": "A3 地块湿度偏低", "currentValue": 28.6, "thresholdValue": 30,
            "startedAt": "2026-08-22T08:20:00+08:00",
        }]}
    query = dict(args)
    # plot_id(snake_case) → plotId(camelCase)，否则 Go 的 plotId 过滤读不到，静默返回全部告警
    if "plot_id" in query:
        query["plotId"] = query.pop("plot_id")
    data = get_go_client().get_alerts(authorization, **{k: v for k, v in query.items() if v})
    return {"ok": True, "data": data}


# ============================================================
# 6. get_alert_rules：阈值规则
# ============================================================

GET_ALERT_RULES_SCHEMA = {
    "plot_id": {"type": "string"},
}


def get_alert_rules(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": [{
            "id": "thr_001", "plotId": args.get("plot_id"), "metric": "soilMoisture",
            "operator": "LT", "value": 28, "unit": "%", "durationSeconds": 300,
            "level": "MEDIUM", "enabled": True,
        }]}
    data = get_go_client().get_thresholds(authorization, args["plot_id"])
    return {"ok": True, "data": data}


# ============================================================
# 7. get_device_status：设备状态
# ============================================================

GET_DEVICE_STATUS_SCHEMA = {
    "device_id": {"type": "string"},
    "plot_id": {"type": "string", "description": "按地块查设备列表（可选）"},
}


def get_device_status(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "deviceId": args.get("device_id") or "dev_gateway_01",
            "status": "ONLINE", "battery": 87, "signal": 76,
            "lastSeenAt": "2026-08-22T08:21:00+08:00",
        }}
    query = {"plotId": args["plot_id"]} if args.get("plot_id") else {}
    data = get_go_client().get_devices(authorization, **query)
    return {"ok": True, "data": data}


# ============================================================
# 8. search_knowledge：知识检索（Milvus 直连）
# ============================================================

SEARCH_KNOWLEDGE_SCHEMA = {
    "query": {"type": "string", "description": "检索问题"},
    "category": {"type": "string", "description": "分类过滤（可选）"},
    "top_k": {"type": "integer"},
}


def search_knowledge(authorization: str, args: dict) -> dict:
    """Milvus 知识 collection 检索（ACTIVE 可用性由 Go 保证，无需状态过滤）。"""
    if _mock_enabled():
        return {"ok": True, "data": [
            {"docId": "doc_001", "title": "番茄温室灌溉建议", "version": 2,
             "source": "knowledge/tomato-irrigation.pdf", "score": 0.91,
             "content": "见干见湿原则：土壤湿度低于 30% 时建议灌溉……"},
        ]}
    from shared.embedding import embed  # 延迟导入
    from shared.milvus_client import search_knowledge as _milvus_search  # 延迟导入
    top_k = args.get("top_k") or _top_k()
    query_vec = embed([args["query"]])[0]
    results = _milvus_search(query_vec, category=args.get("category"), top_k=top_k)
    return {"ok": True, "data": results}


# ============================================================
# 9. get_irrigation_status：灌溉状态
# ============================================================

GET_IRRIGATION_STATUS_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
}


def get_irrigation_status(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "plotId": args.get("plot_id"), "valveDeviceId": "dev_valve_01",
            "state": "OFF", "mode": "MANUAL", "remainingSeconds": 0, "maxSeconds": 900,
        }}
    data = get_go_client().get_irrigation_status(authorization, args["plot_id"])
    return {"ok": True, "data": data}


# ============================================================
# 11. send_irrigation_command：下发灌溉命令（内部函数，不对 LLM 暴露；
#     仅 irrigate_to_target_humidity 闭环内部发 OPEN/CLOSE 使用，复用 Go §6.2）
# ============================================================

SEND_IRRIGATION_COMMAND_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "action": {"type": "string", "enum": ["OPEN", "CLOSE"], "description": "OPEN 开启 / CLOSE 关闭"},
    "duration_seconds": {"type": "integer", "minimum": 60, "maximum": 1800,
                         "description": "开启时长（OPEN 时必填，60-1800 秒）"},
    "reason": {"type": "string", "description": "命令原因（审计用，agent 填建议依据）"},
}


def send_irrigation_command(authorization: str, args: dict) -> dict:
    """下发灌溉命令（JWT 权限由 Go 校验；agent 侧生成幂等键防重复）。"""
    import uuid

    if _mock_enabled():
        return {"ok": True, "data": {
            "commandId": f"cmd_mock_{uuid.uuid4().hex[:10]}",
            "plotId": args.get("plot_id"), "action": args.get("action"),
            "status": "PENDING", "createdAt": "2026-08-22T08:21:10+08:00",
        }}
    # Go §6.2 契约：body 需 action/mode/reason，幂等键经 Idempotency-Key 头传递
    body = {
        "action": args["action"],
        "mode": "MANUAL",  # agent 人工指令统一 MANUAL（AI_SUGGESTED 留待建议采纳链路）
        "reason": args.get("reason", "agent 建议"),
    }
    if args.get("duration_seconds"):
        body["durationSeconds"] = args["duration_seconds"]
    headers = {"Idempotency-Key": uuid.uuid4().hex}
    data = get_go_client().post_irrigation_command(authorization, args["plot_id"], body, headers=headers)
    return {"ok": True, "data": data}


# ============================================================
# 12. get_command_result：命令执行结果（复用 Go §6.3）
# ============================================================

GET_COMMAND_RESULT_SCHEMA = {
    "command_id": {"type": "string", "description": "命令 ID（必填）"},
}
def get_command_result(authorization: str, args: dict) -> dict:
    if _mock_enabled():
        return {"ok": True, "data": {
            "id": args.get("command_id"), "plotId": "plot_a3",
            "action": "OPEN", "status": "SUCCEEDED",
            "ackPayload": {"state": "ON", "remainingSeconds": 600},
            "createdAt": "2026-08-22T08:21:10+08:00", "ackAt": "2026-08-22T08:21:12+08:00",
        }}
    data = get_go_client().get_command_result(authorization, args["command_id"])
    return {"ok": True, "data": data}


# ============================================================
# 13. set_crop：设置地块种植作物（复用 Go POST /plots/{id}/crop）
# "未种植" 表示清除作物（Go 要求 cropName 非空，未种植 作为显式占位）。
# ============================================================

SET_CROP_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "crop_name": {"type": "string",
                  "description": "种植作物名（如 番茄/小麦/玉米），传\"未种植\"表示清除作物"},
}


def set_crop(authorization: str, args: dict) -> dict:
    """设置地块种植作物（JWT 权限由 Go 校验；成功后返回作物名与种植时间）。"""
    if _mock_enabled():
        return {"ok": True, "data": {
            "id": args.get("plot_id"), "cropName": args.get("crop_name"),
            "plantingTime": "2026-08-22T08:21:00+08:00",
        }}
    crop_name = str(args["crop_name"]).strip()
    if not crop_name or len(crop_name) > 64:
        return {"ok": False, "error": "作物名不能为空且长度不能超过 64 个字符"}
    data = get_go_client().update_plot_crop(authorization, args["plot_id"], crop_name)
    return {"ok": True, "data": data}


# ============================================================
# 14. update_alert_rule：修改地块告警阈值规则（复用 Go PUT /plots/{id}/thresholds/{tid}）
# 规则 ID 来自 get_alert_rules；可改 metric/operator/value/hysteresis/
# durationSeconds/level/enabled。
# ============================================================

UPDATE_ALERT_RULE_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "threshold_id": {"type": "string", "description": "阈值规则 ID（来自 get_alert_rules，必填）"},
    "metric": {"type": "string", "description": "指标（soilMoisture/temperature/light）"},
    "operator": {"type": "string", "enum": ["LT", "LTE", "GT", "GTE"],
                 "description": "比较符：LT 低于 / GT 高于 / LTE 不高于 / GTE 不低于"},
    "value": {"type": "number", "description": "阈值数值（如土壤湿度 30）"},
    "hysteresis": {"type": "number", "minimum": 0,
                    "description": "回差（可选，≥0；不传则保留已有回差值）"},
    "duration_seconds": {"type": "integer", "minimum": 0, "maximum": 86400,
                         "description": "持续时长秒（可选，默认 0）"},
    "level": {"type": "string", "enum": ["LOW", "MEDIUM", "HIGH"], "description": "告警级别"},
    "enabled": {"type": "boolean", "description": "是否启用该规则"},
}


def update_alert_rule(authorization: str, args: dict) -> dict:
    """修改地块告警阈值规则（JWT 权限由 Go 校验；返回规则 ID 与更新时间）。"""
    if _mock_enabled():
        return {"ok": True, "data": {"id": args.get("threshold_id"),
                                     "updatedAt": "2026-08-22T08:21:10+08:00"}}
    body = {
        "metric": args["metric"],
        "operator": args["operator"],
        "value": args["value"],
        "durationSeconds": int(args.get("duration_seconds", 0)),
        "level": args["level"],
        "enabled": args["enabled"],
    }
    if args.get("hysteresis") is not None:
        body["hysteresis"] = args["hysteresis"]
    data = get_go_client().update_threshold(authorization, args["plot_id"], args["threshold_id"], body)
    return {"ok": True, "data": data}


# ============================================================
# 16. create_alert_rule：新建地块告警阈值规则（Go POST /plots/{id}/thresholds）
# 创建即触发版本化下发到地块绑定设备；返回规则 ID/configVersion/targetCount。
# ============================================================

CREATE_ALERT_RULE_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "metric": {"type": "string", "enum": ["soilMoisture", "temperature", "light"],
               "description": "指标（必填）：soilMoisture 土壤湿度 / temperature 温度 / light 光照"},
    "operator": {"type": "string", "enum": ["LT", "LTE", "GT", "GTE"],
                 "description": "比较符（必填）：LT 低于 / GT 高于 / LTE 不高于 / GTE 不低于"},
    "value": {"type": "number",
              "description": "阈值数值（必填）：soilMoisture 0~100 / temperature -50~100 / light 0~200000"},
    "hysteresis": {"type": "number", "minimum": 0, "description": "回差（可选，≥0，防抖）"},
    "enabled": {"type": "boolean", "description": "是否启用（可选，默认 true）"},
}


def create_alert_rule(authorization: str, args: dict) -> dict:
    """新建地块告警阈值规则（JWT 权限由 Go 校验；创建即触发版本化下发到地块设备）。
    level 默认 MEDIUM、durationSeconds 默认 60（服务端默认，硬件判断不参与）。"""
    if _mock_enabled():
        return {"ok": True, "data": {"id": 99, "configVersion": 1,
                                     "syncStatus": "PENDING", "targetCount": 0}}
    body = {
        "metric": args["metric"],
        "operator": args["operator"],
        "value": args["value"],
        "enabled": bool(args.get("enabled", True)),
    }
    if args.get("hysteresis") is not None:
        body["hysteresis"] = args["hysteresis"]
    data = get_go_client().create_threshold(authorization, args["plot_id"], body)
    return {"ok": True, "data": data}


# ============================================================
# 16-18. 定时任务：按农户（user_id）创建/查询/取消周期性任务。
# 到点由 ingest worker 触发，将任务消息发给 agent 自主处理（复用告警主动推送模式）。
# ============================================================

CREATE_SCHEDULED_TASK_SCHEMA = {
    "name": {"type": "string", "description": "任务名称"},
    "trigger_type": {"type": "string", "enum": ["interval", "cron", "once"],
                     "description": "触发类型：interval=间隔秒 / cron=周期 / once=一次性"},
    "trigger": {"type": "string", "description": "触发值，如 3600 / \"0 8 * * *\" / \"2026-09-01T08:00:00+08:00\""},
    "message": {"type": "string", "description": "到点提醒内容"},
}

LIST_SCHEDULED_TASKS_SCHEMA = {
    "status": {"type": "string", "enum": ["all", "enabled"], "description": "筛选（可选）"},
}

CANCEL_SCHEDULED_TASK_SCHEMA = {
    "task_id": {"type": "string", "description": "任务 ID（来自 list_scheduled_tasks）"},
}


def _user_id_from_auth(authorization: str) -> str:
    from shared.jwt import JWTError, user_id_from_token

    if not authorization.startswith("Bearer "):
        raise ValueError("缺少 JWT")
    try:
        return user_id_from_token(authorization[7:])
    except JWTError as e:
        raise ValueError(f"JWT 无效: {e}") from e


def create_scheduled_task(authorization: str, args: dict) -> dict:
    """创建定时任务（绑定当前登录用户，到点由 worker 触发 agent 处理）。"""
    import time as _t
    import uuid

    from shared.redis_client import get_redis
    from shared.scheduler import next_run

    try:
        user_id = _user_id_from_auth(authorization)
    except ValueError as e:
        return {"ok": False, "error": str(e)}

    trigger_type = (args.get("trigger_type") or "").strip().lower()
    trigger = str(args.get("trigger") or "").strip()
    name = (args.get("name") or "").strip()
    message = (args.get("message") or "").strip()
    if not name or not message:
        return {"ok": False, "error": "name 和 message 不能为空"}
    if trigger_type not in ("interval", "cron", "once"):
        return {"ok": False, "error": "trigger_type 必须为 interval/cron/once"}

    try:
        import datetime as _dt

        nxt = next_run(trigger_type, trigger,
                       now=_dt.datetime.now(_dt.timezone.utc))
    except Exception as e:
        return {"ok": False, "error": f"trigger 非法: {e}"}
    if nxt is None:
        return {"ok": False, "error": "触发时间已过，任务不会执行，请检查 trigger"}

    task_id = "task_" + uuid.uuid4().hex[:12]
    task = {
        "task_id": task_id,
        "user_id": user_id,
        "name": name,
        "trigger_type": trigger_type,
        "trigger": trigger,
        "message": message,
        "next_run_at": str(int(nxt)),
        "created_at": _t.strftime("%Y-%m-%dT%H:%M:%SZ", _t.gmtime()),
        "enabled": "1",
    }
    get_redis().task_create(user_id, task_id, task)
    return {"ok": True, "data": {"task_id": task_id, "next_run_at": task["next_run_at"]}}


def list_scheduled_tasks(authorization: str, args: dict) -> dict:
    """查询当前用户的定时任务列表。"""
    from shared.redis_client import get_redis

    try:
        user_id = _user_id_from_auth(authorization)
    except ValueError as e:
        return {"ok": False, "error": str(e)}
    tasks = get_redis().task_list(user_id)
    status = (args.get("status") or "all").strip().lower()
    if status == "enabled":
        tasks = [t for t in tasks if t.get("enabled") == "1"]
    return {"ok": True, "data": tasks}


def cancel_scheduled_task(authorization: str, args: dict) -> dict:
    """取消（删除）定时任务。"""
    from shared.redis_client import get_redis

    try:
        user_id = _user_id_from_auth(authorization)
    except ValueError as e:
        return {"ok": False, "error": str(e)}
    task_id = (args.get("task_id") or "").strip()
    if not task_id:
        return {"ok": False, "error": "task_id 不能为空"}
    get_redis().task_cancel(user_id, task_id)
    return {"ok": True, "data": {"task_id": task_id, "cancelled": True}}


# ============================================================
# 15. irrigate_to_target_humidity：目标湿度灌溉（闭环，无新增 Go 接口）
# 用户说"把湿度浇到 X%"时调用。执行内闭环：
#   当前湿度 >= 目标 → 跳过；
#   否则 OPEN 水泵 → 每 _POLL_SECONDS 轮询 Go telemetry/latest →
#   达到目标湿度 → CLOSE（达标）；超过 _MAX_WAIT_SECONDS → CLOSE（超时兜底）。
# ============================================================
IRRIGATE_TO_TARGET_SCHEMA = {
    "plot_id": {"type": "string", "description": "地块 ID（必填）"},
    "target_humidity": {"type": "number", "minimum": 0, "maximum": 100,
                        "description": "目标土壤湿度（%），达到后自动关闭水泵"},
    "reason": {"type": "string", "description": "灌溉原因（审计用）"},
}

_POLL_SECONDS = 10          # 湿度轮询间隔
# 兜底超时：必须严格小于 agent 侧 tool_call_timeout_seconds（默认 720s），
# 保证"工具调用超时前"水泵已被工具自身关闭（工具先超时→关泵→返回，agent 侧超时时泵已关）。
_MAX_WAIT_SECONDS = 600


def _current_humidity(authorization: str, plot_id: str) -> float | None:
    try:
        data = get_go_client().get_plot_telemetry_latest(authorization, plot_id, metrics="soilMoisture")
        sm = (data.get("metrics") or {}).get("soilMoisture") or {}
        value = sm.get("value")
        return float(value) if value is not None else None
    except Exception:
        return None


def _close_pump_with_retry(authorization: str, plot_id: str, reason: str, retries: int = 3) -> bool:
    """关泵并重试（CLOSE 失败会补发），返回是否已确认关闭。"""
    import time as _t

    for _ in range(retries):
        try:
            result = send_irrigation_command(authorization, {
                "plot_id": plot_id, "action": "CLOSE", "reason": reason,
            })
            if result.get("ok"):
                return True
        except Exception:
            pass
        _t.sleep(1)
    return False


def irrigate_to_target_humidity(authorization: str, args: dict) -> dict:
    import time

    plot_id = args["plot_id"]
    target = float(args["target_humidity"])
    reason = args.get("reason", "目标湿度灌溉")

    current = _current_humidity(authorization, plot_id)
    if current is None:
        return {"ok": False, "error": "无法获取当前土壤湿度，请稍后重试"}
    if current >= target:
        return {"ok": True, "data": {
            "status": "SKIPPED", "plotId": plot_id, "current": current, "target": target,
            "message": f"当前湿度 {current:.1f}% 已不低于目标 {target:.1f}%，无需灌溉",
        }}

    # 开启水泵（Go 命令时长取上限作兜底，实际以湿度达标为准）
    opened = send_irrigation_command(authorization, {
        "plot_id": plot_id, "action": "OPEN", "duration_seconds": _MAX_WAIT_SECONDS, "reason": reason,
    })
    if not opened.get("ok"):
        return {"ok": False, "error": f"开启水泵失败: {opened}"}

    # 保证关泵铁律：OPEN 之后的所有退出路径（达标/超时/异常）都会关闭水泵
    pump_open = True
    close_reason: str | None = None
    try:
        started = time.time()
        last = current
        while time.time() - started < _MAX_WAIT_SECONDS:
            time.sleep(_POLL_SECONDS)
            current = _current_humidity(authorization, plot_id)
            if current is None:
                continue
            last = current
            if current >= target:
                close_reason = f"达到目标湿度 {target:.1f}%（当前 {current:.1f}%）"
                pump_open = not _close_pump_with_retry(authorization, plot_id, close_reason)
                return {"ok": True, "data": {
                    "status": "DONE", "plotId": plot_id, "current": current, "target": target,
                    "durationSeconds": int(time.time() - started),
                    "message": f"已灌溉至目标湿度 {target:.1f}%（当前 {current:.1f}%），水泵已自动关闭",
                }}

        # 超时：关泵（必然在 agent 调用超时之前完成，见 _MAX_WAIT_SECONDS 说明）
        close_reason = f"目标湿度灌溉超时（{_MAX_WAIT_SECONDS}s），当前 {last:.1f}%"
        pump_open = not _close_pump_with_retry(authorization, plot_id, close_reason)
        return {"ok": True, "data": {
            "status": "TIMEOUT", "plotId": plot_id, "current": last, "target": target,
            "message": f"轮询 {_MAX_WAIT_SECONDS}s 未达到目标湿度 {target:.1f}%（当前 {last:.1f}%），已自动关闭水泵",
        }}
    finally:
        # 兜底：任何未捕获异常导致提前退出，也保证水泵被关闭（含补发重试）
        if pump_open:
            try:
                _close_pump_with_retry(authorization, plot_id, close_reason or "目标湿度灌溉异常退出，兜底关闭水泵")
            except Exception:
                pass
            pump_open = False


# ============================================================
# 19. get_order_intents：查询审批通过的采购意向（供农户种植建议）
# 数据源 Go GET /orders?status=APPROVED（固定审批通过状态；FARMER/ADMIN/WAREHOUSE_MANAGER 全量）
# ============================================================

GET_ORDER_INTENTS_SCHEMA = {}


def get_order_intents(authorization: str, args: dict) -> dict:
    """查询审批通过（APPROVED）的采购意向订单，返回订单号/物料/数量/期望时间，供安排种植参考。"""
    if _mock_enabled():
        return {"ok": True, "data": {
            "items": [{"orderNo": "INT-20260830-001", "materialName": "番茄",
                       "quantity": "500.000", "unit": "kg", "expectedTime": "2026-09-15T00:00:00Z"}],
        }}
    data = get_go_client().get_orders(authorization, status="APPROVED")
    items = []
    for order in (data.get("items") or []):
        for it in (order.get("items") or []):
            items.append({
                "orderId": order.get("id"),
                "orderNo": order.get("orderNo"),
                "status": order.get("status"),
                "materialId": it.get("materialId"),
                "materialName": it.get("materialName"),
                "quantity": it.get("quantity"),
                "unit": it.get("unit"),
                "availableQuantity": it.get("availableQuantity"),
                "expectedTime": order.get("expectedTime"),
            })
    return {"ok": True, "data": {"items": items}}


# ============================================================
# 20. harvest_inbound：收获入库（调 Go POST /stocks/inbound）
# FARMER/WAREHOUSE_MANAGER/SYSTEM_ADMIN 可调；Idempotency-Key 幂等
# ============================================================

HARVEST_INBOUND_SCHEMA = {
    "warehouse_id": {"type": "integer", "description": "仓库 ID（必填）"},
    "material_id": {"type": "integer", "description": "物料 ID（必填）"},
    "quantity": {"type": "number", "description": "收获数量（必填，>0）"},
    "plot_id": {"type": "integer", "description": "来源地块 ID（必填）"},
    "remark": {"type": "string", "description": "备注（可选）"},
}


def harvest_inbound(authorization: str, args: dict) -> dict:
    """收获入库：把本次收获登记进成品仓库存（Go POST /stocks/inbound，幂等）。"""
    import time as _t
    import uuid

    if _mock_enabled():
        return {"ok": True, "data": {"recordId": 1, "stockQuantity": args.get("quantity")}}
    body = {
        "warehouseId": args["warehouse_id"],
        "materialId": args["material_id"],
        "quantity": str(args["quantity"]),
        "plotId": args["plot_id"],
    }
    if args.get("remark"):
        body["remark"] = args["remark"]
    key = f"harvest-{int(_t.time() * 1000)}-{uuid.uuid4().hex[:8]}"
    data = get_go_client().post_stock_inbound(authorization, body, headers={"Idempotency-Key": key})
    return {"ok": True, "data": data}


# ============================================================
# 注册表（启动时调用）
# ============================================================

def register_all() -> None:
    from registry import get_registry

    reg = get_registry()
    reg.register("get_user_plots", "1.0", "获取当前用户权限范围内的田块列表", GET_USER_PLOTS_SCHEMA, [], get_user_plots)
    reg.register("get_latest_telemetry", "1.0", "查询地块最新遥测值（带采样时间）", GET_LATEST_TELEMETRY_SCHEMA, ["plot_id"], get_latest_telemetry)
    reg.register("get_telemetry_history", "1.0", "查询历史趋势（支持聚合粒度）", GET_TELEMETRY_HISTORY_SCHEMA, ["plot_id", "metric"], get_telemetry_history)
    reg.register("get_farm_overview", "1.0", "查询农场/地块总览汇总", GET_FARM_OVERVIEW_SCHEMA, [], get_farm_overview)
    reg.register("get_active_alerts", "1.0", "查询活动告警", GET_ACTIVE_ALERTS_SCHEMA, [], get_active_alerts)
    reg.register("get_alert_rules", "1.0", "查询地块阈值规则（含 operator/时长）", GET_ALERT_RULES_SCHEMA, ["plot_id"], get_alert_rules)
    reg.register("get_device_status", "1.0", "查询设备在线状态", GET_DEVICE_STATUS_SCHEMA, [], get_device_status)
    reg.register("search_knowledge", "1.0", "检索农业知识库（RAG）", SEARCH_KNOWLEDGE_SCHEMA, ["query"], search_knowledge)
    # 控制类（组内已允许 agent 直接发送命令；复用 Go 已有控制接口）
    reg.register("get_irrigation_status", "1.0", "查询地块灌溉状态（ON/OFF、剩余时长）", GET_IRRIGATION_STATUS_SCHEMA, ["plot_id"], get_irrigation_status)
    reg.register("get_command_result", "1.0", "查询命令执行结果（SUCCEEDED/FAILED/TIMEOUT）", GET_COMMAND_RESULT_SCHEMA, ["command_id"], get_command_result)
    # 注：按时间控泵的 send_irrigation_command 已从工具列表移除，灌溉统一走目标湿度闭环
    reg.register("irrigate_to_target_humidity", "1.0", "目标湿度灌溉：开启水泵并自动监测，达到目标湿度或超时后自动关闭", IRRIGATE_TO_TARGET_SCHEMA, ["plot_id", "target_humidity"], irrigate_to_target_humidity)
    reg.register("set_crop", "1.0", "设置地块种植作物（传\"未种植\"表示清除作物）", SET_CROP_SCHEMA, ["plot_id", "crop_name"], set_crop)
    reg.register("update_alert_rule", "1.0", "修改地块告警阈值规则（指标/比较符/阈值/回差/级别/启停）", UPDATE_ALERT_RULE_SCHEMA, ["plot_id", "threshold_id", "metric", "operator", "value", "level", "enabled"], update_alert_rule)
    reg.register("create_alert_rule", "1.0", "新建地块告警阈值规则（创建即下发到地块绑定设备）", CREATE_ALERT_RULE_SCHEMA, ["plot_id", "metric", "operator", "value"], create_alert_rule)
    # 定时任务（按农户；到点由 worker 触发 agent 自主处理）
    reg.register("create_scheduled_task", "1.0", "创建定时任务（周期/间隔/一次性，到点提醒）", CREATE_SCHEDULED_TASK_SCHEMA, ["name", "trigger_type", "trigger", "message"], create_scheduled_task)
    reg.register("list_scheduled_tasks", "1.0", "查询我的定时任务", LIST_SCHEDULED_TASKS_SCHEMA, [], list_scheduled_tasks)
    reg.register("cancel_scheduled_task", "1.0", "取消定时任务", CANCEL_SCHEDULED_TASK_SCHEMA, ["task_id"], cancel_scheduled_task)
    # 仓储/意向订单
    reg.register("get_order_intents", "1.0", "查询审批通过的采购意向订单（物料/数量/期望时间），供种植安排参考", GET_ORDER_INTENTS_SCHEMA, [], get_order_intents)
    reg.register("harvest_inbound", "1.0", "收获入库：登记本次收获进成品仓库存（幂等）", HARVEST_INBOUND_SCHEMA, ["warehouse_id", "material_id", "quantity", "plot_id"], harvest_inbound)
