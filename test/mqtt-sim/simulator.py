#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
MQTT 传感器 / 水泵模拟器 + 简易 Web UI
=======================================
功能:
  - 连接 MQTT broker, 按固定间隔发布遥测到 {prefix}/{owner_id}/{device_sn}/telemetry
  - 传感器: 在 UI 设置目标值, 每 tick 变化量 = 差距 * 变化系数(差距越大变化越快),
    并叠加随机扰动, 当前值逐渐向目标值靠近
  - 水泵:  开启后每 tick 给土壤湿度增加"设定增量 + 随机扰动"
  - payload 与 Go 后端严格一致:
    {temperature, soilMoisture, light, temperatureWarning, soilMoistureWarning, lightWarning}

用法:
  python simulator.py [--port 8090] [--host 0.0.0.0]
然后浏览器打开 http://127.0.0.1:8090
"""
import argparse
import json
import random
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

try:
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")
except Exception:
    pass

try:
    from paho.mqtt.client import CallbackAPIVersion, Client as MqttClient
    PAHO_V2 = True
except ImportError:  # paho-mqtt 1.x
    from paho.mqtt.client import Client as MqttClient
    PAHO_V2 = False

# ---------------------------------------------------------------- 全局状态
LOCK = threading.Lock()

SENSORS = [
    {"id": "soilMoisture", "name": "土壤湿度", "unit": "%",  "current": 45.0,
     "target": 45.0, "factor": 0.08, "noise": 0.5, "min": 20, "max": 80,
     "phys_min": 0, "phys_max": 100},
    {"id": "temperature", "name": "温度",     "unit": "°C", "current": 25.0,
     "target": 25.0, "factor": 0.08, "noise": 0.2, "min": 5, "max": 40,
     "phys_min": -20, "phys_max": 80},
    {"id": "light",       "name": "光照",     "unit": "lx", "current": 800.0,
     "target": 800.0, "factor": 0.08, "noise": 5.0, "min": 200, "max": 1000,
     "phys_min": 0, "phys_max": 3000},
]

PUMP = {"enabled": False, "delta": 2.0, "noise": 0.5}

# 后端 MQTT 命令日志: 最近 20 条
COMMANDS = []

CFG = {
    "broker": "tcp://127.0.0.1:1883",
    "prefix": "agri",
    "owner_id": "2",
    "device_sn": "SN-BEARPI-001",
    "interval_ms": 1000,
    "connected": False,
    "running": False,
}

HISTORY = {s["id"]: [] for s in SENSORS}   # 每传感器最近 N 个值, 供 UI 画曲线
HISTORY_LIMIT = 150

STATE = {
    "publish_count": 0,
    "last_payload": None,
    "last_error": None,
    "last_ts": None,
}

_client = None          # paho client
_client_broker = None   # 当前 client 对应的 broker
_client_prefix = None   # 当前 client 订阅命令的 prefix
_stop_event = threading.Event()
_tick_thread = None

# ---------------------------------------------------------------- MQTT
def _on_connect(client, userdata, flags, rc, properties=None):
    if hasattr(rc, "is_failure"):
        ok = not rc.is_failure
    else:
        ok = (rc == 0)
    with LOCK:
        CFG["connected"] = ok
        if not ok:
            STATE["last_error"] = f"MQTT 连接被拒绝: {rc}"
    if ok:
        # 订阅命令下发 topic: {prefix}/+/+/command
        with LOCK:
            sub = f"{CFG['prefix']}/+/+/command"
        client.subscribe(sub, 1)
        print(f"[mqtt] subscribed {sub}", flush=True)
    print(f"[mqtt] connected={ok}", flush=True)

def _on_disconnect(client, userdata, rc, properties=None):
    with LOCK:
        CFG["connected"] = False
        if rc != 0:
            STATE["last_error"] = f"MQTT 连接断开 (rc={rc})"
    print(f"[mqtt] disconnected rc={rc}", flush=True)

def _on_message(client, userdata, msg):
    """收到后端下发的控制命令: {prefix}/{owner}/{deviceSn}/command → 控制水泵"""
    try:
        parts = msg.topic.split("/")
        if len(parts) != 4 or parts[3] != "command":
            return
        owner_id, device_sn = parts[1], parts[2]
        payload = json.loads(msg.payload or b"{}")
        action = str(payload.get("action", "")).upper()
        with LOCK:
            match = (owner_id == CFG["owner_id"])
        if not match:
            print(f"[cmd] ignore other owner: {msg.topic}", flush=True)
            return
        if action in ("OPEN", "IRRIGATION_ON"):
            with LOCK:
                PUMP["enabled"] = True
            state_str = "开"
        elif action in ("CLOSE", "IRRIGATION_OFF"):
            with LOCK:
                PUMP["enabled"] = False
            state_str = "关"
        else:
            print(f"[cmd] unknown action: {action}", flush=True)
            return
        entry = {
            "ts": time.time(),
            "commandId": payload.get("commandId"),
            "action": action,
            "state": state_str,
            "mode": payload.get("mode"),
            "reason": payload.get("reason"),
            "durationSeconds": payload.get("durationSeconds"),
            "topic": msg.topic,
            "deviceSn": device_sn,
        }
        with LOCK:
            COMMANDS.insert(0, entry)
            del COMMANDS[20:]
            STATE["last_command"] = entry
        # 设备回执 ack (Go 端暂未消费, 模拟真实设备行为)
        try:
            ack = {"commandId": payload.get("commandId"), "status": "ACKNOWLEDGED", "deviceSn": device_sn}
            client.publish(f"{parts[0]}/{owner_id}/{device_sn}/command/ack", json.dumps(ack), qos=1)
        except Exception:
            pass
        print(f"[cmd] pump {state_str}: {payload.get('commandId')} from {msg.topic}", flush=True)
    except Exception as e:
        print(f"[cmd] error: {e}", flush=True)

def _parse_broker(url):
    """tcp://host:port 或 host:port → (host, port)"""
    url = url.strip()
    if "://" in url:
        url = url.split("://", 1)[1]
    host, port = url, 1883
    if ":" in url:
        host, _, port_s = url.rpartition(":")
        port = int(port_s)
    return host, port

