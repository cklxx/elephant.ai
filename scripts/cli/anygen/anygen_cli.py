#!/usr/bin/env python3
"""Unified AnyGen CLI runtime with progressive help for skills.

This runtime wraps the core task-manager capabilities from
https://github.com/AnyGenIO/anygen-skills and exposes a stable JSON API:
- command=help (overview/modules/module/action)
- command=task (create/status/poll/download/run)
"""

from __future__ import annotations

import base64
import json
import mimetypes
import os
import time
from pathlib import Path
from typing import Any

try:
    import requests
except ImportError:  # pragma: no cover - exercised in runtime, not unit tests.
    requests = None

API_BASE = "https://www.anygen.io"
COMMAND_CHOICES = ("help", "task")
HELP_TOPICS = ("overview", "modules", "module", "action")
TASK_ACTIONS = ("create", "status", "poll", "download", "run")
VALID_OPERATIONS = ("chat", "slide", "doc", "storybook", "data_analysis", "website", "smart_draw")
DOWNLOADABLE_OPERATIONS = {"slide", "doc", "smart_draw"}
DEFAULT_POLL_INTERVAL_SECONDS = 3
DEFAULT_MAX_POLL_SECONDS = 900

MODULES = {
    "task-manager": {
        "title": "AnyGen Task Manager",
        "description": "AnyGen OpenAPI content generation (slide/doc/storybook/data_analysis/website/chat/smart_draw).",
        "actions": list(TASK_ACTIONS),
        "supports_execution": True,
        "upstream_path": "task-manager/skill.md",
    },
    "finance-report": {
        "title": "AnyGen Finance Report",
        "description": "Equity research report playbook (methodology + workflows). Guide-only in this wrapper.",
        "actions": ["guide"],
        "supports_execution": False,
        "upstream_path": "finance-report/skill.md",
    },
}

MODULE_ALIASES = {
    "task": "task-manager",
    "task_manager": "task-manager",
    "manager": "task-manager",
    "finance": "finance-report",
    "finance_report": "finance-report",
}

ACTION_DOCS = {
    "create": {
        "description": "Create an AnyGen task and return task_id.",
        "required": ["operation", "prompt"],
        "optional": [
            "language",
            "slide_count",
            "template",
            "ratio",
            "export_format",
            "files",
            "style",
            "headers",
        ],
        "example": {
            "action": "task",
            "task_action": "create",
            "operation": "slide",
            "prompt": "Quarterly roadmap presentation",
            "style": "business formal",
        },
    },
    "status": {
        "description": "Single non-blocking status query for a task.",
        "required": ["task_id"],
        "optional": ["headers"],
        "example": {
            "action": "task",
            "task_action": "status",
            "task_id": "task_abc123",
        },
    },
    "poll": {
        "description": "Poll a task until completed/failed/timeout. Optionally auto-download output.",
        "required": ["task_id"],
        "optional": ["max_time", "poll_interval", "output", "headers"],
        "example": {
            "action": "task",
            "task_action": "poll",
            "task_id": "task_abc123",
            "output": "./output",
        },
    },
    "download": {
        "description": "Download completed task file to output directory.",
        "required": ["task_id"],
        "optional": ["output", "headers"],
        "example": {
            "action": "task",
            "task_action": "download",
            "task_id": "task_abc123",
            "output": "./output",
        },
    },
    "run": {
        "description": "Create + poll (+auto-download for downloadable operations).",
        "required": ["operation", "prompt"],
        "optional": [
            "language",
            "slide_count",
            "template",
            "ratio",
            "export_format",
            "files",
            "style",
            "output",
            "max_time",
            "poll_interval",
            "headers",
        ],
        "example": {
            "action": "task",
            "task_action": "run",
            "operation": "doc",
            "prompt": "Technical design document for realtime notifications",
            "output": "./output",
        },
    },
}


def execute(request: dict[str, Any]) -> dict[str, Any]:
    """Execute a JSON-form AnyGen CLI request."""
    if not isinstance(request, dict):
        return {"success": False, "error": "request must be an object"}

    payload = dict(request)
    command = _resolve_command(payload)

    if command == "help":
        return _handle_help(payload)
    if command == "task":
        return _handle_task(payload)

    return {
        "success": False,
        "error": f"unknown command: {command}",
        "valid_commands": list(COMMAND_CHOICES),
    }


