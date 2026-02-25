#!/usr/bin/env python3
"""Generate repository-wide file catalog and code capability mapping exports."""

from __future__ import annotations

import csv
import json
import os
import subprocess
import codecs
from datetime import date
from collections import defaultdict
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable, List, Tuple

CODE_EXTENSIONS = {
    ".go",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".mjs",
    ".cjs",
    ".py",
    ".sh",
    ".sql",
    ".rs",
}

CONFIG_EXTENSIONS = {
    ".yaml",
    ".yml",
    ".json",
    ".toml",
    ".ini",
    ".conf",
    ".env",
    ".xml",
}

DOC_EXTENSIONS = {
    ".md",
    ".rst",
    ".txt",
}

ASSET_EXTENSIONS = {
    ".png",
    ".jpg",
    ".jpeg",
    ".gif",
    ".svg",
    ".ico",
    ".webp",
    ".pdf",
    ".woff",
    ".woff2",
    ".ttf",
    ".eot",
    ".mp4",
    ".mov",
}

ROOT_CODE_FILENAMES = {
    "Makefile",
    "Dockerfile",
}

PREFIX_CAPABILITY: List[Tuple[str, str]] = [
    ("internal/domain/agent/react", "ReAct 推理循环、工具编排与后台任务运行时"),
    ("internal/domain/agent/ports", "Agent 域端口定义与依赖抽象"),
    ("internal/domain/agent/presets", "Agent 预设与策略模板"),
    ("internal/domain/agent", "Agent 领域模型、事件与核心行为"),
    ("internal/domain/workflow", "工作流 DAG 结构与执行规则"),
    ("internal/domain/task", "任务领域实体与状态语义"),
    ("internal/domain/materials", "附件/材料领域模型与迁移"),
    ("internal/domain/kernel", "内核级领域抽象"),
    ("internal/app/agent/coordinator", "应用层会话编排与事件翻译"),
    ("internal/app/agent/kernel", "应用层 Agent 内核装配"),
    ("internal/app/agent/llmclient", "LLM 客户端适配与调用控制"),
    ("internal/app/agent/preparation", "上下文准备与请求预处理"),
    ("internal/app/agent/sessiontitle", "会话标题生成"),
    ("internal/app/agent/context", "Agent 上下文装配与透传"),
    ("internal/app/agent", "应用层 Agent 用例与协调"),
    ("internal/app/context", "上下文预算与策略管理"),
    ("internal/app/reminder", "提醒业务应用服务"),
    ("internal/app/scheduler", "计划调度应用逻辑"),
    ("internal/app/subscription", "订阅业务应用逻辑"),
    ("internal/app/notification", "通知分发应用逻辑"),
    ("internal/app/toolregistry", "工具注册与生命周期管理"),
    ("internal/app/toolcontext", "工具调用上下文编排"),
    ("internal/app/di", "依赖注入与装配入口"),
    ("internal/app/lifecycle", "应用生命周期管理"),
    ("internal/app", "应用层服务与用例编排"),
    ("internal/delivery/server/http", "HTTP API 路由、中间件与会话接口"),
    ("internal/delivery/server/app", "Server 应用服务（任务、会话、快照、事件广播）"),
    ("internal/delivery/server/bootstrap", "Server 启动与依赖装配"),
    ("internal/delivery/server", "服务端交付层入口"),
    ("internal/delivery/channels/lark", "Lark 渠道网关、适配与会话桥接"),
    ("internal/delivery/channels", "多渠道交付适配"),
    ("internal/delivery/eval", "评测交付层接口与引导"),
    ("internal/delivery/output", "输出渲染与格式化交付"),
    ("internal/delivery/presentation", "展示层格式器与视图模型"),
    ("internal/delivery/schedulerapi", "调度 API 交付接口"),
    ("internal/delivery", "交付层整体实现"),
    ("internal/infra/tools/builtin", "内建工具实现（文件、记忆、日历、子代理等）"),
    ("internal/infra/tools", "工具基础设施、策略与 SLA"),
    ("internal/infra/memory", "长期记忆存储、索引与检索"),
    ("internal/infra/observability", "日志、指标、追踪可观测性基础设施"),
    ("internal/infra/llm/router", "LLM 路由与模型策略"),
    ("internal/infra/llm", "LLM 提供方适配与调用基础设施"),
    ("internal/infra/lark", "Lark 基础设施客户端与 OAuth/日历能力"),
    ("internal/infra/session", "会话状态持久化与文件存储"),
    ("internal/infra/storage", "通用存储基础设施"),
    ("internal/infra/httpclient", "HTTP 客户端封装"),
    ("internal/infra/attachments", "附件基础设施处理"),
    ("internal/infra/filestore", "文件存储基础设施"),
    ("internal/infra/acp", "ACP 协议与执行基础设施"),
    ("internal/infra/runtime", "运行时基础设施"),
    ("internal/infra/external", "外部代理/子进程桥接能力"),
    ("internal/infra", "基础设施层实现"),
    ("internal/shared", "跨层共享基础能力"),
    ("internal/devops", "运维与进程管理能力"),
    ("cmd/server", "Server 可执行入口"),
    ("cmd/lark", "Lark 运行入口"),
    ("cmd/cli", "CLI 入口"),
    ("cmd", "命令行程序入口"),
    ("web/app/dev", "Web 运维/调试界面"),
    ("web/app/api", "Next.js API 路由"),
    ("web/app/conversation", "Web 会话主界面"),
    ("web/app/sessions", "会话列表与详情页"),
    ("web/components/ui", "Web 通用 UI 组件"),
    ("web/components/agent", "Agent 事件与卡片 UI 组件"),
    ("web/components", "Web 组件层"),
    ("web/hooks/useSSE", "SSE 连接与流式事件 Hook"),
    ("web/hooks", "Web Hook 层"),
    ("web/lib", "Web API 客户端与共享库"),
    ("web/e2e", "Web 端到端测试"),
    ("web/tests", "Web 测试与性能校验"),
    ("web", "Web 前端应用"),
    ("evaluation", "评测数据、脚本与结果"),
    ("tests", "集成测试与回归测试"),
    ("scripts", "自动化脚本与研发工具链"),
    ("configs", "运行配置与架构策略"),
    ("docs", "文档与设计记录"),
    ("skills", "技能模板与流程资产"),
]

