"""定时任务调度：触发时间计算（interval / cron / once）。

纯函数模块，不依赖外部库：
- interval：每 N 秒，next = last + N
- once：指定时刻，触发后无下一次
- cron：5 段式（分 时 日 月 周），支持 * 与数字；简化实现（不含 */步进与范围，够用即可）

设计：按"下一次触发时刻"驱动调度索引（ZSET agent:task:schedule），
worker 到期取出任务触发 agent。
"""
from __future__ import annotations

import datetime
from typing import Optional

# 5 段 cron 的字段区间
_CRON_RANGES = (
    (0, 59),    # 分
    (0, 23),    # 时
    (1, 31),    # 日
    (1, 12),    # 月
    (0, 6),     # 周（0=周日）
)


class TriggerError(ValueError):
    """trigger 参数非法。"""


def _parse_trigger_type(trigger_type: str) -> str:
    t = (trigger_type or "").strip().lower()
    if t not in ("interval", "cron", "once"):
        raise TriggerError(f"trigger_type 必须为 interval/cron/once，收到: {trigger_type!r}")
    return t


def _validate_cron(expr: str) -> list:
    """解析 5 段 cron 表达式为各字段取值集合；非法抛 TriggerError。

    返回 [(values, 是否通配)]，如 ('0',False) / ('*',True)。
    简化实现：每段支持 * 或单个数字。
    """
    parts = expr.split()
    if len(parts) != 5:
        raise TriggerError(f"cron 表达式必须为 5 段（分 时 日 月 周），收到: {expr!r}")
    fields: list[tuple[int, bool]] = []
    for idx, part in enumerate(parts):
        lo, hi = _CRON_RANGES[idx]
        if part == "*":
            fields.append((0, True))  # 通配
        elif part.isdigit():
            value = int(part)
            if not (lo <= value <= hi):
                raise TriggerError(f"cron 第 {idx + 1} 段 {part!r} 超出范围 {lo}-{hi}")
            fields.append((value, False))
        else:
            raise TriggerError(
                f"cron 第 {idx + 1} 段 {part!r} 非法（仅支持 * 或数字，如 \"0 8 * * *\"）"
            )
    return fields


def _next_cron(expr: str, after: datetime.datetime) -> datetime.datetime:
    """计算 cron 表达式的下一次触发时刻（>= after，秒级归零）。"""
    minute, hour, day, month, weekday = _validate_cron(expr)
    # 简化实现：从 after 起按分钟步进扫描（上限防死循环）
    cursor = after.replace(second=0, microsecond=0) + datetime.timedelta(minutes=1)
    for _ in range(24 * 60 * 366):  # 最多扫一年
        if ((minute[1] or cursor.minute == minute[0]) and
                (hour[1] or cursor.hour == hour[0]) and
                (day[1] or cursor.day == day[0]) and
                (month[1] or cursor.month == month[0]) and
                (weekday[1] or cursor.weekday() == weekday[0])):
            return cursor
        cursor += datetime.timedelta(minutes=1)
    raise TriggerError(f"cron 表达式 {expr!r} 在一年内无触发时刻")


def next_run(trigger_type: str, trigger: str, last_run_at: Optional[str] = None,
              now: Optional[datetime.datetime] = None) -> Optional[float]:
    """计算下一次触发时间（毫秒时间戳，用于 ZSET score）。

    返回 None 表示不再触发（once 执行后）。
    """
    t = _parse_trigger_type(trigger_type)
    base = now or datetime.datetime.now(datetime.timezone.utc)
    if t == "interval":
        try:
            seconds = float(trigger)
        except (TypeError, ValueError):
            raise TriggerError(f"interval 的 trigger 必须是秒数，收到: {trigger!r}")
        if seconds <= 0:
            raise TriggerError("interval 秒数必须 > 0")
        # 距上次执行间隔 seconds；无上次则以当前为基准
        anchor = last_run_at
        if anchor:
            try:
                last_dt = datetime.datetime.fromisoformat(anchor)
                if last_dt.tzinfo is None:
                    last_dt = last_dt.replace(tzinfo=datetime.timezone.utc)
            except ValueError:
                last_dt = base
            return (last_dt + datetime.timedelta(seconds=seconds)).timestamp() * 1000
        return (base + datetime.timedelta(seconds=seconds)).timestamp() * 1000
    if t == "once":
        try:
            target = datetime.datetime.fromisoformat(trigger.replace("Z", "+00:00"))
        except (TypeError, ValueError):
            raise TriggerError(
                f"once 的 trigger 必须是 ISO 时间，如 2026-09-01T08:00:00+08:00，收到: {trigger!r}"
            )
        if target.tzinfo is None:
            target = target.replace(tzinfo=datetime.timezone.utc)
        if target <= base:
            return None  # 已过 → 不再触发
        return target.timestamp() * 1000
    if t == "cron":
        nxt = _next_cron(trigger, base)
        return nxt.timestamp() * 1000
    return None  # pragma: no cover
