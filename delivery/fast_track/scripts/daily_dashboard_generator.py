#!/usr/bin/env python3
"""
daily_dashboard_generator.py
自动生成日报看板（触达率、回复率、转化率）
"""

from __future__ import annotations

import datetime as dt
import json
from pathlib import Path
from typing import Dict, Any


def calc_metrics(total_targets: int, reached: int, replied: int, converted: int) -> Dict[str, float]:
    def safe_rate(n: int, d: int) -> float:
        return round((n / d) * 100, 2) if d else 0.0

    return {
        "触达率": safe_rate(reached, total_targets),
        "回复率": safe_rate(replied, reached),
        "转化率": safe_rate(converted, replied),
    }


def build_dashboard_payload() -> Dict[str, Any]:
    # MVP 示例数据；生产环境改为从数据库/接口读取
    total_targets = 500
    reached = 380
    replied = 142
    converted = 36

    metrics = calc_metrics(total_targets, reached, replied, converted)

    return {
        "date": dt.date.today().isoformat(),
        "base": {
            "目标触达客户数": total_targets,
            "实际触达数": reached,
            "回复数": replied,
            "转化数": converted,
        },
        "metrics": metrics,
    }


def write_report(output_dir: Path) -> Path:
    payload = build_dashboard_payload()
    output_dir.mkdir(parents=True, exist_ok=True)
    out = output_dir / f"daily_dashboard_{payload['date']}.json"
    out.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    return out


if __name__ == "__main__":
    output = write_report(Path("/Users/bytedance/.alex/kernel/default/delivery/fast_track/docs"))
    print(f"Dashboard generated: {output}")

