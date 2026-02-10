#!/usr/bin/env python3
"""code-review skill — 收集代码变更，输出结构化审查输入。

LLM 调用此脚本收集 diff + 文件内容，然后基于返回的结构化数据做多维审查。
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
from pathlib import Path


def _sh(cmd: str, cwd: str | None = None) -> str:
    r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=30, cwd=cwd)
    return r.stdout.strip()


def collect(args: dict) -> dict:
    """收集代码变更用于审查。"""
    cwd = args.get("cwd", ".")
    base = args.get("base", "HEAD~1")
    paths = args.get("paths", [])

    # Git diff
    diff_cmd = f"git diff {base}"
    if paths:
        diff_cmd += " -- " + " ".join(paths)
    diff = _sh(diff_cmd, cwd=cwd)

    if not diff:
        # Try staged
        diff = _sh(f"git diff --cached {' -- ' + ' '.join(paths) if paths else ''}", cwd=cwd)

    if not diff:
        return {"success": False, "error": "no changes found to review"}

    # Changed files
    changed = _sh(f"git diff {base} --name-only", cwd=cwd).split("\n")
    changed = [f for f in changed if f.strip()]

    # Commit messages
    log = _sh(f"git log {base}..HEAD --oneline", cwd=cwd)

    # Stats
    stats = _sh(f"git diff {base} --stat", cwd=cwd)

    # Read full content of changed files (max 5)
    file_contents = {}
    for f in changed[:5]:
        fp = Path(cwd) / f
        if fp.exists() and fp.stat().st_size < 50000:
            file_contents[f] = fp.read_text(encoding="utf-8", errors="replace")

    return {
        "success": True,
        "diff": diff[:100000],  # cap at 100KB
        "changed_files": changed,
        "file_count": len(changed),
        "stats": stats,
        "commits": log,
        "file_contents": file_contents,
        "review_prompt": (
            "请基于以上 diff 和文件内容，按以下维度进行代码审查：\n"
            "1. **架构/SOLID** — 职责划分、依赖方向、接口抽象\n"
            "2. **安全性** — 注入、XSS、敏感数据、权限检查\n"
            "3. **正确性** — 边界条件、错误处理、并发安全\n"
            "4. **可维护性** — 命名、复杂度、测试覆盖\n"
            "5. **性能** — N+1、内存、缓存\n\n"
            "对每个发现标注严重度：P0(阻塞)/P1(重要)/P2(建议)/P3(微调)。"
        ),
    }


def run(args: dict) -> dict:
    action = args.pop("action", "collect")
    if action == "collect":
        return collect(args)
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