PREFIX_CAPABILITY.sort(key=lambda item: len(item[0]), reverse=True)


@dataclass
class Row:
    path: str
    bytes: int
    line_count: int
    is_binary: bool
    extension: str
    file_kind: str
    is_code: bool
    is_test: bool
    is_generated: bool
    logical_layer: str
    module_l1: str
    module_l2: str
    module_l3: str
    capability: str
    capability_source: str


def run(cmd: List[str]) -> str:
    return subprocess.check_output(cmd, text=True).strip()


def git_tracked_files(ref: str) -> List[str]:
    out = run(["git", "ls-tree", "-r", "--name-only", ref])
    files = [line for line in out.splitlines() if line.strip()]
    files.sort()
    return files


def is_probably_binary(data: bytes) -> bool:
    if not data:
        return False
    sample = data[:8192]
    if b"\x00" in sample:
        return True
    try:
        decoder = codecs.getincrementaldecoder("utf-8")()
        decoder.decode(sample, final=False)
        return False
    except UnicodeDecodeError:
        return True


def file_stats(path: Path) -> Tuple[int, bool]:
    try:
        with path.open("rb") as f:
            sample = f.read(8192)
            if not sample:
                return 0, False
            binary = is_probably_binary(sample)
            if binary:
                return 0, True
            line_count = sample.count(b"\n")
            while True:
                chunk = f.read(65536)
                if not chunk:
                    break
                line_count += chunk.count(b"\n")
            return line_count, False
    except OSError:
        return 0, False


def is_test_path(path: str) -> bool:
    name = os.path.basename(path)
    lower = path.lower()
    return (
        name.endswith("_test.go")
        or name.endswith(".test.ts")
        or name.endswith(".test.tsx")
        or name.endswith(".spec.ts")
        or name.endswith(".spec.tsx")
        or "/__tests__/" in lower
        or "/tests/" in lower
        or lower.startswith("tests/")
        or lower.startswith("web/e2e/")
    )


