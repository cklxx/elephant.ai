"use client";

import { useCallback, useEffect, useEffectEvent, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Play,
  RefreshCw,
  Search,
  Sparkles,
  Type,
  Wand2,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { isWorkflowResultFinalEvent } from "@/lib/typeGuards";
import { AnyAgentEvent, eventMatches } from "@/lib/types";
import { cn } from "@/lib/utils";

type PromptSuggestion = {
  id: string;
  title: string;
  content: string;
  priority: number;
};

type SearchSuggestion = {
  id: string;
  query: string;
  reason?: string;
  priority: number;
};

type LlmSuggestionPayload = {
  prompts?: Array<Pick<PromptSuggestion, "title" | "content" | "priority">>;
  searches?: Array<{ query: string; reason?: string; priority?: number }>;
};

const DEFAULT_DRAFT =
  "写作目的：重构心流写作模式，让提示由 Agent 自动生成。\n要求：简洁、权重排序、带自动搜索建议。\n\n草稿：这里粘贴你正在写的正文，助手会自动给出下一步提示。";

type FlowModePanelProps = {
  events: AnyAgentEvent[];
  onRunTask: (task: string) => Promise<{ session_id: string; task_id: string }>;
};

type SuggestionSnapshot = {
  id: string;
  prompts: PromptSuggestion[];
  searches: SearchSuggestion[];
  createdAt: string;
};

type ToolRunStatus = "pending" | "waiting" | "running" | "succeeded" | "failed";

type ToolRun = {
  id: string;
  query: string;
  reason?: string;
  taskId?: string;
  sessionId?: string;
  status: ToolRunStatus;
  result?: string;
  error?: string;
  startedAt: string;
};

type WriteAction = {
  id: string;
  title: string;
  description: string;
  instruction: string;
};

type WritingRun = {
  id: string;
  actionId: string;
  title: string;
  status: ToolRunStatus;
  taskId?: string;
  sessionId?: string;
  result?: string;
  error?: string;
  startedAt: string;
};

const WRITING_ACTIONS: WriteAction[] = [
  {
    id: "continue",
    title: "续写一段",
    description: "延续现有语气写出下一段，保持行文节奏。",
    instruction: "基于当前草稿续写 1 段，延续语气与结构，保持论点一致。",
  },
  {
    id: "polish",
    title: "润色表达",
    description: "收紧冗余，强化节奏，突出关键信息。",
    instruction: "对草稿进行润色，收紧冗余，增强节奏与逻辑，可直接输出改写稿。",
  },
  {
    id: "bullets",
    title: "要点提炼",
    description: "提炼核心要点与行动项，便于继续展开。",
    instruction: "提炼 4-6 条核心要点或行动项，便于下一步写作，保持中文表达。",
  },
  {
    id: "outline",
    title: "提纲生成",
    description: "输出分层提纲，标明关键论点与佐证。",
    instruction: "生成分层提纲，包含关键论点、证据与过渡提示，确保结构清晰、条理分明。",
  },
  {
    id: "tighten",
    title: "压缩精简",
    description: "将草稿压缩到约一半字数，突出主线。",
    instruction: "在不丢失关键信息的前提下，将草稿压缩到约 50% 字数，突出主线与论点。",
  },
  {
    id: "examples",
    title: "补充案例",
    description: "为论点添加 2-3 个可引用的案例或数据。",
    instruction: "为草稿中的主要论点补充 2-3 个可引用的案例或数据，注明来源或出处。",
  },
];

export function buildWriteActionPrompt(action: WriteAction, draft: string): string {
  const payload = {
    action: action.id,
    draft: draft.trim(),
    notes: action.instruction,
    attachment_name: `${action.id}-draft.txt`,
  };
  return [
    "你是写作工具调度员，仅调用 flow_write 工具完成请求。",
    "使用以下 JSON 作为 flow_write 的调用参数：",
    JSON.stringify(payload),
    "调用工具，不要直接生成正文；等待工具结果即可。",
  ].join("\n");
}

function normalizePriority(value: unknown, fallback = 3): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function extractJsonCandidate(raw: string): string | null {
  const fencedMatch = raw.match(/```json\s*([\s\S]*?)```/i);
  if (fencedMatch?.[1]) {
    return fencedMatch[1].trim();
  }

  const trimmed = raw.trim();
  if (trimmed.startsWith("{") && trimmed.endsWith("}")) {
    return trimmed;
  }

  const firstBraceIndex = raw.indexOf("{");
  if (firstBraceIndex === -1) {
    return null;
  }

  let depth = 0;
  for (let i = firstBraceIndex; i < raw.length; i += 1) {
    const char = raw[i];
    if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        return raw.slice(firstBraceIndex, i + 1).trim();
      }
    }
  }

  return null;
}

