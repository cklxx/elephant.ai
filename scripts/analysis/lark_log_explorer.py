#!/usr/bin/env python3
"""Progressive Lark log explorer.

Usage examples:
  python3 scripts/analysis/lark_log_explorer.py
  python3 scripts/analysis/lark_log_explorer.py --message-id om_xxx
  python3 scripts/analysis/lark_log_explorer.py --session-id lark-xxx --show-evidence
"""

from __future__ import annotations

import argparse
import collections
import dataclasses
import datetime as dt
import pathlib
import re
import sys
from typing import Iterable, Optional

TS_RE = re.compile(r"^(?P<ts>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\s")
LOG_ID_RE = re.compile(r"\[log_id=(?P<log_id>[^\]]+)\]")

MESSAGE_RE = re.compile(
    r"Lark message received: chat_id=(?P<chat_id>\S+)\s+msg_id=(?P<msg_id>\S+)\s+"
    r"sender=(?P<sender>\S+)\s+group=(?P<group>\w+)\s+len=(?P<length>\d+)"
)
ROUTING_RE = re.compile(
    r"Lark session routing: chat=(?P<chat_id>\S+)\s+source=(?P<source>\S+)\s+session=(?P<session_id>\S+)"
)
EXEC_START_RE = re.compile(
    r"ExecuteTask started: session=(?P<session_short>\S+)\s+task_chars=(?P<task_chars>\d+)"
)
SESSION_SUMMARY_RE = re.compile(
    r"Session summary: requests=(?P<requests>\d+),\s+total_tokens=(?P<total_tokens>\d+)\s+"
    r"\(input=(?P<input_tokens>\d+),\s+output=(?P<output_tokens>\d+)\),\s+"
    r"cost=\$(?P<cost>[\d.]+),\s+duration=(?P<duration>\S+)"
)
TOOL_FAIL_RE = re.compile(r"Tool\s+\d+\s+failed:\s+(?P<error>.+)$")
STOP_CANCEL_RE = re.compile(
    r"Lark task cancelled by stop command: chat=(?P<chat_id>\S+)\s+msg=(?P<msg_id>\S+)"
)

DURATION_PART_RE = re.compile(r"(?P<value>\d+(?:\.\d+)?)(?P<unit>ms|h|m|s)")

HANDLER_TIMEOUT_MARKER = "Failed to encode JSON response: http: Handler timeout"
DISPATCH_FAIL_MARKER = "Lark dispatch message failed:"
REACTION_FAIL_MARKER = "Lark add reaction failed:"
NO_CLIENTS_MARKER = "No clients found for session "
CONTEXT_CANCELED_MARKER = "context canceled"


@dataclasses.dataclass(frozen=True)
class LogEntry:
    ts: dt.datetime
    raw: str
    log_id: str
    index: int


@dataclasses.dataclass(frozen=True)
class MessageEvent:
    ts: dt.datetime
    chat_id: str
    msg_id: str
    sender: str
    is_group: bool
    length: int
    line: LogEntry


@dataclasses.dataclass(frozen=True)
class RoutingEvent:
    ts: dt.datetime
    chat_id: str
    source: str
    session_id: str
    line: LogEntry


@dataclasses.dataclass(frozen=True)
class ExecStartEvent:
    ts: dt.datetime
    log_id: str
    session_short: str
    task_chars: int
    line: LogEntry


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Progressive explorer for Lark runtime logs")
    parser.add_argument("--logs-dir", default="logs", help="directory containing log files (default: logs)")
    parser.add_argument(
        "--service-log",
        default="alex-service.log",
        help="service log file name under logs-dir (default: alex-service.log)",
    )
    parser.add_argument("--latest", type=int, default=1, help="analyze latest N messages (default: 1)")
    parser.add_argument("--message-id", default="", help="specific Lark message ID to analyze")
    parser.add_argument("--chat-id", default="", help="specific chat ID to analyze (latest message in chat)")
    parser.add_argument("--session-id", default="", help="specific session ID to analyze")
    parser.add_argument(
        "--window-seconds",
        type=int,
        default=120,
        help="max seconds for message->routing/start matching (default: 120)",
    )
    parser.add_argument(
        "--lifecycle-max-minutes",
        type=int,
        default=30,
        help="max minutes to inspect lifecycle when no terminal summary found (default: 30)",
    )
    parser.add_argument(
        "--selection-mode",
        choices=("latest", "routed", "executed"),
        default="executed",
        help=(
            "default target selection strategy when no explicit id is provided: "
            "latest message, latest routed message, or latest executed message "
            "(default: executed)"
        ),
    )
    parser.add_argument(
        "--scan-back",
        type=int,
        default=200,
        help="how many latest messages to scan when selecting by mode (default: 200)",
    )
    parser.add_argument(
        "--show-candidates",
        action="store_true",
        help="print candidate selection table for recent messages",
    )
    parser.add_argument(
        "--include-injected",
        action="store_true",
        help="include synthetic/injected test messages (default: false)",
    )
    parser.add_argument("--show-evidence", action="store_true", help="print key raw evidence lines")
    return parser.parse_args()


