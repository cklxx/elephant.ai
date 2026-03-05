"""Shared CLI parsing and text rendering contract for skill runners."""

from __future__ import annotations

import ast
import json
import re
from typing import Any, Sequence

try:
    import yaml
except Exception:  # pragma: no cover - optional dependency guard
    yaml = None

_INT_RE = re.compile(r"^[+-]?\d+$")
_FLOAT_RE = re.compile(
    r"^[+-]?(?:\d+\.\d*|\.\d+|\d+\.\d*[eE][+-]?\d+|\.\d+[eE][+-]?\d+|\d+[eE][+-]?\d+)$"
)

_TRUE_LITERALS = {"true", "yes", "on"}
_FALSE_LITERALS = {"false", "no", "off"}
_NULL_LITERALS = {"null", "none", "nil", "~"}


def _normalize_key(key: str) -> str:
    return key.strip().replace("-", "_")


def _split_top_level(value: str) -> list[str]:
    parts: list[str] = []
    buffer: list[str] = []
    depth = 0
    quote: str | None = None

    for char in value:
        if quote:
            buffer.append(char)
            if char == quote:
                quote = None
            continue

        if char in ('"', "'"):
            quote = char
            buffer.append(char)
            continue

        if char in "[{(":
            depth += 1
            buffer.append(char)
            continue

        if char in "]})":
            depth = max(depth - 1, 0)
            buffer.append(char)
            continue

        if char == "," and depth == 0:
            parts.append("".join(buffer).strip())
            buffer.clear()
            continue

        buffer.append(char)

    tail = "".join(buffer).strip()
    if tail:
        parts.append(tail)
    return parts


def _parse_yaml_like_container(value: str) -> Any:
    stripped = value.strip()
    if stripped.startswith("[") and stripped.endswith("]"):
        items = _split_top_level(stripped[1:-1])
        return [_coerce_value(item) for item in items if item]

    if stripped.startswith("{") and stripped.endswith("}"):
        items = _split_top_level(stripped[1:-1])
        result: dict[str, Any] = {}
        for item in items:
            if ":" not in item:
                return value
            raw_key, raw_val = item.split(":", 1)
            key = raw_key.strip().strip("'\"")
            if not key:
                return value
            result[key] = _coerce_value(raw_val.strip())
        return result

    return value


def _coerce_container_literal(value: str) -> Any:
    for parser in (json.loads, ast.literal_eval):
        try:
            parsed = parser(value)
        except Exception:
            continue
        if isinstance(parsed, (list, dict)):
            return parsed

    if yaml is not None:
        try:
            parsed = yaml.safe_load(value)
        except Exception:
            return value
        if isinstance(parsed, (list, dict)):
            return parsed

    return _parse_yaml_like_container(value)


def _coerce_value(raw_value: str) -> Any:
    value = raw_value.strip()
    if value == "":
        return ""

    lowered = value.lower()
    if lowered in _TRUE_LITERALS:
        return True
    if lowered in _FALSE_LITERALS:
        return False
    if lowered in _NULL_LITERALS:
        return None

    if _INT_RE.fullmatch(value):
        try:
            return int(value)
        except ValueError:
            pass

    if _FLOAT_RE.fullmatch(value):
        try:
            return float(value)
        except ValueError:
            pass

    if (value.startswith("[") and value.endswith("]")) or (
        value.startswith("{") and value.endswith("}")
    ):
        return _coerce_container_literal(value)

    return value


def _store_arg(args: dict[str, Any], key: str, value: Any) -> None:
    if key in args:
        existing = args[key]
        if isinstance(existing, list):
            existing.append(value)
        else:
            args[key] = [existing, value]
        return
    args[key] = value


def parse_cli_args(argv: Sequence[str], primary_key: str = "action") -> dict[str, Any]:
    """Parse CLI arguments into a payload dict.

    Supported forms:
    - First positional token maps to ``primary_key``.
    - ``--key value`` and ``--key=value``.
    - Bool flags (`--flag`, `--no-flag`).
    - Repeated keys accumulate into a list.
    """

    args: dict[str, Any] = {}
    positionals: list[str] = []

    index = 0
    while index < len(argv):
        token = argv[index]

        if token == "--":
            positionals.extend(argv[index + 1 :])
            break

        if token.startswith("--") and len(token) > 2:
            raw = token[2:]
            if "=" in raw:
                key, raw_value = raw.split("=", 1)
                normalized_key = _normalize_key(key)
                if normalized_key:
                    _store_arg(args, normalized_key, _coerce_value(raw_value))
                index += 1
                continue

            key = _normalize_key(raw)
            if not key:
                index += 1
                continue

            next_token = argv[index + 1] if index + 1 < len(argv) else None
            if key.startswith("no_") and (
                next_token is None or next_token.startswith("--")
            ):
                _store_arg(args, key[3:], False)
                index += 1
                continue

            if next_token is not None and not next_token.startswith("--"):
                _store_arg(args, key, _coerce_value(next_token))
                index += 2
                continue

            _store_arg(args, key, True)
            index += 1
            continue

        positionals.append(token)
        index += 1

    if positionals and primary_key and primary_key not in args:
        args[primary_key] = positionals[0]
        positionals = positionals[1:]

    if positionals:
        _store_arg(args, "positionals", [_coerce_value(item) for item in positionals])

    return args


def _scalar_text(value: Any) -> str:
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value).replace("\n", "\\n")


def _format_text(value: Any, indent: int = 0) -> str:
    prefix = " " * indent

    if isinstance(value, dict):
        if not value:
            return f"{prefix}{{}}"
        lines: list[str] = []
        for key, item in value.items():
            if isinstance(item, (dict, list)):
                lines.append(f"{prefix}{key}:")
                lines.append(_format_text(item, indent + 2))
            else:
                lines.append(f"{prefix}{key}: {_scalar_text(item)}")
        return "\n".join(lines)

    if isinstance(value, list):
        if not value:
            return f"{prefix}[]"
        lines = []
        for item in value:
            if isinstance(item, (dict, list)):
                lines.append(f"{prefix}-")
                lines.append(_format_text(item, indent + 2))
            else:
                lines.append(f"{prefix}- {_scalar_text(item)}")
        return "\n".join(lines)

    return f"{prefix}{_scalar_text(value)}"


def render_result(result: Any) -> tuple[str, str, int]:
    """Convert run() output to text stdout/stderr and process exit code."""

    success = True
    payload: Any = result
    if isinstance(result, dict):
        success = bool(result.get("success", True))
        payload = dict(result)
        payload.pop("success", None)

    if success:
        stdout_parts: list[str] = []
        if isinstance(payload, dict):
            message = payload.pop("message", "")
            if message not in ("", None):
                stdout_parts.append(str(message))
            if payload:
                stdout_parts.append(_format_text(payload))
        elif payload not in (None, ""):
            stdout_parts.append(_format_text(payload))
        return "\n".join(stdout_parts).strip(), "", 0

    stderr_parts: list[str] = []
    if isinstance(payload, dict):
        error_message = payload.pop("error", "")
        if error_message in ("", None):
            error_message = payload.pop("message", "")
        if error_message not in ("", None):
            stderr_parts.append(str(error_message))
        if payload:
            stderr_parts.append(_format_text(payload))
    elif payload not in (None, ""):
        stderr_parts.append(_format_text(payload))

    if not stderr_parts:
        stderr_parts.append("command failed")

    return "", "\n".join(stderr_parts).strip(), 1