function parseTextSuggestions(raw: string): { prompts: PromptSuggestion[]; searches: SearchSuggestion[] } | null {
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  if (!lines.length) {
    return null;
  }

  const prompts: PromptSuggestion[] = [];
  const searches: SearchSuggestion[] = [];
  let target: "prompts" | "searches" = "prompts";

  lines.forEach((line, index) => {
    const normalized = line.replace(/^\d+[\.\)]\s*/, "").replace(/^[-•]\s*/, "");
    const lower = normalized.toLowerCase();
    if (/^search(?:es)?[:：]?$/.test(lower) || /^(?:自动)?搜索[:：]?$/.test(lower)) {
      target = "searches";
      return;
    }
    if (/^prompt(?:s)?[:：]?$/.test(lower) || /^(?:写作)?提示[:：]?$/.test(lower)) {
      target = "prompts";
      return;
    }

    const [titlePart, ...rest] = normalized.split(/[:：]\s*/);
    const title = titlePart?.trim() ?? "";
    const remainder = rest.join(":").trim();
    const content = remainder || title;
    const primary = content || title;

    if (primary) {
      if (target === "searches") {
        searches.push({
          id: `${primary}-${searches.length}`,
          query: primary,
          reason: remainder ? title : undefined,
          priority: searches.length + 1,
        });
      } else {
        prompts.push({
          id: `${primary}-${prompts.length}`,
          title: title || primary,
          content: content || primary,
          priority: prompts.length + 1,
        });
      }
    }
  });

  if (!prompts.length && !searches.length) {
    return null;
  }

  return { prompts, searches };
}

function parseJsonSuggestions(raw: string): { prompts: PromptSuggestion[]; searches: SearchSuggestion[] } | null {
  const candidate = extractJsonCandidate(raw);
  if (!candidate) {
    return null;
  }

  const parsed = JSON.parse(candidate) as LlmSuggestionPayload;
  const promptItems = Array.isArray(parsed.prompts)
    ? parsed.prompts
    : parsed.prompts
      ? [parsed.prompts]
      : [];
  const searchItems = Array.isArray(parsed.searches)
    ? parsed.searches
    : parsed.searches
      ? [parsed.searches]
      : [];

  const prompts = promptItems
    .filter((item) => item.title?.trim() && item.content?.trim())
    .map((item, index) => ({
      id: `${item.title}-${index}`,
      title: item.title.trim(),
      content: item.content.trim(),
      priority: normalizePriority(item.priority, index + 1),
    }));

  const searches = searchItems
    .filter((item) => item.query?.trim())
    .map((item, index) => ({
      id: `${item.query}-${index}`,
      query: item.query.trim(),
      reason: item.reason?.trim() || undefined,
      priority: normalizePriority(item.priority, index + 1),
    }));

  return {
    prompts: prompts.sort((a, b) => a.priority - b.priority),
    searches: searches.sort((a, b) => a.priority - b.priority),
  };
}

