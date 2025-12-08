import { createSignal, For, onCleanup, Show } from "solid-js";
import { apiClient, createTask } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn, formatTimestamp, toRelativeTime } from "@/lib/utils";

const WORKFLOW_EVENT_TYPES = [
  "connected",
  "workflow.node.started",
  "workflow.node.completed",
  "workflow.node.failed",
  "workflow.node.output.delta",
  "workflow.node.output.summary",
  "workflow.tool.started",
  "workflow.tool.progress",
  "workflow.tool.completed",
  "workflow.input.received",
  "workflow.subflow.progress",
  "workflow.subflow.completed",
  "workflow.result.final",
  "workflow.result.cancelled",
  "workflow.diagnostic.error",
  "workflow.diagnostic.context_compression",
  "workflow.diagnostic.tool_filtering",
  "workflow.diagnostic.browser_info",
  "workflow.diagnostic.environment_snapshot",
  "workflow.diagnostic.sandbox_progress",
] as const;

type StreamEvent = {
  id: string;
  type: string;
  timestamp: string;
  payload?: unknown;
  summary: string;
};

function describePayload(type: string, payload: any): string {
  if (!payload) return type;
  if (typeof payload === "string") return payload;
  if (typeof payload === "object") {
    if (typeof payload.message === "string") return payload.message;
    if (typeof payload.content === "string") return payload.content;
    if (typeof payload.event_type === "string" && typeof payload.status === "string") {
      return `${payload.event_type} • ${payload.status}`;
    }
  }
  return type;
}

function toneForEvent(type: string): "success" | "warning" | "destructive" | "secondary" {
  if (type.includes("failed") || type.includes("error")) return "destructive";
  if (type.includes("progress") || type.includes("delta")) return "secondary";
  if (type.includes("completed") || type.includes("final")) return "success";
  return "secondary";
}

