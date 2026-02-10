#!/usr/bin/env python3
"""eval-systematic-optimization skill — 评测基线运行 + 失败簇归因。

运行 foundation eval suite，解析结果 JSON，聚类失败 case。
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
from pathlib import Path


def run_baseline(args: dict) -> dict:
    """运行 foundation suite 基线评测。"""
    suite = args.get("suite", "evaluation/agent_eval/datasets/foundation_eval_suite.yaml")
    tag = args.get("tag", time.strftime("%Y%m%d-%H%M"))
    output_dir = args.get("output", f"tmp/foundation-suite-{tag}-baseline")
    cwd = args.get("cwd", ".")

    cmd = f"go run ./cmd/alex eval foundation-suite --suite {suite} --output {output_dir} --format markdown"
    try:
        result = subprocess.run(
            cmd, shell=True, capture_output=True, text=True,
            timeout=600, cwd=cwd,
        )
    except subprocess.TimeoutExpired:
        return {"success": False, "error": "eval execution timeout (600s)"}
    except FileNotFoundError:
        return {"success": False, "error": "go not found in PATH"}

    return {
        "success": result.returncode == 0,
        "output_dir": output_dir,
        "exit_code": result.returncode,
        "stdout": result.stdout[:10000],
        "stderr": result.stderr[:5000] if result.returncode != 0 else "",
    }


def analyze_failures(args: dict) -> dict:
    """从结果 JSON 提取并聚类失败 case。"""
    result_file = args.get("result_file", "")
    if not result_file:
        return {"success": False, "error": "result_file is required"}

    path = Path(result_file)
    if not path.exists():
        return {"success": False, "error": f"{result_file} not found"}

    data = json.loads(path.read_text(encoding="utf-8"))

    # Extract failures: hit_rank > 1 and not N/A
    failures = []
    clusters: dict[str, list] = {}

    cases = data if isinstance(data, list) else data.get("cases", data.get("results", []))
    for case in cases:
        hit_rank = case.get("hit_rank", 1)
        if hit_rank <= 1:
            continue
        if case.get("status") == "N/A":
            continue

        expected = case.get("expected_tools", case.get("expected_tool", ""))
        top1 = case.get("top1_tool", case.get("actual_tool", ""))
        collection = case.get("collection", "")
        failure = {
            "case": case.get("name", case.get("intent", "")),
            "collection": collection,
            "expected": expected,
            "actual": top1,
            "hit_rank": hit_rank,
            "score": case.get("score", 0),
        }
        failures.append(failure)

        # Cluster by expected → actual
        key = f"{expected} → {top1}"
        clusters.setdefault(key, []).append(failure)

    # Sort clusters by frequency
    sorted_clusters = sorted(clusters.items(), key=lambda x: -len(x[1]))

    return {
        "success": True,
        "total_failures": len(failures),
        "clusters": [{"conflict": k, "count": len(v), "cases": v} for k, v in sorted_clusters[:20]],
        "optimization_prompt": (
            "请基于以上失败簇分析：\n"
            "1. 按频次排序 Top conflict families\n"
            "2. 优先改规则层（token alias、冲突惩罚、意图增益）\n"
            "3. 不要单点刷题，要系统性收敛\n"
            "4. sandbox 语义收敛到执行工具\n"
            "5. 输出标准化 x/x 报告"
        ),
    }


def run(args: dict) -> dict:
    action = args.pop("action", "analyze")
    handlers = {"baseline": run_baseline, "analyze": analyze_failures}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


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
