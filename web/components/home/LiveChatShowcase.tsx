"use client";

// Design refresh guided by @anthropics/skills/frontend-design.

import { useEffect, useState } from "react";
import {
  ArrowRight,
  Pause,
  PlayCircle,
  RefreshCw,
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
    if (!isPlaying) return undefined;

    const id = setInterval(() => {
      setActiveIndex((current) => (current + 1) % script.length);
    }, 3200);

    return () => clearInterval(id);
  }, [isPlaying, script.length]);

  const activeStage = script[activeIndex]?.stage ?? stages[0].key;
  const progress = ((activeIndex + 1) / script.length) * 100;
  const visibleTurns = script.slice(0, activeIndex + 1);

  return (
    <Card className="bg-card/70 backdrop-blur">
      <CardHeader className="space-y-3">
        <div className="flex items-start justify-between gap-3">
          <div className="space-y-1">
            <CardTitle className="flex items-center gap-2 text-sm">
              <PlayCircle className="h-4 w-4" aria-hidden />
              {copy.title}
            </CardTitle>
            <p className="text-sm text-muted-foreground">{copy.description}</p>
          </div>
          <div className="flex items-center gap-2">
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
                  <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
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
            </Button>
          </div>
        </div>

        <div className="grid gap-2 sm:grid-cols-3">
          {stages.map((stage) => (
            <div
              key={stage.key}
              className={cn(
                "flex items-center justify-between gap-2 rounded-xl border bg-background/70 px-3 py-2 text-left",
                stage.key === activeStage ? "border-foreground/60" : "border-border/70",
              )}
            >
              <div>
                <div className="text-xs font-semibold text-foreground">{stage.label}</div>
                <div className="text-[11px] text-muted-foreground">{stage.summary}</div>
              </div>
              <span
                className={cn(
                  "h-2 w-2 rounded-full bg-gradient-to-r shadow-sm",
                  stage.accent,
                )}
                aria-hidden
              />
            </div>
          ))}
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="relative h-1.5 overflow-hidden rounded-full bg-border/70">
          <div
            className={cn(
              "absolute inset-y-0 left-0 rounded-full bg-gradient-to-r from-emerald-400 via-sky-400 to-indigo-500 transition-[width]",
            )}
            style={{ width: `${progress}%` }}
            aria-hidden
          />
        </div>

        <div className="grid gap-4 lg:grid-cols-[1.05fr,0.95fr]">
          <div className="rounded-2xl border border-border/70 bg-background/70 p-4">
            <div className="flex items-center justify-between text-xs font-semibold text-muted-foreground">
              <span>{copy.agentLabel}</span>
              <div className="inline-flex items-center gap-1 rounded-full border border-border/70 bg-card/70 px-2 py-1 text-[11px] text-foreground">
                <SquareTerminal className="h-3.5 w-3.5" aria-hidden />
                {lang === "zh" ? "自动播放" : "Auto-play"}
              </div>
            </div>

            <ScrollArea className="mt-3 h-[360px] pr-1">
              <div className="space-y-3">
                {visibleTurns.map((turn, index) => (
                  <ChatBubble
                    key={`${turn.role}-${index}`}
                    turn={turn}
                    stage={stages.find((item) => item.key === turn.stage)}
                    roleLabels={{
                      user: copy.userLabel,
                      agent: copy.agentLabel,
                      tool: copy.toolLabel,
                    }}
                    isActive={index === activeIndex}
                  />
                ))}
              </div>
            </ScrollArea>
          </div>

          <div className="space-y-3">
            {stages.map((stage) => (
              <div
                key={stage.key}
                className={cn(
                  "rounded-2xl border bg-background/70 p-4",
                  stage.key === activeStage
                    ? "border-foreground/60 shadow-[0_0_0_1px_rgba(16,185,129,0.25)]"
                    : "border-border/70",
                )}
              >
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                      <Badge
                        variant="secondary"
                        className={cn("bg-gradient-to-r text-background", stage.accent)}
                      >
                        {stage.label}
                      </Badge>
                      <ArrowRight className="h-4 w-4 text-muted-foreground" aria-hidden />
                      <span className="text-sm text-muted-foreground">{copy.evidenceLabel}</span>
                    </div>
                    <div className="mt-2 text-sm text-muted-foreground">{stage.summary}</div>
                  </div>
                  <div
                    className={cn(
                      "h-9 w-9 rounded-full bg-gradient-to-br p-2 text-background",
                      stage.accent,
                    )}
                  >
                    <User className="h-full w-full" aria-hidden />
                  </div>
                </div>
              </div>
            ))}
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
        "flex items-start gap-2",
        isUser ? "justify-end text-right" : "justify-start",
      )}
    >
      {!isUser && <MessageAvatar label={roleLabels[turn.role]} role={turn.role} />}

      <div
        className={cn(
          "max-w-[640px] space-y-2 rounded-2xl border px-4 py-3 text-sm shadow-sm transition",
          isUser
            ? "ml-auto border-foreground/30 bg-foreground text-background"
            : "border-border/70 bg-background/80",
          isTool && "font-mono text-[12px]",
          isActive && "ring-2 ring-emerald-200 ring-offset-2 ring-offset-background",
        )}
      >
        <div className="flex items-center gap-2 text-xs font-semibold">
          <span
            className={cn(
              "rounded-full px-2 py-0.5",
              isUser ? "bg-background/20" : "bg-border/70",
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

        <div className="leading-relaxed text-foreground/90">{turn.content}</div>
      </div>

      {isUser && <MessageAvatar label={roleLabels[turn.role]} role={turn.role} />}
    </div>
  );
}

function MessageAvatar({
  label,
  role,
}: {
  label: string;
  role: ChatTurn["role"];
}) {
  return (
    <div className="mt-0.5 inline-flex h-9 w-9 items-center justify-center rounded-full border border-border/70 bg-card/70 text-[11px] font-semibold text-foreground">
      {role === "user" ? <User className="h-4 w-4" aria-hidden /> : <SquareTerminal className="h-4 w-4" aria-hidden />}
      <span className="sr-only">{label}</span>
    </div>
  );
}
