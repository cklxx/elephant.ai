#!/usr/bin/env python3
"""Unified Feishu CLI for local agent skills.

Design goals:
- Single CLI entry point for Feishu auth + Open API calls + common tool actions.
- Progressive help so LLM/agents can self-discover commands incrementally.
- Built-in token cache and OAuth flow support with redirect-uri precheck.
"""

from __future__ import annotations

import argparse
import copy
import json
import mimetypes
import os
import secrets
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Callable

_SCRIPTS_ROOT = Path(__file__).resolve().parents[2]
if str(_SCRIPTS_ROOT) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_ROOT))

from skill_runner import lark_auth as legacy_lark_auth

OPEN_API_DEFAULT_BASE = "https://open.feishu.cn/open-apis"
TOKEN_REFRESH_SKEW_SECONDS = 60
TOKEN_ERROR_CODES = {
    99991661,
    99991663,
    99991664,
    99991665,
    99991668,
}
COMMAND_CHOICES = ("help", "auth", "tool", "api")
HELP_TOPICS = ("overview", "auth", "modules", "module", "action")
AUTH_SUBCOMMANDS = ("status", "tenant_token", "oauth_url", "exchange_code", "refresh_user", "user_token")


@dataclass
class TokenStore:
    path: Path = field(default_factory=lambda: _resolve_token_store_path())

    def load(self) -> dict[str, Any]:
        if not self.path.is_file():
            return {"tenant": {}, "app": {}, "users": {}}
        try:
            payload = json.loads(self.path.read_text(encoding="utf-8"))
        except Exception:
            return {"tenant": {}, "app": {}, "users": {}}
        if not isinstance(payload, dict):
            return {"tenant": {}, "app": {}, "users": {}}
        payload.setdefault("tenant", {})
        payload.setdefault("app", {})
        payload.setdefault("users", {})
        if not isinstance(payload["users"], dict):
            payload["users"] = {}
        return payload

    def save(self, payload: dict[str, Any]) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        temp_path = self.path.with_suffix(self.path.suffix + ".tmp")
        temp_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
        temp_path.replace(self.path)


def _resolve_token_store_path() -> Path:
    raw = os.environ.get("LARK_TOKEN_STORE", "").strip() or os.environ.get("FEISHU_TOKEN_STORE", "").strip()
    if raw:
        return Path(raw).expanduser().resolve()
    return (Path.home() / ".alex" / "feishu" / "tokens.json").resolve()


def _strip_quotes(value: str) -> str:
    text = value.strip()
    if len(text) >= 2 and text[0] == text[-1] and text[0] in {"'", '"'}:
        return text[1:-1]
    return text


def _read_simple_yaml_scalars(path: Path) -> dict[str, str]:
    if not path.is_file():
        return {}

    values: dict[str, str] = {}
    stack: list[tuple[int, str]] = []

    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.rstrip()
        stripped = line.strip()
        if not stripped or stripped.startswith("#") or ":" not in stripped:
            continue

        indent = len(line) - len(line.lstrip(" "))
        while stack and stack[-1][0] >= indent:
            stack.pop()

        key_raw, value_raw = stripped.split(":", 1)
        key = key_raw.strip()
        value = _strip_quotes(value_raw.strip())

        if not value:
            stack.append((indent, key))
            continue

        dotted = ".".join([item[1] for item in stack] + [key])
        values[dotted] = value
        values.setdefault(key, value)

    return values


def _split_csv_like(value: str) -> list[str]:
    if not value.strip():
        return []
    items: list[str] = []
    for chunk in value.replace("\n", ",").split(","):
        normalized = chunk.strip()
        if normalized:
            items.append(normalized)
    return items


def _mask_token(token: str) -> str:
    if not token:
        return ""
    if len(token) <= 10:
        return "*" * len(token)
    return f"{token[:4]}...{token[-4:]}"


def _json_request(
    url: str,
    *,
    method: str,
    body: dict[str, Any] | None,
    headers: dict[str, str] | None,
    timeout: int,
) -> dict[str, Any]:
    request_headers = {"Content-Type": "application/json"}
    if headers:
        request_headers.update(headers)

    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, headers=request_headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        raw = ""
        try:
            raw = exc.read().decode("utf-8")
        except Exception:
            raw = ""
        parsed = _parse_json_dict(raw)
        if parsed:
            parsed.setdefault("http_status", exc.code)
            parsed.setdefault("error", parsed.get("msg") or parsed.get("message") or f"HTTP Error {exc.code}")
            return parsed
        return {"http_status": exc.code, "error": f"HTTP Error {exc.code}: {raw or str(exc)}"}
    except urllib.error.URLError as exc:
        return {"error": str(exc)}

    parsed = _parse_json_dict(raw)
    if parsed:
        return parsed
    return {"error": "invalid JSON response", "raw": raw}


def _parse_json_dict(raw: str) -> dict[str, Any]:
    try:
        data = json.loads(raw)
    except Exception:
        return {}
    if isinstance(data, dict):
        return data
    return {"data": data}


def _build_open_api_base() -> str:
    raw = (
        os.environ.get("LARK_OPEN_BASE_URL", "").strip()
        or os.environ.get("FEISHU_OPEN_BASE_URL", "").strip()
        or OPEN_API_DEFAULT_BASE
    )
    normalized = raw.rstrip("/")
    if normalized.endswith("/open-apis"):
        return normalized
    return normalized + "/open-apis"


def _origin_from_open_api_base(base: str) -> str:
    parsed = urllib.parse.urlparse(base)
    if not parsed.scheme or not parsed.netloc:
        return "https://open.feishu.cn"
    return f"{parsed.scheme}://{parsed.netloc}"


def _canonical_redirect_uri(uri: str) -> str:
    text = uri.strip()
    parsed = urllib.parse.urlparse(text)
    if not parsed.scheme or not parsed.netloc:
        return ""
    path = parsed.path.rstrip("/") or "/"
    if parsed.query:
        return f"{parsed.scheme.lower()}://{parsed.netloc.lower()}{path}?{parsed.query}"
    return f"{parsed.scheme.lower()}://{parsed.netloc.lower()}{path}"