def detect_extension(path: str) -> str:
    name = os.path.basename(path)
    ext = Path(name).suffix.lower()
    if ext:
        return ext
    if name in ROOT_CODE_FILENAMES:
        return name.lower()
    return ""


def detect_file_kind(path: str, ext: str, is_test: bool) -> str:
    if is_test:
        return "test"
    lower = path.lower()
    if ext in ASSET_EXTENSIONS:
        return "asset"
    if lower.startswith("docs/") or ext in DOC_EXTENSIONS:
        return "doc"
    if ext in CONFIG_EXTENSIONS or lower.startswith("configs/") or "/config" in lower:
        return "config"
    if lower.startswith(".github/"):
        return "ci"
    if lower.startswith("scripts/"):
        return "automation"
    if ext in CODE_EXTENSIONS or os.path.basename(path) in ROOT_CODE_FILENAMES:
        return "code"
    return "other"


def detect_layer(path: str) -> str:
    mapping = [
        ("internal/domain/", "domain"),
        ("internal/app/", "app"),
        ("internal/delivery/", "delivery"),
        ("internal/infra/", "infra"),
        ("internal/shared/", "shared"),
        ("internal/devops/", "devops"),
        ("web/", "web"),
        ("docs/", "docs"),
        ("scripts/", "scripts"),
        ("configs/", "configs"),
        ("skills/", "skills"),
        ("evaluation/", "evaluation"),
        ("tests/", "tests"),
        ("cmd/", "cmd"),
    ]
    for prefix, layer in mapping:
        if path.startswith(prefix):
            return layer
    return "root"


def module_levels(path: str) -> Tuple[str, str, str]:
    raw_parts = path.split("/")
    if len(raw_parts) == 1:
        return (raw_parts[0], raw_parts[0], raw_parts[0])

    parts = raw_parts[:-1]
    if not parts:
        return (raw_parts[0], raw_parts[0], raw_parts[0])

    def join_prefix(n: int) -> str:
        return "/".join(parts[: min(n, len(parts))])

    return (join_prefix(2), join_prefix(3), join_prefix(4))


def generated_hint(path: str) -> bool:
    lower = path.lower()
    return (
        "/generated/" in lower
        or lower.endswith(".pb.go")
        or lower.endswith(".gen.ts")
        or lower.endswith("_generated.ts")
        or lower.endswith(".generated.ts")
    )


def capability_for(path: str, layer: str, module_l2: str, file_kind: str, is_test: bool) -> Tuple[str, str]:
    for prefix, capability in PREFIX_CAPABILITY:
        if path.startswith(prefix):
            return capability, "prefix"

    lower = path.lower()
    keywords = [
        ("scheduler", "调度与提醒策略实现"),
        ("reminder", "提醒能力实现"),
        ("memory", "记忆读写与检索能力"),
        ("lark", "Lark 平台集成能力"),
        ("oauth", "OAuth 鉴权流程"),
        ("observability", "可观测性能力"),
        ("metrics", "指标采集与监控"),
        ("trace", "链路追踪能力"),
        ("tool", "工具调用与策略控制"),
        ("workflow", "工作流编排能力"),
        ("session", "会话状态管理"),
        ("api", "API 交付接口"),
        ("router", "路由与请求分发"),
        ("render", "输出渲染与格式化"),
        ("eval", "评测流程实现"),
    ]
    for key, capability in keywords:
        if key in lower:
            return capability, "keyword"

    if is_test or file_kind == "test":
        return "测试与回归保障", "fallback"

    if layer in {"docs", "skills"}:
        return "知识沉淀与流程文档", "fallback"

    if layer in {"scripts", "configs", "evaluation"}:
        return f"{layer} 支撑资产", "fallback"

    return f"{module_l2} 子模块实现", "fallback"