export default function ConsolePage() {
  const [task, setTask] = createSignal("");
  const [sessionId, setSessionId] = createSignal<string | null>(null);
  const [taskId, setTaskId] = createSignal<string | null>(null);
  const [events, setEvents] = createSignal<StreamEvent[]>([]);
  const [status, setStatus] = createSignal<"idle" | "running" | "error">("idle");
  const [error, setError] = createSignal<string | null>(null);
  const [manualSessionId, setManualSessionId] = createSignal("");

  let eventSource: EventSource | null = null;

  const addEvent = (event: StreamEvent) => {
    setEvents((current) => {
      const next = [...current, event];
      return next.slice(-400);
    });
  };

  const connect = (id: string) => {
    eventSource?.close();
    try {
      const source = apiClient.createSSEConnection(id);
      WORKFLOW_EVENT_TYPES.forEach((eventType) => {
        source.addEventListener(eventType, (evt) => {
          try {
            const payload = evt.data ? JSON.parse(evt.data) : null;
            addEvent({
              id: `${Date.now()}-${Math.random()}`,
              type: eventType,
              payload,
              timestamp: payload?.timestamp ?? new Date().toISOString(),
              summary: describePayload(eventType, payload),
            });
          } catch (err) {
            console.error("Failed to parse SSE payload", err);
          }
        });
      });
      source.onerror = (evt) => {
        addEvent({
          id: `${Date.now()}-${Math.random()}`,
          type: "connection.error",
          timestamp: new Date().toISOString(),
          summary: evt instanceof Event ? evt.type : "Stream error",
        });
        setStatus("error");
      };
      eventSource = source;
      setStatus("running");
    } catch (err) {
      setStatus("error");
      setError(err instanceof Error ? err.message : "Unable to start SSE stream");
    }
  };

  const handleSubmit = async (event: Event) => {
    event.preventDefault();
    const description = task().trim();
    if (!description) return;
    setError(null);
    setStatus("running");
    setEvents([]);

    try {
      const response = await createTask({ task: description, session_id: sessionId() ?? undefined });
      setSessionId(response.session_id);
      setTaskId(response.task_id);
      addEvent({
        id: `${Date.now()}-${Math.random()}`,
        type: "task.created",
        timestamp: new Date().toISOString(),
        summary: `Task ${response.task_id} created for session ${response.session_id}`,
      });
      connect(response.session_id);
    } catch (err) {
      setStatus("error");
      setError(err instanceof Error ? err.message : "Failed to create task");
    }
  };

  const handleManualConnect = () => {
    const id = manualSessionId().trim();
    if (!id) return;
    setSessionId(id);
    connect(id);
  };

  onCleanup(() => eventSource?.close());

  return (
    <div class="flex flex-col gap-4">
      <div class="grid gap-4 lg:grid-cols-3">
        <Card class="lg:col-span-2">
          <CardHeader>
            <div class="flex items-center justify-between gap-2">
              <div>
                <CardTitle>Research Console</CardTitle>
                <CardDescription>Streamlined terminal-style viewport for long running workflows.</CardDescription>
              </div>
              <Badge variant={status() === "running" ? "success" : status() === "error" ? "destructive" : "secondary"}>
                {status() === "running" ? "Connected" : status() === "error" ? "Error" : "Idle"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div class="mb-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span class="rounded border border-dashed px-2 py-1">
                Session: {sessionId() ?? "none"}
              </span>
              <span class="rounded border border-dashed px-2 py-1">Task: {taskId() ?? "pending"}</span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setEvents([]);
                  setStatus("idle");
                  eventSource?.close();
                }}
              >
                Clear
              </Button>
            </div>
            <div class="flex h-[420px] flex-col rounded-md border bg-muted/30 p-3">
              <div class="mb-2 flex items-center justify-between text-xs text-muted-foreground">
                <span>Live event stream</span>
                <span>{events().length} events</span>
              </div>
              <div class="flex-1 space-y-2 overflow-y-auto rounded-md bg-background p-3 font-mono text-xs">
                <Show when={events().length === 0}>
                  <p class="text-muted-foreground">Waiting for events. Start a task or reconnect to a session.</p>
                </Show>
                <For each={events()}>
                  {(event) => (
                    <div
                      class={cn(
                        "flex items-start gap-3 rounded border px-3 py-2 transition-colors",
                        toneForEvent(event.type) === "success" && "border-emerald-200 bg-emerald-50",
                        toneForEvent(event.type) === "warning" && "border-amber-200 bg-amber-50",
                        toneForEvent(event.type) === "destructive" && "border-destructive/30 bg-destructive/5",
                        toneForEvent(event.type) === "secondary" && "border-border bg-card",
                      )}
                    >
                      <div class="min-w-[120px] text-[11px] text-muted-foreground">
                        <div>{formatTimestamp(event.timestamp)}</div>
                        <div>{toRelativeTime(event.timestamp)}</div>
                      </div>
                      <div class="flex-1">
                        <div class="flex items-center gap-2">
                          <Badge variant={toneForEvent(event.type)}> {event.type} </Badge>
                          <span class="text-[11px] text-muted-foreground">{event.id}</span>
                        </div>
                        <div class="mt-1 whitespace-pre-wrap text-sm leading-tight">{event.summary}</div>
                      </div>
                    </div>
                  )}
                </For>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Start or Reconnect</CardTitle>
            <CardDescription>Kick off a new run or attach to an existing session id.</CardDescription>
          </CardHeader>
          <CardContent class="space-y-4">
            <form class="space-y-3" onSubmit={handleSubmit}>
              <label class="text-sm font-medium">Task description</label>
              <Textarea
                value={task()}
                onInput={(evt) => setTask(evt.currentTarget.value)}
                placeholder="Describe the workflow you want ALEX to run..."
              />
              <Button type="submit" class="w-full">
                Launch & stream
              </Button>
            </form>
            <div class="space-y-2 rounded-md border p-3">
              <div class="text-sm font-medium">Attach to existing session</div>
              <p class="text-xs text-muted-foreground">
                Useful when the backend is still running but the UI refreshed. Paste the session id to resume the feed.
              </p>
              <Input
                value={manualSessionId()}
                placeholder="session id"
                onInput={(evt) => setManualSessionId(evt.currentTarget.value)}
              />
              <Button variant="secondary" class="w-full" onClick={handleManualConnect}>
                Connect
              </Button>
            </div>
            <Show when={error()}>
              <div class="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
                {error()}
              </div>
            </Show>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
