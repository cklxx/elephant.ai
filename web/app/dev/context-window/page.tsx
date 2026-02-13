"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { toast } from "@/components/ui/toast";
import { APIError, getContextWindowPreview, listSessions } from "@/lib/api";
import type {
  ContextWindowPreviewResponse,
  Message,
  PlanNode,
  Session,
  StaticContext,
  DynamicContext,
  MetaContext,
} from "@/lib/types";
import { cn } from "@/lib/utils";

function AttachmentList({ message }: { message: Message }) {
  const attachments = message.attachments ?? undefined;
  if (!attachments) return null;

  const entries = Object.entries(attachments);
  if (entries.length === 0) return null;

  return (
    <div className="mt-2 space-y-1 text-xs text-muted-foreground">
      {entries.map(([name, attachment]) => (
        <div key={name} className="flex items-center justify-between rounded border bg-muted/40 px-2 py-1">
          <span className="font-medium text-foreground">{name}</span>
          <span className="text-[11px] uppercase tracking-wide">{attachment.media_type || "data"}</span>
        </div>
      ))}
    </div>
  );
}

function MessageCard({ message, index }: { message: Message; index: number }) {
  return (
    <div className="rounded-lg border bg-card px-3 py-2">
      <div className="flex items-center gap-2">
        <Badge variant="secondary" className="uppercase">
          {message.role || "unknown"}
        </Badge>
        {message.source && <Badge variant="outline">{message.source}</Badge>}
        <span className="text-[11px] text-muted-foreground">#{index + 1}</span>
      </div>
      <div className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground">{message.content || "—"}</div>
      <AttachmentList message={message} />
      {message.tool_calls && message.tool_calls.length > 0 && (
        <div className="mt-2 text-xs text-muted-foreground">Tool calls: {message.tool_calls.length}</div>
      )}
    </div>
  );
}

