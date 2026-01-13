"use client";

import Link from "next/link";
import { Suspense, useEffect, useRef, useState } from "react";
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
    badge: "elephant.ai · 主动代理",
    title: "把成本、token、时间节省做成看得见的体验",
    subtitle: "先澄清背景与目标，再动手拿结果：真实世界可用、过程可追踪。",
    actions: {
      primary: "进入控制台",
    },
  },
  en: {
    badge: "elephant.ai · proactive agent",
    title: "Make cost, tokens, and time saved visible",
    subtitle:
      "Clarify context and goals first, then ship real-world results with traceable steps.",
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
      title: "先澄清背景与目标",
      body: "把目标、边界与依赖讲清楚，再进入执行。",
      accent: "from-emerald-500/20 via-lime-500/20 to-amber-400/20",
      icon: Sparkles,
    },
    {
      title: "减少确认回合",
      body: "规划一次到位，减少来回确认和催促。",
      accent: "from-orange-500/20 via-amber-400/20 to-rose-400/20",
      icon: ShieldCheck,
    },
    {
      title: "获取真实世界可用结果",
      body: "交付可落地的产出，而非停在纸面描述。",
      accent: "from-teal-500/20 via-sky-400/20 to-emerald-400/20",
      icon: Layers,
    },
  ],
  en: [
    {
      title: "Clarify context and goals",
      body: "Define scope, constraints, and dependencies before execution.",
      accent: "from-emerald-500/20 via-lime-500/20 to-amber-400/20",
      icon: Sparkles,
    },
    {
      title: "Fewer confirmation loops",
      body: "Plan once, reduce back-and-forth approvals.",
      accent: "from-orange-500/20 via-amber-400/20 to-rose-400/20",
      icon: ShieldCheck,
    },
    {
      title: "Real-world results",
      body: "Deliver outputs that can be used immediately, not just talked about.",
      accent: "from-teal-500/20 via-sky-400/20 to-emerald-400/20",
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
    badge: "执行宣言 · 可观测结果",
    title: "用事实交付，而不是口头承诺",
    lede: "首页即总览：从背景、目标到产出，全部可追踪、可量化。",
    points: [
      {
        title: "先澄清，再执行",
        body: "明确背景、目标与依赖，减少后续确认与返工。",
        accent: "from-emerald-500 via-lime-500 to-amber-400",
      },
      {
        title: "可衡量的成本与时间",
        body: "成本、token 与时间节省统一可视化，收益一眼看清。",
        accent: "from-orange-500 via-amber-400 to-rose-400",
      },
      {
        title: "真实世界可用结果",
        body: "输出可直接落地的成果，而非停留在描述。",
        accent: "from-teal-500 via-sky-400 to-emerald-400",
      },
    ],
    closing: "慢即快：减少确认，把结果做实。",
  },
  en: {
    badge: "Execution manifesto · measurable outcomes",
    title: "Deliver facts, not promises",
    lede: "The homepage is the overview: context, goals, and results stay observable and measurable.",
    points: [
      {
        title: "Clarify before execution",
        body: "Lock in context, goals, and dependencies to cut rework.",
        accent: "from-emerald-500 via-lime-500 to-amber-400",
      },
      {
        title: "Measurable cost and time",
        body: "Cost, token usage, and time saved stay visible at all times.",
        accent: "from-orange-500 via-amber-400 to-rose-400",
      },
      {
        title: "Real-world usable results",
        body: "Outputs are deployable and ready to use, not just described.",
        accent: "from-teal-500 via-sky-400 to-emerald-400",
      },
    ],
    closing: "Slow is fast: fewer confirmations, more real outcomes.",
  },
};

type SlogCopy = {
  badge: string;
  title: string;
  points: { title: string; body: string }[];
};

