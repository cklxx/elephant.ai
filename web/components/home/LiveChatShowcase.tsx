"use client";

// Design refresh guided by @anthropics/skills/frontend-design.

import { useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  Pause,
  Play,
  PlayCircle,
  RefreshCw,
  Sparkles,
  SquareTerminal,
  User,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Button } from "@/components/ui/button";

import type { HomeLang } from "./types";

export type StageKey = "plan" | "clearify" | "react";

export type StageCopy = {
  key: StageKey;
  label: string;
  summary: string;
  accent: string;
};

export type ChatTurn = {
  role: "user" | "agent" | "tool";
  content: string;
  stage?: StageKey;
};

type DemoCopy = {
  title: string;
  description: string;
  userLabel: string;
  agentLabel: string;
  toolLabel: string;
  evidenceLabel: string;
};

export function LiveChatShowcase({
  copy,
  stages,
  script,
  lang,
}: {
  copy: DemoCopy;
  stages: StageCopy[];
  script: ChatTurn[];
  lang: HomeLang;
}) {
  const [activeIndex, setActiveIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(true);

  useEffect(() => {
    if (!isPlaying) {
      return;
    }

    const id = setInterval(() => {
      setActiveIndex((current) => (current + 1) % script.length);
    }, 2000);

    return () => clearInterval(id);
  }, [isPlaying, script.length]);

  const stageIndicesByKey = useMemo<Record<StageKey, number[]>>(() => {
    const result: Record<StageKey, number[]> = { plan: [], clearify: [], react: [] };
    script.forEach((turn, index) => {
      if (turn.stage) {
        result[turn.stage].push(index);
      }
    });
    return result;
  }, [script]);

  const activeStage = script[activeIndex]?.stage ?? stages[0].key;
  const activeStageCopy = stages.find((stage) => stage.key === activeStage) ?? stages[0];
  const progress = ((activeIndex + 1) / script.length) * 100;
  const visibleTurns = script.slice(0, activeIndex + 1);
  const actionLog = visibleTurns.filter((turn) => turn.role !== "user").slice(-5);
  const roleLabels: Record<ChatTurn["role"], string> = {
    user: copy.userLabel,
    agent: copy.agentLabel,
    tool: copy.toolLabel,
  };

  const handleStageSelect = (stageKey: StageKey) => {
    const indices = stageIndicesByKey[stageKey];
    if (indices.length === 0) {
      return;
    }
    setActiveIndex(indices[0]);
    setIsPlaying(false);
  };

  const stageProgress = (stageKey: StageKey) => {
    const indices = stageIndicesByKey[stageKey];
    if (indices.length === 0) {
      return 0;
    }
    const completed = indices.filter((index) => index <= activeIndex).length;
    return (completed / indices.length) * 100;
  };

  const conversationLabel = lang === "zh" ? "对话演示" : "Conversation";
  const actionLogLabel = lang === "zh" ? "行动日志" : "Action log";

  return (
    <Card className="h-full bg-card/70 shadow-sm backdrop-blur">
      <CardHeader className="space-y-4 border-b border-border/60 pb-4">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2 text-sm">
              <Sparkles className="h-4 w-4" aria-hidden />
              {copy.title}
            </CardTitle>
            <p className="text-sm text-muted-foreground">{copy.description}</p>
          </div>
          <div className="flex items-center gap-2">
            <div className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-background/80 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
              <SquareTerminal className="h-3.5 w-3.5" aria-hidden />
              {lang === "zh" ? "自动播放" : "Auto-play"}
            </div>
            <Button
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={() => setIsPlaying((current) => !current)}
            >
              {isPlaying ? (
                <>
                  <Pause className="mr-2 h-4 w-4" aria-hidden />
                  {lang === "zh" ? "暂停" : "Pause"}
                </>
              ) : (
                <>
                  <Play className="mr-2 h-4 w-4" aria-hidden />
                  {lang === "zh" ? "播放" : "Play"}
                </>
              )}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="rounded-full"
              onClick={() => setActiveIndex(0)}
            >
              <RefreshCw className="h-4 w-4" aria-hidden />
              <span className="sr-only">Reset</span>
            </Button>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-3">
          {stages.map((stage) => (
            <button
              key={stage.key}
              type="button"
              onClick={() => handleStageSelect(stage.key)}
              className={cn(
                "group flex flex-col gap-1 rounded-xl border bg-background/70 px-3 py-2 text-left transition hover:-translate-y-0.5 hover:border-foreground/50",
                stage.key === activeStage ? "border-foreground/60 shadow-[0_0_0_1px_rgba(16,185,129,0.2)]" : "border-border/70",
              )}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="text-xs font-semibold text-foreground">{stage.label}</div>
                <span
                  className={cn("h-2 w-2 rounded-full bg-gradient-to-r shadow-sm", stage.accent)}
                  aria-hidden
                />
              </div>
              <div className="text-[11px] text-muted-foreground">{stage.summary}</div>
              <div className="relative mt-2 h-1.5 overflow-hidden rounded-full bg-border/70">
                <div
                  className={cn("absolute inset-y-0 left-0 rounded-full bg-gradient-to-r", stage.accent)}
                  style={{ width: `${stageProgress(stage.key)}%` }}
                  aria-hidden
                />
              </div>
            </button>
          ))}
        </div>
      </CardHeader>

      <CardContent className="space-y-5">
        <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-background/70">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(99,102,241,0.06),transparent_35%),radial-gradient(circle_at_80%_10%,rgba(34,211,238,0.08),transparent_35%)]" />
          <div className="relative flex items-center justify-between px-4 py-3 text-xs font-semibold text-muted-foreground">
            <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/80 px-2 py-1">
              <PlayCircle className="h-3.5 w-3.5 text-emerald-500" aria-hidden />
              {conversationLabel}
            </div>
            <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/80 px-2 py-1">
              <Badge variant="secondary" className="bg-gradient-to-r from-emerald-500/80 via-teal-500/80 to-cyan-500/80 text-background">
                {activeStageCopy.label}
              </Badge>
              <span className="text-[11px] text-muted-foreground">
                {Math.round(progress)}%
              </span>
            </div>
          </div>

          <ScrollArea className="relative h-[360px] px-4 pb-5">
            <div className="space-y-3">
              {visibleTurns.map((turn, index) => (
                <ChatBubble
                  key={`${turn.role}-${index}`}
                  turn={turn}
                  stage={stages.find((item) => item.key === turn.stage)}
                  roleLabels={roleLabels}
                  isActive={index === activeIndex}
                />
              ))}
            </div>
          </ScrollArea>
        </div>

        <div className="grid gap-4 lg:grid-cols-[1.05fr,0.95fr]">
          <div className="rounded-2xl border border-border/70 bg-background/70">
            <div className="flex items-center justify-between px-4 py-3 text-xs font-semibold text-foreground">
              <span>{actionLogLabel}</span>
              <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/80 px-2 py-1 text-[11px] text-muted-foreground">
                <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                {copy.evidenceLabel}
              </div>
            </div>
            <div className="divide-y divide-border/70">
              {actionLog.map((turn, index) => (
                <EvidenceRow
                  key={`${turn.role}-${index}`}
                  turn={turn}
                  stage={stages.find((item) => item.key === turn.stage)}
                  roleLabels={roleLabels}
                  lang={lang}
                />
              ))}
            </div>
          </div>

          <div className="rounded-2xl border border-border/70 bg-background/70 p-4">
            <div className="flex items-center justify-between text-xs font-semibold text-muted-foreground">
              <span>{lang === "zh" ? "阶段对齐" : "Stage alignment"}</span>
              <div className="inline-flex items-center gap-1 rounded-full border border-border/70 bg-card/80 px-2 py-1">
                <ArrowRight className="h-3.5 w-3.5 text-emerald-500" aria-hidden />
                {lang === "zh" ? "可跳转" : "Jumpable"}
              </div>
            </div>
            <div className="mt-3 space-y-2">
              {stages.map((stage) => (
                <div
                  key={stage.key}
                  className={cn(
                    "flex items-center justify-between gap-3 rounded-xl border bg-background/70 px-3 py-2",
                    stage.key === activeStage ? "border-foreground/60" : "border-border/70",
                  )}
                >
                  <div>
                    <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                      <Badge
                        variant="secondary"
                        className={cn("bg-gradient-to-r text-background", stage.accent)}
                      >
                        {stage.label}
                      </Badge>
                      <span className="text-xs text-muted-foreground">{stage.summary}</span>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => handleStageSelect(stage.key)}
                    className="inline-flex items-center gap-1 rounded-full border border-border/70 px-2 py-1 text-[11px] font-semibold text-foreground transition hover:border-foreground/60"
                  >
                    {lang === "zh" ? "跳转" : "Jump"}
                    <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function ChatBubble({
  turn,
  roleLabels,
  isActive,
  stage,
}: {
  turn: ChatTurn;
  roleLabels: Record<ChatTurn["role"], string>;
  isActive: boolean;
  stage?: StageCopy;
}) {
  const isUser = turn.role === "user";
  const isTool = turn.role === "tool";

  return (
    <div
      className={cn(
        "flex items-start gap-3",
        isUser ? "justify-end text-right" : "justify-start",
      )}
    >
      {!isUser && <MessageAvatar label={roleLabels[turn.role]} role={turn.role} accent={stage?.accent} />}

      <div
        className={cn(
          "max-w-[640px] space-y-2 rounded-2xl border px-4 py-3 text-sm shadow-sm transition",
          isUser
            ? "ml-auto border-transparent bg-gradient-to-br from-emerald-500 via-sky-500 to-indigo-500 text-white shadow-[0_10px_40px_-20px_rgba(59,130,246,0.7)]"
            : "border-border/70 bg-background/85",
          isTool && "font-mono text-[12px]",
          isActive && "ring-2 ring-emerald-200 ring-offset-2 ring-offset-background",
        )}
      >
        <div className="flex items-center gap-2 text-[11px] font-semibold">
          <span
            className={cn(
              "rounded-full px-2 py-0.5",
              isUser ? "bg-white/20 text-white" : "bg-border/70 text-foreground",
            )}
          >
            {roleLabels[turn.role]}
          </span>
          {stage ? (
            <Badge
              variant="secondary"
              className={cn("bg-gradient-to-r text-background", stage.accent)}
            >
              {stage.label}
            </Badge>
          ) : null}
        </div>

        <div className={cn("leading-relaxed", isUser ? "text-white/95" : "text-foreground/90")}>
          {turn.content}
        </div>
      </div>

      {isUser && <MessageAvatar label={roleLabels[turn.role]} role={turn.role} accent={stage?.accent} />}
    </div>
  );
}

function MessageAvatar({
  label,
  role,
  accent,
}: {
  label: string;
  role: ChatTurn["role"];
  accent?: string;
}) {
  return (
    <div
      className={cn(
        "mt-0.5 inline-flex h-9 w-9 items-center justify-center rounded-full border border-border/70 bg-card/80 text-[11px] font-semibold text-foreground shadow-sm",
        accent && "bg-gradient-to-br text-background",
        accent,
      )}
    >
      {role === "user" ? <User className="h-4 w-4" aria-hidden /> : <SquareTerminal className="h-4 w-4" aria-hidden />}
      <span className="sr-only">{label}</span>
    </div>
  );
}

function EvidenceRow({
  turn,
  stage,
  lang,
  roleLabels,
}: {
  turn: ChatTurn;
  stage?: StageCopy;
  lang: HomeLang;
  roleLabels: Record<ChatTurn["role"], string>;
}) {
  const roleLabel = roleLabels[turn.role];
  return (
    <div className="grid grid-cols-[auto,1fr] gap-3 px-4 py-3">
      <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-border/70 bg-card/80 text-xs font-semibold text-foreground">
        {roleLabel}
      </div>
      <div className="space-y-2">
        <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
          {stage ? (
            <Badge
              variant="secondary"
              className={cn("bg-gradient-to-r text-background", stage.accent)}
            >
              {stage.label}
            </Badge>
          ) : null}
          <span className="text-muted-foreground">{lang === "zh" ? "最新证据" : "Latest evidence"}</span>
        </div>
        <div className="rounded-xl border border-border/70 bg-background/80 px-3 py-2 text-sm leading-relaxed text-foreground/90">
          {turn.content}
        </div>
      </div>
    </div>
  );
}
