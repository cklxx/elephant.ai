import { createQuery } from "@tanstack/solid-query";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { listSessions } from "@/lib/api";
import { formatTimestamp, toRelativeTime } from "@/lib/utils";
import { For, Show } from "solid-js";

export default function SessionsPage() {
  const sessionsQuery = createQuery(() => ({
    queryKey: ["sessions"],
    queryFn: listSessions,
    refetchOnWindowFocus: false,
  }));

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sessions</CardTitle>
        <CardDescription>Inspect existing sessions emitted by the agent backend.</CardDescription>
      </CardHeader>
      <CardContent>
        <Show when={sessionsQuery.isPending}>
          <div class="rounded-md border bg-muted/40 p-3 text-sm text-muted-foreground">Loading sessions...</div>
        </Show>
        <Show when={sessionsQuery.isError}>
          <div class="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
            Failed to load sessions: {sessionsQuery.error?.message}
          </div>
        </Show>
        <Show when={sessionsQuery.data?.sessions?.length === 0}>
          <p class="text-sm text-muted-foreground">No sessions yet. Start a task to see activity.</p>
        </Show>
        <div class="divide-y">
          <For each={sessionsQuery.data?.sessions ?? []}>
            {(session) => (
              <div class="flex flex-col gap-2 py-3 lg:flex-row lg:items-center lg:justify-between">
                <div>
                  <div class="text-sm font-semibold">{session.id}</div>
                  <div class="text-xs text-muted-foreground">
                    Updated {toRelativeTime(session.updated_at)} • Created {formatTimestamp(session.created_at)}
                  </div>
                </div>
                <div class="flex items-center gap-2 text-xs text-muted-foreground">
                  <Badge variant="secondary">{session.task_count} tasks</Badge>
                  <Show when={session.last_task}>
                    <span class="rounded border border-dashed px-2 py-1">Last task: {session.last_task}</span>
                  </Show>
                </div>
              </div>
            )}
          </For>
        </div>
      </CardContent>
    </Card>
  );
}