def _resolve_command(payload: dict[str, Any]) -> str:
    command = str(payload.get("command", "")).strip().lower()
    action = str(payload.get("action", "")).strip().lower()

    if command in COMMAND_CHOICES:
        return command
    if action in COMMAND_CHOICES:
        return action

    if action in TASK_ACTIONS:
        return "task"
    if payload.get("task_action") or payload.get("operation") or payload.get("task_id"):
        return "task"

    return "help"


def _normalize_module(value: str) -> str:
    key = value.strip().lower()
    if key in MODULES:
        return key
    return MODULE_ALIASES.get(key, key)


def _handle_help(payload: dict[str, Any]) -> dict[str, Any]:
    topic = str(payload.get("topic", "overview")).strip().lower() or "overview"
    if topic not in HELP_TOPICS:
        return {
            "success": False,
            "command": "help",
            "error": f"unsupported help topic: {topic}",
            "valid_topics": list(HELP_TOPICS),
        }

    if topic == "overview":
        return {
            "success": True,
            "command": "help",
            "topic": "overview",
            "summary": "AnyGen wrapper for upstream anygen-skills with progressive disclosure.",
            "next": [
                {"command": "help", "topic": "modules"},
                {"command": "help", "topic": "module", "module": "task-manager"},
                {"command": "help", "topic": "action", "module": "task-manager", "action_name": "create"},
            ],
            "commands": {
                "help": "Discover usage incrementally (overview/modules/module/action)",
                "task": "Execute AnyGen Task Manager actions",
            },
        }

    if topic == "modules":
        modules = []
        for module_name, module in MODULES.items():
            modules.append(
                {
                    "module": module_name,
                    "title": module["title"],
                    "description": module["description"],
                    "supports_execution": module["supports_execution"],
                    "actions": module["actions"],
                }
            )

        return {
            "success": True,
            "command": "help",
            "topic": "modules",
            "modules": modules,
            "next": [{"command": "help", "topic": "module", "module": "task-manager"}],
        }

    module_name = _normalize_module(str(payload.get("module", "")))
    if not module_name:
        return {
            "success": False,
            "command": "help",
            "topic": topic,
            "error": "module is required for help topic module/action",
        }

    module = MODULES.get(module_name)
    if not module:
        return {
            "success": False,
            "command": "help",
            "topic": topic,
            "error": f"unknown module: {module_name}",
            "valid_modules": sorted(MODULES.keys()),
        }

    if topic == "module":
        return {
            "success": True,
            "command": "help",
            "topic": "module",
            "module": module_name,
            "title": module["title"],
            "description": module["description"],
            "supports_execution": module["supports_execution"],
            "actions": module["actions"],
            "upstream": {
                "repo": "https://github.com/AnyGenIO/anygen-skills",
                "path": module["upstream_path"],
            },
            "next": [{"command": "help", "topic": "action", "module": module_name, "action_name": module["actions"][0]}],
        }

    action_name = str(payload.get("action_name", "") or payload.get("task_action", "") or payload.get("action", "")).strip().lower()
    if not action_name:
        return {
            "success": False,
            "command": "help",
            "topic": "action",
            "module": module_name,
            "error": "action_name is required for help topic action",
        }

    if module_name != "task-manager":
        return {
            "success": True,
            "command": "help",
            "topic": "action",
            "module": module_name,
            "action": action_name,
            "description": "Guide-only module in this wrapper.",
            "supports_execution": False,
        }

    action_doc = ACTION_DOCS.get(action_name)
    if not action_doc:
        return {
            "success": False,
            "command": "help",
            "topic": "action",
            "module": module_name,
            "error": f"unknown action: {action_name}",
            "valid_actions": list(TASK_ACTIONS),
        }

    return {
        "success": True,
        "command": "help",
        "topic": "action",
        "module": module_name,
        "action": action_name,
        **action_doc,
    }