def parse_timestamp(line: str) -> Optional[dt.datetime]:
    match = TS_RE.match(line)
    if not match:
        return None
    try:
        return dt.datetime.strptime(match.group("ts"), "%Y-%m-%d %H:%M:%S")
    except ValueError:
        return None


def parse_log_id(line: str) -> str:
    match = LOG_ID_RE.search(line)
    if not match:
        return ""
    return match.group("log_id").strip()


def load_entries(path: pathlib.Path) -> list[LogEntry]:
    entries: list[LogEntry] = []
    with path.open("r", encoding="utf-8", errors="replace") as fh:
        for index, raw in enumerate(fh):
            line = raw.rstrip("\n")
            ts = parse_timestamp(line)
            if ts is None:
                continue
            entries.append(LogEntry(ts=ts, raw=line, log_id=parse_log_id(line), index=index))
    return entries


def parse_messages(entries: Iterable[LogEntry]) -> list[MessageEvent]:
    events: list[MessageEvent] = []
    for entry in entries:
        match = MESSAGE_RE.search(entry.raw)
        if not match:
            continue
        events.append(
            MessageEvent(
                ts=entry.ts,
                chat_id=match.group("chat_id"),
                msg_id=match.group("msg_id"),
                sender=match.group("sender"),
                is_group=match.group("group").lower() == "true",
                length=int(match.group("length")),
                line=entry,
            )
        )
    return events


def parse_routings(entries: Iterable[LogEntry]) -> list[RoutingEvent]:
    routings: list[RoutingEvent] = []
    for entry in entries:
        match = ROUTING_RE.search(entry.raw)
        if not match:
            continue
        routings.append(
            RoutingEvent(
                ts=entry.ts,
                chat_id=match.group("chat_id"),
                source=match.group("source"),
                session_id=match.group("session_id"),
                line=entry,
            )
        )
    return routings


def parse_exec_starts(entries: Iterable[LogEntry]) -> list[ExecStartEvent]:
    starts: list[ExecStartEvent] = []
    for entry in entries:
        if not entry.log_id:
            continue
        match = EXEC_START_RE.search(entry.raw)
        if not match:
            continue
        starts.append(
            ExecStartEvent(
                ts=entry.ts,
                log_id=entry.log_id,
                session_short=match.group("session_short"),
                task_chars=int(match.group("task_chars")),
                line=entry,
            )
        )
    return starts


def session_short_matches(full_session_id: str, short_session: str) -> bool:
    full_session_id = full_session_id.strip()
    short_session = short_session.strip()
    if not full_session_id or not short_session:
        return False
    if short_session == full_session_id:
        return True
    if "..." not in short_session:
        return False
    prefix, suffix = short_session.split("...", 1)
    return full_session_id.startswith(prefix) and full_session_id.endswith(suffix)


def parse_go_duration_seconds(text: str) -> Optional[float]:
    total = 0.0
    found = False
    for part in DURATION_PART_RE.finditer(text):
        found = True
        value = float(part.group("value"))
        unit = part.group("unit")
        if unit == "h":
            total += value * 3600
        elif unit == "m":
            total += value * 60
        elif unit == "s":
            total += value
        elif unit == "ms":
            total += value / 1000.0
    return total if found else None


