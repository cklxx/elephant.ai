#!/usr/bin/env python3
"""anygen-task-creator skill — AI content generation via AnyGen OpenAPI.

Supports slides (PPT), documents, storybooks, data analysis, websites, and chat.
Requires ANYGEN_API_KEY environment variable.
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import os

# Import anygen module from local scripts directory
_SKILL_SCRIPTS = Path(__file__).resolve().parent / "scripts"
if str(_SKILL_SCRIPTS) not in sys.path:
    sys.path.insert(0, str(_SKILL_SCRIPTS))

import anygen


VALID_OPERATIONS = {"chat", "slide", "doc", "storybook", "data_analysis", "website"}


def _get_api_key() -> str | None:
    """Resolve API key from env (loaded via dotenv)."""
    return os.environ.get("ANYGEN_API_KEY")


def create(args: dict) -> dict:
    """Create an AnyGen generation task."""
    api_key = _get_api_key()
    if not api_key:
        return {"success": False, "error": "ANYGEN_API_KEY not set"}

    operation = args.get("operation", "")
    prompt = args.get("prompt", "")
    if not operation:
        return {"success": False, "error": "operation is required"}
    if operation not in VALID_OPERATIONS:
        return {"success": False, "error": f"invalid operation: {operation}. Must be one of: {', '.join(sorted(VALID_OPERATIONS))}"}
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    task_id = anygen.create_task(
        api_key=api_key,
        operation=operation,
        prompt=prompt,
        language=args.get("language"),
        slide_count=args.get("slide_count"),
        template=args.get("template"),
        ratio=args.get("ratio"),
        doc_format=args.get("doc_format"),
        files=args.get("files"),
        style=args.get("style"),
    )

    if not task_id:
        return {"success": False, "error": "failed to create task"}

    return {"success": True, "task_id": task_id, "message": f"Task created: {task_id}"}


def poll(args: dict) -> dict:
    """Poll an AnyGen task until completion."""
    api_key = _get_api_key()
    if not api_key:
        return {"success": False, "error": "ANYGEN_API_KEY not set"}

    task_id = args.get("task_id", "")
    if not task_id:
        return {"success": False, "error": "task_id is required"}

    task = anygen.poll_task(api_key, task_id)
    if not task:
        return {"success": False, "error": "poll timed out or failed"}

    status = task.get("status")
    if status == "completed":
        output = task.get("output", {})
        return {
            "success": True,
            "status": "completed",
            "file_name": output.get("file_name"),
            "file_url": output.get("file_url"),
            "message": f"Task completed. File: {output.get('file_name', 'N/A')}",
        }

    return {
        "success": False,
        "status": status,
        "error": task.get("error", "task did not complete successfully"),
    }


def download(args: dict) -> dict:
    """Download a completed AnyGen task's output file."""
    api_key = _get_api_key()
    if not api_key:
        return {"success": False, "error": "ANYGEN_API_KEY not set"}

    task_id = args.get("task_id", "")
    output_dir = args.get("output", "./output/")
    if not task_id:
        return {"success": False, "error": "task_id is required"}

    ok = anygen.download_file(api_key, task_id, output_dir)
    if ok:
        return {"success": True, "message": f"File downloaded to {output_dir}"}
    return {"success": False, "error": "download failed"}


def run_workflow(args: dict) -> dict:
    """Full workflow: create -> poll -> download."""
    api_key = _get_api_key()
    if not api_key:
        return {"success": False, "error": "ANYGEN_API_KEY not set"}

    operation = args.get("operation", "")
    prompt = args.get("prompt", "")
    if not operation:
        return {"success": False, "error": "operation is required"}
    if operation not in VALID_OPERATIONS:
        return {"success": False, "error": f"invalid operation: {operation}. Must be one of: {', '.join(sorted(VALID_OPERATIONS))}"}
    if not prompt:
        return {"success": False, "error": "prompt is required"}

    ok = anygen.run_full_workflow(
        api_key=api_key,
        operation=operation,
        prompt=prompt,
        output_dir=args.get("output"),
        language=args.get("language"),
        slide_count=args.get("slide_count"),
        template=args.get("template"),
        ratio=args.get("ratio"),
        doc_format=args.get("doc_format"),
        files=args.get("files"),
        style=args.get("style"),
    )

    if ok:
        return {"success": True, "message": f"Workflow completed for {operation} task"}
    return {"success": False, "error": "workflow failed"}


def run(args: dict) -> dict:
    """Main dispatcher."""
    action = args.pop("action", "run")
    handlers = {
        "create": create,
        "poll": poll,
        "download": download,
        "run": run_workflow,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}. Must be one of: {', '.join(sorted(handlers))}"}
    return handler(args)


def main() -> None:
    """Parse input, execute, output JSON."""
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}

    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