def _handle_task(payload: dict[str, Any]) -> dict[str, Any]:
    if requests is None:
        return {"success": False, "error": "requests library not installed. Install with: pip3 install requests"}

    module_name = _normalize_module(str(payload.get("module", "task-manager")))
    if module_name not in MODULES:
        return {
            "success": False,
            "command": "task",
            "error": f"unknown module: {module_name}",
            "valid_modules": sorted(MODULES.keys()),
        }

    if module_name != "task-manager":
        return {
            "success": False,
            "command": "task",
            "module": module_name,
            "error": "finance-report is guide-only in this wrapper; no executable task action",
            "hint": "Use help(topic=module,module=finance-report) for workflow guidance.",
        }

    action_name = str(
        payload.get("task_action")
        or payload.get("action_name")
        or (payload.get("action") if str(payload.get("action", "")).strip().lower() in TASK_ACTIONS else "")
        or ""
    ).strip().lower()

    if not action_name:
        return {
            "success": False,
            "command": "task",
            "module": module_name,
            "error": "task_action is required",
            "valid_actions": list(TASK_ACTIONS),
        }
    if action_name not in TASK_ACTIONS:
        return {
            "success": False,
            "command": "task",
            "module": module_name,
            "error": f"unknown task action: {action_name}",
            "valid_actions": list(TASK_ACTIONS),
        }

    api_key = _resolve_api_key(payload)
    if not api_key:
        return {
            "success": False,
            "command": "task",
            "module": module_name,
            "action": action_name,
            "error": "ANYGEN_API_KEY not set and api_key not provided",
        }

    task_payload = _task_payload(payload)
    headers = _parse_headers(task_payload.get("headers"))

    if action_name == "create":
        return _create_task(api_key, task_payload, headers)

    if action_name == "status":
        task_id = str(task_payload.get("task_id", "")).strip()
        if not task_id:
            return {"success": False, "error": "task_id is required"}
        return _status_task(api_key, task_id, headers)

    if action_name == "poll":
        task_id = str(task_payload.get("task_id", "")).strip()
        if not task_id:
            return {"success": False, "error": "task_id is required"}
        max_time = _int_value(task_payload.get("max_time"), DEFAULT_MAX_POLL_SECONDS)
        poll_interval = _int_value(task_payload.get("poll_interval"), DEFAULT_POLL_INTERVAL_SECONDS)
        output_dir = str(task_payload.get("output", "")).strip()
        return _poll_task(api_key, task_id, headers, max_time=max_time, poll_interval=poll_interval, output_dir=output_dir)

    if action_name == "download":
        task_id = str(task_payload.get("task_id", "")).strip()
        if not task_id:
            return {"success": False, "error": "task_id is required"}
        output_dir = str(task_payload.get("output", "")).strip() or "."
        return _download_task(api_key, task_id, output_dir, headers)

    # run
    return _run_task(api_key, task_payload, headers)


def _task_payload(payload: dict[str, Any]) -> dict[str, Any]:
    dropped = {
        "command",
        "module",
        "topic",
        "task_action",
        "action_name",
    }
    out: dict[str, Any] = {}
    for key, value in payload.items():
        if key in dropped:
            continue
        if key == "action" and str(value).strip().lower() in COMMAND_CHOICES:
            continue
        out[key] = value
    return out


def _resolve_api_key(payload: dict[str, Any]) -> str:
    direct = str(payload.get("api_key", "")).strip()
    if direct:
        return direct
    nested = payload.get("args")
    if isinstance(nested, dict):
        nested_key = str(nested.get("api_key", "")).strip()
        if nested_key:
            return nested_key
    return os.environ.get("ANYGEN_API_KEY", "").strip()


def _int_value(raw: Any, default: int) -> int:
    try:
        parsed = int(raw)
    except (TypeError, ValueError):
        return default
    if parsed <= 0:
        return default
    return parsed


def _parse_headers(raw: Any) -> dict[str, str]:
    if isinstance(raw, dict):
        out: dict[str, str] = {}
        for key, value in raw.items():
            k = str(key).strip()
            v = str(value).strip()
            if k and v:
                out[k] = v
        return out

    if isinstance(raw, list):
        out = {}
        for item in raw:
            text = str(item)
            if ":" not in text:
                continue
            key, value = text.split(":", 1)
            k = key.strip()
            v = value.strip()
            if k and v:
                out[k] = v
        return out

    return {}


