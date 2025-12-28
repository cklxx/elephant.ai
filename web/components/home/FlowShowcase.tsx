"use client";

import { useEffect, useState } from "react";
import { ArrowRight, CheckCircle2, PlayCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

import type { HomeLang } from "./types";

export type FlowStep = {
  stage: string;
  headline: string;
  summary: string;
  highlights: string[];
  log: string[];
  accent: string;
};

export type FlowCopy = {
  title: string;
  description: string;
  timelineLabel: string;
  logLabel: string;
  liveLabel: string;
};

export function FlowShowcase({
  lang,
  copy,
  steps,
}: {
  lang: HomeLang;
  copy: FlowCopy;
  steps: FlowStep[];
}) {
  const [activeIndex, setActiveIndex] = useState(0);
  const activeStep = steps[activeIndex];

  const progress = ((activeIndex + 1) / steps.length) * 100;

  useEffect(() => {
    const id = setInterval(() => {
      setActiveIndex((current) => (current + 1) % steps.length);
    }, 3400);

    return () => clearInterval(id);
  }, [steps.length]);

  return (
    <Card className="bg-card/70 backdrop-blur">
      <CardHeader className="space-y-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <PlayCircle className="h-4 w-4" aria-hidden />
          {copy.title}
        </CardTitle>
        <p className="text-sm text-muted-foreground">{copy.description}</p>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="space-y-3">
          <div className="flex items-center justify-between text-xs font-semibold text-muted-foreground">
            <span>{copy.timelineLabel}</span>
            <div className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-background/80 px-2 py-1 text-[11px] font-semibold text-foreground">
              <PlayCircle className="h-3.5 w-3.5 text-emerald-500" aria-hidden />
              <ArrowRight className="h-3.5 w-3.5 text-muted-foreground" aria-hidden />
              <span className="text-muted-foreground">
                {lang === "zh" ? `${steps.length} 步` : `${steps.length} steps`}
              </span>
            </div>
          </div>
          <div className="grid gap-2 sm:grid-cols-3">
            {steps.map((step, index) => (
              <button
                key={step.stage}
                type="button"
                onClick={() => setActiveIndex(index)}
                className={cn(
                  "flex flex-col gap-2 rounded-xl border p-3 text-left transition",
                  index === activeIndex
                    ? "border-foreground/60 bg-background"
                    : "border-border/70 bg-background/60 hover:border-border",
                )}
              >
                <div className="flex items-center gap-2">
                  <span
                    className={cn(
                      "h-2 w-2 rounded-full",
                      index === activeIndex ? "bg-emerald-400" : "bg-muted-foreground/40",
                    )}
                  />
                  <span className="text-xs font-semibold text-foreground">{step.stage}</span>
                </div>
              <div className="text-sm font-semibold text-foreground">{step.headline}</div>
              <div className="text-xs text-muted-foreground">{step.summary}</div>
              <div
                className={cn(
                  "h-1 rounded-full bg-border/70",
                  index === activeIndex && ["bg-gradient-to-r", step.accent],
                )}
              />
            </button>
          ))}
        </div>
          <div className="relative h-1 overflow-hidden rounded-full bg-border/80">
            <div
              className="absolute inset-y-0 left-0 rounded-full bg-gradient-to-r from-emerald-400 via-sky-400 to-indigo-500"
              style={{ width: `${progress}%` }}
              aria-hidden
            />
          </div>
        </div>

        <div className="grid gap-4 lg:grid-cols-[1.1fr,0.9fr]">
          <div className="rounded-2xl border border-border/70 bg-background/60 p-4">
            <div className="flex items-center justify-between">
              <div className="text-sm font-semibold text-foreground">{activeStep.headline}</div>
              <span
                className={cn(
                  "rounded-full px-2 py-1 text-[11px] font-semibold text-background shadow-sm",
                  "bg-gradient-to-r",
                  activeStep.accent,
                )}
              >
                {activeStep.stage}
              </span>
            </div>
            <div className="mt-3 space-y-2 text-sm text-muted-foreground">
              {activeStep.highlights.map((line) => (
                <div key={line} className="flex items-start gap-2">
                  <CheckCircle2 className="mt-0.5 h-4 w-4 text-emerald-500" aria-hidden />
                  <span>{line}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="rounded-2xl border border-border/70 bg-background/60 p-4">
            <div className="flex items-center justify-between">
              <div className="text-sm font-semibold text-foreground">{copy.logLabel}</div>
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 px-2 py-1 text-[11px] font-semibold text-emerald-500">
                <PlayCircle className="h-3.5 w-3.5" aria-hidden />
                {copy.liveLabel}
              </span>
            </div>
            <div className="mt-3 space-y-2 font-mono text-[11px] leading-relaxed text-foreground/90">
              {activeStep.log.map((line) => (
                <div key={line} className="flex items-start gap-2">
                  <span className="mt-1 h-1.5 w-1.5 rounded-full bg-emerald-400" aria-hidden />
                  <span className="whitespace-pre-wrap">{line}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