def build_rows(repo_root: Path, files: Iterable[str]) -> List[Row]:
    rows: List[Row] = []
    for rel in files:
        abs_path = repo_root / rel
        try:
            size = abs_path.stat().st_size
        except OSError:
            size = 0
        line_count, is_binary = file_stats(abs_path)
        ext = detect_extension(rel)
        is_test = is_test_path(rel)
        file_kind = detect_file_kind(rel, ext, is_test)
        is_code = file_kind in {"code", "automation", "ci"} or (
            ext in CODE_EXTENSIONS or os.path.basename(rel) in ROOT_CODE_FILENAMES
        )
        layer = detect_layer(rel)
        module_l1, module_l2, module_l3 = module_levels(rel)
        capability, source = capability_for(rel, layer, module_l2, file_kind, is_test)
        rows.append(
            Row(
                path=rel,
                bytes=size,
                line_count=line_count,
                is_binary=is_binary,
                extension=ext,
                file_kind=file_kind,
                is_code=is_code,
                is_test=is_test,
                is_generated=generated_hint(rel),
                logical_layer=layer,
                module_l1=module_l1,
                module_l2=module_l2,
                module_l3=module_l3,
                capability=capability,
                capability_source=source,
            )
        )
    return rows


def write_repo_catalog(path: Path, rows: List[Row]) -> None:
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(
            [
                "path",
                "bytes",
                "line_count",
                "is_binary",
                "extension",
                "file_kind",
                "is_code",
                "is_test",
                "is_generated",
                "logical_layer",
                "module_l1",
                "module_l2",
                "module_l3",
                "capability",
                "capability_source",
            ]
        )
        for r in rows:
            writer.writerow(
                [
                    r.path,
                    r.bytes,
                    r.line_count,
                    str(r.is_binary).lower(),
                    r.extension,
                    r.file_kind,
                    str(r.is_code).lower(),
                    str(r.is_test).lower(),
                    str(r.is_generated).lower(),
                    r.logical_layer,
                    r.module_l1,
                    r.module_l2,
                    r.module_l3,
                    r.capability,
                    r.capability_source,
                ]
            )


def write_code_mapping(path: Path, rows: List[Row]) -> None:
    code_rows = [r for r in rows if r.is_code]
    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(
            [
                "path",
                "line_count",
                "bytes",
                "is_binary",
                "logical_layer",
                "module_l1",
                "module_l2",
                "module_l3",
                "is_test",
                "is_generated",
                "capability",
                "capability_source",
            ]
        )
        for r in code_rows:
            writer.writerow(
                [
                    r.path,
                    r.line_count,
                    r.bytes,
                    str(r.is_binary).lower(),
                    r.logical_layer,
                    r.module_l1,
                    r.module_l2,
                    r.module_l3,
                    str(r.is_test).lower(),
                    str(r.is_generated).lower(),
                    r.capability,
                    r.capability_source,
                ]
            )


def write_module_summary(path: Path, rows: List[Row]) -> None:
    buckets = defaultdict(
        lambda: {
            "layer": "",
            "module_l1": "",
            "module_l2": "",
            "module_l3": "",
            "files": 0,
            "code_files": 0,
            "test_files": 0,
            "lines": 0,
            "code_lines": 0,
            "capability_lines": defaultdict(int),
        }
    )
    for r in rows:
        b = buckets[r.module_l2]
        b["layer"] = r.logical_layer
        b["module_l1"] = r.module_l1
        b["module_l2"] = r.module_l2
        b["module_l3"] = r.module_l3
        b["files"] += 1
        b["lines"] += r.line_count
        if r.is_code:
            b["code_files"] += 1
            b["code_lines"] += r.line_count
            b["capability_lines"][r.capability] += r.line_count
        if r.is_test:
            b["test_files"] += 1

    rows_out = []
    for module_l2, b in buckets.items():
        capability_lines = b["capability_lines"]
        if capability_lines:
            dominant_capability, dominant_lines = max(capability_lines.items(), key=lambda kv: kv[1])
        else:
            dominant_capability, dominant_lines = ("", 0)
        share = 0.0
        if b["code_lines"] > 0:
            share = dominant_lines / b["code_lines"]
        rows_out.append(
            [
                b["layer"],
                b["module_l1"],
                module_l2,
                b["module_l3"],
                b["files"],
                b["code_files"],
                b["test_files"],
                b["lines"],
                b["code_lines"],
                dominant_capability,
                f"{share:.4f}",
            ]
        )
    rows_out.sort(key=lambda x: int(x[8]), reverse=True)

    with path.open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(
            [
                "logical_layer",
                "module_l1",
                "module_l2",
                "module_l3",
                "file_count",
                "code_file_count",
                "test_file_count",
                "total_lines",
                "code_lines",
                "dominant_capability",
                "dominant_capability_line_share",
            ]
        )
        for row in rows_out:
            writer.writerow(row)