def _create_task(api_key: str, payload: dict[str, Any], headers: dict[str, str]) -> dict[str, Any]:
    operation = str(payload.get("operation", "")).strip().lower()
    prompt = str(payload.get("prompt", "")).strip()
    if not operation:
        return {"success": False, "error": "operation is required"}
    if operation not in VALID_OPERATIONS:
        return {"success": False, "error": f"invalid operation: {operation}", "valid_operations": list(VALID_OPERATIONS)}
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    files_raw = payload.get("files")
    if isinstance(files_raw, str):
        files = [files_raw]
    elif isinstance(files_raw, list):
        files = [str(item) for item in files_raw if str(item).strip()]
    else:
        files = []

    encoded_files: list[dict[str, str]] = []
    for file_path in files:
        encoded, err = _encode_file(file_path)
        if err:
            return {"success": False, "error": err}
        encoded_files.append(encoded)

    final_prompt = prompt
    style = str(payload.get("style", "")).strip()
    if style:
        final_prompt = f"{prompt}\n\nStyle requirement: {style}"

    request_body: dict[str, Any] = {
        "auth_token": api_key if api_key.startswith("Bearer ") else f"Bearer {api_key}",
        "operation": operation,
        "prompt": final_prompt,
    }

    for key in ("language", "slide_count", "template", "ratio", "export_format"):
        value = payload.get(key)
        if value not in (None, ""):
            request_body[key] = value

    if encoded_files:
        request_body["files"] = encoded_files

    request_headers = {"Content-Type": "application/json"}
    request_headers.update(headers)

    try:
        response = requests.post(f"{API_BASE}/v1/openapi/tasks", json=request_body, headers=request_headers, timeout=30)
    except requests.RequestException as exc:
        return {"success": False, "error": f"request failed: {exc}"}

    try:
        body = response.json()
    except ValueError:
        body = {"success": False, "error": f"invalid JSON response: {response.text[:500]}"}

    if response.status_code != 200:
        return {
            "success": False,
            "error": body.get("error") or body.get("message") or f"HTTP {response.status_code}",
            "http_status": response.status_code,
            "response": body,
        }

    if not body.get("success"):
        return {
            "success": False,
            "error": body.get("error") or "task creation failed",
            "response": body,
        }

    task_id = str(body.get("task_id", "")).strip()
    return {
        "success": True,
        "module": "task-manager",
        "action": "create",
        "operation": operation,
        "task_id": task_id,
        "task_url": f"{API_BASE}/task/{task_id}" if task_id else "",
        "response": body,
    }


def _query_task(api_key: str, task_id: str, headers: dict[str, str]) -> tuple[dict[str, Any] | None, str]:
    auth_header = api_key if api_key.startswith("Bearer ") else f"Bearer {api_key}"
    request_headers = {"Authorization": auth_header}
    request_headers.update(headers)

    try:
        response = requests.get(f"{API_BASE}/v1/openapi/tasks/{task_id}", headers=request_headers, timeout=30)
    except requests.RequestException as exc:
        return None, f"request failed: {exc}"

    try:
        body = response.json()
    except ValueError:
        return None, f"invalid JSON response: {response.text[:500]}"

    if response.status_code != 200 and not isinstance(body, dict):
        return None, f"HTTP {response.status_code}"

    if isinstance(body, dict):
        return body, ""
    return None, "invalid response type"


def _status_task(api_key: str, task_id: str, headers: dict[str, str]) -> dict[str, Any]:
    task, err = _query_task(api_key, task_id, headers)
    if err:
        return {"success": False, "error": err}
    if task is None:
        return {"success": False, "error": "task not found"}

    status = str(task.get("status", "")).strip().lower()
    progress = int(task.get("progress", 0) or 0)
    output = task.get("output") if isinstance(task.get("output"), dict) else {}
    task_url = str(output.get("task_url") or f"{API_BASE}/task/{task_id}")

    result: dict[str, Any] = {
        "success": True,
        "module": "task-manager",
        "action": "status",
        "task_id": task_id,
        "status": status,
        "progress": progress,
        "task_url": task_url,
    }

    for field in ("file_name", "file_url", "thumbnail_url"):
        value = output.get(field)
        if value:
            result[field] = value

    if status == "failed":
        result["task_error"] = task.get("error", "unknown error")

    return result


def _poll_task(
    api_key: str,
    task_id: str,
    headers: dict[str, str],
    *,
    max_time: int,
    poll_interval: int,
    output_dir: str,
) -> dict[str, Any]:
    start = time.time()
    last_progress = -1
    history: list[dict[str, Any]] = []

    while True:
        if time.time() - start > max_time:
            return {
                "success": False,
                "module": "task-manager",
                "action": "poll",
                "task_id": task_id,
                "error": f"poll timeout after {max_time}s",
                "progress_history": history,
            }

        status_result = _status_task(api_key, task_id, headers)
        if not status_result.get("success"):
            return {
                "success": False,
                "module": "task-manager",
                "action": "poll",
                "task_id": task_id,
                "error": status_result.get("error", "status query failed"),
                "progress_history": history,
            }

        status = str(status_result.get("status", "")).lower()
        progress = int(status_result.get("progress", 0) or 0)
        if progress != last_progress:
            history.append({"status": status, "progress": progress})
            last_progress = progress

        if status == "completed":
            completed_result: dict[str, Any] = {
                "success": True,
                "module": "task-manager",
                "action": "poll",
                "task_id": task_id,
                "status": status,
                "progress": progress,
                "task_url": status_result.get("task_url", ""),
                "progress_history": history,
            }

            for field in ("file_name", "file_url", "thumbnail_url"):
                if status_result.get(field):
                    completed_result[field] = status_result[field]

            if output_dir and status_result.get("file_url"):
                download_result = _download_task(api_key, task_id, output_dir, headers)
                if download_result.get("success"):
                    completed_result["local_file"] = download_result.get("local_file", "")
                else:
                    completed_result["download_error"] = download_result.get("error", "download failed")

            return completed_result

        if status == "failed":
            return {
                "success": False,
                "module": "task-manager",
                "action": "poll",
                "task_id": task_id,
                "status": status,
                "error": status_result.get("task_error", "task failed"),
                "task_url": status_result.get("task_url", ""),
                "progress_history": history,
            }

        time.sleep(poll_interval)