const slogCopy: Record<HomeLang, SlogCopy> = {
  zh: {
    badge: "slog · 透明指标",
    title: "成本、token、节省时间就是这套理念",
    points: [
      {
        title: "成本一目了然",
        body: "每次运行的成本分解直达日志，避免隐性消耗。",
      },
      {
        title: "token 账本",
        body: "请求与响应的 token 统计清晰列示，方便调整策略。",
      },
      {
        title: "节省时间",
        body: "对比人工与自动化用时，让效率收益可衡量。",
      },
    ],
  },
  en: {
    badge: "slog · transparency",
    title: "Cost, tokens, time saved—this is the core idea",
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
        title: "Time saved",
        body: "Compare manual vs. automated runtime to quantify gains.",
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
    title: "演示视频即将呈现",
    body: "真实录屏展示：先澄清背景与目标，再交付可用结果。",
    note: "无模拟对话框：只保留真实上下文与产出。",
  },
  en: {
    title: "Demo video coming soon",
    body: "Reserved for a real recording: clarify context, then ship usable outcomes.",
    note: "No simulated chat—just real context and artifacts.",
  },
};

function Reveal({ children, delay = 0 }: { children: React.ReactNode; delay?: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setIsVisible(true);
          }
        });
      },
      { threshold: 0.2 },
    );

    if (ref.current) {
      observer.observe(ref.current);
    }

    return () => {
      observer.disconnect();
    };
  }, []);

  return (
    <div
      ref={ref}
      className={cn(
        "transition-all duration-700 ease-out will-change-transform",
        isVisible ? "translate-y-0 opacity-100" : "translate-y-6 opacity-0",
      )}
      style={{ transitionDelay: `${delay}ms` }}
    >
      {children}
    </div>
  );
}

