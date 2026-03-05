#!/usr/bin/env python3
"""Standalone NotebookLM CLI runtime for LLM-friendly local execution."""

from __future__ import annotations

from pathlib import Path
import os
import re
import shlex
import shutil
import subprocess
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

NLM_BIN = os.environ.get("NOTEBOOKLM_CLI_BIN", "nlm")
DEFAULT_TIMEOUT = int(os.environ.get("NOTEBOOKLM_CLI_TIMEOUT", "60"))

COMMAND_CHOICES = ("help", "auth", "notebook", "source", "query", "report", "studio", "raw")
HELP_TOPICS = ("overview", "schema", "auth", "notebook", "source", "query", "report", "studio", "raw", "progressive")

COMMAND_SPECS = {
    "auth": {
        "description": "Authentication and profile management",
        "default_op": "login",
        "ops": {
            "login": {"required": [], "optional": ["profile"]},
            "check": {"required": [], "optional": ["profile"]},
            "switch": {"required": ["profile"], "optional": []},
            "profile_list": {"required": [], "optional": []},
            "profile_rename": {"required": ["old_name", "new_name"], "optional": []},
            "profile_delete": {"required": ["profile", "confirm"], "optional": []},
            "help": {"required": [], "optional": []},
        },
    },
    "notebook": {
        "description": "Notebook lifecycle and notebook-scoped query",
        "default_op": "list",
        "ops": {
            "list": {"required": [], "optional": ["full", "json", "quiet", "title", "profile"]},
            "create": {"required": ["title"], "optional": ["profile"]},
            "get": {"required": ["notebook_id"], "optional": ["json", "profile"]},
            "describe": {"required": ["notebook_id"], "optional": ["json", "profile"]},
            "rename": {"required": ["notebook_id", "title"], "optional": ["profile"]},
            "query": {
                "required": ["notebook_id", "question"],
                "optional": ["conversation_id", "source_ids", "json", "timeout", "profile"],
            },
            "delete": {"required": ["notebook_id", "confirm"], "optional": ["profile"]},
            "help": {"required": [], "optional": []},
        },
    },
    "source": {
        "description": "Source lifecycle under a notebook",
        "default_op": "list",
        "ops": {
            "list": {
                "required": ["notebook_id"],
                "optional": ["full", "drive", "skip_freshness", "json", "quiet", "url", "profile"],
            },
            "add_url": {"required": ["notebook_id", "url|urls"], "optional": ["title", "wait", "profile"]},
            "add_text": {"required": ["notebook_id", "text", "title"], "optional": ["wait", "profile"]},
            "add_drive": {"required": ["notebook_id", "drive_id"], "optional": ["doc_type", "title", "wait", "profile"]},
            "add_youtube": {"required": ["notebook_id", "youtube"], "optional": ["title", "wait", "profile"]},
            "add_file": {"required": ["notebook_id", "file"], "optional": ["title", "wait", "profile"]},
            "get": {"required": ["source_id"], "optional": ["json", "profile"]},
            "describe": {"required": ["source_id"], "optional": ["json", "profile"]},
            "content": {"required": ["source_id"], "optional": ["json", "output", "profile"]},
            "rename": {"required": ["source_id", "title", "notebook_id"], "optional": ["profile"]},
            "delete": {"required": ["source_id|source_ids", "confirm"], "optional": ["profile"]},
            "help": {"required": [], "optional": []},
        },
    },
    "query": {
        "description": "Shorthand for notebook query",
        "default_op": "run",
        "ops": {
            "run": {
                "required": ["notebook_id", "question"],
                "optional": ["conversation_id", "source_ids", "json", "timeout", "profile"],
            }
        },
    },
    "report": {
        "description": "Create report artifacts",
        "default_op": "create",
        "ops": {
            "create": {
                "required": ["notebook_id", "confirm"],
                "optional": ["format", "prompt", "language", "source_ids", "profile"],
            },
            "help": {"required": [], "optional": []},
        },
    },
    "studio": {
        "description": "Inspect and manage studio artifacts",
        "default_op": "status",
        "ops": {
            "status": {"required": ["notebook_id"], "optional": ["full", "json", "profile"]},
            "rename": {"required": ["artifact_id", "title"], "optional": ["profile"]},
            "delete": {"required": ["notebook_id", "artifact_id", "confirm"], "optional": ["profile"]},
            "help": {"required": [], "optional": []},
        },
    },
    "raw": {
        "description": "Pass through nlm args directly",
        "default_op": "exec",
        "ops": {
            "exec": {"required": ["argv"], "optional": ["confirm"]},
            "safety": {
                "required": [],
                "optional": [
                    "blocks interactive `nlm chat start`",
                    "all delete commands require confirm=true unless argv already has --confirm/-y",
                ],
            },
        },
    },
}