def _new_client():
    if PAHO_V2:
        c = MqttClient(CallbackAPIVersion.VERSION2, client_id="mqtt-sim-ui-001")
    else:
        c = MqttClient(client_id="mqtt-sim-ui-001")
    c.on_connect = _on_connect
    c.on_disconnect = _on_disconnect
    c.on_message = _on_message
    c.reconnect_delay_set(min_delay=1, max_delay=10)
    return c

def _ensure_connected():
    """按当前 CFG 的 broker/prefix 建立连接(自动重建), 异步重连由 paho loop 负责."""
    global _client, _client_broker, _client_prefix
    with LOCK:
        broker = CFG["broker"]
        prefix = CFG["prefix"]
    if _client is not None and _client_broker == broker and _client_prefix == prefix and _client.is_connected():
        return
    if _client is not None:
        try:
            _client.disconnect()
            _client.loop_stop()
        except Exception:
            pass
        _client = None
    try:
        host, port = _parse_broker(broker)
        c = _new_client()
        c.connect_async(host, port, keepalive=30)
        c.loop_start()
        _client, _client_broker, _client_prefix = c, broker, prefix
    except Exception as e:
        with LOCK:
            STATE["last_error"] = f"MQTT 连接失败: {e}"
        print(f"[mqtt] connect failed: {e}", flush=True)

def _publish(payload: dict):
    global _client
    with LOCK:
        broker = CFG["broker"]
        topic = f"{CFG['prefix']}/{CFG['owner_id']}/{CFG['device_sn']}/telemetry"
    if _client is None or not _client.is_connected():
        with LOCK:
            STATE["last_error"] = "MQTT 未连接"
        return
    try:
        info = _client.publish(topic, json.dumps(payload, ensure_ascii=False), qos=1)
        if info.rc != 0:
            with LOCK:
                STATE["last_error"] = f"publish rc={info.rc}"
            return
        with LOCK:
            STATE["publish_count"] += 1
            STATE["last_payload"] = payload
            STATE["last_error"] = None
            STATE["last_ts"] = time.time()
    except Exception as e:
        with LOCK:
            STATE["last_error"] = f"publish 异常: {e}"

# ---------------------------------------------------------------- 模拟 tick
def _clamp(v, lo, hi):
    return max(lo, min(hi, v))