function PlanNodeList({ plans, depth = 0 }: { plans?: PlanNode[]; depth?: number }) {
  if (!plans || plans.length === 0) return null;

  return (
    <div className="space-y-2">
      {plans.map((plan) => (
        <div key={plan.id} className={cn("rounded-md border px-3 py-2", depth > 0 && "bg-muted/40")}>
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <Badge variant="secondary">{plan.status}</Badge>
              <span className="font-medium text-sm text-foreground">{plan.title}</span>
            </div>
            <span className="text-[11px] text-muted-foreground">#{plan.id}</span>
          </div>
          {plan.description && <p className="mt-1 text-xs text-muted-foreground">{plan.description}</p>}
          {plan.children && plan.children.length > 0 && (
            <div className="mt-2 border-l-2 border-dashed border-muted-foreground/40 pl-3">
              <PlanNodeList plans={plan.children} depth={depth + 1} />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function StaticContextSummary({ context }: { context?: StaticContext }) {
  if (!context) return null;
  return (
    <div className="grid gap-3 md:grid-cols-2">
      <div className="space-y-1">
        <p className="text-xs text-muted-foreground">Persona</p>
        <p className="text-sm font-medium text-foreground">{context.persona?.id || "—"}</p>
        {context.persona?.tone && <p className="text-xs text-muted-foreground">Tone: {context.persona.tone}</p>}
      </div>
      <div className="space-y-1">
        <p className="text-xs text-muted-foreground">Operating environment</p>
        <p className="text-sm font-medium text-foreground">{context.world?.environment || "—"}</p>
        {context.environment_summary && (
          <p className="text-xs text-muted-foreground">Summary: {context.environment_summary}</p>
        )}
      </div>
      {context.goal && (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">Goals</p>
          <p className="text-sm font-medium text-foreground">{context.goal.id || "—"}</p>
          {context.goal.long_term && context.goal.long_term.length > 0 && (
            <p className="text-xs text-muted-foreground">Long term: {context.goal.long_term.join("; ")}</p>
          )}
        </div>
      )}
      {context.tools && context.tools.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">Tools</p>
          <div className="flex flex-wrap gap-1">
            {context.tools.map((tool) => (
              <Badge key={tool} variant="outline">
                {tool}
              </Badge>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function DynamicContextSummary({ dynamic }: { dynamic?: DynamicContext }) {
  if (!dynamic) return null;

  return (
    <div className="space-y-3">
      {(dynamic.turn_id !== undefined || dynamic.llm_turn_seq !== undefined) && (
        <div className="flex gap-3 text-xs text-muted-foreground">
          {dynamic.turn_id !== undefined && <span>Turn #{dynamic.turn_id}</span>}
          {dynamic.llm_turn_seq !== undefined && <span>LLM turn #{dynamic.llm_turn_seq}</span>}
        </div>
      )}
      <PlanNodeList plans={dynamic.plans} />
      {dynamic.beliefs && dynamic.beliefs.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-semibold text-foreground">Beliefs</p>
          <ul className="list-disc space-y-1 pl-4 text-sm text-muted-foreground">
            {dynamic.beliefs.map((belief, index) => (
              <li key={`${belief.statement}-${index}`}>
                {belief.statement}{" "}
                {belief.confidence !== undefined && (
                  <span className="text-[11px] text-muted-foreground/80">(confidence: {belief.confidence})</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {dynamic.world_state && Object.keys(dynamic.world_state).length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-semibold text-foreground">World state</p>
          <pre className="whitespace-pre-wrap rounded-md bg-muted/40 p-3 text-xs text-foreground">
            {JSON.stringify(dynamic.world_state, null, 2)}
          </pre>
        </div>
      )}
    </div>
  );
}

function MetaContextSummary({ meta }: { meta?: MetaContext }) {
  if (!meta) return null;

  return (
    <div className="space-y-2">
      {meta.persona_version && (
        <p className="text-xs text-muted-foreground">Persona version: {meta.persona_version}</p>
      )}
      {meta.memories && meta.memories.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-semibold text-foreground">Memories</p>
          <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
            {meta.memories.map((memory) => (
              <li key={memory.key}>
                <span className="font-medium text-foreground">{memory.key}: </span>
                {memory.content}
              </li>
            ))}
          </ul>
        </div>
      )}
      {meta.recommendations && meta.recommendations.length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-semibold text-foreground">Recommendations</p>
          <ul className="list-disc space-y-1 pl-4 text-xs text-muted-foreground">
            {meta.recommendations.map((item, index) => (
              <li key={`${item}-${index}`}>{item}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function ContextWindowPreview({ preview }: { preview: ContextWindowPreviewResponse }) {
  const messages = preview.window?.messages ?? [];
  const staticContext = preview.window?.static;
  const dynamicContext = preview.window?.dynamic;
  const metaContext = preview.window?.meta;

  const tokenLabel = useMemo(() => {
    if (preview.token_limit && preview.token_estimate) {
      return `${preview.token_estimate} / ${preview.token_limit}`;
    }
    if (preview.token_estimate) return `${preview.token_estimate}`;
    return "Not available";
  }, [preview.token_estimate, preview.token_limit]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-3 justify-between">
            <div>
              <CardTitle className="text-lg">Context window</CardTitle>
              <CardDescription>Resolved view for debugging in development mode.</CardDescription>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {preview.persona_key && <Badge variant="outline">Persona: {preview.persona_key}</Badge>}
              {preview.tool_mode && <Badge variant="outline">Mode: {preview.tool_mode}</Badge>}
              {preview.tool_preset && <Badge variant="outline">Preset: {preview.tool_preset}</Badge>}
            </div>
          </div>
        </CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-3">
          <div>
            <p className="text-xs text-muted-foreground">Token estimate</p>
            <p className="text-2xl font-semibold text-foreground">{tokenLabel}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Messages</p>
            <p className="text-2xl font-semibold text-foreground">{messages.length}</p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">Session</p>
            <p className="text-sm font-medium text-foreground">{preview.session_id}</p>
          </div>
        </CardContent>
      </Card>

      {preview.window?.system_prompt && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">System prompt</CardTitle>
            <CardDescription>Captured from the assembled context window.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="whitespace-pre-wrap rounded-lg bg-muted/40 p-4 text-sm text-foreground">
              {preview.window.system_prompt}
            </pre>
          </CardContent>
        </Card>
      )}

      {staticContext && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Static context</CardTitle>
            <CardDescription>Persona, tools, and environment state.</CardDescription>
          </CardHeader>
          <CardContent>
            <StaticContextSummary context={staticContext} />
          </CardContent>
        </Card>
      )}

      {dynamicContext && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Dynamic context</CardTitle>
            <CardDescription>Plans, beliefs, and world state included in the window.</CardDescription>
          </CardHeader>
          <CardContent>
            <DynamicContextSummary dynamic={dynamicContext} />
          </CardContent>
        </Card>
      )}

      {metaContext && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Meta context</CardTitle>
            <CardDescription>Long-horizon memories and persona metadata.</CardDescription>
          </CardHeader>
          <CardContent>
            <MetaContextSummary meta={metaContext} />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Messages ({messages.length})</CardTitle>
          <CardDescription>Ordered payload delivered to the LLM.</CardDescription>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-[440px]">
            <div className="space-y-3 pr-2">
              {messages.map((message, index) => (
                <MessageCard key={`${message.role}-${index}`} message={message} index={index} />
              ))}
            </div>
          </ScrollArea>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Raw context JSON</CardTitle>
          <CardDescription>Full serialized payload for quick inspection.</CardDescription>
        </CardHeader>
        <CardContent>
          <ScrollArea className="h-[360px]">
            <pre className="whitespace-pre rounded-lg bg-muted/40 p-4 text-xs text-foreground">
              {JSON.stringify(preview.window, null, 2)}
            </pre>
          </ScrollArea>
        </CardContent>
      </Card>
    </div>
  );
}

function DevContextWindowPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [selectedSession, setSelectedSession] = useState("");
  const [preview, setPreview] = useState<ContextWindowPreviewResponse | null>(null);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadSessions = useCallback(async () => {
    setLoadingSessions(true);
    setError(null);
    try {
      const data = await listSessions();
      const ordered = data.sessions ?? [];
      setSessions(ordered);
      if (ordered.length > 0) {
        setSelectedSession((current) => current || ordered[0].id);
      }
    } catch (err) {
      setError("无法加载会话列表，请检查后端是否处于开发模式。");
    } finally {
      setLoadingSessions(false);
    }
  }, []);

  const loadPreview = async () => {
    const sessionId = selectedSession.trim();
    if (!sessionId) {
      setError("请输入需要调试的 session ID。");
      return;
    }
    setLoadingPreview(true);
    setError(null);
    try {
      const response = await getContextWindowPreview(sessionId);
      setPreview(response);
      toast.success("已刷新上下文窗口", `当前会话：${sessionId}`);
    } catch (err) {
      setPreview(null);
      if (err instanceof APIError && err.status === 404) {
        setError("开发模式未启用或接口不可用，无法获取上下文窗口。");
      } else {
        setError(err instanceof Error ? err.message : "无法获取上下文窗口");
      }
    } finally {
      setLoadingPreview(false);
    }
  };

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  const sessionHints = useMemo(() => {
    if (sessions.length === 0) return null;
    return (
      <div className="flex flex-wrap gap-2">
        {sessions.map((session) => (
          <Button
            key={session.id}
            type="button"
            variant={session.id === selectedSession ? "secondary" : "outline"}
            size="sm"
            onClick={() => setSelectedSession(session.id)}
          >
            {session.id}
          </Button>
        ))}
      </div>
    );
  }, [sessions, selectedSession]);

  return (
    <div className="space-y-6">
      <div className="space-y-2">
        <p className="text-[11px] uppercase tracking-wide text-muted-foreground">Development only</p>
        <h1 className="text-2xl font-semibold text-foreground">Context window explorer</h1>
        <p className="text-sm text-muted-foreground">
          可视化当前会话的完整上下文窗口，便于调试和评估。仅在后端环境为 development/dev 时可用。
        </p>
      </div>

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between gap-3">
            <div>
              <CardTitle className="text-base">选择 Session</CardTitle>
              <CardDescription>选定一个已存在的 session，实时查看组装好的上下文窗口。</CardDescription>
            </div>
            <Button variant="ghost" size="sm" onClick={loadSessions} disabled={loadingSessions}>
              <RefreshCw className={cn("h-4 w-4", loadingSessions && "animate-spin")} />
              <span className="sr-only">Refresh sessions</span>
            </Button>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={selectedSession}
              placeholder="Session ID"
              onChange={(event) => setSelectedSession(event.target.value)}
              className="flex-1"
            />
            <Button onClick={loadPreview} disabled={loadingPreview}>
              {loadingPreview && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              拉取上下文
            </Button>
          </div>
          {sessionHints}
          {error && <p className="text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>

      {preview ? (
        <ContextWindowPreview preview={preview} />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">尚未加载上下文窗口</CardTitle>
            <CardDescription>选择一个 session 并点击“拉取上下文”以查看。</CardDescription>
          </CardHeader>
        </Card>
      )}
    </div>
  );
}

export default function Page() {
  return (
      <DevContextWindowPage />
  );
}
