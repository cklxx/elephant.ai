"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Loader2, RefreshCw, Search, Sparkles, Wand2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import { apiClient } from "@/lib/api";
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

const TERMINAL_STATUSES = new Set(["completed", "failed", "cancelled", "error"]);
const DEFAULT_DRAFT =
  "写作目的：重构心流写作模式，让提示由 LLM 自动生成。\n要求：简洁、权重排序、带自动搜索建议。\n\n草稿：这里粘贴你正在写的正文，助手会自动给出下一步提示。";

function normalizePriority(value: unknown, fallback = 3): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function parseLlmSuggestions(raw: string): { prompts: PromptSuggestion[]; searches: SearchSuggestion[] } | null {
  const fencedMatch = raw.match(/```json\s*([\s\S]*?)```/i);
  const candidate = fencedMatch?.[1]?.trim() ?? raw.trim();

  try {
    const parsed = JSON.parse(candidate) as LlmSuggestionPayload;
    const prompts = (parsed.prompts ?? [])
      .filter((item) => item.title?.trim() && item.content?.trim())
      .map((item, index) => ({
        id: `${item.title}-${index}`,
        title: item.title.trim(),
        content: item.content.trim(),
        priority: normalizePriority(item.priority, index + 1),
      }));

    const searches = (parsed.searches ?? [])
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
  } catch (error) {
    console.error("[flow] Failed to parse LLM suggestions", error);
    return null;
  }
}

function buildFlowTaskPrompt(draft: string): string {
  const compactDraft = draft.trim();
  return [
    "你是心流写作助手，负责基于用户草稿产出下一步动作。",
    "必须返回 JSON，不要加入解释或 markdown。",
    "JSON 结构：{\"prompts\":[{\"title\":\"\",\"content\":\"\",\"priority\":1}],\"searches\":[{\"query\":\"\",\"reason\":\"\",\"priority\":1}]}",
    "prompts 用于直接给用户的写作提示，priority 数字越小越靠前，务必排序。",
    "searches 用于自动检索/案例建议，query 面向搜索引擎，reason 简要说明价值。",
    `草稿：${compactDraft}`,
  ].join("\n");
}

export function FlowModePanel() {
  const [draft, setDraft] = useState(DEFAULT_DRAFT);
  const [prompts, setPrompts] = useState<PromptSuggestion[]>([]);
  const [searches, setSearches] = useState<SearchSuggestion[]>([]);
  const [status, setStatus] = useState<"idle" | "pending" | "waiting" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const [activeTask, setActiveTask] = useState<{ id: string; version: number } | null>(null);
  const requestVersion = useRef(0);

  const statusLabel = useMemo(() => {
    switch (status) {
      case "pending":
        return "LLM 正在理解草稿…";
      case "waiting":
        return "等待 LLM 输出…";
      case "error":
        return "生成失败，稍后重试";
      default:
        return "自动生成提示与搜索建议";
    }
  }, [status]);

  const runSuggestionTask = useCallback(
    async (input: string) => {
      const version = requestVersion.current + 1;
      setStatus("pending");
      setError(null);
      requestVersion.current = version;

      try {
        const task = await apiClient.createTask({ task: buildFlowTaskPrompt(input) });
        if (version === requestVersion.current) {
          setActiveTask({ id: task.task_id, version });
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
    [],
  );

  useEffect(() => {
    const trimmed = draft.trim();
    if (!trimmed) return undefined;

    const timer = setTimeout(() => {
      void runSuggestionTask(trimmed);
    }, 800);

    return () => clearTimeout(timer);
  }, [draft, runSuggestionTask]);

  useEffect(() => {
    if (!activeTask) return undefined;

    let attempts = 0;
    const { id, version } = activeTask;
    const poll = async () => {
      attempts += 1;
      const isCurrent = version === requestVersion.current;
      try {
        const response = await apiClient.getTaskStatus(id);
        if (response.status === "completed" && response.final_answer) {
          if (isCurrent) {
            const parsed = parseLlmSuggestions(response.final_answer);
            if (parsed) {
              setPrompts(parsed.prompts);
              setSearches(parsed.searches);
              setStatus("idle");
            } else {
              setError("LLM 返回格式异常");
              setStatus("error");
            }
          }
          if (isCurrent) {
            setActiveTask(null);
          }
          return;
        }

        if (TERMINAL_STATUSES.has(response.status) || attempts > 40) {
          if (isCurrent) {
            setStatus("error");
            setError("任务未返回有效结果");
            setActiveTask(null);
          }
        }
      } catch (err) {
        console.error("[flow] polling failed", err);
        if (isCurrent) {
          setError("任务状态查询失败");
          setStatus("error");
          setActiveTask(null);
        }
      }
    };

    const interval = setInterval(poll, 1500);
    void poll();

    return () => clearInterval(interval);
  }, [activeTask]);

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
        setStatus("idle");
        setError(null);
      }
    },
    [],
  );

  return (
    <div className="grid gap-4 lg:grid-cols-[1.2fr,0.8fr]">
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
            只保留一个书写框，其他交给 LLM：它会自动生成优先级排序的提示和搜索建议。
          </p>
        </CardHeader>
        <CardContent className="space-y-3">
          <Textarea
            value={draft}
            rows={12}
            onChange={(event) => handleDraftChange(event.target.value)}
            placeholder="开始写作，LLM 会自动给出下一步提示"
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
              重新生成提示
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
                正在等待 LLM 生成写作提示…
              </div>
            )}
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
              </div>
            ))}
            {!searches.length && (
              <div className="rounded-xl border border-dashed border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
                自动搜索提示会根据正文实时生成。
              </div>
            )}
          </div>

          {!hasSuggestions && status !== "error" ? (
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