class AuthManager:
    def __init__(self, *, token_store: TokenStore | None = None, now_fn: Callable[[], float] | None = None):
        self._open_api_base = _build_open_api_base()
        self._origin = _origin_from_open_api_base(self._open_api_base)
        self._token_store = token_store or TokenStore()
        self._now = now_fn or time.time
        self._config = _read_simple_yaml_scalars(Path.home() / ".alex" / "config.yaml")

    def _resolve_app_credentials(self) -> tuple[str, str]:
        app_id = (
            os.environ.get("LARK_APP_ID", "").strip()
            or os.environ.get("FEISHU_APP_ID", "").strip()
            or self._config.get("channels.lark.app_id", "").strip()
            or self._config.get("lark_app_id", "").strip()
            or self._config.get("feishu_app_id", "").strip()
            or self._config.get("app_id", "").strip()
        )
        app_secret = (
            os.environ.get("LARK_APP_SECRET", "").strip()
            or os.environ.get("FEISHU_APP_SECRET", "").strip()
            or self._config.get("channels.lark.app_secret", "").strip()
            or self._config.get("lark_app_secret", "").strip()
            or self._config.get("feishu_app_secret", "").strip()
            or self._config.get("app_secret", "").strip()
        )
        return app_id, app_secret

    def _resolve_redirect_uri(self, requested: str) -> str:
        if requested.strip():
            return requested.strip()
        return (
            os.environ.get("FEISHU_OAUTH_REDIRECT_URI", "").strip()
            or os.environ.get("LARK_OAUTH_REDIRECT_URI", "").strip()
            or self._config.get("channels.lark.oauth_redirect_uri", "").strip()
            or self._config.get("lark_oauth_redirect_uri", "").strip()
            or self._config.get("oauth_redirect_uri", "").strip()
        )

    def _resolve_redirect_allowlist(self) -> list[str]:
        raw = (
            os.environ.get("FEISHU_OAUTH_REDIRECT_ALLOWLIST", "").strip()
            or os.environ.get("LARK_OAUTH_REDIRECT_ALLOWLIST", "").strip()
            or self._config.get("channels.lark.oauth_redirect_allowlist", "").strip()
            or self._config.get("lark_oauth_redirect_allowlist", "").strip()
            or self._config.get("oauth_redirect_allowlist", "").strip()
        )
        if not raw:
            return []
        normalized = []
        for item in _split_csv_like(raw):
            canonical = _canonical_redirect_uri(item)
            if canonical:
                normalized.append(canonical)
        return normalized

    def _resolve_scopes(self, scopes: list[str] | str | None) -> list[str]:
        if isinstance(scopes, list):
            items = [str(item).strip() for item in scopes if str(item).strip()]
            if items:
                return items
        if isinstance(scopes, str) and scopes.strip():
            return _split_csv_like(scopes)
        fallback = (
            os.environ.get("FEISHU_OAUTH_SCOPES", "").strip()
            or os.environ.get("LARK_OAUTH_SCOPES", "").strip()
            or self._config.get("channels.lark.oauth_scopes", "").strip()
            or self._config.get("lark_oauth_scopes", "").strip()
        )
        return _split_csv_like(fallback)

    def _load_state(self) -> dict[str, Any]:
        return self._token_store.load()

    def _save_state(self, payload: dict[str, Any]) -> None:
        self._token_store.save(payload)

    def _is_fresh(self, expires_at: float) -> bool:
        return expires_at > self._now()

    def status(self) -> dict[str, Any]:
        app_id, app_secret = self._resolve_app_credentials()
        state = self._load_state()
        now = self._now()
        tenant_token, tenant_err = legacy_lark_auth.get_lark_tenant_token(timeout=5)

        tenant = state.get("tenant", {})
        users = state.get("users", {})
        user_rows = []
        for key, item in users.items():
            if not isinstance(item, dict):
                continue
            expires_at = float(item.get("expires_at", 0.0) or 0.0)
            refresh_expires_at = float(item.get("refresh_expires_at", 0.0) or 0.0)
            user_rows.append(
                {
                    "user_key": key,
                    "display_name": item.get("name", ""),
                    "token_preview": _mask_token(str(item.get("access_token", ""))),
                    "expires_at": int(expires_at),
                    "expires_in_seconds": max(int(expires_at - now), 0),
                    "refresh_expires_at": int(refresh_expires_at),
                    "refresh_expires_in_seconds": max(int(refresh_expires_at - now), 0),
                }
            )

        allowlist = self._resolve_redirect_allowlist()
        return {
            "success": True,
            "open_api_base": self._open_api_base,
            "auth": {
                "app_id": app_id,
                "has_app_secret": bool(app_secret),
                "redirect_allowlist_count": len(allowlist),
                "redirect_allowlist": allowlist,
            },
            "tenant": {
                "token_preview": _mask_token(tenant_token or str(tenant.get("access_token", ""))),
                "expires_at": int(float(tenant.get("expires_at", 0.0) or 0.0)),
                "expires_in_seconds": max(int(float(tenant.get("expires_at", 0.0) or 0.0) - now), 0),
                "source": "legacy_lark_auth",
                "error": tenant_err or "",
            },
            "users": user_rows,
            "token_store": str(self._token_store.path),
        }

    def get_tenant_token(self, *, force_refresh: bool = False, timeout: int = 15) -> tuple[str, str | None]:
        token, err = legacy_lark_auth.get_lark_tenant_token(force_refresh=force_refresh, timeout=timeout)
        if err:
            return "", err
        return token, None

    def _get_app_access_token(self, *, force_refresh: bool = False, timeout: int = 15) -> tuple[str, str | None]:
        state = self._load_state()
        app_state = state.get("app", {})
        cached_token = str(app_state.get("access_token", "")).strip()
        cached_expiry = float(app_state.get("expires_at", 0.0) or 0.0)
        if cached_token and self._is_fresh(cached_expiry) and not force_refresh:
            return cached_token, None

        app_id, app_secret = self._resolve_app_credentials()
        if not app_id or not app_secret:
            return "", "LARK_APP_ID/LARK_APP_SECRET not configured"

        payload = _json_request(
            f"{self._open_api_base}/auth/v3/app_access_token/internal",
            method="POST",
            body={"app_id": app_id, "app_secret": app_secret},
            headers=None,
            timeout=timeout,
        )
        if payload.get("code", 0) != 0:
            message = payload.get("msg") or payload.get("message") or payload.get("error") or "app auth failed"
            return "", f"app auth failed: {message}"

        token = str(payload.get("app_access_token", "")).strip()
        if not token:
            return "", "app auth failed: app_access_token missing"

        expire = int(payload.get("expire", 7200) or 7200)
        state["app"] = {
            "access_token": token,
            "expires_at": self._now() + max(expire - TOKEN_REFRESH_SKEW_SECONDS, 60),
        }
        self._save_state(state)
        return token, None

    def _pick_user_key(self, user_key: str) -> str:
        if user_key.strip():
            return user_key.strip()
        from_env = os.environ.get("FEISHU_DEFAULT_USER_KEY", "").strip()
        if from_env:
            return from_env
        users = self._load_state().get("users", {})
        for key in users:
            if str(key).strip():
                return str(key).strip()
        return ""

    def get_user_access_token(
        self,
        *,
        user_key: str = "",
        force_refresh: bool = False,
        timeout: int = 15,
    ) -> tuple[str, str | None]:
        env_token = os.environ.get("LARK_USER_ACCESS_TOKEN", "").strip()
        if env_token and not force_refresh:
            return env_token, None

        key = self._pick_user_key(user_key)
        if not key:
            return "", "no user token found; run auth oauth_url + auth exchange_code first"

        state = self._load_state()
        users = state.get("users", {})
        entry = users.get(key, {}) if isinstance(users, dict) else {}
        if not isinstance(entry, dict):
            entry = {}
        cached_access = str(entry.get("access_token", "")).strip()
        cached_expiry = float(entry.get("expires_at", 0.0) or 0.0)
        if cached_access and self._is_fresh(cached_expiry) and not force_refresh:
            return cached_access, None

        refresh_token = str(entry.get("refresh_token", "")).strip()
        if not refresh_token:
            return "", f"user token for {key} missing refresh_token; run auth exchange_code again"

        refreshed = self.refresh_user_token(user_key=key, timeout=timeout)
        if not refreshed.get("success"):
            return "", str(refreshed.get("error", "refresh user token failed"))
        token = str(refreshed.get("access_token", "")).strip()
        if not token:
            return "", "refresh succeeded but access_token missing"
        return token, None

    def build_oauth_url(
        self,
        *,
        redirect_uri: str,
        scopes: list[str] | str | None,
        state: str,
    ) -> dict[str, Any]:
        resolved_redirect_uri = self._resolve_redirect_uri(redirect_uri)
        if not resolved_redirect_uri:
            return {
                "success": False,
                "error": "redirect_uri is required (set FEISHU_OAUTH_REDIRECT_URI or pass redirect_uri)",
            }

        canonical_redirect = _canonical_redirect_uri(resolved_redirect_uri)
        if not canonical_redirect:
            return {"success": False, "error": f"invalid redirect_uri: {resolved_redirect_uri}"}

        allowlist = self._resolve_redirect_allowlist()
        if not allowlist:
            return {
                "success": False,
                "error": "redirect_uri allowlist is empty; configure FEISHU_OAUTH_REDIRECT_ALLOWLIST before generating OAuth URL",
            }
        if canonical_redirect not in allowlist:
            return {
                "success": False,
                "error": "redirect_uri not in allowlist; update app whitelist first",
                "redirect_uri": canonical_redirect,
                "allowlist": allowlist,
            }

        scope_items = self._resolve_scopes(scopes)
        if not scope_items:
            return {
                "success": False,
                "error": "scopes required (set FEISHU_OAUTH_SCOPES or pass scopes)",
            }

        oauth_state = state.strip() or secrets.token_urlsafe(16)
        params = {
            "app_id": self._resolve_app_credentials()[0],
            "redirect_uri": canonical_redirect,
            "scope": " ".join(scope_items),
            "state": oauth_state,
        }
        if not params["app_id"]:
            return {"success": False, "error": "LARK_APP_ID not configured"}

        url = f"{self._origin}/open-apis/authen/v1/authorize?{urllib.parse.urlencode(params)}"
        return {
            "success": True,
            "url": url,
            "redirect_uri": canonical_redirect,
            "scopes": scope_items,
            "state": oauth_state,
        }

    def exchange_code(self, *, code: str, redirect_uri: str, timeout: int = 15) -> dict[str, Any]:
        auth_code = code.strip()
        if not auth_code:
            return {"success": False, "error": "code is required"}

        oauth_url = self.build_oauth_url(
            redirect_uri=redirect_uri,
            scopes=["authen:openid"],
            state="precheck",
        )
        if not oauth_url.get("success"):
            return oauth_url

        app_token, app_err = self._get_app_access_token(timeout=timeout)
        if app_err:
            return {"success": False, "error": app_err}

        payload = _json_request(
            f"{self._open_api_base}/authen/v1/oidc/access_token",
            method="POST",
            body={
                "grant_type": "authorization_code",
                "code": auth_code,
                "redirect_uri": oauth_url["redirect_uri"],
            },
            headers={"Authorization": f"Bearer {app_token}"},
            timeout=timeout,
        )
        if payload.get("code", 0) != 0:
            message = payload.get("msg") or payload.get("message") or payload.get("error") or "exchange code failed"
            return {"success": False, "error": message, "raw": payload}

        data = payload.get("data", {})
        if not isinstance(data, dict):
            return {"success": False, "error": "exchange code failed: invalid response"}

        access_token = str(data.get("access_token", "")).strip()
        refresh_token = str(data.get("refresh_token", "")).strip()
        open_id = str(data.get("open_id", "")).strip()
        if not access_token or not open_id:
            return {"success": False, "error": "exchange code failed: access_token/open_id missing"}

        expires_in = int(data.get("expires_in", 7200) or 7200)
        refresh_expires_in = int(data.get("refresh_expires_in", 0) or 0)

        state = self._load_state()
        users = state.setdefault("users", {})
        users[open_id] = {
            "name": str(data.get("name", "")),
            "access_token": access_token,
            "expires_at": self._now() + max(expires_in - TOKEN_REFRESH_SKEW_SECONDS, 60),
            "refresh_token": refresh_token,
            "refresh_expires_at": self._now() + max(refresh_expires_in - TOKEN_REFRESH_SKEW_SECONDS, 0),
            "scope": data.get("scope", ""),
        }
        self._save_state(state)

        return {
            "success": True,
            "user_key": open_id,
            "access_token": access_token,
            "access_token_preview": _mask_token(access_token),
            "refresh_token_preview": _mask_token(refresh_token),
            "expires_in": expires_in,
            "refresh_expires_in": refresh_expires_in,
        }

    def refresh_user_token(self, *, user_key: str, timeout: int = 15) -> dict[str, Any]:
        key = self._pick_user_key(user_key)
        if not key:
            return {"success": False, "error": "user_key is required"}

        state = self._load_state()
        users = state.get("users", {})
        entry = users.get(key, {}) if isinstance(users, dict) else {}
        if not isinstance(entry, dict):
            return {"success": False, "error": f"user_key {key} not found"}

        refresh_token = str(entry.get("refresh_token", "")).strip()
        if not refresh_token:
            return {"success": False, "error": f"refresh_token missing for {key}"}

        app_token, app_err = self._get_app_access_token(timeout=timeout)
        if app_err:
            return {"success": False, "error": app_err}

        payload = _json_request(
            f"{self._open_api_base}/authen/v1/oidc/access_token",
            method="POST",
            body={
                "grant_type": "refresh_token",
                "refresh_token": refresh_token,
            },
            headers={"Authorization": f"Bearer {app_token}"},
            timeout=timeout,
        )
        if payload.get("code", 0) != 0:
            message = payload.get("msg") or payload.get("message") or payload.get("error") or "refresh failed"
            return {"success": False, "error": message, "raw": payload}

        data = payload.get("data", {})
        if not isinstance(data, dict):
            return {"success": False, "error": "refresh failed: invalid response"}

        access_token = str(data.get("access_token", "")).strip()
        if not access_token:
            return {"success": False, "error": "refresh failed: access_token missing"}

        next_refresh = str(data.get("refresh_token", "")).strip() or refresh_token
        expires_in = int(data.get("expires_in", 7200) or 7200)
        refresh_expires_in = int(data.get("refresh_expires_in", 0) or 0)

        users[key] = {
            "name": str(data.get("name", "") or entry.get("name", "")),
            "access_token": access_token,
            "expires_at": self._now() + max(expires_in - TOKEN_REFRESH_SKEW_SECONDS, 60),
            "refresh_token": next_refresh,
            "refresh_expires_at": self._now() + max(refresh_expires_in - TOKEN_REFRESH_SKEW_SECONDS, 0),
            "scope": data.get("scope", entry.get("scope", "")),
        }
        state["users"] = users
        self._save_state(state)

        return {
            "success": True,
            "user_key": key,
            "access_token": access_token,
            "access_token_preview": _mask_token(access_token),
            "refresh_token_preview": _mask_token(next_refresh),
            "expires_in": expires_in,
            "refresh_expires_in": refresh_expires_in,
        }


def _build_url(base: str, path: str, query: dict[str, Any] | str | None) -> str:
    normalized_path = path if path.startswith("/") else f"/{path}"
    url = f"{base}{normalized_path}"
    if query is None:
        return url
    if isinstance(query, str):
        query_text = query.strip()
        if not query_text:
            return url
        if query_text.startswith("?"):
            return f"{url}{query_text}"
        return f"{url}?{query_text}"

    encoded = urllib.parse.urlencode(query, doseq=True)
    if not encoded:
        return url
    return f"{url}?{encoded}"


def _is_auth_error(payload: dict[str, Any]) -> bool:
    status = int(payload.get("http_status", 0) or 0)
    if status == 401:
        return True

    code = payload.get("code")
    if isinstance(code, int) and code in TOKEN_ERROR_CODES:
        return True

    text = str(payload.get("msg") or payload.get("message") or payload.get("error") or "").lower()
    return "access_token" in text and ("invalid" in text or "expire" in text)


def api_request(
    method: str,
    path: str,
    body: dict[str, Any] | None = None,
    *,
    query: dict[str, Any] | str | None = None,
    timeout: int = 15,
    retry_on_auth_error: bool = True,
    auth: str = "tenant",
    user_key: str = "",
    auth_manager: AuthManager | None = None,
) -> dict[str, Any]:
    manager = auth_manager or AuthManager()

    if auth != "user":
        return legacy_lark_auth.lark_api_json(
            method,
            path,
            body,
            query=query,
            timeout=timeout,
            retry_on_auth_error=retry_on_auth_error,
        )

    def _resolve_token(force_refresh: bool) -> tuple[str, str | None]:
        return manager.get_user_access_token(user_key=user_key, force_refresh=force_refresh, timeout=timeout)

    token, token_err = _resolve_token(False)
    if token_err:
        return {"error": token_err}

    first = _json_request(
        _build_url(manager._open_api_base, path, query),
        method=method.upper(),
        body=body,
        headers={"Authorization": f"Bearer {token}"},
        timeout=timeout,
    )
    if not retry_on_auth_error or "error" not in first or not _is_auth_error(first):
        return first

    refreshed, refresh_err = _resolve_token(True)
    if refresh_err:
        return first

    return _json_request(
        _build_url(manager._open_api_base, path, query),
        method=method.upper(),
        body=body,
        headers={"Authorization": f"Bearer {refreshed}"},
        timeout=timeout,
    )


@dataclass(frozen=True)
class ActionSpec:
    summary: str
    required: tuple[str, ...]
    optional: tuple[str, ...]
    example: dict[str, Any]


def _api_failure(result: dict[str, Any]) -> dict[str, Any] | None:
    if "error" in result:
        return {"success": False, **result}

    code = result.get("code", 0)
    if isinstance(code, int) and code != 0:
        return {
            "success": False,
            "code": code,
            "error": result.get("msg") or result.get("message") or f"Feishu API error {code}",
        }
    return None


def _parse_ts(value: str) -> str | None:
    text = str(value).strip()
    if not text:
        return None
    if text.isdigit():
        return text

    normalized = text.replace(" ", "T")
    for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M", "%Y-%m-%d"):
        try:
            parsed = datetime.strptime(normalized, fmt)
            if fmt == "%Y-%m-%d":
                parsed = datetime.combine(parsed.date(), datetime.min.time())
            return str(int(parsed.replace(tzinfo=timezone.utc).timestamp()))
        except ValueError:
            continue

    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError:
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return str(int(parsed.timestamp()))


def _parse_duration_seconds(value: str | int | float | None) -> int:
    if isinstance(value, (int, float)):
        return max(int(value), 60)

    text = str(value or "60m").strip().lower()
    if text.endswith("m") and text[:-1].isdigit():
        return int(text[:-1]) * 60
    if text.endswith("h") and text[:-1].isdigit():
        return int(text[:-1]) * 3600
    if text.isdigit():
        return max(int(text), 60)
    return 3600


def _clamp_page_size(value: Any, *, default: int = 20, minimum: int = 1, maximum: int = 200) -> int:
    try:
        parsed = int(value)
    except (TypeError, ValueError):
        parsed = default
    if parsed < minimum:
        return minimum
    if parsed > maximum:
        return maximum
    return parsed


def _resolve_chat_id(args: dict[str, Any]) -> str:
    return (
        str(args.get("chat_id", "")).strip()
        or os.environ.get("LARK_CHAT_ID", "").strip()
        or os.environ.get("FEISHU_CHAT_ID", "").strip()
    )


def _resolve_reply_to_message_id(args: dict[str, Any]) -> str:
    candidate = str(args.get("reply_to_message_id", "")).strip() or str(args.get("message_id", "")).strip()
    if candidate.startswith("inject_"):
        return ""
    return candidate


def _message_text_payload(content: str) -> str:
    return json.dumps({"text": content}, ensure_ascii=False)


