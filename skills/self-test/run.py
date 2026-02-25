#!/usr/bin/env python3
"""self-test skill — 执行 Go 测试套件并结构化输出。

运行 go test -json，解析输出，分类失败。
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
import subprocess
import sys
import time


def execute(args: dict) -> dict:
    """运行测试套件并解析 JSON 输出。"""
    package = args.get("package", "./internal/channels/lark/testing/")
    timeout = args.get("timeout", "120s")
    cwd = args.get("cwd", ".")

    cmd = f"CGO_ENABLED=0 go test {package} -v -json -timeout {timeout}"
    try:
        result = subprocess.run(
            cmd, shell=True, capture_output=True, text=True,
            timeout=180, cwd=cwd,
        )
    except subprocess.TimeoutExpired:
        return {"success": False, "error": "test execution timeout (180s)"}
    except FileNotFoundError:
        return {"success": False, "error": "go not found in PATH"}

    # Parse JSON lines
    passed, failed, skipped = [], [], []
    for line in result.stdout.strip().split("\n"):
        if not line.strip():
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        if event.get("Action") == "pass" and event.get("Test"):
            passed.append(event["Test"])
        elif event.get("Action") == "fail" and event.get("Test"):
            failed.append({
                "test": event["Test"],
                "package": event.get("Package", ""),
                "elapsed": event.get("Elapsed", 0),
                "output": event.get("Output", ""),
            })
        elif event.get("Action") == "skip" and event.get("Test"):
            skipped.append(event["Test"])

    return {
        "success": True,
        "summary": {
            "passed": len(passed),
            "failed": len(failed),
            "skipped": len(skipped),
            "total": len(passed) + len(failed) + len(skipped),
        },
        "passed": passed,
        "failed": failed,
        "skipped": skipped,
        "exit_code": result.returncode,
        "timestamp": time.strftime("%Y-%m-%d %H:%M"),
        "analysis_prompt": (
            "请基于以上测试结果，分析每个失败场景：\n"
            "1. 读取场景 YAML 理解预期行为\n"
            "2. 分类根因: test_drift / prompt_issue / tool_bug / gateway_logic / context_issue / llm_quality / architecture\n"
            "3. 确定修复 Tier: 1(自主) / 2(轻量审批) / 3(Plan Review) / 4(仅报告)\n"
            "4. 对 Tier 1/2 提供修复 diff\n"
            "5. 对 Tier 3/4 提供诊断报告"
        ),
    }


def run(args: dict) -> dict:
    action = args.pop("action", "execute")
    if action == "execute":
        return execute(args)
    return {"success": False, "error": f"unknown action: {action}"}


def main() -> None:
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