def human_seconds(seconds: float) -> str:
    if seconds < 60:
        return f"{seconds:.1f}s"
    minutes = int(seconds // 60)
    remain = int(seconds % 60)
    if minutes < 60:
        return f"{minutes}m{remain:02d}s"
    hours = minutes // 60
    mins = minutes % 60
    return f"{hours}h{mins:02d}m{remain:02d}s"


def pick_messages(
    messages: list[MessageEvent],
    routings: list[RoutingEvent],
    starts: list[ExecStartEvent],
    args: argparse.Namespace,
) -> tuple[list[MessageEvent], str]:
    if not messages:
        return [], "no_messages"

    if args.message_id:
        chosen = [m for m in messages if m.msg_id == args.message_id.strip()]
        return (chosen[-1:] if chosen else []), "message_id"

    if args.chat_id:
        chosen = [m for m in messages if m.chat_id == args.chat_id.strip()]
        return (chosen[-1:] if chosen else []), "chat_id"

    if args.session_id:
        sid = args.session_id.strip()
        related = [r for r in routings if r.session_id == sid]
        if not related:
            return [], "session_id_not_found"
        last_routing = related[-1]
        # nearest preceding message in same chat
        candidates = [m for m in messages if m.chat_id == last_routing.chat_id and m.ts <= last_routing.ts]
        return (candidates[-1:] if candidates else []), "session_id"

    n = max(1, args.latest)
    recent = messages[-max(n, args.scan_back) :]
    if not args.include_injected:
        filtered = [m for m in recent if not is_injected_message(m)]
        if filtered:
            recent = filtered
    chosen: list[MessageEvent] = []

    for message in reversed(recent):
        routing = find_first_routing(message, routings, args.window_seconds)
        has_routing = routing is not None
        has_exec = False
        if has_routing:
            has_exec = find_exec_start(message, routing, starts, args.window_seconds) is not None

        if args.selection_mode == "latest":
            chosen.append(message)
        elif args.selection_mode == "routed" and has_routing:
            chosen.append(message)
        elif args.selection_mode == "executed" and has_exec:
            chosen.append(message)

        if len(chosen) >= n:
            return chosen[:n], f"mode:{args.selection_mode}"

    if args.selection_mode == "latest":
        return messages[-n:], "mode:latest_fallback"
    return messages[-n:], f"mode:{args.selection_mode}_fallback_to_latest"


def show_candidates_table(
    messages: list[MessageEvent],
    routings: list[RoutingEvent],
    starts: list[ExecStartEvent],
    args: argparse.Namespace,
) -> None:
    print("[Selection] Recent message candidates")
    print("  ts                  msg_id                         routed  executed  len")
    print("  ------------------- ------------------------------ ------- --------- ----")
    recent = messages[-max(1, args.scan_back) :]
    if not args.include_injected:
        filtered = [m for m in recent if not is_injected_message(m)]
        if filtered:
            recent = filtered
    for message in reversed(recent[-12:]):
        routing = find_first_routing(message, routings, args.window_seconds)
        routed = "yes" if routing is not None else "no"
        executed = "no"
        if routing is not None:
            executed = "yes" if find_exec_start(message, routing, starts, args.window_seconds) else "no"
        print(
            "  "
            f"{message.ts.strftime('%Y-%m-%d %H:%M:%S')} "
            f"{message.msg_id:<30} {routed:<7} {executed:<9} {message.length:>4}"
        )


def find_first_routing(
    message: MessageEvent, routings: list[RoutingEvent], window_seconds: int
) -> Optional[RoutingEvent]:
    deadline = message.ts + dt.timedelta(seconds=window_seconds)
    for routing in routings:
        if routing.chat_id != message.chat_id:
            continue
        if routing.ts < message.ts:
            continue
        if routing.ts > deadline:
            break
        return routing
    return None


def find_exec_start(
    message: MessageEvent,
    routing: Optional[RoutingEvent],
    starts: list[ExecStartEvent],
    window_seconds: int,
) -> Optional[ExecStartEvent]:
    deadline = message.ts + dt.timedelta(seconds=window_seconds)
    for start in starts:
        if start.ts < message.ts:
            continue
        if start.ts > deadline:
            break
        if routing and not session_short_matches(routing.session_id, start.session_short):
            continue
        return start
    return None


def collect_lifecycle(
    entries: list[LogEntry], log_id: str, start_ts: dt.datetime, max_minutes: int
) -> tuple[list[LogEntry], Optional[LogEntry], Optional[dict[str, str]]]:
    cutoff = start_ts + dt.timedelta(minutes=max_minutes)
    lifecycle: list[LogEntry] = []
    summary_line: Optional[LogEntry] = None
    summary_data: Optional[dict[str, str]] = None

    for entry in entries:
        if entry.ts < start_ts:
            continue
        if max_minutes > 0 and entry.ts > cutoff:
            break
        if entry.log_id != log_id:
            continue
        lifecycle.append(entry)
        sm = SESSION_SUMMARY_RE.search(entry.raw)
        if sm:
            summary_line = entry
            summary_data = sm.groupdict()
            break
    return lifecycle, summary_line, summary_data


def select_window_entries(
    entries: list[LogEntry],
    start_ts: dt.datetime,
    end_ts: dt.datetime,
) -> list[LogEntry]:
    return [entry for entry in entries if start_ts <= entry.ts <= end_ts]


def format_evidence(entry: LogEntry) -> str:
    return f"{entry.ts.strftime('%Y-%m-%d %H:%M:%S')} | {entry.raw.split(' - ', 1)[-1]}"


def analyze_one(
    message: MessageEvent,
    entries: list[LogEntry],
    routings: list[RoutingEvent],
    starts: list[ExecStartEvent],
    args: argparse.Namespace,
) -> int:
    print("=" * 88)
    print(f"Message: {message.msg_id}  chat: {message.chat_id}  at: {message.ts.strftime('%Y-%m-%d %H:%M:%S')}")
    print("=" * 88)

    print("[Stage 1] Inbound Message")
    print(f"- sender={message.sender} group={message.is_group} len={message.length}")
    if args.show_evidence:
        print(f"- evidence: {format_evidence(message.line)}")

    routing = find_first_routing(message, routings, args.window_seconds)
    print("[Stage 2] Session Routing")
    if routing is None:
        print("- routing: not found in window")
    else:
        print(f"- source={routing.source} session={routing.session_id} at {routing.ts.strftime('%H:%M:%S')}")
        if args.show_evidence:
            print(f"- evidence: {format_evidence(routing.line)}")

    start = find_exec_start(message, routing, starts, args.window_seconds)
    print("[Stage 3] Agent Execution")
    lifecycle: list[LogEntry] = []
    summary_line: Optional[LogEntry] = None
    summary_data: Optional[dict[str, str]] = None
    if start is None:
        print("- execute_task: not found in window")
    else:
        print(f"- log_id={start.log_id} session_short={start.session_short} task_chars={start.task_chars}")
        if args.show_evidence:
            print(f"- evidence: {format_evidence(start.line)}")
        lifecycle, summary_line, summary_data = collect_lifecycle(
            entries, start.log_id, start.ts, args.lifecycle_max_minutes
        )
        tool_failures = collections.Counter()
        context_canceled = 0
        for line in lifecycle:
            match = TOOL_FAIL_RE.search(line.raw)
            if match:
                tool_failures[match.group("error").strip()] += 1
            if CONTEXT_CANCELED_MARKER in line.raw:
                context_canceled += 1
        print(f"- lifecycle_lines={len(lifecycle)} tool_failures={sum(tool_failures.values())} context_canceled={context_canceled}")
        if tool_failures:
            top_err, top_count = tool_failures.most_common(1)[0]
            print(f"- top_tool_failure={top_err} (x{top_count})")
        if summary_data:
            seconds = parse_go_duration_seconds(summary_data["duration"])
            duration_txt = summary_data["duration"]
            if seconds is not None:
                duration_txt = f"{duration_txt} (~{human_seconds(seconds)})"
            print(
                "- summary: "
                f"requests={summary_data['requests']} total_tokens={summary_data['total_tokens']} "
                f"cost=${summary_data['cost']} duration={duration_txt}"
            )
            if args.show_evidence and summary_line is not None:
                print(f"- evidence: {format_evidence(summary_line)}")
        elif lifecycle:
            elapsed = lifecycle[-1].ts - start.ts
            print(f"- summary: missing in inspection window, observed runtime={human_seconds(elapsed.total_seconds())}")

    # Delivery window: from message time to execution end (or configured fallback).
    if start is not None and lifecycle:
        end_ts = lifecycle[-1].ts
    else:
        end_ts = message.ts + dt.timedelta(minutes=max(5, args.lifecycle_max_minutes // 2))
    window_entries = select_window_entries(entries, message.ts, end_ts)

    dispatch_fail_for_msg = [
        line
        for line in window_entries
        if DISPATCH_FAIL_MARKER in line.raw and message.msg_id in line.raw
    ]
    reaction_fail_for_msg = [
        line
        for line in window_entries
        if REACTION_FAIL_MARKER in line.raw and message.msg_id in line.raw
    ]
    stop_cancel_for_msg = []
    for line in window_entries:
        stop_match = STOP_CANCEL_RE.search(line.raw)
        if stop_match and stop_match.group("msg_id") == message.msg_id:
            stop_cancel_for_msg.append(line)
    handler_timeouts = [line for line in window_entries if HANDLER_TIMEOUT_MARKER in line.raw]

    no_client_count = 0
    if routing is not None:
        session_marker = NO_CLIENTS_MARKER + routing.session_id
        no_client_count = sum(1 for line in window_entries if session_marker in line.raw)

    print("[Stage 4] Delivery & Surface Signals")
    print(
        f"- dispatch_fail_for_msg={len(dispatch_fail_for_msg)} "
        f"reaction_fail_for_msg={len(reaction_fail_for_msg)} "
        f"stop_cancel_for_msg={len(stop_cancel_for_msg)} "
        f"http_handler_timeout={len(handler_timeouts)} "
        f"no_sse_client_events={no_client_count}"
    )
    if args.show_evidence:
        if dispatch_fail_for_msg:
            print(f"- evidence(dispatch): {format_evidence(dispatch_fail_for_msg[0])}")
        if reaction_fail_for_msg:
            print(f"- evidence(reaction): {format_evidence(reaction_fail_for_msg[0])}")
        if stop_cancel_for_msg:
            print(f"- evidence(stop-cancel): {format_evidence(stop_cancel_for_msg[0])}")
        if handler_timeouts:
            print(f"- evidence(timeout): {format_evidence(handler_timeouts[0])}")

    reasons: list[str] = []
    if routing is not None and routing.source in {"persisted_binding", "last_session"}:
        reasons.append(f"会话路由走 `{routing.source}`，复用旧 session，可能继承了较重上下文/历史状态。")

    if start is not None and lifecycle:
        elapsed_s = (lifecycle[-1].ts - start.ts).total_seconds()
        if summary_data:
            summary_seconds = parse_go_duration_seconds(summary_data["duration"])
            if summary_seconds is not None and summary_seconds > 180:
                reasons.append(f"该次执行持续 {summary_data['duration']}，属于长运行任务。")
        elif elapsed_s > 180:
            reasons.append(f"在可观测窗口内执行已持续 {human_seconds(elapsed_s)} 仍未看到完成 summary。")

    tool_failure_counter = collections.Counter()
    for line in lifecycle:
        match = TOOL_FAIL_RE.search(line.raw)
        if match:
            tool_failure_counter[match.group("error").strip()] += 1
    if tool_failure_counter:
        top_error, top_count = tool_failure_counter.most_common(1)[0]
        reasons.append(f"执行期工具错误高频：`{top_error}`（{top_count} 次），循环重试拖长总耗时。")

    if any("invalid {open_message_id}" in line.raw for line in dispatch_fail_for_msg + reaction_fail_for_msg):
        reasons.append("Lark 回复链路含无效 open_message_id 错误，导致结果无法正常回帖。")

    if handler_timeouts:
        reasons.append("同时间段存在 HTTP handler timeout，说明部分非流式接口在 30s 超时截断。")

    if stop_cancel_for_msg:
        reasons.append("任务被 `/stop` 取消，导致本次执行未自然结束并出现后续取消链路报错。")

    if any(CONTEXT_CANCELED_MARKER in line.raw for line in lifecycle):
        reasons.append("执行后期出现 context canceled，任务被外部取消/超时中断而非正常收敛。")

    print("[Stage 5] Diagnosis")
    if not reasons:
        print("- no obvious blocker detected in selected window")
    else:
        for reason in reasons:
            print(f"- {reason}")

    return 0


def is_injected_message(message: MessageEvent) -> bool:
    return (
        message.sender == "ou_inject_user"
        or message.msg_id.startswith("inject_")
        or message.chat_id.startswith("e2e-")
    )


def main() -> int:
    args = parse_args()
    logs_dir = pathlib.Path(args.logs_dir)
    service_log = logs_dir / args.service_log

    if not service_log.exists():
        print(f"service log not found: {service_log}", file=sys.stderr)
        return 2

    entries = load_entries(service_log)
    if not entries:
        print(f"no parseable timestamped lines in: {service_log}", file=sys.stderr)
        return 2

    messages = parse_messages(entries)
    routings = parse_routings(entries)
    starts = parse_exec_starts(entries)
    if args.show_candidates:
        show_candidates_table(messages, routings, starts, args)
    targets, selector = pick_messages(messages, routings, starts, args)
    if not targets:
        print("no target message found for the given filters", file=sys.stderr)
        return 1
    print(f"[Selection] selector={selector} target_count={len(targets)}")

    for target in targets:
        analyze_one(target, entries, routings, starts, args)
    return 0


if __name__ == "__main__":
    sys.exit(main())
