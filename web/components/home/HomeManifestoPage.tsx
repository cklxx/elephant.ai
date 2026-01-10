import Link from "next/link";
import { Suspense } from "react";
import { BookOpenText, Layers, PlayCircle, ShieldCheck, Sparkles } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { Header } from "@/components/layout";
import { PageContainer } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

import type { HomeLang } from "./types";

type HomeCopy = {
  badge: string;
  title: string;
  subtitle: string;
  actions: {
    primary: string;
  };
};

const copy: Record<HomeLang, HomeCopy> = {
  zh: {
    badge: "elephant.ai · proactive agent",
    title: "把任务当成项目来交付",
    subtitle: "首页与 Console 同框：主动 Agent 先对齐目标、澄清边界，再把执行与证据并排交付。",
    actions: {
      primary: "进入控制台",
    },
  },
  en: {
    badge: "elephant.ai · proactive agent",
    title: "Deliver tasks like real projects",
    subtitle:
      "Homepage meets the Console: proactive agents align goals, clarify guardrails, and ship evidence alongside execution.",
    actions: {
      primary: "Open console",
    },
  },
};

type HighlightCopy = {
  title: string;
  body: string;
  accent: string;
  icon: LucideIcon;
};

const highlightCopy: Record<HomeLang, HighlightCopy[]> = {
  zh: [
    {
      title: "先对齐，再行动",
      body: "目标、依赖与风险先明示，执行才高效。",
      accent: "from-amber-200/70 via-orange-200/50 to-rose-200/40",
      icon: Sparkles,
    },
    {
      title: "结果可验证",
      body: "每一步附带证据与摘要，交付可追溯。",
      accent: "from-emerald-200/60 via-lime-200/40 to-amber-200/40",
      icon: ShieldCheck,
    },
    {
      title: "全过程透明",
      body: "时间线、工具调用与阶段成果一目了然。",
      accent: "from-sky-200/70 via-teal-200/50 to-emerald-200/40",
      icon: Layers,
    },
  ],
  en: [
    {
      title: "Align, then act",
      body: "Make goals, dependencies, and risks explicit before execution.",
      accent: "from-amber-200/70 via-orange-200/50 to-rose-200/40",
      icon: Sparkles,
    },
    {
      title: "Evidence-ready output",
      body: "Every step ships with proof and a crisp summary.",
      accent: "from-emerald-200/60 via-lime-200/40 to-amber-200/40",
      icon: ShieldCheck,
    },
    {
      title: "Full traceability",
      body: "Timelines, tool calls, and milestones stay visible end-to-end.",
      accent: "from-sky-200/70 via-teal-200/50 to-emerald-200/40",
      icon: Layers,
    },
  ],
};

type ManifestoCopy = {
  badge: string;
  title: string;
  lede: string;
  points: { title: string; body: string; accent: string }[];
  closing: string;
};

const manifestoCopy: Record<HomeLang, ManifestoCopy> = {
  zh: {
    badge: "纯文本 · 方法论",
    title: "主动式 Agent 的交付宣言",
    lede: "不是对话，而是交付：主动 Agent 先构建任务模型，再把执行过程和证据整合呈现。",
    points: [
      {
        title: "把意图翻译成计划",
        body: "先拆解目标、约束与依赖，形成可执行的路径图。",
        accent: "from-emerald-400 via-lime-400 to-amber-400",
      },
      {
        title: "用澄清降低返工",
        body: "主动提出关键问题，提前暴露风险和阻塞点。",
        accent: "from-amber-400 via-orange-400 to-rose-400",
      },
      {
        title: "用证据证明完成",
        body: "每个步骤都留下可检索的产出，便于验收与复盘。",
        accent: "from-sky-400 via-teal-400 to-emerald-400",
      },
    ],
    closing: "慢即快：少一次来回确认，多一次真实结果与证据。",
  },
  en: {
    badge: "Plain text · methodology",
    title: "A delivery manifesto for proactive agents",
    lede: "Not just chat—delivery. Proactive agents model the task first, then surface execution and evidence together.",
    points: [
      {
        title: "Translate intent into a plan",
        body: "Unpack goals, constraints, and dependencies into an executable path.",
        accent: "from-emerald-400 via-lime-400 to-amber-400",
      },
      {
        title: "Clarify to cut rework",
        body: "Ask the hard questions early and surface risks before they land.",
        accent: "from-amber-400 via-orange-400 to-rose-400",
      },
      {
        title: "Prove completion with evidence",
        body: "Leave inspectable outputs at every step for validation and handoff.",
        accent: "from-sky-400 via-teal-400 to-emerald-400",
      },
    ],
    closing: "Slow is fast: fewer loops, more real outcomes with proof.",
  },
};