def _message_post_payload(content: str) -> str:
    lines = [line for line in str(content).splitlines() if line.strip()]
    if not lines:
        lines = [str(content)]
    rich_rows = [[{"tag": "text", "text": line}] for line in lines]
    payload = {"post": {"zh_cn": {"content": rich_rows}}}
    return json.dumps(payload, ensure_ascii=False)


def _select_im_file_type(file_name: str) -> str:
    extension = Path(file_name).suffix.lower().lstrip(".")
    if extension in {"opus", "mp4", "pdf", "doc", "xls", "ppt"}:
        return extension
    return "stream"


def _is_image_file(file_name: str) -> bool:
    extension = Path(file_name).suffix.lower().lstrip(".")
    return extension in {"jpg", "jpeg", "png", "gif", "webp", "bmp", "ico", "tif", "tiff"}


def _is_audio_file(file_name: str) -> bool:
    extension = Path(file_name).suffix.lower().lstrip(".")
    return extension in {"m4a", "mp3", "opus", "wav", "aac"}


def _resolve_source_file(args: dict[str, Any]) -> tuple[Path | None, dict[str, Any] | None]:
    source = str(args.get("source", "")).strip()
    source_kind = str(args.get("source_kind", "path")).strip().lower() or "path"
    if not source:
        return None, {"success": False, "error": "source is required"}
    if source_kind not in {"path", "attachment"}:
        return None, {"success": False, "error": "source_kind must be 'path' or 'attachment'"}

    # In CLI mode attachment aliases are treated as local file paths.
    if source_kind == "attachment" and source.startswith("[") and source.endswith("]"):
        source = source[1:-1].strip()
    path = Path(source).expanduser().resolve()
    if not path.exists() or not path.is_file():
        return None, {"success": False, "error": f"source file not found: {source}"}
    return path, None


def _multipart_json_request(
    url: str,
    *,
    token: str,
    fields: dict[str, str],
    file_field: str,
    file_name: str,
    file_bytes: bytes,
    timeout: int = 30,
) -> dict[str, Any]:
    boundary = f"----feishu-cli-{secrets.token_hex(12)}"
    mime_type = mimetypes.guess_type(file_name)[0] or "application/octet-stream"

    chunks: list[bytes] = []
    for key, value in fields.items():
        chunks.append(f"--{boundary}\r\n".encode("utf-8"))
        chunks.append(f'Content-Disposition: form-data; name="{key}"\r\n\r\n'.encode("utf-8"))
        chunks.append(str(value).encode("utf-8"))
        chunks.append(b"\r\n")

    chunks.append(f"--{boundary}\r\n".encode("utf-8"))
    chunks.append(
        f'Content-Disposition: form-data; name="{file_field}"; filename="{file_name}"\r\n'.encode("utf-8")
    )
    chunks.append(f"Content-Type: {mime_type}\r\n\r\n".encode("utf-8"))
    chunks.append(file_bytes)
    chunks.append(b"\r\n")
    chunks.append(f"--{boundary}--\r\n".encode("utf-8"))

    body = b"".join(chunks)
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {token}",
            "Content-Type": f"multipart/form-data; boundary={boundary}",
        },
    )

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        raw = ""
        try:
            raw = exc.read().decode("utf-8")
        except Exception:
            raw = ""
        parsed = _parse_json_dict(raw)
        if parsed:
            parsed.setdefault("http_status", exc.code)
            parsed.setdefault("error", parsed.get("msg") or parsed.get("message") or f"HTTP Error {exc.code}")
            return parsed
        return {"http_status": exc.code, "error": f"HTTP Error {exc.code}: {raw or str(exc)}"}
    except urllib.error.URLError as exc:
        return {"error": str(exc)}

    parsed = _parse_json_dict(raw)
    if parsed:
        return parsed
    return {"error": "invalid JSON response", "raw": raw}


def _resolve_bearer_token(auth_manager: AuthManager, *, auth: str = "tenant", user_key: str = "", timeout: int = 15) -> tuple[str, str | None]:
    if auth == "user":
        return auth_manager.get_user_access_token(user_key=user_key, timeout=timeout)
    return legacy_lark_auth.get_lark_tenant_token(timeout=timeout)


def _normalize_task_create_fields(args: dict[str, Any]) -> tuple[str, str]:
    summary = str(args.get("summary", "")).strip()
    description = str(args.get("description", "")).strip()
    content = str(args.get("content", "")).strip()
    if not description and content:
        description = content
    if summary:
        lines = summary.splitlines()
        if len(lines) > 1 and not description:
            summary = lines[0].strip()
            description = "\n".join(line.rstrip() for line in lines[1:]).strip()
    if not summary and description:
        first, *rest = description.splitlines()
        summary = first.strip()
        if rest:
            description = "\n".join(line.rstrip() for line in rest).strip()
        else:
            description = ""
    return summary, description


def _normalize_task_update_fields(args: dict[str, Any]) -> tuple[str, str]:
    summary = str(args.get("summary", "")).strip()
    description = str(args.get("description", "")).strip()
    content = str(args.get("content", "")).strip()
    if not description and content:
        description = content
    if summary and not description:
        lines = summary.splitlines()
        if len(lines) > 1:
            summary = lines[0].strip()
            description = "\n".join(line.rstrip() for line in lines[1:]).strip()
    return summary, description


def _parse_unix_seconds(value: Any, *, field_name: str) -> tuple[int | None, dict[str, Any] | None]:
    text = str(value or "").strip()
    if not text:
        return None, {"success": False, "error": f"{field_name} is required"}
    if not text.isdigit():
        return None, {"success": False, "error": f"{field_name} must be unix timestamp in seconds"}
    return int(text), None


def _build_task_due(args: dict[str, Any]) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    due_at = str(args.get("due_at", "")).strip()
    due_date = str(args.get("due_date", "")).strip()
    if due_at and due_date:
        return None, {"success": False, "error": "provide only one of due_at or due_date"}
    if due_date:
        try:
            parsed = datetime.strptime(due_date, "%Y-%m-%d")
        except ValueError:
            return None, {"success": False, "error": "due_date must be YYYY-MM-DD"}
        return {"timestamp": str(int(parsed.replace(tzinfo=timezone.utc).timestamp() * 1000)), "is_all_day": True}, None
    if due_at:
        seconds, due_err = _parse_unix_seconds(due_at, field_name="due_at")
        if due_err:
            return None, due_err
        return {"timestamp": str(seconds * 1000)}, None
    return None, None


def _task_request_mode(args: dict[str, Any]) -> tuple[str, str]:
    user_key = str(args.get("user_key", "")).strip()
    if user_key:
        return "user", user_key
    return "tenant", ""


def _resolve_calendar_id(args: dict[str, Any], auth_manager: AuthManager) -> tuple[str, dict[str, Any] | None]:
    provided = (
        str(args.get("calendar_id", "")).strip()
        or str(args.get("calendar_token", "")).strip()
        or os.environ.get("LARK_CALENDAR_ID", "").strip()
        or os.environ.get("FEISHU_CALENDAR_ID", "").strip()
    )
    if provided:
        return provided, None

    listing = api_request("GET", "/calendar/v4/calendars", auth_manager=auth_manager)
    failure = _api_failure(listing)
    if failure:
        return "primary", None

    data = listing.get("data", {})
    items = data.get("items", [])
    if not isinstance(items, list) or not items:
        items = data.get("calendar_list", [])
    if isinstance(items, list) and items:
        for item in items:
            if not isinstance(item, dict):
                continue
            calendar_type = str(item.get("type", "")).strip().lower()
            calendar_id = str(item.get("calendar_id", "")).strip()
            if calendar_type == "primary" and calendar_id:
                return calendar_id, None
        calendar_id = str(items[0].get("calendar_id", "")).strip()
        if calendar_id:
            return calendar_id, None

    return "primary", None


def _resolve_drive_default_folder() -> str:
    for key in ("LARK_DRIVE_FOLDER_TOKEN", "LARK_DRIVE_DEFAULT_FOLDER_TOKEN", "LARK_DRIVE_FOLDER_ID"):
        value = os.environ.get(key, "").strip()
        if value:
            return value
    return ""


def _resolve_drive_root_folder(auth_manager: AuthManager) -> str:
    result = api_request("GET", "/drive/explorer/v2/root_folder/meta", auth_manager=auth_manager)
    if _api_failure(result):
        return ""
    data = result.get("data", {})
    if not isinstance(data, dict):
        return ""
    return str(data.get("token") or data.get("root_folder_token") or data.get("folder_token") or "").strip()


def _resolve_bitable_app_token(args: dict[str, Any]) -> str:
    return str(args.get("app_token", "")).strip() or os.environ.get("LARK_BITABLE_APP_TOKEN", "").strip()


def _extract_bitable_app_token(data: dict[str, Any]) -> str:
    return str(data.get("app_token") or data.get("token") or data.get("app", {}).get("app_token") or "").strip()


def _is_transient_failure(failure: dict[str, Any]) -> bool:
    text = str(failure.get("error", "")).lower()
    status = int(failure.get("http_status", 0) or 0)
    return (
        "timed out" in text
        or "temporarily unavailable" in text
        or "connection reset" in text
        or status in {429, 500, 502, 503, 504}
    )


def _is_invalid_bitable_app_failure(failure: dict[str, Any]) -> bool:
    code = failure.get("code")
    if isinstance(code, int) and code in {91402, 91403, 1254040}:
        return True
    text = str(failure.get("error", "")).lower()
    return "notexist" in text or ("app_token" in text and ("invalid" in text or "not found" in text))


def _create_temp_bitable_app(args: dict[str, Any], auth_manager: AuthManager) -> tuple[str, dict[str, Any] | None]:
    prefix = str(args.get("app_name", "")).strip() or "elephant-cli"
    last_failure: dict[str, Any] | None = None

    for attempt in range(2):
        name = f"{prefix}-{int(time.time())}-{attempt}"
        result = api_request("POST", "/bitable/v1/apps", {"name": name}, auth_manager=auth_manager)
        failure = _api_failure(result)
        if failure:
            last_failure = failure
            if _is_transient_failure(failure) and attempt == 0:
                continue
            return "", failure

        app_token = _extract_bitable_app_token(result.get("data", {}))
        if app_token:
            return app_token, None
        last_failure = {"success": False, "error": "failed to create bitable app: app_token missing"}

    return "", (last_failure or {"success": False, "error": "failed to create bitable app"})


def _is_permission_failure(failure: dict[str, Any]) -> bool:
    code = failure.get("code")
    if isinstance(code, int) and code in {40003, 40004, 40013, 41050, 99991400, 99991401}:
        return True
    text = str(failure.get("error", "")).lower()
    return any(
        token in text
        for token in (
            "permission",
            "authority",
            "forbidden",
            "no dept authority",
            "insufficient scope",
            "access denied",
        )
    )


ToolHandler = Callable[[dict[str, Any], AuthManager], dict[str, Any]]