TOPIC_TO_CLI_HELP_ARGS = {
    "overview": ["--help"],
    "auth": ["login", "--help"],
    "notebook": ["notebook", "--help"],
    "source": ["source", "--help"],
    "query": ["notebook", "query", "--help"],
    "report": ["report", "--help"],
    "studio": ["studio", "--help"],
}


def _as_bool(value: object) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    if isinstance(value, str):
        return value.strip().lower() in {"1", "true", "yes", "y", "on"}
    return False


def _base_result(success: bool, *, command: str = "", error: str = "", hints: list[str] | None = None) -> dict:
    result = {
        "success": success,
        "command": command,
        "exit_code": 0 if success else 1,
        "stdout": "",
        "stderr": "",
        "hints": hints or [],
    }
    if error:
        result["error"] = error
    return result


def _require_text(payload: dict, key: str) -> str:
    value = str(payload.get(key, "")).strip()
    if not value:
        raise ValueError(f"{key} is required")
    return value


def _optional_text(payload: dict, key: str) -> str:
    return str(payload.get(key, "")).strip()


def _as_items(value: object) -> list[str]:
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return []
        if "," in text or " " in text:
            return [item.strip() for item in re.split(r"[\s,]+", text) if item.strip()]
        return [text]
    return []


def _as_csv(value: object) -> str:
    if isinstance(value, str):
        return value.strip()
    if isinstance(value, list):
        items = [str(item).strip() for item in value if str(item).strip()]
        return ",".join(items)
    return ""


def _append_flag(args: list[str], payload: dict, key: str, *, flag: str | None = None) -> None:
    if _as_bool(payload.get(key, False)):
        args.append(flag or f"--{key.replace('_', '-')}")


def _append_value(args: list[str], payload: dict, key: str, *, flag: str) -> None:
    value = _optional_text(payload, key)
    if value:
        args.extend([flag, value])


def _append_profile(args: list[str], payload: dict) -> None:
    _append_value(args, payload, "profile", flag="--profile")


def _ensure_nlm_available() -> dict | None:
    if "/" in NLM_BIN:
        if os.path.isfile(NLM_BIN) and os.access(NLM_BIN, os.X_OK):
            return None
    elif shutil.which(NLM_BIN):
        return None
    return _base_result(
        False,
        error=f"`{NLM_BIN}` command not found",
        hints=[
            "Install notebooklm-mcp-cli and ensure the binary is executable.",
            "Set NOTEBOOKLM_CLI_BIN to override the binary path if needed.",
        ],
    )