type SlogCopy = {
  badge: string;
  title: string;
  points: { title: string; body: string }[];
};

const slogCopy: Record<HomeLang, SlogCopy> = {
  zh: {
    badge: "slog · 可观测性",
    title: "成本、token、节省时间一眼看清",
    points: [
      {
        title: "成本可视化",
        body: "每次运行的成本拆解直达日志，避免隐性消耗。",
      },
      {
        title: "token 账本",
        body: "请求与响应的 token 统计清晰列示，便于优化策略。",
      },
      {
        title: "效率收益",
        body: "对比人工与自动化用时，让节省时间可衡量。",
      },
    ],
  },
  en: {
    badge: "slog · observability",
    title: "Cost, tokens, time saved—measured in slog",
    points: [
      {
        title: "Cost clarity",
        body: "Per-run cost breakdowns land in the logs with no blind spots.",
      },
      {
        title: "Token ledger",
        body: "Prompt/response token counts stay visible for tuning.",
      },
      {
        title: "Efficiency gains",
        body: "Compare manual vs. automated runtime to quantify savings.",
      },
    ],
  },
};

type VideoCopy = {
  title: string;
  body: string;
  note: string;
};

const videoCopy: Record<HomeLang, VideoCopy> = {
  zh: {
    title: "主动 Agent 演示视频将放在这里",
    body: "这块预留给真实录屏：从澄清依赖到执行交付，全流程可见。",
    note: "不做模拟对话，留白给真实上下文与产出。",
  },
  en: {
    title: "Proactive agent demo video will live here",
    body: "Reserved for a real recording: clarify dependencies, execute, and ship evidence.",
    note: "No simulated chat—save the space for real context and outputs.",
  },
};

function LanguageToggle({ lang, className }: { lang: HomeLang; className?: string }) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-2 rounded-full border border-foreground/15 bg-white/70 px-2 py-1 text-xs font-semibold text-foreground/70 shadow-sm",
        className,
      )}
    >
      <Link
        href="/zh"
        className={cn(
          "rounded-full px-3 py-1 transition",
          lang === "zh"
            ? "bg-foreground text-background"
            : "text-foreground/70 hover:text-foreground",
        )}
      >
        中文
      </Link>
      <span aria-hidden>·</span>
      <Link
        href="/"
        className={cn(
          "rounded-full px-3 py-1 transition",
          lang === "en"
            ? "bg-foreground text-background"
            : "text-foreground/70 hover:text-foreground",
        )}
      >
        EN
      </Link>
    </div>
  );
}

function Hero({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="relative overflow-hidden rounded-[32px] border border-foreground/10 bg-[#fff8e6] p-6 shadow-[0_20px_80px_-60px_rgba(15,23,42,0.45)]">
      <div className="pointer-events-none absolute -right-24 top-6 h-44 w-44 rounded-full bg-emerald-200/60 blur-3xl" aria-hidden />
      <div className="pointer-events-none absolute -left-16 bottom-6 h-36 w-36 rounded-full bg-amber-200/70 blur-3xl" aria-hidden />
      <div className="relative space-y-5">
        <div className="inline-flex items-center gap-2 rounded-full border border-foreground/15 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-foreground/70">
          <Sparkles className="h-3.5 w-3.5" aria-hidden />
          {t.badge}
        </div>

        <div className="space-y-3">
          <h1 className="text-3xl font-semibold leading-tight tracking-tight text-foreground sm:text-4xl">
            {t.title}
          </h1>
          <p className="max-w-2xl text-base leading-relaxed text-foreground/70 sm:text-lg">
            {t.subtitle}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Link href="/conversation">
            <Button className="rounded-full bg-foreground text-background shadow-sm hover:bg-foreground/90">
              <PlayCircle className="mr-2 h-5 w-5" aria-hidden />
              {t.actions.primary}
            </Button>
          </Link>
          <LanguageToggle lang={lang} className="sm:hidden" />
        </div>
      </div>
    </div>
  );
}