export function parseLlmSuggestions(raw: string): { prompts: PromptSuggestion[]; searches: SearchSuggestion[] } | null {
  if (!raw?.trim()) {
    return null;
  }

  try {
    const structured = parseJsonSuggestions(raw);
    if (structured) {
      return structured;
    }
  } catch (error) {
    console.error("[flow] Failed to parse JSON suggestions", error);
  }

  return parseTextSuggestions(raw);
}

function buildFlowTaskPrompt(draft: string): string {
  const compactDraft = draft.trim();
  return [
    "你是心流写作助手，负责基于用户草稿产出下一步动作。",
    "通过工具返回写作建议，直接输出要点文字，不需要 JSON。",
    "写作提示请包含优先级、标题与简要内容；自动搜索建议包含搜索关键词与原因。",
    "保持语言简洁有力，条目按重要性排序。",
    "prompts 用于直接给用户的写作提示，priority 数字越小越靠前，务必排序。",
    "searches 用于自动检索/案例建议，query 面向搜索引擎，reason 简要说明价值。",
    `草稿：${compactDraft}`,
  ].join("\n");
}

function buildSearchToolPrompt(search: SearchSuggestion): string {
  return [
    "请作为写作研究助手，使用可用的搜索/浏览工具获取关键素材。",
    `核心搜索关键词：${search.query}`,
    search.reason ? `检索意图：${search.reason}` : null,
    "输出结构：",
    "- 先给出 3-5 条要点摘要（带来源或链接提示）。",
    "- 如有数据/案例，请简要列出并注明出处。",
    "- 保持简洁可引用，不要冗长解释。",
  ]
    .filter(Boolean)
    .join("\n");
}

