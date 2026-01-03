"use client";

// Design refresh guided by @anthropics/skills/frontend-design.

import { useEffect, useMemo, useState } from "react";
import {
  ArrowRight,
  ListChecks,
  Pause,
  Play,
  PlayCircle,
  RefreshCw,
  Sparkles,
  Target,
  SquareTerminal,
  User,
  Zap,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

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

const stageIcons: Record<StageKey, LucideIcon> = {
  plan: Target,
  clearify: ListChecks,
  react: Zap,
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
  const ActiveStageIcon = stageIcons[activeStage];
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

  const conversationLabel = lang === "zh" ? "对话演示" : "Conversation";
  const actionLogLabel = lang === "zh" ? "行动日志" : "Action log";

  return (
    <Card className="h-full bg-card/70 shadow-sm backdrop-blur">
      <CardHeader className="space-y-4 border-b border-border/60 pb-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2 text-sm">
              <Sparkles className="h-4 w-4" aria-hidden />
              {copy.title}
            </CardTitle>
            <p className="text-sm text-muted-foreground">{copy.description}</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
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

      </CardHeader>

      <CardContent className="space-y-5">
        <div className="relative overflow-hidden rounded-2xl border border-border/70 bg-background/70">
          <div className="absolute inset-0 bg-[radial-gradient(circle_at_20%_20%,rgba(99,102,241,0.06),transparent_35%),radial-gradient(circle_at_80%_10%,rgba(34,211,238,0.08),transparent_35%)]" />
          <div className="relative flex flex-wrap items-center justify-between gap-3 px-4 py-3 text-xs font-semibold text-muted-foreground">
            <div className="inline-flex items-center gap-3 rounded-full border border-border/70 bg-card/80 px-3 py-1.5 shadow-sm">
              <div className="relative inline-flex h-8 w-8 items-center justify-center overflow-hidden rounded-full border border-border/70 bg-background/90">
                <div className="absolute inset-0 bg-gradient-to-br from-emerald-500/30 via-sky-500/20 to-indigo-500/30" aria-hidden />
                <PlayCircle className="relative h-4 w-4 text-emerald-500" aria-hidden />
              </div>
              <span className="text-foreground">{conversationLabel}</span>
            </div>
            <div className="inline-flex items-center gap-3 rounded-full border border-border/70 bg-card/80 px-3 py-1.5 shadow-sm">
              <div className="relative inline-flex h-8 w-8 items-center justify-center overflow-hidden rounded-full border border-border/70 bg-background/90">
                <div
                  className={cn("absolute inset-0 bg-gradient-to-br opacity-70", activeStageCopy.accent)}
                  aria-hidden
                />
                <ActiveStageIcon className="relative h-4 w-4 text-foreground" aria-hidden />
              </div>
              <Badge variant="secondary" className="bg-gradient-to-r from-emerald-500/80 via-teal-500/80 to-cyan-500/80 text-background">
                {activeStageCopy.label}
              </Badge>
              <span className="rounded-full bg-border/70 px-2 py-1 text-[11px] text-muted-foreground">
                {Math.round(progress)}%
              </span>
            </div>
          </div>

          <ScrollArea className="relative h-[320px] px-4 pb-5 sm:h-[360px] lg:h-[420px]">
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
              <div className="inline-flex items-center gap-3">
                <div className="relative inline-flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border border-border/70 bg-card/80 shadow-sm">
                  <div className="absolute inset-0 bg-gradient-to-br from-emerald-500/25 via-sky-500/20 to-indigo-500/25" aria-hidden />
                  <Sparkles className="relative h-4 w-4 text-foreground" aria-hidden />
                </div>
                <span className="text-sm font-semibold text-foreground">{actionLogLabel}</span>
              </div>
              <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/80 px-3 py-1.5 text-[11px] font-semibold text-muted-foreground shadow-sm">
                <ArrowRight className="h-3.5 w-3.5 text-emerald-500" aria-hidden />
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
              {stages.map((stage) => {
                const StageIcon = stageIcons[stage.key];
                return (
                  <div
                    key={stage.key}
                    className={cn(
                      "flex items-center justify-between gap-4 rounded-2xl border bg-background/70 px-3 py-3",
                      stage.key === activeStage ? "border-foreground/60 shadow-[0_6px_28px_-20px_rgba(16,185,129,0.6)]" : "border-border/70",
                    )}
                  >
                    <div className="flex items-center gap-3">
                      <div className="relative inline-flex h-11 w-11 items-center justify-center overflow-hidden rounded-2xl border border-border/70 bg-card/80 shadow-sm">
                        <div
                          className={cn("absolute inset-0 bg-gradient-to-br opacity-70", stage.accent)}
                          aria-hidden
                        />
                        <div className="relative inline-flex h-8 w-8 items-center justify-center rounded-xl bg-background/95 text-foreground shadow-sm">
                          <StageIcon className="h-4 w-4" aria-hidden />
                        </div>
                      </div>
                      <div className="space-y-1">
                        <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-foreground">
                          <Badge
                            variant="secondary"
                            className={cn("bg-gradient-to-r text-background", stage.accent)}
                          >
                            {stage.label}
                          </Badge>
                          <span className="rounded-full bg-border/70 px-2 py-1 text-[11px] text-muted-foreground">
                            {lang === "zh" ? "在播" : "In flow"}
                          </span>
                        </div>
                        <div className="text-xs text-muted-foreground">{stage.summary}</div>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleStageSelect(stage.key)}
                      className="inline-flex items-center gap-1 rounded-full border border-border/70 px-3 py-1.5 text-[11px] font-semibold text-foreground transition hover:border-foreground/60"
                    >
                      {lang === "zh" ? "跳转" : "Jump"}
                      <ArrowRight className="h-3.5 w-3.5" aria-hidden />
                    </button>
                  </div>
                );
              })}
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
  const isAgent = turn.role === "agent";
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
          "max-w-[640px] rounded-3xl border px-5 py-4 text-sm shadow-sm transition",
          !isAgent && "space-y-2",
          isUser
            ? "ml-auto border-transparent bg-gradient-to-br from-emerald-500 via-sky-500 to-indigo-500 text-white shadow-[0_16px_55px_-28px_rgba(59,130,246,0.9)]"
            : "border-border/60 bg-background/90 backdrop-blur-sm",
          isTool && "font-mono text-[12px]",
          isActive && "shadow-[0_18px_40px_-28px_rgba(16,185,129,0.8)] ring-2 ring-emerald-200 ring-offset-2 ring-offset-background",
        )}
      >
        {isAgent ? (
          <div className="flex min-w-0 flex-nowrap items-center gap-2 text-sm leading-relaxed">
            <span className="shrink-0 rounded-full bg-border/70 px-2 py-0.5 text-[11px] font-semibold text-foreground">
              {roleLabels[turn.role]}
            </span>
            {stage ? (
              <Badge
                variant="secondary"
                className={cn("bg-gradient-to-r text-background", stage.accent, "shrink-0")}
              >
                {stage.label}
              </Badge>
            ) : null}
            <span className="truncate text-foreground/90">{turn.content}</span>
          </div>
        ) : (
          <>
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
          </>
        )}
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
      className="relative mt-0.5 inline-flex h-11 w-11 items-center justify-center overflow-hidden rounded-2xl border border-border/70 bg-card/80 shadow-sm"
    >
      <div
        className={cn("absolute inset-0 bg-gradient-to-br opacity-70", accent ?? "from-border via-border to-border")}
        aria-hidden
      />
      <div className="relative inline-flex h-9 w-9 items-center justify-center rounded-xl border border-border/70 bg-background/95 text-foreground shadow-sm">
        {role === "user" ? (
          <User className="h-4 w-4" aria-hidden />
        ) : (
          <SquareTerminal className="h-4 w-4" aria-hidden />
        )}
      </div>
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
  const RoleIcon = turn.role === "user" ? User : turn.role === "agent" ? Sparkles : SquareTerminal;
  return (
    <div className="grid grid-cols-[auto,1fr] items-start gap-4 px-4 py-3">
      <div className="relative flex h-12 w-12 items-center justify-center overflow-hidden rounded-2xl border border-border/70 bg-card/80 shadow-sm">
        <div
          className={cn("absolute inset-0 bg-gradient-to-br opacity-70", stage?.accent ?? "from-border via-border to-border")}
          aria-hidden
        />
        <div className="relative inline-flex h-9 w-9 items-center justify-center rounded-xl border border-border/70 bg-background/95 text-foreground shadow-sm">
          <RoleIcon className="h-4 w-4" aria-hidden />
        </div>
      </div>
      <div className="space-y-2">
        <div className="flex flex-wrap items-center gap-2 text-xs font-semibold text-foreground">
          {stage ? (
            <Badge
              variant="secondary"
              className={cn("bg-gradient-to-r text-background", stage.accent)}
            >
              {stage.label}
            </Badge>
          ) : null}
          <span className="rounded-full bg-border/70 px-2 py-1 text-[11px] text-muted-foreground">
            {roleLabel}
          </span>
          <span className="text-muted-foreground">{lang === "zh" ? "最新证据" : "Latest evidence"}</span>
        </div>
        <div className="rounded-xl border border-border/70 bg-background/80 px-3 py-2 text-sm leading-relaxed text-foreground/90">
          {turn.content}
        </div>
      </div>
    </div>
  );
}