def _step():
    with LOCK:
        sm = next(s for s in SENSORS if s["id"] == "soilMoisture")
        # 1) 水泵先作用于土壤湿度: 设定增量 + 扰动
        if PUMP["enabled"]:
            sm["current"] += PUMP["delta"] + random.uniform(-PUMP["noise"], PUMP["noise"])
        # 2) 传感器向目标值逼近: 变化量 = 差距 * 系数 + 扰动 (clamp 用物理范围)
        for s in SENSORS:
            diff = s["target"] - s["current"]
            s["current"] += diff * s["factor"] + random.uniform(-s["noise"], s["noise"])
            s["current"] = _clamp(s["current"], s["phys_min"], s["phys_max"])
            HISTORY[s["id"]].append(round(s["current"], 3))
            if len(HISTORY[s["id"]]) > HISTORY_LIMIT:
                HISTORY[s["id"]] = HISTORY[s["id"]][-HISTORY_LIMIT:]
        vals = {s["id"]: s["current"] for s in SENSORS}
        payload = {
            "temperature": round(vals["temperature"], 2),
            "soilMoisture": round(vals["soilMoisture"], 2),
            "light": round(vals["light"], 1),
            "temperatureWarning": bool(vals["temperature"] < SENSORS[1]["min"] or vals["temperature"] > SENSORS[1]["max"]),
            "soilMoistureWarning": bool(vals["soilMoisture"] < SENSORS[0]["min"] or vals["soilMoisture"] > SENSORS[0]["max"]),
            "lightWarning": bool(vals["light"] < SENSORS[2]["min"] or vals["light"] > SENSORS[2]["max"]),
        }
    _publish(payload)

def _tick_loop():
    while not _stop_event.is_set():
        with LOCK:
            interval = CFG["interval_ms"] / 1000.0
            running = CFG["running"]
        if running:
            _ensure_connected()
            _step()
        _stop_event.wait(interval)

def _start():
    with LOCK:
        CFG["running"] = True
        STATE["last_error"] = None
    print("[sim] started", flush=True)

def _stop():
    with LOCK:
        CFG["running"] = False
    print("[sim] stopped", flush=True)

# ---------------------------------------------------------------- HTTP
def _state_snapshot():
    with LOCK:
        return {
            "config": {k: CFG[k] for k in ("broker", "prefix", "owner_id", "device_sn", "interval_ms", "connected", "running")},
            "sensors": list(SENSORS),
            "pump": dict(PUMP),
            "commands": list(COMMANDS),
            "history": {k: list(v) for k, v in HISTORY.items()},
            "state": {k: STATE[k] for k in ("publish_count", "last_payload", "last_error", "last_ts")},
        }

def _apply_sensor(patch):
    with LOCK:
        for s in SENSORS:
            if s["id"] == patch.get("id"):
                for k in ("target", "factor", "noise", "min", "max"):
                    if k in patch and patch[k] is not None:
                        s[k] = float(patch[k])
                return True
    return False

def _apply_pump(patch):
    with LOCK:
        for k in ("enabled", "delta", "noise"):
            if k in patch and patch[k] is not None:
                PUMP[k] = bool(patch[k]) if k == "enabled" else float(patch[k])
    return True

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):  # 静默访问日志
        pass

    def _send(self, code, body, ctype="application/json; charset=utf-8"):
        if isinstance(body, (dict, list)):
            body = json.dumps(body, ensure_ascii=False).encode("utf-8")
        elif isinstance(body, str):
            body = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(body)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(body)

    def _read_json(self):
        try:
            n = int(self.headers.get("Content-Length", "0"))
            return json.loads(self.rfile.read(n) or b"{}")
        except Exception:
            return {}

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/" or path == "/index.html":
            self._send(200, _HTML, "text/html; charset=utf-8")
        elif path == "/api/state":
            self._send(200, _state_snapshot())
        else:
            self._send(404, {"error": "not found"})

    def do_POST(self):
        path = urlparse(self.path).path
        body = self._read_json()
        if path == "/api/start":
            _start()
            self._send(200, {"ok": True})
        elif path == "/api/stop":
            _stop()
            self._send(200, {"ok": True})
        elif path == "/api/config":
            with LOCK:
                for k in ("broker", "prefix", "owner_id", "device_sn"):
                    if k in body and body[k]:
                        CFG[k] = str(body[k]).strip()
                if "interval_ms" in body and body["interval_ms"]:
                    CFG["interval_ms"] = max(50, int(body["interval_ms"]))
            self._send(200, {"ok": True})
        elif path == "/api/sensor":
            ok = _apply_sensor(body)
            self._send(200, {"ok": ok})
        elif path == "/api/pump":
            ok = _apply_pump(body)
            self._send(200, {"ok": ok})
        else:
            self._send(404, {"error": "not found"})