export function FlowModePanel({ events, onRunTask }: FlowModePanelProps) {
  const [draft, setDraft] = useState(DEFAULT_DRAFT);
  const [prompts, setPrompts] = useState<PromptSuggestion[]>([]);
  const [searches, setSearches] = useState<SearchSuggestion[]>([]);
  const [history, setHistory] = useState<SuggestionSnapshot[]>([]);
  const [status, setStatus] = useState<"idle" | "pending" | "waiting" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const [activeTask, setActiveTask] = useState<{ id: string; sessionId: string; version: number } | null>(null);
  const [toolRuns, setToolRuns] = useState<ToolRun[]>([]);
  const [writingRuns, setWritingRuns] = useState<WritingRun[]>([]);
  const enrichRunsWithEvents = useCallback(
    <T extends ToolRun | WritingRun>(runs: T[]): T[] => {
      if (!runs.length) return runs;

      return runs.map((run) => {
        if (!run.taskId) return run;
        const relevant = events.filter((event) => event.task_id === run.taskId);
        if (!relevant.length) return run;

        let nextStatus: ToolRunStatus = run.status;
        let nextResult = run.result;
        let nextError = run.error;

        relevant.forEach((event) => {
          if (eventMatches(event, "workflow.node.failed", "workflow.result.cancelled", "workflow.diagnostic.error")) {
            nextStatus = "failed";
            if ("error" in event && typeof event.error === "string") {
              nextError = event.error;
            }
          } else if (eventMatches(event, "workflow.result.final")) {
            nextStatus = "succeeded";
            if ("final_answer" in event && typeof event.final_answer === "string") {
              nextResult = event.final_answer.trim() || nextResult;
            }
          } else if (
            eventMatches(
              event,
              "workflow.tool.started",
              "workflow.tool.progress",
              "workflow.tool.completed",
              "workflow.node.started",
            )
          ) {
            if (nextStatus === "pending" || nextStatus === "waiting") {
              nextStatus = "running";
            }
            if ("result" in event && typeof event.result === "string" && event.result.trim()) {
              nextResult = event.result;
            }
          }
        });

        return {
          ...run,
          status: nextStatus,
          result: nextResult,
          error: nextError,
        };
      });
    },
    [events],
  );
  const requestVersion = useRef(0);
  const processedTaskIds = useRef<Set<string>>(new Set());
  const lastSnapshotRef = useRef<{ prompts: PromptSuggestion[]; searches: SearchSuggestion[] } | null>(null);

  const statusLabel = useMemo(() => {
    switch (status) {
      case "pending":
        return "Agent 正在理解草稿…";
      case "waiting":
        return "等待 Agent 输出…";
      case "error":
        return "生成失败，稍后重试";
      default:
        return "手动触发后生成提示与搜索建议";
    }
  }, [status]);

  const runSuggestionTask = useCallback(
    async (input: string) => {
      const version = requestVersion.current + 1;
      setStatus("pending");
      setError(null);
      requestVersion.current = version;

      try {
        const task = await onRunTask(buildFlowTaskPrompt(input));
        if (version === requestVersion.current) {
          setActiveTask({ id: task.task_id, sessionId: task.session_id, version });
          setStatus("waiting");
        }
      } catch (err) {
        console.error("[flow] createTask failed", err);
        if (version === requestVersion.current) {
          setError("无法创建任务，请稍后重试");
          setStatus("error");
        }
      }
    },
    [onRunTask],
  );

  const handleWritingAction = useCallback(
    async (action: WriteAction) => {
      if (!draft.trim()) return;

      const runId = `${action.id}-${Date.now()}`;
      const prompt = buildWriteActionPrompt(action, draft);
      setWritingRuns((prev) => [
        {
          id: runId,
          actionId: action.id,
          title: action.title,
          status: "pending",
          startedAt: new Date().toISOString(),
        },
        ...prev,
      ]);

      try {
        const task = await onRunTask(prompt);
        setWritingRuns((prev) =>
          prev.map((run) =>
            run.id === runId
              ? {
                  ...run,
                  taskId: task.task_id,
                  sessionId: task.session_id,
                  status: "waiting",
                }
              : run,
          ),
        );
      } catch (err) {
        console.error("[flow] writing action failed", err);
        setWritingRuns((prev) =>
          prev.map((run) =>
            run.id === runId
              ? {
                  ...run,
                  status: "failed",
                  error: "无法执行写作动作",
                }
              : run,
          ),
        );
      }
    },
    [draft, onRunTask],
  );

  const handleFlowEvent = useEffectEvent((event: AnyAgentEvent, taskVersion: number) => {
    if (!event.task_id) return;

    if (
      eventMatches(event, "workflow.node.failed", "workflow.result.cancelled", "workflow.diagnostic.error")
    ) {
      processedTaskIds.current.add(event.task_id);
      setStatus("error");
      setError("任务未返回有效结果");
      setActiveTask(null);
      return;
    }

    if (eventMatches(event, "workflow.tool.completed") && "result" in event) {
      const parsed = parseLlmSuggestions(event.result || "");
      if (parsed) {
        setPrompts(parsed.prompts);
        setSearches(parsed.searches);
        setStatus("idle");
        setError(null);
      }
    }

    if (!isWorkflowResultFinalEvent(event)) return;
    if (processedTaskIds.current.has(event.task_id)) return;

    const streamInProgress = event.is_streaming === true || event.stream_finished === false;
    if (streamInProgress) return;

    processedTaskIds.current.add(event.task_id);
    const previousSnapshot = lastSnapshotRef.current;
    if (previousSnapshot && (previousSnapshot.prompts.length || previousSnapshot.searches.length)) {
      setHistory((prev) => [
        {
          id: `${event.task_id}-${event.timestamp ?? Date.now()}`,
          prompts: previousSnapshot.prompts,
          searches: previousSnapshot.searches,
          createdAt: new Date().toISOString(),
        },
        ...prev,
      ]);
    }

    const parsed = parseLlmSuggestions(event.final_answer ?? "");
    if (parsed) {
      setPrompts(parsed.prompts);
      setSearches(parsed.searches);
      setStatus("idle");
      setError(null);
    } else if (lastSnapshotRef.current && (lastSnapshotRef.current.prompts.length || lastSnapshotRef.current.searches.length)) {
      setStatus("idle");
      setError(null);
    } else {
      setError("Agent 返回格式异常");
      setStatus("error");
    }

    if (requestVersion.current === taskVersion) {
      setActiveTask(null);
    }
  });

  useEffect(() => {
    if (!activeTask) return undefined;

    for (let i = events.length - 1; i >= 0; i -= 1) {
      const event = events[i];
      if (event.task_id !== activeTask.id) continue;
      handleFlowEvent(event, activeTask.version);
      break;
    }

    return undefined;
  }, [activeTask, events]);

  const handleApplyPrompt = useCallback((content: string) => {
    setDraft((current) => `${current.trim()}\n\n${content}`.trim());
  }, []);

  const hasSuggestions = prompts.length > 0 || searches.length > 0;
  const handleDraftChange = useCallback(
    (next: string) => {
      setDraft(next);
      if (!next.trim()) {
        setPrompts([]);
        setSearches([]);
        setActiveTask(null);
        setHistory([]);
        setStatus("idle");
        setError(null);
      }
    },
    [],
  );

  useEffect(() => {
    lastSnapshotRef.current = { prompts, searches };
  }, [prompts, searches]);

  const handleSendSearchToAgent = useCallback(
    async (search: SearchSuggestion) => {
      const runId = `${search.id}-${Date.now()}`;
      const prompt = buildSearchToolPrompt(search);
      setToolRuns((prev) => [
        {
          id: runId,
          query: search.query,
          reason: search.reason,
          status: "pending",
          startedAt: new Date().toISOString(),
        },
        ...prev,
      ]);

      try {
        const task = await onRunTask(prompt);
        setToolRuns((prev) =>
          prev.map((run) =>
            run.id === runId
              ? {
                  ...run,
                  taskId: task.task_id,
                  sessionId: task.session_id,
                  status: "waiting",
                }
              : run,
          ),
        );
      } catch (err) {
        console.error("[flow] send search to agent failed", err);
        setToolRuns((prev) =>
          prev.map((run) =>
            run.id === runId
              ? {
                  ...run,
                  status: "failed",
                  error: "无法派发搜索任务",
                }
              : run,
          ),
        );
      }
    },
    [onRunTask],
  );

  const displayedToolRuns = useMemo(() => enrichRunsWithEvents(toolRuns), [enrichRunsWithEvents, toolRuns]);
  const displayedWritingRuns = useMemo(
    () => enrichRunsWithEvents(writingRuns),
    [enrichRunsWithEvents, writingRuns],
  );

  const renderToolRunStatus = (run: { status: ToolRunStatus }) => {
    switch (run.status) {
      case "pending":
      case "waiting":
      case "running":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-1 text-[11px] font-semibold text-amber-700">
            <Loader2 className="h-3 w-3 animate-spin" aria-hidden />
            执行中
          </span>
        );
      case "succeeded":
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-1 text-[11px] font-semibold text-emerald-700">
            <CheckCircle2 className="h-3 w-3" aria-hidden />
            已完成
          </span>
        );
      case "failed":
      default:
        return (
          <span className="inline-flex items-center gap-1 rounded-full bg-destructive/10 px-2 py-1 text-[11px] font-semibold text-destructive">
            <AlertTriangle className="h-3 w-3" aria-hidden />
            失败
          </span>
        );
    }
  };

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr),360px] xl:grid-cols-[minmax(0,1fr),420px]">
      <Card className="bg-card/70 backdrop-blur">
        <CardHeader className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-2">
            <div className="inline-flex items-center gap-2 rounded-full bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-500">
              <Wand2 className="h-3.5 w-3.5" aria-hidden />
              心流写作
            </div>
            <Badge
              variant="secondary"
              className={cn(
                "border border-border/60 bg-background/60 text-[11px] font-semibold",
                status === "error" && "border-destructive/40 text-destructive",
              )}
            >
              {statusLabel}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground">
            只保留一个书写框，其他交给 Agent：它会自动生成优先级排序的提示和搜索建议，并通过流式返回。
          </p>
        </CardHeader>
        <CardContent className="space-y-3">
          <Textarea
            value={draft}
            rows={12}
            onChange={(event) => handleDraftChange(event.target.value)}
            placeholder="开始写作，Agent 会自动给出下一步提示"
            className="min-h-[260px] rounded-2xl border-border/70 bg-background/80 text-base leading-7"
          />
          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="rounded-full"
              onClick={() => runSuggestionTask(draft.trim())}
              disabled={!draft.trim() || status === "pending" || status === "waiting"}
            >
              {status === "pending" || status === "waiting" ? (
                <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
              ) : (
                <RefreshCw className="mr-2 h-4 w-4" aria-hidden />
              )}
              生成提示
            </Button>
            {error ? <span className="text-destructive">{error}</span> : null}
          </div>
        </CardContent>
      </Card>

      <Card className="bg-card/70 backdrop-blur">
        <CardHeader className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-sm font-semibold">
            <Sparkles className="h-4 w-4" aria-hidden />
            智能提示与自动搜索
          </div>
          <Badge variant="outline" className="border-dashed text-[11px]">
            数字越小越优先
          </Badge>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Wand2 className="h-3.5 w-3.5" aria-hidden />
              写作提示（点击可插入正文）
            </div>
            {prompts.map((prompt) => (
              <div
                key={prompt.id}
                className="flex flex-col gap-2 rounded-2xl border border-border/70 bg-background/70 p-3"
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-foreground">
                    <Badge variant="secondary" className="border-border/60 bg-background/60 text-[11px]">
                      优先级 {prompt.priority}
                    </Badge>
                    <span>{prompt.title}</span>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="rounded-full"
                    onClick={() => handleApplyPrompt(prompt.content)}
                  >
                    应用
                  </Button>
                </div>
                <p className="text-sm leading-6 text-muted-foreground whitespace-pre-wrap">
                  {prompt.content}
                </p>
              </div>
            ))}
            {!prompts.length && (
              <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
                正在等待 Agent 生成写作提示…
              </div>
            )}
            {history.length ? (
              <details className="rounded-xl border border-dashed border-border/60 bg-muted/30 p-3 text-xs text-muted-foreground">
                <summary className="flex cursor-pointer items-center justify-between gap-2 font-semibold text-foreground">
                  历史提示（折叠）
                  <span className="text-[11px] text-muted-foreground">{history.length} 条</span>
                </summary>
                <div className="mt-3 space-y-3">
                  {history.map((item) => (
                    <div
                      key={item.id}
                      className="space-y-2 rounded-lg border border-border/50 bg-background/70 p-3 text-foreground"
                    >
                      <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                        <span>{new Date(item.createdAt).toLocaleString()}</span>
                        <span>
                          提示 {item.prompts.length} · 搜索 {item.searches.length}
                        </span>
                      </div>
                      {item.prompts.length ? (
                        <div className="space-y-1 text-xs">
                          {item.prompts.slice(0, 2).map((prompt) => (
                            <div key={prompt.id} className="truncate font-semibold">
                              {prompt.title}
                            </div>
                          ))}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </details>
            ) : null}
          </div>

          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Type className="h-3.5 w-3.5" aria-hidden />
              写作工具（直接输出文本）
            </div>
            <div className="grid gap-2 md:grid-cols-2 lg:grid-cols-3">
              {WRITING_ACTIONS.map((action) => {
                const matched = displayedWritingRuns.find((run) => run.actionId === action.id);
                const running =
                  matched?.status === "pending" || matched?.status === "waiting" || matched?.status === "running";
                return (
                  <div
                    key={action.id}
                    className="flex flex-col gap-2 rounded-xl border border-border/70 bg-background/70 p-3 text-sm text-foreground"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-semibold">{action.title}</span>
                      <Badge variant="secondary" className="border-border/60 bg-background/60 text-[11px]">
                        即用
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground">{action.description}</p>
                    <div className="pt-1">
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="rounded-full"
                        onClick={() => handleWritingAction(action)}
                        disabled={!draft.trim() || running}
                      >
                        <Wand2 className="mr-2 h-4 w-4" aria-hidden />
                        生成
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>

            {displayedWritingRuns.length ? (
              <div className="space-y-2 rounded-xl border border-border/70 bg-background/80 p-3">
                <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                  <Type className="h-3.5 w-3.5" aria-hidden />
                  写作结果
                </div>
                {displayedWritingRuns.map((run) => (
                  <div key={run.id} className="rounded-lg border border-border/60 bg-muted/30 p-3 text-sm">
                    <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <div className="font-semibold">{run.title}</div>
                      {renderToolRunStatus(run)}
                    </div>
                    {run.result ? (
                      <p className="mt-2 text-xs text-foreground whitespace-pre-wrap">{run.result.slice(0, 400)}</p>
                    ) : null}
                    {run.error && run.status === "failed" ? (
                      <p className="mt-2 text-xs text-destructive">{run.error}</p>
                    ) : null}
                    {run.result && (
                      <div className="mt-2">
                        <Button
                          type="button"
                          size="sm"
                          variant="secondary"
                          className="rounded-full"
                          onClick={() => handleApplyPrompt(run.result || "")}
                        >
                          应用到正文
                        </Button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : null}
          </div>

          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Search className="h-3.5 w-3.5" aria-hidden />
              自动搜索与案例建议
            </div>
            {searches.map((item) => (
              <div
                key={item.id}
                className="flex flex-col gap-1 rounded-xl border border-border/70 bg-background/70 p-3 text-sm text-foreground"
              >
                <div className="flex items-center gap-2">
                  <Badge variant="secondary" className="border-border/60 bg-background/60 text-[11px]">
                    优先级 {item.priority}
                  </Badge>
                  <span className="font-semibold">{item.query}</span>
                </div>
                {item.reason ? (
                  <p className="text-xs text-muted-foreground">{item.reason}</p>
                ) : null}
                <div className="pt-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="rounded-full"
                    onClick={() => handleSendSearchToAgent(item)}
                  >
                    <Play className="mr-2 h-4 w-4" aria-hidden />
                    发送给 Agent
                  </Button>
                </div>
              </div>
            ))}
            {!searches.length && (
              <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
                自动搜索提示会根据正文实时生成。
              </div>
            )}
          </div>

          {toolRuns.length ? (
            <div className="space-y-3 rounded-xl border border-border/70 bg-background/80 p-3">
              <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                <Search className="h-3.5 w-3.5" aria-hidden />
                搜索任务派发
              </div>
              <div className="space-y-2">
                {displayedToolRuns.map((run) => (
                  <div key={run.id} className="rounded-lg border border-border/60 bg-muted/30 p-3 text-sm">
                    <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                      <div className="font-semibold">{run.query}</div>
                      {renderToolRunStatus(run)}
                    </div>
                    {run.reason ? <p className="text-xs text-muted-foreground">{run.reason}</p> : null}
                    {run.result ? (
                      <p className="mt-2 text-xs text-foreground whitespace-pre-wrap">{run.result.slice(0, 280)}</p>
                    ) : null}
                    {run.error && run.status === "failed" ? (
                      <p className="mt-2 text-xs text-destructive">{run.error}</p>
                    ) : null}
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {(!hasSuggestions && (status === "pending" || status === "waiting")) ? (
            <div className="flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50/80 p-3 text-xs text-emerald-900">
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
              正在为当前草稿构思提示…
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}