def _run_nlm(args: list[str], *, timeout: int = DEFAULT_TIMEOUT) -> dict:
    command = NLM_BIN + " " + " ".join(shlex.quote(part) for part in args)
    try:
        completed = subprocess.run(
            [NLM_BIN, *args],
            capture_output=True,
            text=True,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return {
            "success": False,
            "command": command,
            "exit_code": 124,
            "stdout": "",
            "stderr": f"command timed out after {timeout}s",
            "error": "command timeout",
            "hints": ["Narrow the command scope or increase NOTEBOOKLM_CLI_TIMEOUT."],
        }

    success = completed.returncode == 0
    stderr = completed.stderr.strip()
    hints: list[str] = []
    lower_stderr = stderr.lower()
    if "auth" in lower_stderr or "login" in lower_stderr:
        hints.append("Run command=auth, op=login (or `nlm login`) and retry.")

    result = {
        "success": success,
        "command": command,
        "exit_code": completed.returncode,
        "stdout": (completed.stdout or "").strip(),
        "stderr": stderr,
        "hints": hints,
    }
    if not success:
        result["error"] = "nlm command failed"
    return result


def _resolve_command(payload: dict) -> tuple[str, dict]:
    working = dict(payload)
    command = str(working.pop("command", "")).strip().lower()
    action = str(working.get("action", "")).strip().lower()

    action_used_as_command = False
    if not command:
        if action in COMMAND_CHOICES:
            command = action
            action_used_as_command = True
        elif any(key in working for key in ("notebook_action", "source_action", "auth_action")):
            command = "notebook" if "notebook_action" in working else "source" if "source_action" in working else "auth"
        else:
            command = "help"

    if action_used_as_command:
        working.pop("action", None)
    return command, working


def _resolve_op(payload: dict, command: str) -> str:
    direct_action = str(payload.pop("action", "")).strip().lower()
    op = str(payload.pop("op", "")).strip().lower()
    alias = str(payload.pop(f"{command}_action", "")).strip().lower()
    action_name = str(payload.pop("action_name", "")).strip().lower()

    for candidate in (op, alias, action_name, direct_action):
        if candidate:
            return candidate
    return ""


def _overview_payload() -> dict:
    return {
        "entrypoint": "python3 skills/notebooklm-cli/run.py <command> <op> [--flag value ...]",
        "runtime": {
            "binary_env": "NOTEBOOKLM_CLI_BIN",
            "timeout_env": "NOTEBOOKLM_CLI_TIMEOUT",
            "binary_default": "nlm",
            "timeout_default_seconds": DEFAULT_TIMEOUT,
        },
        "commands": {
            command: {
                "default_op": COMMAND_SPECS.get(command, {}).get("default_op", ""),
                "description": COMMAND_SPECS.get(command, {}).get("description", ""),
            }
            for command in COMMAND_CHOICES
        },
        "compatibility": {
            "legacy_aliases": ["action", "notebook_action", "source_action", "auth_action"],
            "recommended": ["command", "op"],
        },
    }


def _dispatch_help(payload: dict) -> dict:
    topic = _optional_text(payload, "topic").lower() or "overview"
    include_cli = _as_bool(payload.get("include_cli", False))
    if topic not in HELP_TOPICS:
        return _base_result(False, error=f"unknown help topic: {topic}", hints=[f"valid topics: {list(HELP_TOPICS)}"])

    if topic == "progressive":
        ordered_topics = ["overview", "auth", "notebook", "source", "query", "report", "studio", "raw"]
        steps: list[dict] = []
        for step_topic in ordered_topics:
            step = {"topic": step_topic}
            if step_topic == "overview":
                step["overview"] = _overview_payload()
            else:
                step["schema"] = COMMAND_SPECS[step_topic]
            if include_cli and step_topic in TOPIC_TO_CLI_HELP_ARGS:
                step["cli_help"] = _run_nlm(TOPIC_TO_CLI_HELP_ARGS[step_topic])
            steps.append(step)
        return {
            "success": True,
            "command": "help progressive",
            "exit_code": 0,
            "stdout": "",
            "stderr": "",
            "hints": ["Use the command-specific schema from each step before issuing write operations."],
            "steps": steps,
        }

    result = _base_result(True, command=f"help {topic}")
    if topic == "overview":
        result["overview"] = _overview_payload()
    elif topic == "schema":
        result["schema"] = COMMAND_SPECS
    elif topic in COMMAND_SPECS:
        result["schema"] = COMMAND_SPECS[topic]

    if include_cli and topic in TOPIC_TO_CLI_HELP_ARGS:
        result["cli_help"] = _run_nlm(TOPIC_TO_CLI_HELP_ARGS[topic])
    return result


def _dispatch_auth(payload: dict) -> dict:
    op = _resolve_op(payload, "auth") or "login"
    if op == "help":
        return _run_nlm(["login", "--help"])
    if op == "login":
        args = ["login"]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "check":
        args = ["login", "--check"]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "switch":
        profile = _require_text(payload, "profile")
        return _run_nlm(["login", "switch", profile])
    if op == "profile_list":
        return _run_nlm(["login", "profile", "list"])
    if op == "profile_rename":
        old_name = _require_text(payload, "old_name")
        new_name = _require_text(payload, "new_name")
        return _run_nlm(["login", "profile", "rename", old_name, new_name])
    if op == "profile_delete":
        profile = _require_text(payload, "profile")
        if not _as_bool(payload.get("confirm", False)):
            return _base_result(False, error="profile_delete requires confirm=true")
        return _run_nlm(["login", "profile", "delete", profile, "--confirm"])
    return _base_result(False, error=f"unknown auth op: {op}")


def _build_query_args(payload: dict) -> list[str]:
    notebook_id = _require_text(payload, "notebook_id")
    question = _require_text(payload, "question")
    args = ["notebook", "query", notebook_id, question]
    _append_value(args, payload, "conversation_id", flag="--conversation-id")
    source_ids = _as_csv(payload.get("source_ids"))
    if source_ids:
        args.extend(["--source-ids", source_ids])
    _append_flag(args, payload, "json")
    _append_value(args, payload, "timeout", flag="--timeout")
    _append_profile(args, payload)
    return args


def _dispatch_notebook(payload: dict) -> dict:
    op = _resolve_op(payload, "notebook") or "list"
    if op == "help":
        return _run_nlm(["notebook", "--help"])
    if op == "list":
        args = ["notebook", "list"]
        _append_flag(args, payload, "full")
        _append_flag(args, payload, "json")
        _append_flag(args, payload, "quiet")
        _append_flag(args, payload, "title")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "create":
        title = _require_text(payload, "title")
        args = ["notebook", "create", title]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "get":
        notebook_id = _require_text(payload, "notebook_id")
        args = ["notebook", "get", notebook_id]
        _append_flag(args, payload, "json")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "describe":
        notebook_id = _require_text(payload, "notebook_id")
        args = ["notebook", "describe", notebook_id]
        _append_flag(args, payload, "json")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "rename":
        notebook_id = _require_text(payload, "notebook_id")
        title = _require_text(payload, "title")
        args = ["notebook", "rename", notebook_id, title]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "query":
        return _run_nlm(_build_query_args(payload))
    if op == "delete":
        notebook_id = _require_text(payload, "notebook_id")
        if not _as_bool(payload.get("confirm", False)):
            return _base_result(False, error="notebook delete requires confirm=true")
        args = ["notebook", "delete", notebook_id, "--confirm"]
        _append_profile(args, payload)
        return _run_nlm(args)
    return _base_result(False, error=f"unknown notebook op: {op}")


def _dispatch_source(payload: dict) -> dict:
    op = _resolve_op(payload, "source") or "list"
    if op == "help":
        return _run_nlm(["source", "--help"])
    if op == "list":
        notebook_id = _require_text(payload, "notebook_id")
        args = ["source", "list", notebook_id]
        _append_flag(args, payload, "full")
        _append_flag(args, payload, "drive")
        _append_flag(args, payload, "skip_freshness")
        _append_flag(args, payload, "json")
        _append_flag(args, payload, "quiet")
        _append_flag(args, payload, "url")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "add_url":
        notebook_id = _require_text(payload, "notebook_id")
        urls = _as_items(payload.get("urls"))
        if not urls:
            single_url = _optional_text(payload, "url")
            if single_url:
                urls = [single_url]
        if not urls:
            return _base_result(False, error="url or urls is required")
        args = ["source", "add", notebook_id]
        for url in urls:
            args.extend(["--url", url])
        _append_value(args, payload, "title", flag="--title")
        _append_flag(args, payload, "wait")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "add_text":
        notebook_id = _require_text(payload, "notebook_id")
        text = _require_text(payload, "text")
        title = _require_text(payload, "title")
        args = ["source", "add", notebook_id, "--text", text, "--title", title]
        _append_flag(args, payload, "wait")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "add_drive":
        notebook_id = _require_text(payload, "notebook_id")
        drive_id = _require_text(payload, "drive_id")
        args = ["source", "add", notebook_id, "--drive", drive_id]
        _append_value(args, payload, "doc_type", flag="--type")
        _append_value(args, payload, "title", flag="--title")
        _append_flag(args, payload, "wait")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "add_youtube":
        notebook_id = _require_text(payload, "notebook_id")
        youtube = _require_text(payload, "youtube")
        args = ["source", "add", notebook_id, "--youtube", youtube]
        _append_value(args, payload, "title", flag="--title")
        _append_flag(args, payload, "wait")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "add_file":
        notebook_id = _require_text(payload, "notebook_id")
        local_file = _require_text(payload, "file")
        args = ["source", "add", notebook_id, "--file", local_file]
        _append_value(args, payload, "title", flag="--title")
        _append_flag(args, payload, "wait")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "get":
        source_id = _require_text(payload, "source_id")
        args = ["source", "get", source_id]
        _append_flag(args, payload, "json")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "describe":
        source_id = _require_text(payload, "source_id")
        args = ["source", "describe", source_id]
        _append_flag(args, payload, "json")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "content":
        source_id = _require_text(payload, "source_id")
        args = ["source", "content", source_id]
        _append_flag(args, payload, "json")
        _append_value(args, payload, "output", flag="--output")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "rename":
        source_id = _require_text(payload, "source_id")
        title = _require_text(payload, "title")
        notebook_id = _require_text(payload, "notebook_id")
        args = ["source", "rename", source_id, title, "--notebook", notebook_id]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "delete":
        ids = []
        single_id = _optional_text(payload, "source_id")
        if single_id:
            ids.append(single_id)
        ids.extend(_as_items(payload.get("source_ids")))
        if not ids:
            return _base_result(False, error="source_id or source_ids is required")
        if not _as_bool(payload.get("confirm", False)):
            return _base_result(False, error="source delete requires confirm=true")
        args = ["source", "delete", *ids, "--confirm"]
        _append_profile(args, payload)
        return _run_nlm(args)
    return _base_result(False, error=f"unknown source op: {op}")


def _dispatch_query(payload: dict) -> dict:
    return _run_nlm(_build_query_args(payload))


def _dispatch_report(payload: dict) -> dict:
    op = _resolve_op(payload, "report") or "create"
    if op == "help":
        return _run_nlm(["report", "--help"])
    if op != "create":
        return _base_result(False, error=f"unknown report op: {op}")

    notebook_id = _require_text(payload, "notebook_id")
    if not _as_bool(payload.get("confirm", True)):
        return _base_result(False, error="report create requires confirm=true")
    args = ["report", "create", notebook_id, "--confirm"]
    _append_value(args, payload, "format", flag="--format")
    _append_value(args, payload, "prompt", flag="--prompt")
    _append_value(args, payload, "language", flag="--language")
    source_ids = _as_csv(payload.get("source_ids"))
    if source_ids:
        args.extend(["--source-ids", source_ids])
    _append_profile(args, payload)
    return _run_nlm(args)


def _dispatch_studio(payload: dict) -> dict:
    op = _resolve_op(payload, "studio") or "status"
    if op == "help":
        return _run_nlm(["studio", "--help"])
    if op == "status":
        notebook_id = _require_text(payload, "notebook_id")
        args = ["studio", "status", notebook_id]
        _append_flag(args, payload, "full")
        _append_flag(args, payload, "json")
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "rename":
        artifact_id = _require_text(payload, "artifact_id")
        title = _require_text(payload, "title")
        args = ["studio", "rename", artifact_id, title]
        _append_profile(args, payload)
        return _run_nlm(args)
    if op == "delete":
        notebook_id = _require_text(payload, "notebook_id")
        artifact_id = _require_text(payload, "artifact_id")
        if not _as_bool(payload.get("confirm", False)):
            return _base_result(False, error="studio delete requires confirm=true")
        args = ["studio", "delete", notebook_id, artifact_id, "--confirm"]
        _append_profile(args, payload)
        return _run_nlm(args)
    return _base_result(False, error=f"unknown studio op: {op}")


def _dispatch_raw(payload: dict) -> dict:
    raw_argv = payload.get("argv")
    if raw_argv is None:
        return _base_result(False, error="raw command requires argv")

    if isinstance(raw_argv, str):
        parts = shlex.split(raw_argv)
    elif isinstance(raw_argv, list):
        parts = [str(item).strip() for item in raw_argv if str(item).strip()]
    else:
        return _base_result(False, error="argv must be string or list")

    if not parts:
        return _base_result(False, error="argv cannot be empty")

    if parts[0] == "nlm":
        parts = parts[1:]
    if not parts:
        return _base_result(False, error="raw argv must include nlm subcommand")

    lowered = [part.lower() for part in parts]
    if lowered[:2] == ["chat", "start"]:
        return _base_result(False, error="`nlm chat start` is interactive and not supported")

    is_delete = "delete" in lowered
    has_confirm_flag = "--confirm" in lowered or "-y" in lowered
    if is_delete and not has_confirm_flag and not _as_bool(payload.get("confirm", False)):
        return _base_result(False, error="delete commands require confirm=true")
    if is_delete and not has_confirm_flag and _as_bool(payload.get("confirm", False)):
        parts.append("--confirm")

    return _run_nlm(parts)


def run(args: dict) -> dict:
    if not isinstance(args, dict):
        return _base_result(False, error="args must be an object")

    command, payload = _resolve_command(args)
    if command not in COMMAND_CHOICES:
        return _base_result(False, error=f"unknown command: {command}", hints=[f"valid commands: {list(COMMAND_CHOICES)}"])

    requires_nlm = command != "help" or _as_bool(payload.get("include_cli", False))
    if requires_nlm:
        availability_error = _ensure_nlm_available()
        if availability_error:
            return availability_error

    try:
        if command == "help":
            return _dispatch_help(payload)
        if command == "auth":
            return _dispatch_auth(payload)
        if command == "notebook":
            return _dispatch_notebook(payload)
        if command == "source":
            return _dispatch_source(payload)
        if command == "query":
            return _dispatch_query(payload)
        if command == "report":
            return _dispatch_report(payload)
        if command == "studio":
            return _dispatch_studio(payload)
        if command == "raw":
            return _dispatch_raw(payload)
    except ValueError as exc:
        return _base_result(False, error=str(exc))

    return _base_result(False, error=f"unhandled command: {command}")


def main() -> None:
    args = parse_cli_args(sys.argv[1:], primary_key="command", secondary_key="op")
    result = run(args)
    stdout_text, stderr_text, exit_code = render_result(result)
    if stdout_text:
        sys.stdout.write(stdout_text)
        if not stdout_text.endswith("\n"):
            sys.stdout.write("\n")
    if stderr_text:
        sys.stderr.write(stderr_text)
        if not stderr_text.endswith("\n"):
            sys.stderr.write("\n")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