function LanguageToggle({ lang, className }: { lang: HomeLang; className?: string }) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-2 rounded-full border border-border/60 bg-background/60 px-2 py-1 text-xs font-semibold text-muted-foreground shadow-sm",
        className,
      )}
    >
      <Link
        href="/zh"
        className={cn(
          "rounded-full px-3 py-1 transition",
          lang === "zh"
            ? "bg-foreground text-background"
            : "text-muted-foreground hover:text-foreground",
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
            : "text-muted-foreground hover:text-foreground",
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
    <div className="grid gap-6 rounded-[36px] border border-border/60 bg-white/90 p-8 shadow-[0_30px_80px_-50px_rgba(15,23,42,0.35)] lg:grid-cols-[1.1fr_0.9fr] lg:items-center">
      <div className="space-y-5">
        <div className="inline-flex items-center gap-2 rounded-full border border-foreground/10 bg-white/90 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
          <Sparkles className="h-3.5 w-3.5" aria-hidden />
          {t.badge}
        </div>

        <div className="space-y-3">
          <h1 className="text-3xl font-semibold leading-tight tracking-tight text-foreground sm:text-4xl">
            {t.title}
          </h1>
          <p className="max-w-2xl text-base leading-relaxed text-muted-foreground sm:text-lg">
            {t.subtitle}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Link href="/conversation">
            <Button className="rounded-full shadow-sm">
              <PlayCircle className="mr-2 h-5 w-5" aria-hidden />
              {t.actions.primary}
            </Button>
          </Link>
          <LanguageToggle lang={lang} className="sm:hidden" />
        </div>
      </div>

      <div className="relative overflow-hidden rounded-3xl border border-border/60 bg-white/90 p-5 shadow-sm">
        <div className="absolute inset-0 bg-[radial-gradient(260px_circle_at_20%_20%,rgba(52,211,153,0.28),transparent_55%),radial-gradient(320px_circle_at_85%_18%,rgba(251,191,36,0.28),transparent_60%),radial-gradient(320px_circle_at_60%_90%,rgba(45,212,191,0.22),transparent_55%)]" />
        <div className="relative space-y-4">
          <div className="rounded-2xl bg-emerald-500/10 px-4 py-3 text-sm font-semibold text-emerald-900">
            {lang === "zh" ? "执行配方卡" : "Execution recipe"}
          </div>
          <div className="grid gap-3 text-sm text-foreground">
            {[
              lang === "zh" ? "澄清背景与目标" : "Clarify context and goals",
              lang === "zh" ? "减少确认回合" : "Reduce confirmation loops",
              lang === "zh" ? "拿到可用结果" : "Ship usable results",
            ].map((item) => (
              <div
                key={item}
                className="flex items-center gap-2 rounded-xl border border-border/60 bg-white/80 px-3 py-2 shadow-sm"
              >
                <span className="h-2 w-2 rounded-full bg-emerald-500" aria-hidden />
                <span>{item}</span>
              </div>
            ))}
          </div>
          <div className="rounded-2xl border border-dashed border-amber-400/60 bg-amber-100/40 px-4 py-3 text-xs font-semibold uppercase tracking-wide text-amber-900">
            {lang === "zh"
              ? "cost · token · time saved"
              : "cost · tokens · time saved"}
          </div>
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
            className="relative overflow-hidden rounded-2xl border border-border/60 bg-white/90 p-4 shadow-sm"
          >
            <div className={cn("absolute inset-0 bg-gradient-to-br", item.accent)} aria-hidden />
            <div className="relative space-y-3">
              <div className="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-border/70 bg-background/90 text-foreground shadow-sm">
                <Icon className="h-4 w-4" aria-hidden />
              </div>
              <div className="space-y-1">
                <p className="text-sm font-semibold text-foreground">{item.title}</p>
                <p className="text-xs leading-relaxed text-muted-foreground">{item.body}</p>
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
    <article className="space-y-4 rounded-3xl border border-border/60 bg-white/90 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
        <BookOpenText className="h-3.5 w-3.5" aria-hidden />
        {manifesto.badge}
      </div>

      <div className="space-y-2">
        <h2 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
          {manifesto.title}
        </h2>
        <p className="text-sm leading-relaxed text-muted-foreground sm:text-base">
          {manifesto.lede}
        </p>
      </div>

      <div className="space-y-3">
        {manifesto.points.map((point) => (
          <div
            key={point.title}
            className="rounded-2xl border border-border/60 bg-background/90 px-4 py-3 shadow-sm"
          >
            <div className="flex items-center gap-2">
              <span
                className={cn("h-2.5 w-2.5 rounded-full bg-gradient-to-r", point.accent)}
                aria-hidden
              />
              <p className="text-sm font-semibold text-foreground">{point.title}</p>
            </div>
            <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{point.body}</p>
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
    <section className="space-y-4 rounded-3xl border border-border/60 bg-white/90 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-white/80 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
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
            className="rounded-2xl border border-border/60 bg-background/90 px-4 py-3 shadow-sm"
          >
            <p className="text-sm font-semibold text-foreground">{point.title}</p>
            <p className="mt-1 text-xs leading-relaxed text-muted-foreground">{point.body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function VideoPlaceholder({ lang }: { lang: HomeLang }) {
  const copy = videoCopy[lang];

  return (
    <div className="rounded-3xl border border-dashed border-border/80 bg-white/80 p-6 shadow-sm">
      <div className="flex items-start gap-4">
        <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-primary/10 text-primary">
          <PlayCircle className="h-6 w-6" aria-hidden />
        </div>
        <div className="space-y-2">
          <p className="text-lg font-semibold text-foreground">{copy.title}</p>
          <p className="text-sm leading-relaxed text-muted-foreground">{copy.body}</p>
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground/80">
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
    <div className="relative min-h-screen bg-[#fbf6ee] text-foreground">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(960px_circle_at_14%_14%,rgba(52,211,153,0.18),transparent_55%),radial-gradient(980px_circle_at_86%_10%,rgba(251,191,36,0.18),transparent_55%),radial-gradient(860px_circle_at_50%_90%,rgba(45,212,191,0.16),transparent_60%)]" />
      <PageContainer className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-10 px-4 pb-16 pt-6 sm:px-6 lg:px-10 lg:pb-24 lg:pt-10">
        <Suspense fallback={<div className="h-[60px]" />}>
          <Header
            title={heroCopy.title}
            subtitle={heroCopy.subtitle}
            actionsSlot={
              <div className="flex items-center gap-2">
                <LanguageToggle lang={lang} className="hidden sm:inline-flex" />
                <Link href="/conversation">
                  <Button size="sm" className="rounded-full shadow-sm">
                    <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
                    {heroCopy.actions.primary}
                  </Button>
                </Link>
              </div>
            }
          />
        </Suspense>

        <div className="flex flex-col gap-10">
          <Reveal>
            <Hero lang={lang} />
          </Reveal>
          <Reveal delay={80}>
            <Highlights lang={lang} />
          </Reveal>
          <Reveal delay={120}>
            <SlogPanel lang={lang} />
          </Reveal>
          <Reveal delay={160}>
            <ManifestoArticle lang={lang} />
          </Reveal>
          <Reveal delay={200}>
            <VideoPlaceholder lang={lang} />
          </Reveal>
        </div>
      </PageContainer>
    </div>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
