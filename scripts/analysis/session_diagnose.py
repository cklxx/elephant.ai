#!/usr/bin/env python3
"""Diagnose a persisted Alex session by session id.

Outputs:
1) Session-level summary and anomaly buckets
2) Per-message explanation with classification

Example:
  python3 scripts/analysis/session_diagnose.py \
    --session-id lark-3ACMs086dfRzmj9JonXlxEGv6mP
"""

from __future__ import annotations

import argparse
import dataclasses
import json
import re
import sys
from pathlib import Path
from typing import Any


TOOL_CALL_FAILURE_RE = re.compile(r"^Tool call_[^\s]+\s+failed:\s*", re.IGNORECASE)


@dataclasses.dataclass(frozen=True)
class SnapshotCandidate:
    path: Path
    messages: list[dict[str, Any]]


@dataclasses.dataclass
class Row:
    index: int
    role: str
    kind: str
    tool: str
    anomalies: list[str]
    reason: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Diagnose Alex session messages and anomalies")
    parser.add_argument("--session-id", required=True, help="session id, e.g. lark-xxxx")
    parser.add_argument(
        "--sessions-dir",
        default="~/.alex/sessions",
        help="sessions directory (default: ~/.alex/sessions)",
    )
    parser.add_argument(
        "--max-preview",
        type=int,
        default=120,
        help="max chars for content preview in message rows (default: 120)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="emit machine-readable JSON instead of text",
    )
    return parser.parse_args()