# ---------------------------------------------------------------- UI (内嵌)
_HTML = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MQTT 传感器 / 水泵模拟器</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #0f172a; color: #e2e8f0; font-family: "Segoe UI", "Microsoft YaHei", sans-serif; padding: 16px; }
  h1 { font-size: 18px; margin-bottom: 12px; }
  h1 small { color: #64748b; font-weight: normal; margin-left: 8px; }
  .bar { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; background: #1e293b;
         border: 1px solid #334155; border-radius: 10px; padding: 10px 14px; margin-bottom: 14px; }
  .bar label { color: #94a3b8; font-size: 12px; }
  .bar input, .bar select { background: #0f172a; color: #e2e8f0; border: 1px solid #475569;
         border-radius: 6px; padding: 5px 8px; font-size: 13px; width: 150px; }
  .bar input[type=number] { width: 90px; }
  .dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 6px; }
  .dot.on { background: #22c55e; box-shadow: 0 0 6px #22c55e; }
  .dot.off { background: #ef4444; box-shadow: 0 0 6px #ef4444; }
  .btn { border: none; border-radius: 8px; padding: 8px 18px; font-size: 14px; cursor: pointer; font-weight: 600; }
  .btn.start { background: #22c55e; color: #052e16; }
  .btn.stop { background: #ef4444; color: #450a0a; }
  .btn:disabled { opacity: .5; cursor: not-allowed; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 14px; }
  .card { background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 14px; }
  .card .head { display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 6px; }
  .card .name { font-size: 14px; color: #94a3b8; }
  .card .big { font-size: 34px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .card .unit { font-size: 14px; color: #64748b; margin-left: 4px; }
  .warn { color: #f97316; font-size: 12px; margin-left: 8px; }
  .row { display: flex; align-items: center; gap: 8px; margin-top: 8px; font-size: 13px; color: #cbd5e1; }
  .row label { width: 56px; color: #94a3b8; flex-shrink: 0; }
  .row input[type=range] { flex: 1; accent-color: #38bdf8; }
  .row input[type=number] { width: 72px; background: #0f172a; color: #e2e8f0;
         border: 1px solid #475569; border-radius: 6px; padding: 3px 6px; }
  canvas { width: 100%; height: 70px; margin-top: 10px; background: #0f172a;
          border-radius: 8px; border: 1px solid #334155; display: block; }
  .pump-btn { width: 100%; padding: 12px; border-radius: 10px; border: none; font-size: 15px;
          font-weight: 700; cursor: pointer; margin: 8px 0 4px; }
  .pump-btn.on { background: #22c55e; color: #052e16; }
  .pump-btn.off { background: #334155; color: #94a3b8; }
  .foot { margin-top: 14px; background: #1e293b; border: 1px solid #334155; border-radius: 10px; padding: 10px 14px; font-size: 12px; color: #94a3b8; }
  .foot pre { margin-top: 6px; color: #7dd3fc; white-space: pre-wrap; word-break: break-all; font-size: 12px; }
  .err { color: #f87171; }
</style>
</head>
<body>
<h1>MQTT 传感器 / 水泵模拟器 <small>向 {prefix}/{ownerId}/{deviceSn}/telemetry 发布</small></h1>

<div class="bar">
  <label>broker <input id="cfg-broker" value="tcp://127.0.0.1:1883"></label>
  <label>prefix <input id="cfg-prefix" value="agri" style="width:70px"></label>
  <label>ownerId <input id="cfg-owner" value="2" style="width:60px"></label>
  <label>deviceSn <input id="cfg-sn" value="SN-BEARPI-001" style="width:140px"></label>
  <label>间隔ms <input id="cfg-interval" type="number" min="50" value="1000"></label>
  <span id="conn"><span class="dot off"></span>未连接</span>
  <button id="btn-start" class="btn start">▶ 启动</button>
  <button id="btn-stop" class="btn stop" disabled>⏹ 停止</button>
</div>

<div class="grid">
  <div class="card" id="card-soilMoisture">
    <div class="head"><span class="name">土壤湿度</span><span id="big-soilMoisture" class="big">-</span><span class="unit">%</span><span class="warn" id="warn-soilMoisture"></span></div>
    <div class="row"><label>目标</label><input type="range" data-k="target" min="0" max="100" step="0.5"><input type="number" data-num="target" step="0.5"></div>
    <div class="row"><label>变化系数</label><input type="range" data-k="factor" min="0.005" max="0.5" step="0.005"><input type="number" data-num="factor" step="0.005"></div>
    <div class="row"><label>扰动</label><input type="range" data-k="noise" min="0" max="10" step="0.1"><input type="number" data-num="noise" step="0.1"></div>
    <div class="row"><label>告警范围</label><input type="number" data-k="min" style="width:60px"><span>~</span><input type="number" data-k="max" style="width:60px"></div>
    <canvas id="cv-soilMoisture"></canvas>
  </div>

  <div class="card" id="card-temperature">
    <div class="head"><span class="name">温度</span><span id="big-temperature" class="big">-</span><span class="unit">°C</span><span class="warn" id="warn-temperature"></span></div>
    <div class="row"><label>目标</label><input type="range" data-k="target" min="-10" max="60" step="0.5"><input type="number" data-num="target" step="0.5"></div>
    <div class="row"><label>变化系数</label><input type="range" data-k="factor" min="0.005" max="0.5" step="0.005"><input type="number" data-num="factor" step="0.005"></div>
    <div class="row"><label>扰动</label><input type="range" data-k="noise" min="0" max="5" step="0.1"><input type="number" data-num="noise" step="0.1"></div>
    <div class="row"><label>告警范围</label><input type="number" data-k="min" style="width:60px"><span>~</span><input type="number" data-k="max" style="width:60px"></div>
    <canvas id="cv-temperature"></canvas>
  </div>

  <div class="card" id="card-light">
    <div class="head"><span class="name">光照</span><span id="big-light" class="big">-</span><span class="unit">lx</span><span class="warn" id="warn-light"></span></div>
    <div class="row"><label>目标</label><input type="range" data-k="target" min="0" max="2000" step="10"><input type="number" data-num="target" step="10"></div>
    <div class="row"><label>变化系数</label><input type="range" data-k="factor" min="0.005" max="0.5" step="0.005"><input type="number" data-num="factor" step="0.005"></div>
    <div class="row"><label>扰动</label><input type="range" data-k="noise" min="0" max="100" step="1"><input type="number" data-num="noise" step="1"></div>
    <div class="row"><label>告警范围</label><input type="number" data-k="min" style="width:60px"><span>~</span><input type="number" data-k="max" style="width:60px"></div>
    <canvas id="cv-light"></canvas>
  </div>

  <div class="card" id="card-pump">
    <div class="head"><span class="name">水泵 (增加土壤湿度)</span><span id="pump-effect" class="warn"></span></div>
    <button id="pump-btn" class="pump-btn off">水泵 关</button>
    <div class="row"><label>每次增量</label><input type="range" data-k="delta" min="0" max="10" step="0.1"><input type="number" data-num="delta" step="0.1"></div>
    <div class="row"><label>扰动</label><input type="range" data-k="noise" min="0" max="5" step="0.1"><input type="number" data-num="noise" step="0.1"></div>
    <div style="margin-top:10px;font-size:12px;color:#64748b">开启后每个 tick 给土壤湿度增加 <b>增量+扰动</b>；湿度传感器仍会向自己的目标值逼近，两者可形成对抗。</div>
    <div style="margin-top:10px;font-size:12px;color:#94a3b8">后端命令 (<span id="cmd-count">0</span>)</div>
    <div id="cmd-log" style="margin-top:4px;max-height:120px;overflow-y:auto;font-size:11px;color:#7dd3fc;background:#0f172a;border:1px solid #334155;border-radius:6px;padding:6px"></div>
  </div>
</div>

<div class="foot">
  已发布 <b id="pub-count">0</b> 条 &nbsp;|&nbsp; <span id="pub-err"></span>
  <pre id="last-payload">(等待发布…)</pre>
</div>

<script>
const $ = id => document.getElementById(id);
let state = null;
let postTimer = null;

function schedulePost(url, body) {
  clearTimeout(postTimer);
  postTimer = setTimeout(() => {
    fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  }, 150);
}

function bindSensor(id) {
  const card = $(`card-${id}`);
  card.querySelectorAll('input[data-k]').forEach(el => {
    el.addEventListener('input', () => {
      const k = el.dataset.k;
      const num = card.querySelector(`input[data-num="${k}"]`);
      if (num) num.value = el.value;
      schedulePost('/api/sensor', { id, [k]: parseFloat(el.value) });
    });
  });
  card.querySelectorAll('input[data-num]').forEach(el => {
    el.addEventListener('change', () => {
      const k = el.dataset.k;
      const range = card.querySelector(`input[data-k="${k}"]`);
      if (range) range.value = el.value;
      schedulePost('/api/sensor', { id, [k]: parseFloat(el.value) });
    });
  });
}
['soilMoisture', 'temperature', 'light'].forEach(bindSensor);

(function bindPump() {
  const card = $('card-pump');
  card.querySelectorAll('input[data-k]').forEach(el => {
    el.addEventListener('input', () => {
      const k = el.dataset.k;
      const num = card.querySelector(`input[data-num="${k}"]`);
      if (num) num.value = el.value;
      schedulePost('/api/pump', { [k]: parseFloat(el.value) });
    });
  });
  card.querySelectorAll('input[data-num]').forEach(el => {
    el.addEventListener('change', () => {
      const k = el.dataset.k;
      const range = card.querySelector(`input[data-k="${k}"]`);
      if (range) range.value = el.value;
      schedulePost('/api/pump', { [k]: parseFloat(el.value) });
    });
  });
  $('pump-btn').addEventListener('click', () => {
    const on = !(state && state.pump.enabled);
    schedulePost('/api/pump', { enabled: on });
    updatePumpBtn(on);
  });
})();

function bindConfig() {
  ['cfg-broker', 'cfg-prefix', 'cfg-owner', 'cfg-sn'].forEach(id => {
    $(id).addEventListener('change', () => {
      const key = { 'cfg-broker': 'broker', 'cfg-prefix': 'prefix', 'cfg-owner': 'owner_id', 'cfg-sn': 'device_sn' }[id];
      schedulePost('/api/config', { [key]: $(id).value.trim() });
    });
  });
  $('cfg-interval').addEventListener('change', () => {
    schedulePost('/api/config', { interval_ms: parseInt($('cfg-interval').value) });
  });
}
bindConfig();

$('btn-start').addEventListener('click', () => fetch('/api/start', { method: 'POST' }));
$('btn-stop').addEventListener('click', () => fetch('/api/stop', { method: 'POST' }));

function updatePumpBtn(on) {
  const b = $('pump-btn');
  b.className = 'pump-btn ' + (on ? 'on' : 'off');
  b.textContent = on ? '水泵 开 (增加湿度中)' : '水泵 关';
}

function draw(cv, values, lo, hi) {
  const ctx = cv.getContext('2d');
  const W = cv.width, H = cv.height;
  ctx.clearRect(0, 0, W, H);
  if (!values || values.length < 2) return;
  const span = (hi - lo) || 1;
  ctx.strokeStyle = '#38bdf8';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  for (let i = 0; i < values.length; i++) {
    const x = (i / (values.length - 1)) * (W - 2) + 1;
    const y = H - 2 - ((values[i] - lo) / span) * (H - 4);
    i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
  }
  ctx.stroke();
}

function render(s) {
  const cfg = s.config;
  // 配置回显(仅当用户未聚焦时)
  if (document.activeElement !== $('cfg-broker')) $('cfg-broker').value = cfg.broker;
  $('cfg-prefix').value = cfg.prefix;
  $('cfg-owner').value = cfg.owner_id;
  $('cfg-sn').value = cfg.device_sn;
  $('cfg-interval').value = cfg.interval_ms;
  const conn = $('conn');
  conn.innerHTML = cfg.connected
    ? '<span class="dot on"></span>MQTT 已连接'
    : '<span class="dot off"></span>MQTT 未连接';
  $('btn-start').disabled = cfg.running;
  $('btn-stop').disabled = !cfg.running;

  s.sensors.forEach(sen => {
    const card = $(`card-${sen.id}`);
    const big = $(`big-${sen.id}`);
    big.textContent = Number(sen.current).toFixed(sen.id === 'light' ? 0 : 1);
    const warn = $(`warn-${sen.id}`);
    const over = sen.current < sen.min || sen.current > sen.max;
    warn.textContent = over ? '⚠ 告警' : '';
    warn.style.visibility = over ? 'visible' : 'hidden';
    // 回显控件(仅当未聚焦)
    [['target', 'target'], ['factor', 'factor'], ['noise', 'noise'], ['min', 'min'], ['max', 'max']].forEach(([k, sk]) => {
      const range = card.querySelector(`input[data-k="${k}"]`);
      const num = card.querySelector(`input[data-num="${k}"]`);
      if (document.activeElement !== range && document.activeElement !== num) {
        range.value = sen[sk];
        if (num) num.value = sen[sk];
      }
    });
    draw($(`cv-${sen.id}`), s.history[sen.id], sen.min, sen.max);
  });

  updatePumpBtn(s.pump.enabled);
  [['delta', 'delta'], ['noise', 'noise']].forEach(([k, sk]) => {
    const range = $(`card-pump`).querySelector(`input[data-k="${k}"]`);
    const num = $(`card-pump`).querySelector(`input[data-num="${k}"]`);
    if (document.activeElement !== range && document.activeElement !== num) {
      range.value = s.pump[sk];
      num.value = s.pump[sk];
    }
  });
  $('pump-effect').textContent = s.pump.enabled ? '正在增加湿度' : '';

  // 后端命令日志
  $('cmd-count').textContent = (s.commands || []).length;
  const log = $('cmd-log');
  if (!(s.commands || []).length) {
    log.textContent = '(等待后端下发命令…)';
  } else {
    log.innerHTML = s.commands.map(c => {
      const t = new Date(c.ts * 1000).toLocaleTimeString('zh-CN', { hour12: false });
      const d = c.durationSeconds ? ` · ${c.durationSeconds}s` : '';
      return `<div style="margin:2px 0">[${t}] <b style="color:${c.state === '开' ? '#22c55e' : '#f87171'}">泵${c.state}</b> ${c.action}${d} · ${c.commandId}<br><span style="color:#64748b">${c.topic}${c.reason ? ' · ' + c.reason : ''}</span></div>`;
    }).join('');
  }

  $('pub-count').textContent = s.state.publish_count;
  const err = $('pub-err');
  if (s.state.last_error) {
    err.className = 'err';
    err.textContent = '⚠ ' + s.state.last_error;
  } else {
    err.className = '';
    err.textContent = '';
  }
  $('last-payload').textContent = s.state.last_payload
    ? JSON.stringify(s.state.last_payload, null, 2)
    : '(等待发布…)';
}

async function refresh() {
  try {
    const r = await fetch('/api/state');
    state = await r.json();
    render(state);
  } catch (e) { /* 服务重启中 */ }
}
setInterval(refresh, 500);
refresh();
</script>
</body>
</html>
"""

# ---------------------------------------------------------------- main
def main():
    ap = argparse.ArgumentParser(description="MQTT 传感器/水泵模拟器")
    ap.add_argument("--host", default="0.0.0.0")
    ap.add_argument("--port", type=int, default=8090)
    args = ap.parse_args()

    global _tick_thread
    _tick_thread = threading.Thread(target=_tick_loop, daemon=True)
    _tick_thread.start()

    server = ThreadingHTTPServer((args.host, args.port), Handler)
    print(f"[sim] UI: http://{args.host}:{args.port}", flush=True)
    print(f"[sim] 默认发布到 {CFG['prefix']}/{CFG['owner_id']}/{CFG['device_sn']}/telemetry @ {CFG['broker']}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        _stop_event.set()
        if _client is not None:
            try:
                _client.disconnect()
            except Exception:
                pass
        server.server_close()

if __name__ == "__main__":
    main()
