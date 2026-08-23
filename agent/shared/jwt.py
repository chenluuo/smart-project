"""JWT 解析/校验/签发（与 Go 侧同算法、共享 secret）。

- 解析仅校验签名与过期，不做角色判定（权限由 Go 接口按 JWT 数据范围强制）。
- 签发用于"以用户身份"的主动处理（如告警推送触发 agent），
  校验规则与 Go identity.Parse 一致：iss=smart-agriculture-api、sub=uid 字符串、
  accountName/role 非空、iat/exp 有效。
密钥经环境变量 ${JWT_SECRET} 注入。
"""
from __future__ import annotations

import datetime
import time
from typing import Any

import jwt

from shared.config import get_config


class JWTError(RuntimeError):
    pass


def _secret() -> str:
    cfg = get_config("auth")
    secret = cfg.get("jwt_secret")
    if not secret:
        raise JWTError("未配置 JWT_SECRET")
    return secret


def issue_user_token(
    user_id: str | int,
    account_name: str | None = None,
    role: str = "FARMER",
    issuer: str = "smart-agriculture-api",
    ttl_seconds: int = 3600,
) -> str:
    """以指定用户身份签发 JWT（Go 侧可校验通过）。"""
    uid = str(user_id)
    now = int(time.time())
    payload = {
        "uid": int(uid),
        "accountName": account_name or f"user{uid}",
        "role": role,
        "iss": issuer,
        "sub": uid,
        "iat": now - 60,          # Go 校验 iat 不允许超 now+60
        "exp": now + ttl_seconds,
    }
    return jwt.encode(payload, _secret(), algorithm="HS256")


def decode_token(token: str) -> dict[str, Any]:
    """解析 JWT，返回 payload（含 user_id / sub 等）。失败抛 JWTError。"""
    secret = _secret()
    try:
        # verify_iat=False：Go 容器时钟为 UTC，与本地时钟存在偏差时
        # iat 可能被判"未来"导致 ImmatureSignatureError；签名与 exp 仍严格校验
        return jwt.decode(
            token, secret,
            algorithms=["HS256"],
            options={"verify_iat": False},
        )
    except jwt.PyJWTError as e:
        raise JWTError(f"JWT 无效: {e}") from e


def user_id_from_token(token: str) -> str:
    payload = decode_token(token)
    for key in ("user_id", "sub", "id", "uid"):
        if payload.get(key):
            return str(payload[key])
    raise JWTError("JWT payload 缺少用户标识")


def is_expired(payload: dict[str, Any]) -> bool:
    exp = payload.get("exp")
    if not exp:
        return False
    return datetime.datetime.fromtimestamp(exp, tz=datetime.timezone.utc) < datetime.datetime.now(
        tz=datetime.timezone.utc
    )