def build_summary(rows: List[Row]) -> dict:
    total_files = len(rows)
    total_lines = sum(r.line_count for r in rows)
    total_bytes = sum(r.bytes for r in rows)
    binary_files = sum(1 for r in rows if r.is_binary)

    code_rows = [r for r in rows if r.is_code]
    code_files = len(code_rows)
    code_lines = sum(r.line_count for r in code_rows)

    by_layer = defaultdict(lambda: {"files": 0, "lines": 0, "code_files": 0, "code_lines": 0, "bytes": 0})
    by_module_l2 = defaultdict(
        lambda: {
            "layer": "",
            "files": 0,
            "lines": 0,
            "code_files": 0,
            "code_lines": 0,
            "bytes": 0,
        }
    )
    for r in rows:
        layer_bucket = by_layer[r.logical_layer]
        layer_bucket["files"] += 1
        layer_bucket["lines"] += r.line_count
        layer_bucket["bytes"] += r.bytes
        if r.is_code:
            layer_bucket["code_files"] += 1
            layer_bucket["code_lines"] += r.line_count

        module_bucket = by_module_l2[r.module_l2]
        module_bucket["layer"] = r.logical_layer
        module_bucket["files"] += 1
        module_bucket["lines"] += r.line_count
        module_bucket["bytes"] += r.bytes
        if r.is_code:
            module_bucket["code_files"] += 1
            module_bucket["code_lines"] += r.line_count

    top_code_files = sorted(
        [r for r in code_rows if not r.is_test],
        key=lambda r: r.line_count,
        reverse=True,
    )[:30]

    return {
        "total_files": total_files,
        "total_lines": total_lines,
        "total_bytes": total_bytes,
        "binary_files": binary_files,
        "code_files": code_files,
        "code_lines": code_lines,
        "coverage_basis": "git ls-files",
        "by_layer": dict(sorted(by_layer.items(), key=lambda kv: kv[1]["lines"], reverse=True)),
        "by_module_l2": dict(sorted(by_module_l2.items(), key=lambda kv: kv[1]["lines"], reverse=True)),
        "top_code_files": [
            {
                "path": r.path,
                "line_count": r.line_count,
                "logical_layer": r.logical_layer,
                "module_l2": r.module_l2,
                "capability": r.capability,
            }
            for r in top_code_files
        ],
    }


def main() -> None:
    repo_root = Path(__file__).resolve().parents[2]
    os.chdir(repo_root)
    out_dir = repo_root / "docs" / "analysis"
    out_dir.mkdir(parents=True, exist_ok=True)

    today_prefix = os.environ.get("FOOTPRINT_REPORT_DATE", date.today().isoformat())
    catalog_path = out_dir / f"{today_prefix}-repo-file-catalog.csv"
    code_mapping_path = out_dir / f"{today_prefix}-code-file-capability-mapping.csv"
    module_summary_path = out_dir / f"{today_prefix}-module-capability-summary.csv"
    summary_path = out_dir / f"{today_prefix}-code-footprint-summary.json"

    source_ref = os.environ.get("FOOTPRINT_SOURCE_REF", "HEAD")
    files = git_tracked_files(source_ref)
    rows = build_rows(repo_root, files)

    write_repo_catalog(catalog_path, rows)
    write_code_mapping(code_mapping_path, rows)
    write_module_summary(module_summary_path, rows)
    summary = build_summary(rows)
    summary_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")

    print(f"generated: {catalog_path}")
    print(f"generated: {code_mapping_path}")
    print(f"generated: {module_summary_path}")
    print(f"generated: {summary_path}")
    print(f"source_ref={source_ref} tracked_files={summary['total_files']} code_files={summary['code_files']}")


if __name__ == "__main__":
    main()
