import type { ComponentType, SVGProps } from "react";
import Link from "next/link";
import { ArrowRight, CheckCircle2, Layers, LineChart, PlayCircle, Sparkles, Timer } from "lucide-react";

import { PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";

type LogTone = "info" | "primary" | "muted" | "success";

function HeroSection() {
  return (
    <div className="relative isolate flex min-h-screen items-center overflow-hidden bg-gradient-to-br from-slate-800 via-slate-700 to-slate-900 text-white">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_12%_20%,rgba(59,130,246,0.24),transparent_32%),radial-gradient(circle_at_88%_18%,rgba(16,185,129,0.2),transparent_32%),radial-gradient(circle_at_30%_86%,rgba(148,163,184,0.2),transparent_40%)]" />
        <div className="absolute inset-0 bg-[linear-gradient(120deg,rgba(15,23,42,0.58),rgba(15,23,42,0.42))]" />
      </div>
      <div className="relative mx-auto flex w-full max-w-screen-2xl flex-col gap-8 px-6 py-12 sm:px-10 lg:flex-row lg:items-center lg:gap-12 lg:px-16">
        <div className="space-y-6 lg:pr-4">
          <div className="inline-flex items-center gap-2 rounded-full border border-white/25 bg-white/10 px-3 py-1 text-[11px] font-semibold text-white/80 backdrop-blur">
            Standard console homepage
            <Sparkles className="h-4 w-4" />
          </div>
          <div className="space-y-4">
            <h1 className="text-3xl font-semibold leading-tight sm:text-4xl lg:text-[44px] drop-shadow-[0_8px_20px_rgba(0,0,0,0.45)]">
              Run, watch, and replay every agent task
            </h1>
            <p className="max-w-2xl text-base text-slate-100/90 sm:text-lg drop-shadow-[0_4px_12px_rgba(0,0,0,0.45)]">
              A dependable landing for operators: start a console session, follow live tool output, and jump back to any saved timeline without extra setup.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <Link href="/conversation">
              <Button className="group rounded-xl border border-cyan-200/50 bg-cyan-400 px-5 py-3 text-sm font-semibold text-slate-950 shadow-[0_18px_40px_-18px_rgba(6,182,212,0.9)] transition hover:-translate-y-0.5 hover:border-cyan-50 hover:bg-cyan-300">
                <PlayCircle className="h-5 w-5" />
                Start console
                <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-0.5" />
              </Button>
            </Link>
            <Link href="/sessions">
              <Button className="rounded-xl border border-white/35 bg-white/15 px-5 py-3 text-sm font-semibold text-white backdrop-blur transition hover:-translate-y-0.5 hover:border-white/55 hover:bg-white/25">
                View sessions
              </Button>
            </Link>
            <Link
              href="/login"
              className="text-sm font-semibold text-slate-100 underline-offset-4 transition hover:text-white hover:underline"
            >
              Team login
            </Link>
          </div>
          <div className="grid gap-3 sm:grid-cols-3">
            <StatPill label="Live SSE stream" value="Realtime" />
            <StatPill label="Timeline replays" value="Saved & recent" />
            <StatPill label="Team controls" value="Plan + cancel" />
          </div>
        </div>
        <div className="w-full max-w-xl lg:max-w-lg">
          <LivePreviewCard />
        </div>
      </div>
    </div>
  );
}

function StatPill({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-white/15 bg-white/10 px-4 py-3 text-sm text-white backdrop-blur">
      <div className="text-xs text-white/70">{label}</div>
      <div className="mt-1 flex items-center gap-2 text-base font-semibold">
        {value}
        <div className="h-2 w-2 animate-pulse rounded-full bg-emerald-300" />
      </div>
    </div>
  );
}

function LivePreviewCard() {
  const logLines = [
    {
      icon: Sparkles,
      label: "Plan",
      text: "Research console workspace scaffolded",
      tone: "info" as const,
    },
    {
      icon: Layers,
      label: "Tool",
      text: "file_read -> web/app/conversation/page.tsx",
      tone: "primary" as const,
    },
    {
      icon: Timer,
      label: "Working",
      text: "Running quick audit on session timeline",
      tone: "muted" as const,
    },
    {
      icon: CheckCircle2,
      label: "Complete",
      text: "Console view ready - open conversation to continue",
      tone: "success" as const,
    },
  ];

  return (
    <div className="rounded-[24px] border border-white/15 bg-white/10 p-5 text-white shadow-[0_24px_60px_-36px_rgba(0,0,0,0.8)] backdrop-blur">
      <div className="flex items-center justify-between text-xs font-semibold text-white/80">
        <span>Live console feed</span>
        <span className="flex items-center gap-2">
          <div className="h-2 w-2 animate-pulse rounded-full bg-emerald-300" />
          Active
        </span>
      </div>
      <div className="mt-3 rounded-2xl border border-white/10 bg-black/40 p-3 font-mono text-[13px] leading-6 shadow-inner shadow-black/20">
        {logLines.map((line) => (
          <LogLine key={line.label} {...line} />
        ))}
      </div>
      <div className="mt-3 flex items-center gap-2 text-xs text-white/80">
        <LineChart className="h-4 w-4" />
        Streaming SSE / Auto-reconnect / Timeline-friendly
      </div>
    </div>
  );
}

function LogLine({
  icon: Icon,
  label,
  text,
  tone,
}: {
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  label: string;
  text: string;
  tone: LogTone;
}) {
  const toneClasses: Record<LogTone, string> = {
    info: "text-indigo-200",
    primary: "text-cyan-200",
    muted: "text-slate-200/80",
    success: "text-emerald-200",
  };

  return (
    <div className="flex items-center gap-3 rounded-xl px-2 py-1.5 hover:bg-white/5">
      <div className="flex items-center gap-1.5 text-[13px] font-semibold">
        <Icon className="h-4 w-4" />
        <span className={toneClasses[tone]}>{label}</span>
      </div>
      <div className="text-[13px] text-slate-100">{text}</div>
    </div>
  );
}

export default function HomePage() {
  return (
    <PageShell padding="none">
      <HeroSection />
    </PageShell>
  );
}
