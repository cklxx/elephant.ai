"""Shared helpers for Lark tenant auth and API requests."""

from __future__ import annotations

import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
from typing import Any

_LARK_BASE = "https://open.feishu.cn/open-apis"
_TOKEN_CACHE: dict[str, Any] = {
    "token": "",
    "expires_at": 0.0,
}
_TOKEN_REFRESH_SKEW_SECONDS = 60
_TOKEN_ERROR_CODES = {
    99991661,  # tenant_access_token expired
    99991663,  # invalid tenant_access_token
    99991664,
    99991665,
    99991668,
}


def _strip_quotes(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value


def _load_alex_lark_credentials() -> tuple[str, str]:
    config_path = Path.home() / ".alex" / "config.yaml"
    if not config_path.is_file():
        return "", ""

    try:
        text = config_path.read_text(encoding="utf-8")
    except Exception:
        return "", ""

    app_id = ""
    app_secret = ""
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or ":" not in stripped:
            continue
        key, raw = stripped.split(":", 1)
        key = key.strip()
        value = _strip_quotes(raw.strip())
        if not value:
            continue
        if key in {"app_id", "lark_app_id"} and not app_id:
            app_id = value
        elif key in {"app_secret", "lark_app_secret"} and not app_secret:
            app_secret = value
        if app_id and app_secret:
            break
    return app_id, app_secret


def _resolve_lark_credentials() -> tuple[str, str]:
    app_id = os.environ.get("LARK_APP_ID", "").strip()
    app_secret = os.environ.get("LARK_APP_SECRET", "").strip()
    if app_id and app_secret:
        return app_id, app_secret

    cfg_app_id, cfg_app_secret = _load_alex_lark_credentials()
    return app_id or cfg_app_id, app_secret or cfg_app_secret


def _parse_json(text: str) -> dict:
    try:
        data = json.loads(text)
        if isinstance(data, dict):
            return data
        return {"data": data}
    except Exception:
        return {}


def get_lark_tenant_token(*, force_refresh: bool = False, timeout: int = 15) -> tuple[str, str | None]:
    """Resolve tenant token from env or by exchanging app credentials."""
    env_token = os.environ.get("LARK_TENANT_TOKEN", "").strip()
    if env_token and not force_refresh:
        return env_token, None

    now = time.time()
    cached_token = str(_TOKEN_CACHE.get("token", "")).strip()
    cached_expiry = float(_TOKEN_CACHE.get("expires_at", 0.0))
    if not force_refresh and cached_token and cached_expiry > now:
        return cached_token, None

    app_id, app_secret = _resolve_lark_credentials()
    if not app_id or not app_secret:
        return "", "LARK_TENANT_TOKEN not set and LARK_APP_ID/LARK_APP_SECRET not set"

    body = json.dumps({"app_id": app_id, "app_secret": app_secret}).encode()
    req = urllib.request.Request(
        f"{_LARK_BASE}/auth/v3/tenant_access_token/internal",
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            payload = _parse_json(resp.read().decode())
    except urllib.error.HTTPError as exc:
        raw = ""
        try:
            raw = exc.read().decode()
        except Exception:
            raw = ""
        data = _parse_json(raw)
        if data:
            message = data.get("msg") or data.get("message") or f"HTTP Error {exc.code}"
            return "", f"Lark auth failed: {message}"
        return "", f"Lark auth failed: HTTP Error {exc.code}"
    except urllib.error.URLError as exc:
        return "", f"Lark auth failed: {exc}"

    if payload.get("code", 0) != 0:
        message = payload.get("msg") or payload.get("message") or f"code={payload.get('code')}"
        return "", f"Lark auth failed: {message}"

    token = str(payload.get("tenant_access_token", "")).strip()
    if not token:
        return "", "Lark auth failed: tenant_access_token missing in response"

    expire = int(payload.get("expire", 3600))
    _TOKEN_CACHE["token"] = token
    _TOKEN_CACHE["expires_at"] = now + max(expire - _TOKEN_REFRESH_SKEW_SECONDS, 60)
    return token, None


def _auth_error(code: int, payload: dict) -> bool:
    if code == 401:
        return True
    p_code = payload.get("code")
    if isinstance(p_code, int) and p_code in _TOKEN_ERROR_CODES:
        return True
    msg = str(payload.get("msg") or payload.get("message") or "").lower()
    return "tenant_access_token" in msg and ("invalid" in msg or "expire" in msg)


def _build_url(path: str, query: dict | str | None = None) -> str:
    path = path if path.startswith("/") else f"/{path}"
    url = f"{_LARK_BASE}{path}"
    if query is None:
        return url
    if isinstance(query, str):
        if not query:
            return url
        if query.startswith("?"):
            return f"{url}{query}"
        return f"{url}?{query}"
    encoded = urllib.parse.urlencode(query, doseq=True)
    if not encoded:
        return url
    return f"{url}?{encoded}"


def _request_lark(
    *,
    token: str,
    method: str,
    path: str,
    body: dict | None,
    query: dict | str | None,
    timeout: int,
) -> dict:
    url = _build_url(path, query)
    data = json.dumps(body).encode() if body is not None else None
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    req = urllib.request.Request(url, data=data, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode()
            try:
                payload = json.loads(raw)
            except Exception:
                return {"error": "invalid JSON response from Lark API"}
            if isinstance(payload, dict):
                return payload
            return {"data": payload}
    except urllib.error.HTTPError as exc:
        raw = ""
        try:
            raw = exc.read().decode()
        except Exception:
            raw = ""
        payload = _parse_json(raw)
        if payload:
            payload.setdefault("error", payload.get("msg") or payload.get("message") or f"HTTP Error {exc.code}")
            payload.setdefault("http_status", exc.code)
            return payload
        return {"error": f"HTTP Error {exc.code}: {raw or str(exc)}", "http_status": exc.code}
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def lark_api_json(
    method: str,
    path: str,
    body: dict | None = None,
    *,
    query: dict | str | None = None,
    timeout: int = 15,
    retry_on_auth_error: bool = True,
) -> dict:
    token, err = get_lark_tenant_token(timeout=timeout)
    if err:
        return {"error": err}

    result = _request_lark(
        token=token,
        method=method,
        path=path,
        body=body,
        query=query,
        timeout=timeout,
    )
    if not retry_on_auth_error or "error" not in result:
        return result

    http_status = int(result.get("http_status", 0) or 0)
    if not _auth_error(http_status, result):
        return result

    refreshed_token, refresh_err = get_lark_tenant_token(force_refresh=True, timeout=timeout)
    if refresh_err:
        return result

    return _request_lark(
        token=refreshed_token,
        method=method,
        path=path,
        body=body,
        query=query,
        timeout=timeout,
    )