ACTION_SPECS: dict[str, dict[str, ActionSpec]] = {
    "message": {
        "send_message": ActionSpec(
            summary="Send message to current chat",
            required=("content",),
            optional=("chat_id", "content_format", "reply_to_message_id"),
            example={"content": "progress update", "content_format": "text"},
        ),
        "upload_file": ActionSpec(
            summary="Upload and send file/image/audio message",
            required=("source",),
            optional=("source_kind", "file_name", "chat_id", "reply_to_message_id", "max_bytes"),
            example={"source": "/tmp/report.pdf", "source_kind": "path"},
        ),
        "history": ActionSpec(
            summary="Fetch chat message history",
            required=(),
            optional=("chat_id", "page_size", "page_token", "start_time", "end_time"),
            example={"page_size": 20},
        ),
    },
    "calendar": {
        "create": ActionSpec(
            summary="Create calendar event",
            required=("title", "start"),
            optional=("duration", "description", "calendar_id", "timezone"),
            example={"title": "Weekly Sync", "start": "2026-03-06 10:00", "duration": "60m"},
        ),
        "query": ActionSpec(
            summary="Query calendar events",
            required=("start",),
            optional=("end", "calendar_id"),
            example={"start": "2026-03-06", "end": "2026-03-07"},
        ),
        "update": ActionSpec(
            summary="Update calendar event",
            required=("event_id",),
            optional=("summary", "description", "start_time", "end_time", "timezone", "calendar_id"),
            example={"event_id": "evt_xxx", "summary": "Updated title"},
        ),
        "delete": ActionSpec(
            summary="Delete calendar event",
            required=("event_id",),
            optional=("calendar_id",),
            example={"event_id": "evt_xxx"},
        ),
        "list_calendars": ActionSpec(
            summary="List accessible calendars",
            required=(),
            optional=(),
            example={},
        ),
    },
    "contact": {
        "get_user": ActionSpec(
            summary="Get a contact user",
            required=("user_id",),
            optional=("user_id_type",),
            example={"user_id": "ou_xxx", "user_id_type": "open_id"},
        ),
        "list_users": ActionSpec(
            summary="List department users",
            required=("department_id",),
            optional=("page_size", "page_token"),
            example={"department_id": "0", "page_size": 50},
        ),
        "get_department": ActionSpec(
            summary="Get department details",
            required=("department_id",),
            optional=(),
            example={"department_id": "0"},
        ),
        "list_departments": ActionSpec(
            summary="List sub-departments",
            required=(),
            optional=("parent_department_id", "page_size", "page_token"),
            example={"parent_department_id": "0", "page_size": 50},
        ),
        "list_scopes": ActionSpec(
            summary="Show contact permission scopes",
            required=(),
            optional=(),
            example={},
        ),
    },
    "doc": {
        "create": ActionSpec(
            summary="Create docx document",
            required=(),
            optional=("title", "folder_token", "content", "description"),
            example={"title": "Project Notes"},
        ),
        "read": ActionSpec(
            summary="Read document metadata",
            required=("document_id",),
            optional=(),
            example={"document_id": "doccnxxxx"},
        ),
        "read_content": ActionSpec(
            summary="Read raw document content",
            required=("document_id",),
            optional=(),
            example={"document_id": "doccnxxxx"},
        ),
        "list_blocks": ActionSpec(
            summary="List document blocks",
            required=("document_id",),
            optional=("page_size", "page_token"),
            example={"document_id": "doccnxxxx", "page_size": 20},
        ),
        "update_block_text": ActionSpec(
            summary="Update text content of a document block",
            required=("document_id", "block_id", "content"),
            optional=("document_revision_id", "client_token", "user_id_type"),
            example={"document_id": "doccnxxxx", "block_id": "blk_xxx", "content": "Updated content"},
        ),
        "write_markdown": ActionSpec(
            summary="Convert markdown and append to document",
            required=("document_id", "content"),
            optional=(),
            example={"document_id": "doccnxxxx", "content": "# Heading\nBody"},
        ),
    },
    "task": {
        "list": ActionSpec(
            summary="List my tasks",
            required=(),
            optional=("page_size", "page_token", "completed", "user_id_type", "user_key"),
            example={"page_size": 20},
        ),
        "list_subtasks": ActionSpec(
            summary="List subtasks under a parent task",
            required=("parent_task_id",),
            optional=("page_size", "page_token", "user_id_type", "user_key"),
            example={"parent_task_id": "task_xxx"},
        ),
        "create": ActionSpec(
            summary="Create task",
            required=("summary",),
            optional=("description", "content", "due_at", "due_date", "assignee_ids", "assignee_type", "client_token", "user_id_type", "user_key"),
            example={"summary": "Review PR", "description": "Before EOD"},
        ),
        "create_subtask": ActionSpec(
            summary="Create subtask under a parent task",
            required=("parent_task_id", "summary"),
            optional=("description", "content", "due_at", "due_date", "assignee_ids", "assignee_type", "client_token", "user_id_type", "user_key"),
            example={"parent_task_id": "task_xxx", "summary": "Prepare draft"},
        ),
        "update": ActionSpec(
            summary="Update task fields",
            required=("task_id",),
            optional=("summary", "description", "content", "due_time", "user_id_type", "user_key"),
            example={"task_id": "task_xxx", "summary": "Updated task title"},
        ),
        "delete": ActionSpec(
            summary="Delete task",
            required=("task_id",),
            optional=("user_id_type", "user_key"),
            example={"task_id": "task_xxx"},
        ),
    },
    "wiki": {
        "list_spaces": ActionSpec(
            summary="List wiki spaces",
            required=(),
            optional=("page_size", "page_token"),
            example={"page_size": 20},
        ),
        "list_nodes": ActionSpec(
            summary="List wiki nodes in a space",
            required=("space_id",),
            optional=("parent_node_token", "page_size"),
            example={"space_id": "xxx", "page_size": 20},
        ),
        "create_node": ActionSpec(
            summary="Create wiki node",
            required=("space_id",),
            optional=("obj_type", "parent_node_token", "title"),
            example={"space_id": "xxx", "obj_type": "docx", "title": "Spec"},
        ),
        "get_node": ActionSpec(
            summary="Get wiki node by token",
            required=("node_token",),
            optional=(),
            example={"node_token": "wikcnxxxx"},
        ),
    },
    "drive": {
        "list_files": ActionSpec(
            summary="List drive files",
            required=(),
            optional=("folder_token", "page_size", "page_token"),
            example={"folder_token": "fldcnxxxx", "page_size": 20},
        ),
        "create_folder": ActionSpec(
            summary="Create drive folder",
            required=("name",),
            optional=("folder_token",),
            example={"name": "Weekly Reports"},
        ),
        "copy_file": ActionSpec(
            summary="Copy drive file",
            required=("file_token", "name"),
            optional=("folder_token", "file_type"),
            example={"file_token": "boxcnxxxx", "name": "Copy of report"},
        ),
        "delete_file": ActionSpec(
            summary="Delete drive file",
            required=("file_token",),
            optional=("file_type",),
            example={"file_token": "boxcnxxxx", "file_type": "file"},
        ),
    },
    "sheets": {
        "create": ActionSpec(
            summary="Create spreadsheet",
            required=(),
            optional=("title", "folder_token"),
            example={"title": "Q1 Metrics"},
        ),
        "get": ActionSpec(
            summary="Get spreadsheet metadata",
            required=("spreadsheet_token",),
            optional=(),
            example={"spreadsheet_token": "shtcnxxxx"},
        ),
        "list_sheets": ActionSpec(
            summary="List worksheet tabs",
            required=("spreadsheet_token",),
            optional=(),
            example={"spreadsheet_token": "shtcnxxxx"},
        ),
    },
    "mail": {
        "list_mailgroups": ActionSpec(
            summary="List mail groups",
            required=(),
            optional=("page_size", "page_token"),
            example={"page_size": 20},
        ),
        "get_mailgroup": ActionSpec(
            summary="Get mail group details",
            required=("mailgroup_id",),
            optional=(),
            example={"mailgroup_id": "mg_xxx"},
        ),
        "create_mailgroup": ActionSpec(
            summary="Create mail group",
            required=(),
            optional=("email", "name", "description"),
            example={"email": "dev@company.com", "name": "Dev Team"},
        ),
    },
    "meeting": {
        "list_meetings": ActionSpec(
            summary="List meetings by time window",
            required=("start_time", "end_time"),
            optional=("page_size", "page_token"),
            example={"start_time": "1709500000", "end_time": "1709600000"},
        ),
        "get_meeting": ActionSpec(
            summary="Get meeting details",
            required=("meeting_id",),
            optional=(),
            example={"meeting_id": "1234567890"},
        ),
        "list_rooms": ActionSpec(
            summary="List meeting rooms",
            required=(),
            optional=("page_size", "page_token", "room_level_id"),
            example={"page_size": 20},
        ),
    },
    "okr": {
        "list_periods": ActionSpec(
            summary="List OKR periods",
            required=(),
            optional=("page_size", "page_token"),
            example={"page_size": 20},
        ),
        "list_user_okrs": ActionSpec(
            summary="List user OKRs",
            required=("user_id",),
            optional=(),
            example={"user_id": "ou_xxx"},
        ),
        "batch_get": ActionSpec(
            summary="Batch get OKRs",
            required=("okr_ids",),
            optional=(),
            example={"okr_ids": ["okr_1", "okr_2"]},
        ),
    },
    "bitable": {
        "list_tables": ActionSpec(
            summary="List bitable tables",
            required=(),
            optional=("app_token", "auto_create_app", "app_name"),
            example={"app_token": "bascnxxxx"},
        ),
        "list_records": ActionSpec(
            summary="List bitable records",
            required=("app_token", "table_id"),
            optional=("page_size", "page_token"),
            example={"app_token": "bascnxxxx", "table_id": "tblxxx"},
        ),
        "create_record": ActionSpec(
            summary="Create bitable record",
            required=("app_token", "table_id", "fields"),
            optional=(),
            example={"app_token": "bascnxxxx", "table_id": "tblxxx", "fields": {"Name": "Alice"}},
        ),
        "update_record": ActionSpec(
            summary="Update bitable record",
            required=("app_token", "table_id", "record_id", "fields"),
            optional=(),
            example={"app_token": "bascnxxxx", "table_id": "tblxxx", "record_id": "recxxx", "fields": {"Name": "Bob"}},
        ),
        "delete_record": ActionSpec(
            summary="Delete bitable record",
            required=("app_token", "table_id", "record_id"),
            optional=(),
            example={"app_token": "bascnxxxx", "table_id": "tblxxx", "record_id": "recxxx"},
        ),
        "list_fields": ActionSpec(
            summary="List bitable fields",
            required=("app_token", "table_id"),
            optional=(),
            example={"app_token": "bascnxxxx", "table_id": "tblxxx"},
        ),
    },
}


# ---- Tool handlers ---------------------------------------------------------