def _download_task(api_key: str, task_id: str, output_dir: str, headers: dict[str, str]) -> dict[str, Any]:
    status_result = _status_task(api_key, task_id, headers)
    if not status_result.get("success"):
        return status_result

    if status_result.get("status") != "completed":
        return {
            "success": False,
            "module": "task-manager",
            "action": "download",
            "task_id": task_id,
            "error": f"task not completed, current status={status_result.get('status')}",
        }

    file_url = str(status_result.get("file_url", "")).strip()
    file_name = str(status_result.get("file_name", "")).strip() or "output.bin"
    if not file_url:
        return {
            "success": False,
            "module": "task-manager",
            "action": "download",
            "task_id": task_id,
            "error": "no file_url in task output",
        }

    try:
        response = requests.get(file_url, timeout=120)
        response.raise_for_status()
    except requests.RequestException as exc:
        return {
            "success": False,
            "module": "task-manager",
            "action": "download",
            "task_id": task_id,
            "error": f"download failed: {exc}",
        }

    output_path = Path(output_dir or ".").expanduser().resolve()
    output_path.mkdir(parents=True, exist_ok=True)
    local_file = output_path / file_name
    local_file.write_bytes(response.content)

    return {
        "success": True,
        "module": "task-manager",
        "action": "download",
        "task_id": task_id,
        "local_file": str(local_file),
        "task_url": status_result.get("task_url", ""),
        "thumbnail_url": status_result.get("thumbnail_url", ""),
    }


def _run_task(api_key: str, payload: dict[str, Any], headers: dict[str, str]) -> dict[str, Any]:
    create_result = _create_task(api_key, payload, headers)
    if not create_result.get("success"):
        return create_result

    task_id = str(create_result.get("task_id", "")).strip()
    if not task_id:
        return {"success": False, "error": "task created but task_id missing"}

    operation = str(payload.get("operation", "")).strip().lower()
    output_dir = str(payload.get("output", "")).strip()
    if not output_dir and operation in DOWNLOADABLE_OPERATIONS:
        output_dir = "."

    max_time = _int_value(payload.get("max_time"), DEFAULT_MAX_POLL_SECONDS)
    poll_interval = _int_value(payload.get("poll_interval"), DEFAULT_POLL_INTERVAL_SECONDS)
    poll_result = _poll_task(
        api_key,
        task_id,
        headers,
        max_time=max_time,
        poll_interval=poll_interval,
        output_dir=output_dir,
    )

    poll_result.setdefault("module", "task-manager")
    poll_result.setdefault("action", "run")
    poll_result["created_task_id"] = task_id
    return poll_result


def _encode_file(file_path: str) -> tuple[dict[str, str], str]:
    path = Path(file_path).expanduser()
    if not path.is_file():
        return {}, f"file not found: {file_path}"

    content = path.read_bytes()
    mime_type, _ = mimetypes.guess_type(path.name)
    if not mime_type:
        mime_type = "application/octet-stream"

    return (
        {
            "file_name": path.name,
            "file_type": mime_type,
            "file_data": base64.b64encode(content).decode("utf-8"),
        },
        "",
    )


if __name__ == "__main__":  # pragma: no cover
    import argparse

    parser = argparse.ArgumentParser(description="AnyGen JSON CLI")
    parser.add_argument("request", nargs="?", default="{}", help='JSON request, e.g. {"command":"help"}')
    args = parser.parse_args()

    try:
        request_obj = json.loads(args.request)
    except json.JSONDecodeError as exc:
        result = {"success": False, "error": f"invalid JSON request: {exc}"}
    else:
        result = execute(request_obj)

    print(json.dumps(result, ensure_ascii=False, indent=2))