function Highlights({ lang }: { lang: HomeLang }) {
  const highlights = highlightCopy[lang];
  return (
    <div className="grid gap-3 sm:grid-cols-3">
      {highlights.map((item) => {
        const Icon = item.icon;
        return (
          <div
            key={item.title}
            className="relative overflow-hidden rounded-2xl border border-foreground/10 bg-white/80 p-4 shadow-sm"
          >
            <div className={cn("absolute inset-0 bg-gradient-to-br", item.accent)} aria-hidden />
            <div className="relative space-y-3">
              <div className="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-foreground/15 bg-white/90 text-foreground shadow-sm">
                <Icon className="h-4 w-4" aria-hidden />
              </div>
              <div className="space-y-1">
                <p className="text-sm font-semibold text-foreground">{item.title}</p>
                <p className="text-xs leading-relaxed text-foreground/70">{item.body}</p>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function ManifestoArticle({ lang }: { lang: HomeLang }) {
  const manifesto = manifestoCopy[lang];
  return (
    <article className="space-y-4 rounded-3xl border border-foreground/10 bg-white/80 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-foreground/15 bg-white/90 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-foreground/70">
        <BookOpenText className="h-3.5 w-3.5" aria-hidden />
        {manifesto.badge}
      </div>

      <div className="space-y-2">
        <h2 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
          {manifesto.title}
        </h2>
        <p className="text-sm leading-relaxed text-foreground/70 sm:text-base">
          {manifesto.lede}
        </p>
      </div>

      <div className="space-y-3">
        {manifesto.points.map((point) => (
          <div
            key={point.title}
            className="rounded-2xl border border-foreground/10 bg-white/80 px-4 py-3 shadow-sm"
          >
            <div className="flex items-center gap-2">
              <span
                className={cn("h-2.5 w-2.5 rounded-full bg-gradient-to-r", point.accent)}
                aria-hidden
              />
              <p className="text-sm font-semibold text-foreground">{point.title}</p>
            </div>
            <p className="mt-2 text-sm leading-relaxed text-foreground/70">{point.body}</p>
          </div>
        ))}
      </div>

      <p className="text-sm font-semibold text-foreground sm:text-base">{manifesto.closing}</p>
    </article>
  );
}

function SlogPanel({ lang }: { lang: HomeLang }) {
  const slog = slogCopy[lang];
  return (
    <section className="space-y-4 rounded-3xl border border-foreground/10 bg-white/80 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-foreground/15 bg-white/90 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-foreground/70">
        <Layers className="h-3.5 w-3.5" aria-hidden />
        {slog.badge}
      </div>
      <div className="space-y-2">
        <h2 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
          {slog.title}
        </h2>
      </div>
      <div className="grid gap-3 sm:grid-cols-3">
        {slog.points.map((point) => (
          <div
            key={point.title}
            className="rounded-2xl border border-foreground/10 bg-white/80 px-4 py-3 shadow-sm"
          >
            <p className="text-sm font-semibold text-foreground">{point.title}</p>
            <p className="mt-1 text-xs leading-relaxed text-foreground/70">{point.body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function VideoPlaceholder({ lang }: { lang: HomeLang }) {
  const copy = videoCopy[lang];

  return (
    <div className="rounded-3xl border border-dashed border-foreground/20 bg-white/70 p-6 shadow-sm">
      <div className="flex items-start gap-4">
        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-foreground/10 text-foreground">
          <PlayCircle className="h-6 w-6" aria-hidden />
        </div>
        <div className="space-y-2">
          <p className="text-lg font-semibold text-foreground">{copy.title}</p>
          <p className="text-sm leading-relaxed text-foreground/70">{copy.body}</p>
          <p className="text-xs font-semibold uppercase tracking-wide text-foreground/60">
            {copy.note}
          </p>
        </div>
      </div>
    </div>
  );
}

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  const heroCopy = copy[lang];

  return (
    <div className="relative min-h-screen bg-[#f3f0d6] text-foreground">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(900px_circle_at_15%_12%,rgba(250,230,180,0.8),transparent_55%),radial-gradient(900px_circle_at_85%_10%,rgba(190,240,210,0.6),transparent_55%),radial-gradient(900px_circle_at_50%_90%,rgba(255,210,180,0.45),transparent_55%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-32 bg-gradient-to-b from-white/80 to-transparent" />
      <PageContainer className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-6 px-4 pb-12 pt-6 sm:px-6 lg:px-10 lg:pb-16 lg:pt-10">
        <Suspense fallback={<div className="h-[60px]" />}>
          <Header
            title={heroCopy.title}
            subtitle={heroCopy.subtitle}
            actionsSlot={
              <div className="flex items-center gap-2">
                <LanguageToggle lang={lang} className="hidden sm:inline-flex" />
                <Link href="/conversation">
                  <Button size="sm" className="rounded-full bg-foreground text-background shadow-sm">
                    <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
                    {heroCopy.actions.primary}
                  </Button>
                </Link>
              </div>
            }
          />
        </Suspense>

        <div className="grid min-h-0 gap-6 lg:grid-cols-[0.95fr,1.05fr]">
          <div className="flex flex-col gap-4">
            <Hero lang={lang} />
            <Highlights lang={lang} />
            <VideoPlaceholder lang={lang} />
          </div>
          <div className="flex flex-col gap-4">
            <SlogPanel lang={lang} />
            <ManifestoArticle lang={lang} />
          </div>
        </div>
      </PageContainer>
    </div>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
