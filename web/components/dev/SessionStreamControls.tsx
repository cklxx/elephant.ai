"use client";

import { Play, RefreshCw, Square, Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SSEReplayMode } from "@/lib/api";
import type { Session } from "@/lib/types";

type SessionStreamControlsProps = {
  sessionIdInput: string;
  onSessionIdInputChange: (value: string) => void;
  sessionId: string;
  replayMode: SSEReplayMode;
  onReplayModeChange: (replayMode: SSEReplayMode) => void;
  onConnect: () => void;
  onDisconnect: () => void;
  onClear: () => void;
  isConnected: boolean;
  isConnecting: boolean;
  autoScroll: boolean;
  onToggleAutoScroll: () => void;
  larkSessions: Session[];
  isSessionsFetching: boolean;
  onRefreshLarkSessions: () => void;
  error: string | null;
};

export function SessionStreamControls({
  sessionIdInput,
  onSessionIdInputChange,
  sessionId,
  replayMode,
  onReplayModeChange,
  onConnect,
  onDisconnect,
  onClear,
  isConnected,
  isConnecting,
  autoScroll,
  onToggleAutoScroll,
  larkSessions,
  isSessionsFetching,
  onRefreshLarkSessions,
  error,
}: SessionStreamControlsProps) {
  return (
    <>
      <div className="mt-4 grid gap-3 md:grid-cols-[minmax(0,1fr)_140px_auto] md:items-end">
        <div className="space-y-1">
          <p className="text-xs font-semibold text-muted-foreground">Session ID</p>
          <Input
            value={sessionIdInput}
            onChange={(event) => onSessionIdInputChange(event.target.value)}
            placeholder="session-xxxxxxxx"
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                onConnect();
              }
            }}
          />
        </div>
        <div className="space-y-1">
          <p className="text-xs font-semibold text-muted-foreground">Replay</p>
          <select
            value={replayMode}
            onChange={(event) => onReplayModeChange(event.target.value as SSEReplayMode)}
            className="h-9 w-full rounded-md border border-input bg-background px-3 text-sm shadow-sm"
          >
            <option value="full">full</option>
            <option value="session">session</option>
            <option value="none">none</option>
          </select>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" onClick={onConnect} disabled={!sessionId || isConnecting}>
            <Play className="mr-2 h-4 w-4" />
            Connect
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={onDisconnect}
            disabled={!isConnected && !isConnecting}
          >
            <Square className="mr-2 h-4 w-4" />
            Disconnect
          </Button>
          <Button size="sm" variant="outline" onClick={onClear}>
            <Trash2 className="mr-2 h-4 w-4" />
            Clear
          </Button>
          <Button size="sm" variant={autoScroll ? "default" : "outline"} onClick={onToggleAutoScroll}>
            <RefreshCw className="mr-2 h-4 w-4" />
            Auto-scroll {autoScroll ? "on" : "off"}
          </Button>
        </div>
      </div>

      {larkSessions.length > 0 && (
        <div className="mt-3 flex items-center gap-2">
          <p className="text-xs font-semibold text-muted-foreground whitespace-nowrap">Lark Sessions</p>
          <select
            value={larkSessions.some((session) => session.id === sessionIdInput) ? sessionIdInput : ""}
            onChange={(event) => {
              if (event.target.value) {
                onSessionIdInputChange(event.target.value);
              }
            }}
            className="h-8 min-w-[260px] max-w-md rounded-md border border-input bg-background px-2 text-xs shadow-sm"
          >
            <option value="">-- Pick a Lark session --</option>
            {larkSessions.map((session) => (
              <option key={session.id} value={session.id}>
                {session.id}
                {session.title ? ` — ${session.title}` : ""} ({new Date(session.updated_at).toLocaleString()})
              </option>
            ))}
          </select>
          <Button size="sm" variant="outline" onClick={onRefreshLarkSessions} disabled={isSessionsFetching}>
            <RefreshCw className={`mr-2 h-4 w-4${isSessionsFetching ? " animate-spin" : ""}`} />
            Refresh
          </Button>
          <Badge variant="outline" className="text-[10px]">
            {larkSessions.length} Lark
          </Badge>
        </div>
      )}

      {error && (
        <div className="mt-3 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
          {error}
        </div>
      )}
    </>
  );
}