def read_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    with path.open("r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return rows


def first_line(text: str) -> str:
    for raw in text.splitlines():
        line = raw.strip()
        if line:
            return line
    return ""


def compact_text(text: str, limit: int) -> str:
    normalized = re.sub(r"\s+", " ", text.strip())
    if len(normalized) <= limit:
        return normalized
    return normalized[: max(0, limit - 1)] + "…"


def infer_tool_from_result(content: str) -> str:
    line = first_line(content)
    if line.startswith("Search (fallback):"):
        return "web_search"
    if line.startswith("Config override staged"):
        return "update_config"
    if "provide exactly one of 'path' or 'attachment_name'" in content:
        return "channel"
    if line.startswith("File sent successfully."):
        return "channel"
    if line.startswith("Command status:"):
        return "shell_exec_or_execute_code"
    return "unknown"


def load_snapshots(snapshots_dir: Path) -> list[SnapshotCandidate]:
    if not snapshots_dir.exists():
        return []
    out: list[SnapshotCandidate] = []
    for path in sorted(snapshots_dir.glob("turn_*.json")):
        try:
            obj = read_json(path)
        except Exception:
            continue
        messages = obj.get("messages")
        if isinstance(messages, list):
            out.append(SnapshotCandidate(path=path, messages=messages))
    return out


def recover_tool_from_snapshots(
    index: int,
    final_messages: list[dict[str, Any]],
    snapshots: list[SnapshotCandidate],
) -> tuple[str, str]:
    final_msg = final_messages[index]
    if final_msg.get("role") != "assistant":
        return "", ""
    final_content = (final_msg.get("content") or "").strip()
    next_tool_line = ""
    if index + 1 < len(final_messages) and final_messages[index + 1].get("role") == "tool":
        next_tool_line = first_line(str(final_messages[index + 1].get("content") or ""))

    best_tool = ""
    best_snapshot = ""
    for snap in reversed(snapshots):
        if index >= len(snap.messages):
            continue
        sm = snap.messages[index]
        if sm.get("role") != "assistant":
            continue
        if (sm.get("content") or "").strip() != final_content:
            continue
        calls = sm.get("tool_calls") or []
        if not calls:
            continue
        tool_name = str(calls[0].get("name") or "").strip()
        if not tool_name:
            continue

        if next_tool_line and index + 1 < len(snap.messages):
            next_snap = snap.messages[index + 1]
            if next_snap.get("role") == "tool":
                snap_line = first_line(str(next_snap.get("content") or ""))
                if snap_line == next_tool_line:
                    return tool_name, snap.path.name

        if not best_tool:
            best_tool = tool_name
            best_snapshot = snap.path.name
    return best_tool, best_snapshot


def collect_compression_events(events_path: Path) -> list[dict[str, Any]]:
    if not events_path.exists():
        return []
    rows = read_jsonl(events_path)
    out: list[dict[str, Any]] = []
    for row in rows:
        if row.get("event_type") != "workflow.diagnostic.context_compression":
            continue
        payload = row.get("payload") or {}
        out.append(
            {
                "timestamp": row.get("timestamp"),
                "run_id": row.get("run_id"),
                "compressed_count": payload.get("compressed_count"),
                "original_count": payload.get("original_count"),
                "compression_rate": payload.get("compression_rate"),
            }
        )
    return out


def diagnose(messages: list[dict[str, Any]], snapshots: list[SnapshotCandidate], max_preview: int) -> list[Row]:
    rows: list[Row] = []
    assistant_tool_by_index: dict[int, str] = {}

    for i, msg in enumerate(messages):
        role = str(msg.get("role") or "")
        content = str(msg.get("content") or "")
        calls = msg.get("tool_calls") or []
        anomalies: list[str] = []
        kind = "other"
        tool_name = ""
        reason = ""

        if role == "system":
            kind = "system_prompt_or_injected"
            reason = "System prompt or system-injected message"
        elif role == "user":
            kind = "user_input"
            reason = f"User input: {compact_text(content, max_preview)}"
        elif role == "assistant":
            if content.strip():
                kind = "assistant_text"
                reason = f"Assistant text reply: {compact_text(content, max_preview)}"
            else:
                if calls:
                    tool_name = str(calls[0].get("name") or "").strip()
                    assistant_tool_by_index[i] = tool_name
                    kind = "assistant_tool_placeholder"
                    reason = f"Tool-call placeholder for `{tool_name or 'unknown'}`"
                else:
                    recovered_tool, source_snapshot = recover_tool_from_snapshots(i, messages, snapshots)
                    if recovered_tool:
                        tool_name = recovered_tool
                        assistant_tool_by_index[i] = tool_name
                        kind = "assistant_tool_placeholder_compacted"
                        anomalies.append("metadata_stripped_after_compaction")
                        reason = (
                            f"Compacted placeholder; recovered tool `{tool_name}` "
                            f"from snapshot `{source_snapshot}`"
                        )
                    elif i + 1 < len(messages) and messages[i + 1].get("role") == "tool":
                        inferred = infer_tool_from_result(str(messages[i + 1].get("content") or ""))
                        tool_name = inferred
                        assistant_tool_by_index[i] = tool_name
                        kind = "assistant_tool_placeholder_compacted"
                        anomalies.append("metadata_stripped_after_compaction")
                        reason = (
                            f"Compacted placeholder inferred from next tool result "
                            f"(`{tool_name}`)"
                        )
                    else:
                        kind = "assistant_empty_unexpected"
                        anomalies.append("unexpected_empty_assistant")
                        reason = "Empty assistant message without tool-call evidence"
        elif role == "tool":
            prev_tool = assistant_tool_by_index.get(i - 1, "")
            if not prev_tool:
                prev_tool = infer_tool_from_result(content)
            tool_name = prev_tool

            if TOOL_CALL_FAILURE_RE.match(content.strip()):
                kind = "tool_result_error"
                anomalies.append("tool_failure")
            else:
                kind = "tool_result"
            if 'missing required argument "language"' in content:
                anomalies.append("tool_argument_missing_language")
            if "provide exactly one of 'path' or 'attachment_name'" in content:
                anomalies.append("tool_argument_conflict_path_vs_attachment_name")

            reason = f"Result for `{tool_name or 'unknown'}`: {compact_text(content, max_preview)}"
        else:
            kind = "unknown_role"
            anomalies.append("unknown_role")
            reason = f"Unknown role message: {compact_text(content, max_preview)}"

        rows.append(
            Row(
                index=i,
                role=role,
                kind=kind,
                tool=tool_name,
                anomalies=anomalies,
                reason=reason,
            )
        )

    return rows


def trailing_final_assistant_text_indices(messages: list[dict[str, Any]]) -> list[int]:
    indexes: list[int] = []
    i = len(messages) - 1
    while i >= 0 and messages[i].get("role") == "assistant" and str(messages[i].get("content") or "").strip():
        indexes.append(i)
        i -= 1
    indexes.reverse()
    return indexes


def summarize(rows: list[Row], compression_events: list[dict[str, Any]], final_assistant_tail: list[int]) -> dict[str, Any]:
    role_counts: dict[str, int] = {}
    kind_counts: dict[str, int] = {}
    anomaly_counts: dict[str, int] = {}
    for row in rows:
        role_counts[row.role] = role_counts.get(row.role, 0) + 1
        kind_counts[row.kind] = kind_counts.get(row.kind, 0) + 1
        for anomaly in row.anomalies:
            anomaly_counts[anomaly] = anomaly_counts.get(anomaly, 0) + 1

    if len(final_assistant_tail) > 1:
        anomaly_counts["multiple_terminal_assistant_replies"] = len(final_assistant_tail)

    return {
        "total_messages": len(rows),
        "role_counts": role_counts,
        "kind_counts": kind_counts,
        "anomaly_counts": anomaly_counts,
        "compression_events": compression_events,
        "terminal_assistant_text_indexes": final_assistant_tail,
    }


def render_text(
    session_id: str,
    summary: dict[str, Any],
    rows: list[Row],
) -> str:
    out: list[str] = []
    out.append(f"Session: {session_id}")
    out.append(f"Total messages: {summary['total_messages']}")
    out.append("")

    out.append("Role counts:")
    for key in sorted(summary["role_counts"]):
        out.append(f"  - {key}: {summary['role_counts'][key]}")
    out.append("")

    out.append("Kind counts:")
    for key in sorted(summary["kind_counts"]):
        out.append(f"  - {key}: {summary['kind_counts'][key]}")
    out.append("")

    out.append("Anomaly counts:")
    if summary["anomaly_counts"]:
        for key in sorted(summary["anomaly_counts"]):
            out.append(f"  - {key}: {summary['anomaly_counts'][key]}")
    else:
        out.append("  - (none)")
    out.append("")

    if summary["compression_events"]:
        out.append("Context compression events:")
        for ev in summary["compression_events"]:
            out.append(
                "  - ts={timestamp} run={run_id} compressed={compressed_count} "
                "original={original_count} rate={compression_rate}".format(**ev)
            )
        out.append("")

    tail = summary["terminal_assistant_text_indexes"]
    if tail:
        out.append("Terminal assistant text indexes: " + ", ".join(str(i) for i in tail))
        out.append("")

    out.append("Per-message diagnosis:")
    out.append("idx\trole\tkind\ttool\tanomalies\treason")
    for row in rows:
        anomalies = ",".join(row.anomalies) if row.anomalies else "-"
        out.append(
            f"{row.index:03d}\t{row.role}\t{row.kind}\t{row.tool or '-'}\t{anomalies}\t{row.reason}"
        )
    return "\n".join(out)


def main() -> int:
    args = parse_args()
    sessions_dir = Path(args.sessions_dir).expanduser()
    session_path = sessions_dir / f"{args.session_id}.json"
    if not session_path.exists():
        print(f"session file not found: {session_path}", file=sys.stderr)
        return 2

    session_obj = read_json(session_path)
    messages = session_obj.get("messages")
    if not isinstance(messages, list):
        print(f"invalid session JSON format (missing list messages): {session_path}", file=sys.stderr)
        return 2

    snapshots = load_snapshots(sessions_dir / "snapshots" / args.session_id)
    compression_events = collect_compression_events(
        sessions_dir / "_server" / "events" / f"{args.session_id}.jsonl"
    )

    rows = diagnose(messages=messages, snapshots=snapshots, max_preview=args.max_preview)
    tail = trailing_final_assistant_text_indices(messages)
    summary = summarize(rows=rows, compression_events=compression_events, final_assistant_tail=tail)

    if args.json:
        payload = {
            "session_id": args.session_id,
            "summary": summary,
            "rows": [dataclasses.asdict(row) for row in rows],
        }
        json.dump(payload, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
        return 0

    print(render_text(args.session_id, summary, rows))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