def _message_send(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    chat_id = _resolve_chat_id(args)
    if not chat_id:
        return {"success": False, "error": "chat_id is required"}

    content = str(args.get("content", "")).strip()
    if not content:
        return {"success": False, "error": "content is required"}

    content_format = str(args.get("content_format", "text")).strip().lower() or "text"
    msg_type = "post" if content_format in {"post", "richtext", "rich_text"} else "text"
    payload = _message_post_payload(content) if msg_type == "post" else _message_text_payload(content)
    reply_to_message_id = _resolve_reply_to_message_id(args)

    if reply_to_message_id:
        result = api_request(
            "POST",
            f"/im/v1/messages/{reply_to_message_id}/reply",
            {"content": payload, "msg_type": msg_type},
            auth_manager=auth_manager,
        )
    else:
        result = api_request(
            "POST",
            "/im/v1/messages",
            {"receive_id": chat_id, "msg_type": msg_type, "content": payload},
            query={"receive_id_type": "chat_id"},
            auth_manager=auth_manager,
        )
    failure = _api_failure(result)
    if failure:
        return failure

    data = result.get("data", {})
    message_id = str(data.get("message_id", "") or "").strip()
    return {
        "success": True,
        "message_id": message_id,
        "chat_id": chat_id,
        "msg_type": msg_type,
        "message": "message sent successfully",
    }


def _message_history(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    chat_id = _resolve_chat_id(args)
    if not chat_id:
        return {"success": False, "error": "chat_id is required"}

    query: dict[str, Any] = {
        "container_id_type": "chat",
        "container_id": chat_id,
        "sort_type": "ByCreateTimeDesc",
        "page_size": _clamp_page_size(args.get("page_size"), default=20, maximum=50),
    }
    start_time = str(args.get("start_time", "")).strip()
    end_time = str(args.get("end_time", "")).strip()
    page_token = str(args.get("page_token", "")).strip()
    if start_time:
        query["start_time"] = start_time
    if end_time:
        query["end_time"] = end_time
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/im/v1/messages", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    items = data.get("items", [])
    return {
        "success": True,
        "chat_id": chat_id,
        "messages": items,
        "count": len(items) if isinstance(items, list) else 0,
        "page_token": data.get("page_token", ""),
        "has_more": bool(data.get("has_more", False)),
    }


def _message_upload_file(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    source_path, source_err = _resolve_source_file(args)
    if source_err:
        return source_err

    assert source_path is not None
    chat_id = _resolve_chat_id(args)
    if not chat_id:
        return {"success": False, "error": "chat_id is required"}

    max_bytes = int(args.get("max_bytes", 20 * 1024 * 1024) or 20 * 1024 * 1024)
    file_size = source_path.stat().st_size
    if max_bytes > 0 and file_size > max_bytes:
        return {"success": False, "error": f"file too large: {file_size} bytes > {max_bytes}"}

    file_name = str(args.get("file_name", "")).strip() or source_path.name
    file_bytes = source_path.read_bytes()
    token, token_err = _resolve_bearer_token(auth_manager, timeout=30)
    if token_err:
        return {"success": False, "error": token_err}

    if _is_image_file(file_name):
        upload = _multipart_json_request(
            _build_url(auth_manager._open_api_base, "/im/v1/images", None),
            token=token,
            fields={"image_type": "message"},
            file_field="image",
            file_name=file_name,
            file_bytes=file_bytes,
        )
        failure = _api_failure(upload)
        if failure:
            return failure
        image_key = str(upload.get("data", {}).get("image_key", "")).strip()
        if not image_key:
            return {"success": False, "error": "upload image succeeded but image_key is missing"}
        msg_type = "image"
        msg_content = json.dumps({"image_key": image_key}, ensure_ascii=False)
        upload_key = image_key
        upload_field = "image_key"
    else:
        upload = _multipart_json_request(
            _build_url(auth_manager._open_api_base, "/im/v1/files", None),
            token=token,
            fields={"file_type": _select_im_file_type(file_name), "file_name": file_name},
            file_field="file",
            file_name=file_name,
            file_bytes=file_bytes,
        )
        failure = _api_failure(upload)
        if failure:
            return failure
        file_key = str(upload.get("data", {}).get("file_key", "")).strip()
        if not file_key:
            return {"success": False, "error": "upload file succeeded but file_key is missing"}
        msg_type = "audio" if _is_audio_file(file_name) else "file"
        msg_content = json.dumps({"file_key": file_key}, ensure_ascii=False)
        upload_key = file_key
        upload_field = "file_key"

    reply_to_message_id = _resolve_reply_to_message_id(args)
    if reply_to_message_id:
        send = api_request(
            "POST",
            f"/im/v1/messages/{reply_to_message_id}/reply",
            {"msg_type": msg_type, "content": msg_content},
            auth_manager=auth_manager,
        )
    else:
        send = api_request(
            "POST",
            "/im/v1/messages",
            {"receive_id": chat_id, "msg_type": msg_type, "content": msg_content},
            query={"receive_id_type": "chat_id"},
            auth_manager=auth_manager,
        )
    failure = _api_failure(send)
    if failure:
        return failure

    message_id = str(send.get("data", {}).get("message_id", "")).strip()
    return {
        "success": True,
        "chat_id": chat_id,
        "message_id": message_id,
        "msg_type": msg_type,
        upload_field: upload_key,
        "file_name": file_name,
        "bytes": file_size,
        "message": "file sent successfully",
    }


def _calendar_create(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    title = str(args.get("title", "")).strip() or str(args.get("summary", "")).strip()
    start = str(args.get("start", "")).strip() or str(args.get("start_time", "")).strip()
    if not title or not start:
        return {"success": False, "error": "title/summary and start/start_time are required"}

    start_ts = _parse_ts(start)
    if not start_ts:
        return {"success": False, "error": "invalid start format; use timestamp or YYYY-MM-DD[ HH:MM]"}

    end_source = str(args.get("end_time", "")).strip() or str(args.get("end", "")).strip()
    if end_source:
        end_ts = _parse_ts(end_source)
        if not end_ts:
            return {"success": False, "error": "invalid end_time/end format"}
    else:
        duration_seconds = _parse_duration_seconds(args.get("duration"))
        end_ts = str(int(start_ts) + duration_seconds)
    calendar_id, calendar_err = _resolve_calendar_id(args, auth_manager)
    if calendar_err:
        return calendar_err

    body = {
        "summary": title,
        "start_time": {"timestamp": start_ts},
        "end_time": {"timestamp": end_ts},
        "description": args.get("description", ""),
    }
    timezone_name = str(args.get("timezone", "")).strip()
    if timezone_name:
        body["start_time"]["timezone"] = timezone_name
        body["end_time"]["timezone"] = timezone_name
    if "need_notification" in args:
        body["need_notification"] = bool(args.get("need_notification"))
    result = api_request(
        "POST",
        f"/calendar/v4/calendars/{calendar_id}/events",
        body,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "event": result.get("data", {}), "message": f"事件「{title}」已创建"}


def _calendar_query(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    start = str(args.get("start", "")).strip() or str(args.get("start_time", "")).strip()
    end = str(args.get("end", "")).strip() or str(args.get("end_time", "")).strip()
    if not start:
        return {"success": False, "error": "start/start_time is required"}

    start_ts = _parse_ts(start)
    if not start_ts:
        return {"success": False, "error": "invalid start format; use timestamp or YYYY-MM-DD[ HH:MM]"}

    if end:
        end_ts = _parse_ts(end)
    else:
        dt = datetime.fromtimestamp(int(start_ts), tz=timezone.utc) + timedelta(days=1)
        end_ts = str(int(dt.timestamp()))
    if not end_ts:
        return {"success": False, "error": "invalid end format; use timestamp or YYYY-MM-DD[ HH:MM]"}

    calendar_id, calendar_err = _resolve_calendar_id(args, auth_manager)
    if calendar_err:
        return calendar_err

    result = api_request(
        "GET",
        f"/calendar/v4/calendars/{calendar_id}/events",
        query={"start_time": start_ts, "end_time": end_ts},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure

    events = result.get("data", {}).get("items", [])
    return {"success": True, "events": events, "count": len(events)}


def _calendar_update(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    event_id = str(args.get("event_id", "")).strip()
    if not event_id:
        return {"success": False, "error": "event_id is required"}

    body: dict[str, Any] = {}
    summary = str(args.get("summary", "")).strip()
    description = str(args.get("description", "")).strip()
    start_time = str(args.get("start_time", "")).strip()
    end_time = str(args.get("end_time", "")).strip()
    timezone_name = str(args.get("timezone", "")).strip()

    if summary:
        body["summary"] = summary
    if description:
        body["description"] = description

    parsed_start: int | None = None
    parsed_end: int | None = None
    if start_time:
        parsed = _parse_ts(start_time)
        if not parsed:
            return {"success": False, "error": "invalid start_time format"}
        parsed_start = int(parsed)
        body["start_time"] = {"timestamp": parsed}
        if timezone_name:
            body["start_time"]["timezone"] = timezone_name
    if end_time:
        parsed = _parse_ts(end_time)
        if not parsed:
            return {"success": False, "error": "invalid end_time format"}
        parsed_end = int(parsed)
        body["end_time"] = {"timestamp": parsed}
        if timezone_name:
            body["end_time"]["timezone"] = timezone_name
    if parsed_start is not None and parsed_end is not None and parsed_end <= parsed_start:
        return {"success": False, "error": "end_time must be greater than start_time"}
    if not body:
        return {"success": False, "error": "at least one of summary, description, start_time, end_time is required"}

    calendar_id, calendar_err = _resolve_calendar_id(args, auth_manager)
    if calendar_err:
        return calendar_err

    result = api_request(
        "PATCH",
        f"/calendar/v4/calendars/{calendar_id}/events/{event_id}",
        body,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "event": result.get("data", {}).get("event", {}), "message": "event updated"}


def _calendar_delete(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    event_id = str(args.get("event_id", "")).strip()
    if not event_id:
        return {"success": False, "error": "event_id is required"}

    calendar_id, calendar_err = _resolve_calendar_id(args, auth_manager)
    if calendar_err:
        return calendar_err

    result = api_request(
        "DELETE",
        f"/calendar/v4/calendars/{calendar_id}/events/{event_id}",
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"事件 {event_id} 已删除"}


def _calendar_list(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    result = api_request("GET", "/calendar/v4/calendars", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    calendars = data.get("items", [])
    if not isinstance(calendars, list) or not calendars:
        calendars = data.get("calendar_list", [])
    return {"success": True, "calendars": calendars, "count": len(calendars)}


def _contact_list_scopes(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    result = api_request("GET", "/contact/v3/scopes", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "scopes": result.get("data", {})}


def _contact_scope_fallback(action: str, failure: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    scopes = _contact_list_scopes({}, auth_manager)
    if not scopes.get("success"):
        return failure
    return {
        "success": True,
        "source": "scope_fallback",
        "fallback_for": action,
        "warning": failure.get("error", "permission limited"),
        "scopes": scopes.get("scopes", {}),
    }


def _contact_get_user(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    user_id = str(args.get("user_id", "")).strip()
    if not user_id:
        return {"success": False, "error": "user_id is required"}

    user_id_type = str(args.get("user_id_type", "open_id")).strip() or "open_id"
    result = api_request(
        "GET",
        f"/contact/v3/users/{user_id}",
        query={"user_id_type": user_id_type},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _contact_scope_fallback("get_user", failure, auth_manager)
        return failure
    return {"success": True, "user": result.get("data", {}).get("user", {})}


def _contact_list_users(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    department_id = str(args.get("department_id", "")).strip()
    if not department_id:
        return {"success": False, "error": "department_id is required"}

    query: dict[str, Any] = {
        "department_id": department_id,
        "page_size": int(args.get("page_size", 50) or 50),
    }
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/contact/v3/users", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _contact_scope_fallback("list_users", failure, auth_manager)
        return failure

    data = result.get("data", {})
    return {"success": True, "users": data.get("items", []), "has_more": data.get("has_more", False)}


def _contact_get_department(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    department_id = str(args.get("department_id", "")).strip()
    if not department_id:
        return {"success": False, "error": "department_id is required"}

    result = api_request("GET", f"/contact/v3/departments/{department_id}", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _contact_scope_fallback("get_department", failure, auth_manager)
        return failure
    return {"success": True, "department": result.get("data", {}).get("department", {})}


def _contact_list_departments(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    query: dict[str, Any] = {
        "parent_department_id": str(args.get("parent_department_id", "0")).strip() or "0",
        "page_size": int(args.get("page_size", 50) or 50),
    }
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/contact/v3/departments", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _contact_scope_fallback("list_departments", failure, auth_manager)
        return failure

    data = result.get("data", {})
    return {"success": True, "departments": data.get("items", []), "has_more": data.get("has_more", False)}


def _doc_create(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    body: dict[str, Any] = {}
    title = str(args.get("title", "")).strip()
    folder_token = str(args.get("folder_token", "")).strip()
    initial_content = str(args.get("content", "")).strip() or str(args.get("description", "")).strip()
    if title:
        body["title"] = title
    if folder_token:
        body["folder_token"] = folder_token

    result = api_request("POST", "/docx/v1/documents", body or None, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    doc = result.get("data", {}).get("document", {})
    response: dict[str, Any] = {"success": True, "document": doc, "message": f"文档「{title}」已创建" if title else "文档已创建"}
    if initial_content:
        document_id = str(doc.get("document_id", "")).strip()
        if document_id:
            write_result = _doc_write_markdown({"document_id": document_id, "content": initial_content}, auth_manager)
            if not write_result.get("success"):
                return write_result
            response["content_written"] = True
    return response


def _doc_read(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    document_id = str(args.get("document_id", "")).strip()
    if not document_id:
        return {"success": False, "error": "document_id is required"}

    result = api_request("GET", f"/docx/v1/documents/{document_id}", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "document": result.get("data", {}).get("document", {})}


def _doc_read_content(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    document_id = str(args.get("document_id", "")).strip()
    if not document_id:
        return {"success": False, "error": "document_id is required"}

    result = api_request("GET", f"/docx/v1/documents/{document_id}/raw_content", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "content": result.get("data", {}).get("content", "")}


def _doc_list_blocks(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    document_id = str(args.get("document_id", "")).strip()
    if not document_id:
        return {"success": False, "error": "document_id is required"}

    query: dict[str, Any] = {"page_size": _clamp_page_size(args.get("page_size"), default=50, maximum=500)}
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", f"/docx/v1/documents/{document_id}/blocks", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    blocks = data.get("items", [])
    return {
        "success": True,
        "document_id": document_id,
        "blocks": blocks,
        "count": len(blocks) if isinstance(blocks, list) else 0,
        "page_token": data.get("page_token", ""),
        "has_more": bool(data.get("has_more", False)),
    }


def _doc_update_block_text(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    document_id = str(args.get("document_id", "")).strip()
    block_id = str(args.get("block_id", "")).strip()
    content = str(args.get("content", "")).strip()
    if not document_id or not block_id or not content:
        return {"success": False, "error": "document_id, block_id and content are required"}

    revision = int(args.get("document_revision_id", -1) or -1)
    body = {
        "update_text_elements": {
            "elements": [{"text_run": {"content": content}}],
        }
    }
    query: dict[str, Any] = {"document_revision_id": revision}
    client_token = str(args.get("client_token", "")).strip()
    user_id_type = str(args.get("user_id_type", "")).strip()
    if client_token:
        query["client_token"] = client_token
    if user_id_type:
        query["user_id_type"] = user_id_type

    result = api_request(
        "PATCH",
        f"/docx/v1/documents/{document_id}/blocks/{block_id}",
        body,
        query=query,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {
        "success": True,
        "document_id": document_id,
        "block": result.get("data", {}).get("block", {}),
        "document_revision_id": result.get("data", {}).get("document_revision_id", 0),
        "client_token": result.get("data", {}).get("client_token", ""),
        "message": "block updated",
    }


def _doc_write_markdown(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    document_id = str(args.get("document_id", "")).strip()
    content = str(args.get("content", "")).strip()
    if not document_id or not content:
        return {"success": False, "error": "document_id and content are required"}

    blocks_result = api_request(
        "GET",
        f"/docx/v1/documents/{document_id}/blocks",
        query={"page_size": 50},
        auth_manager=auth_manager,
    )
    blocks_failure = _api_failure(blocks_result)
    if blocks_failure:
        return blocks_failure
    page_block_id = document_id
    items = blocks_result.get("data", {}).get("items", [])
    if isinstance(items, list):
        for item in items:
            if not isinstance(item, dict):
                continue
            if int(item.get("block_type", 0) or 0) == 1:
                block_id = str(item.get("block_id", "")).strip()
                if block_id:
                    page_block_id = block_id
                    break

    convert = api_request(
        "POST",
        "/docx/v1/documents/blocks/convert",
        {"content_type": "markdown", "content": content},
        auth_manager=auth_manager,
    )
    convert_failure = _api_failure(convert)
    if convert_failure:
        return convert_failure

    data = convert.get("data", {})
    children_ids = data.get("first_level_block_ids", [])
    descendants = data.get("blocks", [])
    if not isinstance(children_ids, list) or not isinstance(descendants, list) or not descendants:
        return {"success": True, "message": "markdown converted but no blocks to insert", "document_id": document_id}

    create = api_request(
        "POST",
        f"/docx/v1/documents/{document_id}/blocks/{page_block_id}/descendant",
        {"children_id": children_ids, "descendants": descendants, "index": 0},
        query={"document_revision_id": -1},
        auth_manager=auth_manager,
    )
    create_failure = _api_failure(create)
    if create_failure:
        return create_failure
    return {
        "success": True,
        "document_id": document_id,
        "inserted_blocks": len(descendants),
        "first_level_blocks": len(children_ids),
        "document_revision_id": create.get("data", {}).get("document_revision_id", 0),
        "message": "markdown written",
    }


def _task_list(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    auth_mode, user_key = _task_request_mode(args)
    query: dict[str, Any] = {
        "page_size": _clamp_page_size(args.get("page_size"), default=20, maximum=200),
        "type": "my_tasks",
    }
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token
    if "completed" in args:
        query["completed"] = bool(args.get("completed"))
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type

    result = api_request("GET", "/task/v2/tasks", query=query, auth=auth_mode, user_key=user_key, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    items = data.get("items", [])
    return {
        "success": True,
        "tasks": items,
        "count": len(items) if isinstance(items, list) else 0,
        "has_more": bool(data.get("has_more", False)),
        "page_token": data.get("page_token", ""),
        "auth_mode": auth_mode,
    }


def _task_list_subtasks(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    parent_task_id = str(args.get("parent_task_id", "")).strip()
    if not parent_task_id:
        return {"success": False, "error": "parent_task_id is required"}

    auth_mode, user_key = _task_request_mode(args)
    query: dict[str, Any] = {
        "page_size": _clamp_page_size(args.get("page_size"), default=20, maximum=200),
    }
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type

    result = api_request(
        "GET",
        f"/task/v2/tasks/{parent_task_id}/subtasks",
        query=query,
        auth=auth_mode,
        user_key=user_key,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    items = data.get("items", [])
    return {
        "success": True,
        "parent_task_id": parent_task_id,
        "subtasks": items,
        "count": len(items) if isinstance(items, list) else 0,
        "has_more": bool(data.get("has_more", False)),
        "page_token": data.get("page_token", ""),
        "auth_mode": auth_mode,
    }


def _task_create(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    summary, description = _normalize_task_create_fields(args)
    if not summary:
        return {"success": False, "error": "summary is required"}

    auth_mode, user_key = _task_request_mode(args)
    due, due_err = _build_task_due(args)
    if due_err:
        return due_err

    input_task: dict[str, Any] = {"summary": summary}
    if description:
        input_task["description"] = description
    if due:
        input_task["due"] = due

    assignee_ids = args.get("assignee_ids", [])
    if isinstance(assignee_ids, list) and assignee_ids:
        member_type = str(args.get("assignee_type", "")).strip() or "user"
        input_task["members"] = [{"id": str(member).strip(), "type": member_type} for member in assignee_ids if str(member).strip()]
    client_token = str(args.get("client_token", "")).strip()
    if client_token:
        input_task["client_token"] = client_token

    query: dict[str, Any] = {}
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type

    result = api_request(
        "POST",
        "/task/v2/tasks",
        {"input_task": input_task},
        query=query or None,
        auth=auth_mode,
        user_key=user_key,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    task_id = str(result.get("data", {}).get("task", {}).get("guid", "")).strip()
    return {"success": True, "task_id": task_id, "task": result.get("data", {}).get("task", {}), "auth_mode": auth_mode}


def _task_create_subtask(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    parent_task_id = str(args.get("parent_task_id", "")).strip()
    if not parent_task_id:
        return {"success": False, "error": "parent_task_id is required"}

    summary, description = _normalize_task_create_fields(args)
    if not summary:
        return {"success": False, "error": "summary is required"}

    auth_mode, user_key = _task_request_mode(args)
    due, due_err = _build_task_due(args)
    if due_err:
        return due_err

    input_task: dict[str, Any] = {"summary": summary}
    if description:
        input_task["description"] = description
    if due:
        input_task["due"] = due

    assignee_ids = args.get("assignee_ids", [])
    if isinstance(assignee_ids, list) and assignee_ids:
        member_type = str(args.get("assignee_type", "")).strip() or "user"
        input_task["members"] = [{"id": str(member).strip(), "type": member_type} for member in assignee_ids if str(member).strip()]
    client_token = str(args.get("client_token", "")).strip()
    if client_token:
        input_task["client_token"] = client_token

    query: dict[str, Any] = {}
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type

    result = api_request(
        "POST",
        f"/task/v2/tasks/{parent_task_id}/subtasks",
        {"input_task": input_task},
        query=query or None,
        auth=auth_mode,
        user_key=user_key,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    task_id = str(result.get("data", {}).get("subtask", {}).get("guid", "")).strip()
    return {
        "success": True,
        "parent_task_id": parent_task_id,
        "task_id": task_id,
        "subtask": result.get("data", {}).get("subtask", {}),
        "auth_mode": auth_mode,
    }


def _task_update(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    task_id = str(args.get("task_id", "")).strip()
    if not task_id:
        return {"success": False, "error": "task_id is required"}

    summary, description = _normalize_task_update_fields(args)
    task_patch: dict[str, Any] = {}
    update_fields: list[str] = []
    if summary:
        task_patch["summary"] = summary
        update_fields.append("summary")
    if description:
        task_patch["description"] = description
        update_fields.append("description")

    due_time = str(args.get("due_time", "")).strip()
    if due_time:
        due_seconds, due_err = _parse_unix_seconds(due_time, field_name="due_time")
        if due_err:
            return due_err
        task_patch["due"] = {"timestamp": str(due_seconds * 1000)}
        update_fields.append("due")
    if not update_fields:
        return {"success": False, "error": "at least one of summary, description, content or due_time is required"}

    query: dict[str, Any] = {}
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type
    auth_mode, user_key = _task_request_mode(args)

    result = api_request(
        "PATCH",
        f"/task/v2/tasks/{task_id}",
        {"task": task_patch, "update_fields": update_fields},
        query=query or None,
        auth=auth_mode,
        user_key=user_key,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "task_id": task_id, "updated_fields": update_fields, "task": result.get("data", {}).get("task", {})}


def _task_delete(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    task_id = str(args.get("task_id", "")).strip()
    if not task_id:
        return {"success": False, "error": "task_id is required"}

    query: dict[str, Any] = {}
    user_id_type = str(args.get("user_id_type", "")).strip()
    if user_id_type:
        query["user_id_type"] = user_id_type
    auth_mode, user_key = _task_request_mode(args)

    result = api_request(
        "DELETE",
        f"/task/v2/tasks/{task_id}",
        query=query or None,
        auth=auth_mode,
        user_key=user_key,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "task_id": task_id, "message": "task deleted", "auth_mode": auth_mode}


def _wiki_list_spaces(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    query: dict[str, Any] = {}
    if args.get("page_size"):
        query["page_size"] = args["page_size"]
    if args.get("page_token"):
        query["page_token"] = args["page_token"]

    result = api_request("GET", "/wiki/v2/spaces", query=query or None, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    spaces = result.get("data", {}).get("items", [])
    return {"success": True, "spaces": spaces, "count": len(spaces)}


def _wiki_list_nodes(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    space_id = str(args.get("space_id", "")).strip()
    if not space_id:
        return {"success": False, "error": "space_id is required"}

    query: dict[str, Any] = {}
    if args.get("parent_node_token"):
        query["parent_node_token"] = args["parent_node_token"]
    if args.get("page_size"):
        query["page_size"] = args["page_size"]

    result = api_request(
        "GET",
        f"/wiki/v2/spaces/{space_id}/nodes",
        query=query or None,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    nodes = result.get("data", {}).get("items", [])
    return {"success": True, "nodes": nodes, "count": len(nodes)}


def _wiki_create_node(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    space_id = str(args.get("space_id", "")).strip()
    if not space_id:
        return {"success": False, "error": "space_id is required"}

    body: dict[str, Any] = {"obj_type": str(args.get("obj_type", "docx")).strip() or "docx"}
    if args.get("parent_node_token"):
        body["parent_node_token"] = args["parent_node_token"]
    if args.get("title"):
        body["node_type"] = "origin"
        body["title"] = args["title"]

    result = api_request("POST", f"/wiki/v2/spaces/{space_id}/nodes", body, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "node": result.get("data", {}).get("node", {}), "message": "知识节点已创建"}


def _wiki_get_node(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    node_token = str(args.get("node_token", "")).strip()
    if not node_token:
        return {"success": False, "error": "node_token is required"}

    result = api_request(
        "GET",
        "/wiki/v2/spaces/get_node",
        query={"token": node_token},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "node": result.get("data", {}).get("node", {})}


def _drive_list_files(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    folder_token = str(args.get("folder_token", "")).strip() or _resolve_drive_default_folder()
    query: dict[str, Any] = {"folder_token": folder_token}
    if args.get("page_size"):
        query["page_size"] = args["page_size"]
    if args.get("page_token"):
        query["page_token"] = args["page_token"]

    result = api_request("GET", "/drive/v1/files", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    files = result.get("data", {}).get("files", [])
    return {"success": True, "files": files, "count": len(files), "folder_token_used": folder_token}


def _drive_create_folder(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    name = str(args.get("name", "")).strip()
    if not name:
        return {"success": False, "error": "name is required"}

    folder_token = str(args.get("folder_token", "")).strip() or _resolve_drive_default_folder() or _resolve_drive_root_folder(auth_manager)
    body: dict[str, Any] = {"name": name}
    if folder_token:
        body["folder_token"] = folder_token

    result = api_request("POST", "/drive/v1/files/create_folder", body, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "folder": result.get("data", {}), "message": f"文件夹「{name}」已创建"}


def _drive_copy_file(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    file_token = str(args.get("file_token", "")).strip()
    name = str(args.get("name", "")).strip()
    if not file_token or not name:
        return {"success": False, "error": "file_token and name are required"}

    folder_token = str(args.get("folder_token", "")).strip() or _resolve_drive_default_folder() or _resolve_drive_root_folder(auth_manager)
    if not folder_token:
        return {"success": False, "error": "folder_token is required"}

    body = {
        "name": name,
        "folder_token": folder_token,
        "type": str(args.get("file_type", "file")).strip() or "file",
    }
    result = api_request("POST", f"/drive/v1/files/{file_token}/copy", body, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "file": result.get("data", {}).get("file", {}), "message": "文件已复制"}


def _drive_delete_file(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    file_token = str(args.get("file_token", "")).strip()
    if not file_token:
        return {"success": False, "error": "file_token is required"}

    file_type = str(args.get("file_type", "file")).strip() or "file"
    result = api_request(
        "DELETE",
        f"/drive/v1/files/{file_token}",
        query={"type": file_type},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"文件 {file_token} 已删除"}


def _sheets_create(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    spreadsheet: dict[str, Any] = {}
    title = str(args.get("title", "")).strip()
    folder_token = str(args.get("folder_token", "")).strip()
    if title:
        spreadsheet["title"] = title
    if folder_token:
        spreadsheet["folder_token"] = folder_token

    body: dict[str, Any] | None = {"spreadsheet": spreadsheet} if spreadsheet else None
    result = api_request("POST", "/sheets/v3/spreadsheets", body, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "spreadsheet": result.get("data", {}).get("spreadsheet", {})}


def _sheets_get(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    token = str(args.get("spreadsheet_token", "")).strip()
    if not token:
        return {"success": False, "error": "spreadsheet_token is required"}

    result = api_request("GET", f"/sheets/v3/spreadsheets/{token}", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "spreadsheet": result.get("data", {}).get("spreadsheet", {})}


def _sheets_list_sheets(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    token = str(args.get("spreadsheet_token", "")).strip()
    if not token:
        return {"success": False, "error": "spreadsheet_token is required"}

    result = api_request("GET", f"/sheets/v3/spreadsheets/{token}/sheets/query", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "sheets": result.get("data", {}).get("sheets", [])}


def _mail_list_mailgroups(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    query: dict[str, Any] = {"page_size": int(args.get("page_size", 20) or 20)}
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/mail/v1/mailgroups", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    return {"success": True, "mailgroups": data.get("items", []), "has_more": data.get("has_more", False)}


def _mail_get_mailgroup(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    mailgroup_id = str(args.get("mailgroup_id", "")).strip()
    if not mailgroup_id:
        return {"success": False, "error": "mailgroup_id is required"}

    result = api_request("GET", f"/mail/v1/mailgroups/{mailgroup_id}", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "mailgroup": result.get("data", {})}


def _mail_create_mailgroup(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    body: dict[str, Any] = {}
    for key in ("email", "name", "description"):
        text = str(args.get(key, "")).strip()
        if text:
            body[key] = text

    result = api_request("POST", "/mail/v1/mailgroups", body or None, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "mailgroup": result.get("data", {})}


def _meeting_list_meetings(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    start_time = str(args.get("start_time", "")).strip()
    end_time = str(args.get("end_time", "")).strip()
    if not start_time or not end_time:
        return {"success": False, "error": "start_time and end_time are required"}

    query: dict[str, Any] = {
        "start_time": start_time,
        "end_time": end_time,
        "page_size": int(args.get("page_size", 20) or 20),
    }
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/vc/v1/meeting_list", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    return {"success": True, "meetings": data.get("meeting_list", []), "has_more": data.get("has_more", False)}


def _meeting_get_meeting(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    meeting_id = str(args.get("meeting_id", "")).strip()
    if not meeting_id:
        return {"success": False, "error": "meeting_id is required"}

    result = api_request("GET", f"/vc/v1/meetings/{meeting_id}", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "meeting": result.get("data", {}).get("meeting", {})}


def _meeting_list_rooms(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    query: dict[str, Any] = {"page_size": int(args.get("page_size", 20) or 20)}
    page_token = str(args.get("page_token", "")).strip()
    room_level_id = str(args.get("room_level_id", "")).strip()
    if room_level_id:
        query["room_level_id"] = room_level_id
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/vc/v1/rooms", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    return {"success": True, "rooms": data.get("rooms", []), "has_more": data.get("has_more", False)}


def _okr_list_periods(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    query: dict[str, Any] = {"page_size": int(args.get("page_size", 20) or 20)}
    page_token = str(args.get("page_token", "")).strip()
    if page_token:
        query["page_token"] = page_token

    result = api_request("GET", "/okr/v1/periods", query=query, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    data = result.get("data", {})
    return {"success": True, "periods": data.get("items", []), "has_more": data.get("has_more", False)}


def _okr_list_user_okrs(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    user_id = str(args.get("user_id", "")).strip()
    if not user_id:
        return {"success": False, "error": "user_id is required"}

    result = api_request("GET", f"/okr/v1/users/{user_id}/okrs", auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "okrs": result.get("data", {}).get("okr_list", [])}


def _okr_batch_get(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    okr_ids = args.get("okr_ids", [])
    if not isinstance(okr_ids, list) or not okr_ids:
        return {"success": False, "error": "okr_ids is required"}

    result = api_request("GET", "/okr/v1/okrs/batch_get", query={"okr_ids": okr_ids}, auth_manager=auth_manager)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "okrs": result.get("data", {}).get("okr_list", [])}


def _bitable_list_tables(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    auto_create_app = bool(args.get("auto_create_app", True))
    source = "provided"

    if not app_token:
        if not auto_create_app:
            return {"success": False, "error": "app_token is required"}
        app_token, create_err = _create_temp_bitable_app(args, auth_manager)
        if create_err:
            return create_err
        source = "auto_created"

    result = api_request("GET", f"/bitable/v1/apps/{app_token}/tables", auth_manager=auth_manager)
    failure = _api_failure(result)

    if failure and _is_transient_failure(failure):
        result = api_request("GET", f"/bitable/v1/apps/{app_token}/tables", auth_manager=auth_manager)
        failure = _api_failure(result)

    if failure and auto_create_app and source != "auto_created" and _is_invalid_bitable_app_failure(failure):
        retry_token, create_err = _create_temp_bitable_app(args, auth_manager)
        if create_err:
            return failure
        retry = api_request("GET", f"/bitable/v1/apps/{retry_token}/tables", auth_manager=auth_manager)
        retry_failure = _api_failure(retry)
        if retry_failure and _is_transient_failure(retry_failure):
            retry = api_request("GET", f"/bitable/v1/apps/{retry_token}/tables", auth_manager=auth_manager)
            retry_failure = _api_failure(retry)
        if retry_failure:
            return retry_failure
        tables = retry.get("data", {}).get("items", [])
        return {
            "success": True,
            "tables": tables,
            "count": len(tables),
            "app_token": retry_token,
            "app_token_source": "auto_created",
            "recovered_from_app_token": app_token,
        }

    if failure:
        return failure

    tables = result.get("data", {}).get("items", [])
    return {
        "success": True,
        "tables": tables,
        "count": len(tables),
        "app_token": app_token,
        "app_token_source": source,
    }


def _bitable_list_records(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    table_id = str(args.get("table_id", "")).strip()
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    query: dict[str, Any] = {}
    if args.get("page_size"):
        query["page_size"] = args["page_size"]
    if args.get("page_token"):
        query["page_token"] = args["page_token"]

    result = api_request(
        "GET",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/records",
        query=query or None,
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    records = result.get("data", {}).get("items", [])
    return {"success": True, "records": records, "count": len(records)}


def _bitable_create_record(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    table_id = str(args.get("table_id", "")).strip()
    fields = args.get("fields", {})
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}
    if not isinstance(fields, dict) or not fields:
        return {"success": False, "error": "fields is required"}

    result = api_request(
        "POST",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/records",
        {"fields": fields},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "record": result.get("data", {}).get("record", {}), "message": "记录已创建"}


def _bitable_update_record(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    table_id = str(args.get("table_id", "")).strip()
    record_id = str(args.get("record_id", "")).strip()
    fields = args.get("fields", {})
    if not app_token or not table_id or not record_id:
        return {"success": False, "error": "app_token, table_id, and record_id are required"}
    if not isinstance(fields, dict) or not fields:
        return {"success": False, "error": "fields is required"}

    result = api_request(
        "PUT",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}",
        {"fields": fields},
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"记录 {record_id} 已更新"}


def _bitable_delete_record(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    table_id = str(args.get("table_id", "")).strip()
    record_id = str(args.get("record_id", "")).strip()
    if not app_token or not table_id or not record_id:
        return {"success": False, "error": "app_token, table_id, and record_id are required"}

    result = api_request(
        "DELETE",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}",
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"记录 {record_id} 已删除"}


def _bitable_list_fields(args: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    app_token = _resolve_bitable_app_token(args)
    table_id = str(args.get("table_id", "")).strip()
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    result = api_request(
        "GET",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/fields",
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    fields = result.get("data", {}).get("items", [])
    return {"success": True, "fields": fields, "count": len(fields)}


TOOL_HANDLERS: dict[str, dict[str, ToolHandler]] = {
    "message": {
        "send_message": _message_send,
        "upload_file": _message_upload_file,
        "history": _message_history,
    },
    "calendar": {
        "create": _calendar_create,
        "query": _calendar_query,
        "update": _calendar_update,
        "delete": _calendar_delete,
        "list_calendars": _calendar_list,
    },
    "contact": {
        "get_user": _contact_get_user,
        "list_users": _contact_list_users,
        "get_department": _contact_get_department,
        "list_departments": _contact_list_departments,
        "list_scopes": _contact_list_scopes,
    },
    "doc": {
        "create": _doc_create,
        "read": _doc_read,
        "read_content": _doc_read_content,
        "list_blocks": _doc_list_blocks,
        "update_block_text": _doc_update_block_text,
        "write_markdown": _doc_write_markdown,
    },
    "task": {
        "list": _task_list,
        "list_subtasks": _task_list_subtasks,
        "create": _task_create,
        "create_subtask": _task_create_subtask,
        "update": _task_update,
        "delete": _task_delete,
    },
    "wiki": {
        "list_spaces": _wiki_list_spaces,
        "list_nodes": _wiki_list_nodes,
        "create_node": _wiki_create_node,
        "get_node": _wiki_get_node,
    },
    "drive": {
        "list_files": _drive_list_files,
        "create_folder": _drive_create_folder,
        "copy_file": _drive_copy_file,
        "delete_file": _drive_delete_file,
    },
    "sheets": {
        "create": _sheets_create,
        "get": _sheets_get,
        "list_sheets": _sheets_list_sheets,
    },
    "mail": {
        "list_mailgroups": _mail_list_mailgroups,
        "get_mailgroup": _mail_get_mailgroup,
        "create_mailgroup": _mail_create_mailgroup,
    },
    "meeting": {
        "list_meetings": _meeting_list_meetings,
        "get_meeting": _meeting_get_meeting,
        "list_rooms": _meeting_list_rooms,
    },
    "okr": {
        "list_periods": _okr_list_periods,
        "list_user_okrs": _okr_list_user_okrs,
        "batch_get": _okr_batch_get,
    },
    "bitable": {
        "list_tables": _bitable_list_tables,
        "list_records": _bitable_list_records,
        "create_record": _bitable_create_record,
        "update_record": _bitable_update_record,
        "delete_record": _bitable_delete_record,
        "list_fields": _bitable_list_fields,
    },
}

TOOL_ACTION_ALIASES: dict[str, dict[str, str]] = {
    "message": {
        "send": "send_message",
        "chat_history": "history",
    },
    "calendar": {
        "create_event": "create",
        "query_events": "query",
        "update_event": "update",
        "delete_event": "delete",
    },
    "task": {
        "list_tasks": "list",
        "create_task": "create",
        "update_task": "update",
        "delete_task": "delete",
    },
    "doc": {
        "create_doc": "create",
        "read_doc": "read",
        "read_doc_content": "read_content",
        "list_doc_blocks": "list_blocks",
        "update_doc_block": "update_block_text",
        "write_doc_markdown": "write_markdown",
    },
    "sheets": {
        "create_spreadsheet": "create",
        "get_spreadsheet": "get",
    },
    "okr": {
        "list_okr_periods": "list_periods",
        "batch_get_okrs": "batch_get",
    },
}


def _validate_action_specs() -> None:
    for module, actions in ACTION_SPECS.items():
        if not isinstance(actions, dict):
            raise TypeError(f"ACTION_SPECS[{module}] must be a mapping")
        for action, spec in actions.items():
            if not isinstance(spec, ActionSpec):
                raise TypeError(f"ACTION_SPECS[{module}][{action}] must be ActionSpec")


def _aliases_for_action(module: str, canonical_action: str) -> list[str]:
    aliases = []
    for alias, target in TOOL_ACTION_ALIASES.get(module, {}).items():
        if target == canonical_action:
            aliases.append(alias)
    return sorted(set(aliases))


def _help_request_contracts() -> dict[str, Any]:
    return {
        "help": {
            "shape": {"command": "help", "topic": "overview|auth|modules|module|action", "module": "", "action_name": ""},
            "notes": "When topic=action, module + action_name are required.",
        },
        "auth": {
            "shape": {"command": "auth", "subcommand": "|".join(AUTH_SUBCOMMANDS), "args": {}},
            "notes": "Auth args can be nested in args or passed as top-level fields.",
        },
        "tool": {
            "shape": {"command": "tool", "module": "<module>", "tool_action": "<action>", "args": {}},
            "notes": "Tool args can be nested in args or passed as top-level fields.",
        },
        "api": {
            "shape": {
                "command": "api",
                "method": "GET|POST|PUT|DELETE",
                "path": "/open-api-path",
                "body": {},
                "query": {},
                "auth": "tenant|user",
                "user_key": "",
            },
            "notes": "query supports dict or raw query string.",
        },
    }


def _with_context(result: dict[str, Any], **context: Any) -> dict[str, Any]:
    enriched = copy.deepcopy(result)
    for key, value in context.items():
        enriched.setdefault(key, value)
    return enriched


def _collect_request_args(request: dict[str, Any], *, reserved: set[str]) -> tuple[dict[str, Any] | None, dict[str, Any] | None]:
    raw_args = request.get("args", {})
    if raw_args is None:
        raw_args = {}
    if not isinstance(raw_args, dict):
        return None, {"success": False, "error": "args must be an object"}

    merged = copy.deepcopy(raw_args)
    for key, value in request.items():
        if key in reserved or key == "args":
            continue
        merged.setdefault(key, value)
    return merged, None


def _resolve_command(request: dict[str, Any]) -> str:
    explicit = str(request.get("command", "")).strip().lower()
    if explicit:
        return explicit

    action = str(request.get("action", "")).strip().lower()
    if action in COMMAND_CHOICES:
        return action
    if str(request.get("module", "")).strip():
        return "tool"
    if str(request.get("subcommand", "")).strip():
        return "auth"
    if str(request.get("method", "")).strip() and str(request.get("path", "")).strip():
        return "api"
    return "help"


_validate_action_specs()


def _help_overview() -> dict[str, Any]:
    modules = []
    for module, actions in ACTION_SPECS.items():
        modules.append(
            {
                "module": module,
                "action_count": len(actions),
                "actions": sorted(actions.keys()),
                "next": f"python3 scripts/cli/feishu/feishu_cli.py help module --module {module}",
            }
        )

    return {
        "success": True,
        "help_level": "overview",
        "description": "Unified Feishu CLI (auth/tool/api/help)",
        "request_contracts": _help_request_contracts(),
        "commands": {
            "help": "Discover usage progressively",
            "auth": "Tenant + OAuth authorization flows",
            "tool": "High-level tool actions by module",
            "api": "Raw Feishu Open API call",
        },
        "modules": modules,
        "next_steps": [
            "python3 scripts/cli/feishu/feishu_cli.py help auth",
            "python3 scripts/cli/feishu/feishu_cli.py help module --module calendar",
            "python3 scripts/cli/feishu/feishu_cli.py help action --module calendar --action create",
        ],
    }


def _help_auth() -> dict[str, Any]:
    return {
        "success": True,
        "help_level": "auth",
        "request_schema": _help_request_contracts()["auth"],
        "subcommands": {
            "status": {
                "summary": "Show credential and token cache status",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth status",
            },
            "tenant_token": {
                "summary": "Fetch tenant_access_token (with cache)",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth tenant_token",
            },
            "oauth_url": {
                "summary": "Generate official OAuth URL after redirect-uri allowlist precheck",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth oauth_url '{\"redirect_uri\":\"https://example.com/callback\",\"scopes\":[\"contact:user.base:readonly\"]}'",
            },
            "exchange_code": {
                "summary": "Exchange OAuth authorization code for user token",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth exchange_code '{\"code\":\"xxx\",\"redirect_uri\":\"https://example.com/callback\"}'",
            },
            "refresh_user": {
                "summary": "Refresh a cached user token",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth refresh_user '{\"user_key\":\"ou_xxx\"}'",
            },
            "user_token": {
                "summary": "Get current valid user token",
                "example": "python3 scripts/cli/feishu/feishu_cli.py auth user_token '{\"user_key\":\"ou_xxx\"}'",
            },
        },
        "required_env": ["LARK_APP_ID", "LARK_APP_SECRET"],
        "recommended_env": [
            "FEISHU_OAUTH_REDIRECT_URI",
            "FEISHU_OAUTH_REDIRECT_ALLOWLIST",
            "FEISHU_OAUTH_SCOPES",
        ],
    }


def _help_modules() -> dict[str, Any]:
    rows = []
    for module, actions in ACTION_SPECS.items():
        rows.append({"module": module, "actions": sorted(actions.keys())})
    return {
        "success": True,
        "help_level": "modules",
        "modules": rows,
        "next": "python3 scripts/cli/feishu/feishu_cli.py help module --module <module>",
    }


def _help_module(module: str) -> dict[str, Any]:
    actions = ACTION_SPECS.get(module)
    if not actions:
        return {"success": False, "error": f"unknown module: {module}", "available_modules": sorted(ACTION_SPECS.keys())}

    entries = []
    for action, spec in actions.items():
        if not isinstance(spec, ActionSpec):
            return {"success": False, "error": f"invalid action spec for {module}.{action}"}
        entries.append(
            {
                "action": action,
                "aliases": _aliases_for_action(module, action),
                "summary": spec.summary,
                "required": list(spec.required),
                "optional": list(spec.optional),
                "example_args": spec.example,
                "request_schema": {"command": "tool", "module": module, "tool_action": action, "args": spec.example},
                "example": f"python3 scripts/cli/feishu/feishu_cli.py tool {module} {action} '{json.dumps(spec.example, ensure_ascii=False)}'",
            }
        )

    return {
        "success": True,
        "help_level": "module",
        "module": module,
        "actions": entries,
        "next": f"python3 scripts/cli/feishu/feishu_cli.py help action --module {module} --action <action>",
    }


def _help_action(module: str, action: str) -> dict[str, Any]:
    module_actions = ACTION_SPECS.get(module)
    if not module_actions:
        return {"success": False, "error": f"unknown module: {module}", "available_modules": sorted(ACTION_SPECS.keys())}
    normalized = action.strip().lower()
    canonical = TOOL_ACTION_ALIASES.get(module, {}).get(normalized, normalized)
    spec = module_actions.get(canonical)
    if not spec:
        return {
            "success": False,
            "error": f"unknown action: {module}.{action}",
            "available_actions": sorted(module_actions.keys()),
        }

    return {
        "success": True,
        "help_level": "action",
        "module": module,
        "action": canonical,
        "aliases": _aliases_for_action(module, canonical),
        "input_alias": action if canonical != normalized else "",
        "summary": spec.summary,
        "required": list(spec.required),
        "optional": list(spec.optional),
        "example_args": spec.example,
        "request_schema": {"command": "tool", "module": module, "tool_action": canonical, "args": spec.example},
        "example": f"python3 scripts/cli/feishu/feishu_cli.py tool {module} {canonical} '{json.dumps(spec.example, ensure_ascii=False)}'",
    }


def _run_help(request: dict[str, Any]) -> dict[str, Any]:
    topic = str(request.get("topic", "overview")).strip().lower() or "overview"

    if topic in {"overview", "top", "root"}:
        return _with_context(_help_overview(), command="help", topic="overview")
    if topic == "auth":
        return _with_context(_help_auth(), command="help", topic=topic)
    if topic == "modules":
        return _with_context(_help_modules(), command="help", topic=topic)
    if topic == "module":
        module = str(request.get("module", "")).strip().lower()
        if not module:
            return _with_context({"success": False, "error": "module is required for help topic=module"}, command="help", topic=topic)
        return _with_context(_help_module(module), command="help", topic=topic)
    if topic == "action":
        module = str(request.get("module", "")).strip().lower()
        action = str(request.get("action_name", "")).strip()
        if not action:
            action = str(request.get("tool_action", "")).strip()
        if not action:
            fallback = str(request.get("action", "")).strip()
            if fallback.lower() not in COMMAND_CHOICES:
                action = fallback
        if not module or not action:
            return _with_context(
                {"success": False, "error": "module and action_name are required for help topic=action"},
                command="help",
                topic=topic,
            )
        return _with_context(_help_action(module, action), command="help", topic=topic)

    return _with_context(
        {
            "success": False,
            "error": f"unknown help topic: {topic}",
            "available_topics": list(HELP_TOPICS),
        },
        command="help",
        topic=topic,
    )


def _run_auth(request: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    subcommand = str(request.get("subcommand", "status")).strip().lower() or "status"
    args, args_err = _collect_request_args(request, reserved={"command", "action", "subcommand"})
    if args_err:
        return _with_context({"success": False, "error": f"auth {args_err['error']}"}, command="auth", subcommand=subcommand)
    assert args is not None

    result: dict[str, Any]
    if subcommand == "status":
        result = auth_manager.status()
    elif subcommand == "tenant_token":
        token, err = auth_manager.get_tenant_token(force_refresh=bool(args.get("force_refresh", False)))
        if err:
            result = {"success": False, "error": err}
        else:
            result = {"success": True, "access_token": token, "access_token_preview": _mask_token(token)}
    elif subcommand == "oauth_url":
        result = auth_manager.build_oauth_url(
            redirect_uri=str(args.get("redirect_uri", "")),
            scopes=args.get("scopes"),
            state=str(args.get("state", "")),
        )
    elif subcommand == "exchange_code":
        result = auth_manager.exchange_code(
            code=str(args.get("code", "")),
            redirect_uri=str(args.get("redirect_uri", "")),
        )
    elif subcommand == "refresh_user":
        result = auth_manager.refresh_user_token(user_key=str(args.get("user_key", "")))
    elif subcommand == "user_token":
        token, err = auth_manager.get_user_access_token(
            user_key=str(args.get("user_key", "")),
            force_refresh=bool(args.get("force_refresh", False)),
        )
        if err:
            result = {"success": False, "error": err}
        else:
            result = {"success": True, "access_token": token, "access_token_preview": _mask_token(token)}
    else:
        result = {
            "success": False,
            "error": f"unknown auth subcommand: {subcommand}",
            "available": list(AUTH_SUBCOMMANDS),
        }
    return _with_context(result, command="auth", subcommand=subcommand)


def _run_tool(request: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    module = str(request.get("module", "")).strip().lower()
    action = str(request.get("tool_action", "")).strip() or str(request.get("action_name", "")).strip()
    if not action:
        fallback = str(request.get("action", "")).strip()
        if fallback.lower() not in COMMAND_CHOICES:
            action = fallback
    action = action.strip().lower()
    args, args_err = _collect_request_args(request, reserved={"command", "action", "module", "tool_action", "action_name"})
    if args_err:
        return _with_context({"success": False, "error": f"tool {args_err['error']}"}, command="tool", module=module, action=action)
    assert args is not None

    if not module or not action:
        return _with_context({"success": False, "error": "module and tool_action are required"}, command="tool", module=module, action=action)

    module_handlers = TOOL_HANDLERS.get(module)
    if not module_handlers:
        return _with_context(
            {"success": False, "error": f"unknown module: {module}", "available_modules": sorted(TOOL_HANDLERS.keys())},
            command="tool",
            module=module,
            action=action,
        )

    canonical = TOOL_ACTION_ALIASES.get(module, {}).get(action, action)
    handler = module_handlers.get(canonical)
    if handler is None:
        return _with_context(
            {
                "success": False,
                "error": f"unknown action: {module}.{action}",
                "available_actions": sorted(module_handlers.keys()),
            },
            command="tool",
            module=module,
            action=action,
        )

    result = handler(copy.deepcopy(args), auth_manager)
    return _with_context(
        result,
        command="tool",
        module=module,
        action=canonical,
        input_action=action if canonical != action else "",
    )


def _run_api(request: dict[str, Any], auth_manager: AuthManager) -> dict[str, Any]:
    method = str(request.get("method", "")).strip().upper()
    path = str(request.get("path", "")).strip()
    if not method or not path:
        return _with_context({"success": False, "error": "method and path are required"}, command="api", method=method, path=path)

    body = request.get("body")
    if body is not None and not isinstance(body, dict):
        return _with_context({"success": False, "error": "body must be an object"}, command="api", method=method, path=path)
    query = request.get("query")
    if query is not None and not isinstance(query, (dict, str)):
        return _with_context({"success": False, "error": "query must be an object or query string"}, command="api", method=method, path=path)

    auth_mode = str(request.get("auth", "tenant")).strip().lower() or "tenant"
    if auth_mode not in {"tenant", "user"}:
        return _with_context({"success": False, "error": "auth must be tenant or user"}, command="api", method=method, path=path)

    result = api_request(
        method,
        path,
        body,
        query=query,
        auth=auth_mode,
        user_key=str(request.get("user_key", "")),
        retry_on_auth_error=bool(request.get("retry_on_auth_error", True)),
        auth_manager=auth_manager,
    )
    failure = _api_failure(result)
    if failure:
        return _with_context(failure, command="api", method=method, path=path, auth=auth_mode)
    return _with_context({"success": True, "result": result}, command="api", method=method, path=path, auth=auth_mode)


def execute(request: dict[str, Any]) -> dict[str, Any]:
    if not isinstance(request, dict):
        return {"success": False, "error": "request must be an object"}

    command = _resolve_command(request)
    auth_manager = AuthManager()

    if command == "help":
        return _run_help(request)
    if command == "auth":
        return _run_auth(request, auth_manager)
    if command == "tool":
        return _run_tool(request, auth_manager)
    if command == "api":
        return _run_api(request, auth_manager)

    return {
        "success": False,
        "error": f"unknown command: {command}",
        "available": list(COMMAND_CHOICES),
        "command": command,
    }


def _parse_json_arg(raw: str) -> dict[str, Any]:
    text = raw.strip()
    if not text:
        return {}
    parsed = json.loads(text)
    if isinstance(parsed, dict):
        return parsed
    raise ValueError("JSON args must be an object")


def _build_request_from_cli(argv: list[str]) -> dict[str, Any]:
    parser = argparse.ArgumentParser(description="Unified Feishu CLI")
    subparsers = parser.add_subparsers(dest="command", required=False)

    help_parser = subparsers.add_parser("help", help="Progressive help")
    help_parser.add_argument("topic", nargs="?", default="overview", choices=HELP_TOPICS)
    help_parser.add_argument("--module", default="")
    help_parser.add_argument("--action", dest="action_name", default="")

    auth_parser = subparsers.add_parser("auth", help="Auth operations")
    auth_parser.add_argument(
        "subcommand",
        choices=AUTH_SUBCOMMANDS,
    )
    auth_parser.add_argument("args", nargs="?", default="{}")

    tool_parser = subparsers.add_parser("tool", help="Run high-level tool action")
    tool_parser.add_argument("module")
    tool_parser.add_argument("tool_action")
    tool_parser.add_argument("args", nargs="?", default="{}")

    api_parser = subparsers.add_parser("api", help="Raw Open API call")
    api_parser.add_argument("method")
    api_parser.add_argument("path")
    api_parser.add_argument("--body", default="{}")
    api_parser.add_argument("--query", default="")
    api_parser.add_argument("--auth", default="tenant", choices=["tenant", "user"])
    api_parser.add_argument("--user-key", default="")

    parsed = parser.parse_args(argv)
    command = getattr(parsed, "command", None)
    if not command:
        return {"command": "help", "topic": "overview"}

    if command == "help":
        return {
            "command": "help",
            "topic": parsed.topic,
            "module": parsed.module,
            "action_name": parsed.action_name,
        }

    if command == "auth":
        return {
            "command": "auth",
            "subcommand": parsed.subcommand,
            "args": _parse_json_arg(parsed.args),
        }

    if command == "tool":
        return {
            "command": "tool",
            "module": parsed.module,
            "tool_action": parsed.tool_action,
            "args": _parse_json_arg(parsed.args),
        }

    query: dict[str, Any] | str | None
    if parsed.query.strip():
        if parsed.query.strip().startswith("{"):
            query = _parse_json_arg(parsed.query)
        else:
            query = parsed.query.strip()
    else:
        query = None

    return {
        "command": "api",
        "method": parsed.method,
        "path": parsed.path,
        "body": _parse_json_arg(parsed.body) if parsed.body.strip() else None,
        "query": query,
        "auth": parsed.auth,
        "user_key": parsed.user_key,
    }


def main() -> None:
    try:
        request = _build_request_from_cli(sys.argv[1:])
        result = execute(request)
    except Exception as exc:
        result = {"success": False, "error": f"{type(exc).__name__}: {exc}"}

    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
