"use client";

import Link from "next/link";
import { Suspense, useEffect, useRef, useState } from "react";
import { BookOpenText, Layers, MessageSquare, PlayCircle, Sparkles, Zap } from "lucide-react";
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
  en: {
    badge: "elephant.ai · Lark-native proactive agent",
    title: "Your proactive personal agent, native to Lark",
    subtitle:
      "No app switching, no special commands — talk naturally in groups and DMs. It reads the room, remembers everything, and executes real work for you.",
    actions: {
      primary: "Open console",
    },
  },
  zh: {
    badge: "elephant.ai · 飞书原生主动代理",
    title: "住在飞书里的主动性个人 Agent",
    subtitle: "不需要切换应用、不需要特殊指令——在群聊和私信中自然对话，它主动理解上下文、记住一切、替你执行真实工作。",
    actions: {
      primary: "进入控制台",
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
  en: [
    {
      title: "Lives in Lark",
      body: "Always online via WebSocket in your groups and DMs — responds like a team member.",
      accent: "from-emerald-500/20 via-lime-500/20 to-amber-400/20",
      icon: MessageSquare,
    },
    {
      title: "Proactive context awareness",
      body: "Auto-fetches recent chat history and cross-session memory. No need to repeat yourself.",
      accent: "from-orange-500/20 via-amber-400/20 to-rose-400/20",
      icon: Sparkles,
    },
    {
      title: "Autonomous real work",
      body: "Search, code, generate documents, browse pages — from a Lark message to deliverable output.",
      accent: "from-teal-500/20 via-sky-400/20 to-emerald-400/20",
      icon: Zap,
    },
  ],
  zh: [
    {
      title: "住在飞书里",
      body: "通过 WebSocket 常驻群聊和私信，像团队成员一样随时在线、随时响应。",
      accent: "from-emerald-500/20 via-lime-500/20 to-amber-400/20",
      icon: MessageSquare,
    },
    {
      title: "主动理解上下文",
      body: "自动获取近期聊天记录、跨会话记忆，不用你复述背景。",
      accent: "from-orange-500/20 via-amber-400/20 to-rose-400/20",
      icon: Sparkles,
    },
    {
      title: "自主执行真实工作",
      body: "搜索、写代码、生成文档、浏览网页——从一条飞书消息到可交付产出。",
      accent: "from-teal-500/20 via-sky-400/20 to-emerald-400/20",
      icon: Zap,
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
  en: {
    badge: "Why Lark-native",
    title: "Don't let AI live outside your workflow",
    lede: "Most AI assistants are another app, another tab, another context switch. elephant.ai lives right inside Lark.",
    points: [
      {
        title: "Zero switching cost",
        body: "No new app to open — talk in your existing groups and DMs.",
        accent: "from-emerald-500 via-lime-500 to-amber-400",
      },
      {
        title: "Persistent memory",
        body: "Remembers conversations, decisions, and context across sessions. Never repeat yourself.",
        accent: "from-orange-500 via-amber-400 to-rose-400",
      },
      {
        title: "Fully observable",
        body: "Real-time progress, transparent cost and token tracking, every step traceable.",
        accent: "from-teal-500 via-sky-400 to-emerald-400",
      },
    ],
    closing: "AI inside your workflow, not outside it.",
  },
  zh: {
    badge: "为什么是飞书原生",
    title: "别让 AI 在工作流之外",
    lede: "大多数 AI 助手是另一个应用、另一个标签页、另一次上下文切换。elephant.ai 直接住在飞书里。",
    points: [
      {
        title: "零切换成本",
        body: "不需要打开新应用——在你已有的群聊和私信里直接对话。",
        accent: "from-emerald-500 via-lime-500 to-amber-400",
      },
      {
        title: "持续记忆",
        body: "跨会话记住对话、决策和上下文，再也不用重复说明背景。",
        accent: "from-orange-500 via-amber-400 to-rose-400",
      },
      {
        title: "全程可观测",
        body: "执行进度实时反馈、成本与 token 透明可查、每一步可追踪。",
        accent: "from-teal-500 via-sky-400 to-emerald-400",
      },
    ],
    closing: "工作流里的 AI，而不是工作流外的 AI。",
  },
};

type SlogCopy = {
  badge: string;
  title: string;
  points: { title: string; body: string }[];
};

const slogCopy: Record<HomeLang, SlogCopy> = {
  en: {
    badge: "Built-in capabilities",
    title: "Not just chat — an agent that gets things done",
    points: [
      {
        title: "Deep research",
        body: "Multi-step web search and synthesis, auto-generates research reports.",
      },
      {
        title: "Skill-driven",
        body: "Meeting notes, email drafting, slide decks, video production — triggered by natural language.",
      },
      {
        title: "Rich toolset",
        body: "Code execution, file ops, browser automation, MCP extensions — capabilities keep growing.",
      },
    ],
  },
  zh: {
    badge: "内置能力 · 开箱即用",
    title: "不只是聊天——是能做事的 Agent",
    points: [
      {
        title: "深度研究",
        body: "多步骤网络搜索与信息综合，自动生成研究报告。",
      },
      {
        title: "技能驱动",
        body: "会议纪要、邮件撰写、PPT 生成、视频制作——用自然语言触发。",
      },
      {
        title: "工具丰富",
        body: "代码执行、文件操作、浏览器自动化、MCP 扩展——能力持续增长。",
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
  en: {
    title: "Demo video coming soon",
    body: "A real recording: one message in a Lark group triggers a full research, execution, and delivery workflow.",
    note: "No simulated chat — real output from a real Lark group.",
  },
  zh: {
    title: "演示视频即将呈现",
    body: "真实录屏展示：在飞书群里一条消息，触发完整的研究、执行和交付流程。",
    note: "无模拟对话框——真实飞书群聊中的真实产出。",
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
      { threshold: 0.1, rootMargin: "0px 0px -50px 0px" },
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
        "transition-all duration-1000 ease-out will-change-transform",
        isVisible ? "translate-y-0 opacity-100 blur-0" : "translate-y-12 opacity-0 blur-sm",
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
    <div className="relative overflow-hidden rounded-[48px] border border-white/40 bg-gradient-to-br from-white/95 via-white/90 to-white/95 p-12 shadow-[0_20px_70px_-40px_rgba(0,0,0,0.25)] backdrop-blur-xl lg:p-16">
      {/* Animated gradient background */}
      <div className="pointer-events-none absolute inset-0 opacity-60">
        <div className="absolute inset-0 animate-[gradient_8s_ease-in-out_infinite] bg-[radial-gradient(600px_circle_at_30%_30%,rgba(52,211,153,0.15),transparent_50%),radial-gradient(800px_circle_at_80%_20%,rgba(251,191,36,0.12),transparent_55%),radial-gradient(700px_circle_at_60%_80%,rgba(45,212,191,0.10),transparent_50%)]" />
      </div>

      <div className="relative mx-auto max-w-4xl space-y-8 text-center">
        <div className="inline-flex items-center gap-2 rounded-full border border-foreground/5 bg-white/60 px-4 py-1.5 text-xs font-medium tracking-wide text-muted-foreground/90 shadow-sm backdrop-blur-sm">
          <Sparkles className="h-3.5 w-3.5" aria-hidden />
          {t.badge}
        </div>

        <div className="space-y-6">
          <h1 className="bg-gradient-to-br from-foreground via-foreground/95 to-foreground/80 bg-clip-text text-5xl font-bold leading-[1.15] tracking-tight text-transparent sm:text-6xl lg:text-7xl">
            {t.title}
          </h1>
          <p className="mx-auto max-w-2xl text-lg leading-relaxed text-muted-foreground/90 sm:text-xl">
            {t.subtitle}
          </p>
        </div>

        <div className="flex flex-wrap items-center justify-center gap-4 pt-2">
          <Link href="/conversation">
            <Button
              size="lg"
              className="group rounded-full shadow-lg transition-all hover:scale-105 hover:shadow-xl"
            >
              <PlayCircle className="mr-2 h-5 w-5 transition-transform group-hover:scale-110" aria-hidden />
              {t.actions.primary}
            </Button>
          </Link>
          <LanguageToggle lang={lang} className="sm:hidden" />
        </div>

        {/* Subtle feature pills */}
        <div className="flex flex-wrap items-center justify-center gap-3 pt-4 text-sm">
          {[
            lang === "zh" ? "飞书原生" : "Lark-native",
            lang === "zh" ? "跨会话记忆" : "Persistent memory",
            lang === "zh" ? "自主执行" : "Autonomous execution",
          ].map((item) => (
            <div
              key={item}
              className="group inline-flex items-center gap-2 rounded-full border border-border/40 bg-white/50 px-4 py-2 backdrop-blur-sm transition-all hover:border-emerald-500/40 hover:bg-white/70"
            >
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 transition-transform group-hover:scale-125" aria-hidden />
              <span className="font-medium text-foreground/80">{item}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function Highlights({ lang }: { lang: HomeLang }) {
  const highlights = highlightCopy[lang];
  return (
    <div className="grid gap-4 sm:grid-cols-3">
      {highlights.map((item, index) => {
        const Icon = item.icon;
        return (
          <div
            key={item.title}
            className="group relative overflow-hidden rounded-3xl border border-white/40 bg-white/80 p-6 backdrop-blur-md transition-all hover:scale-[1.02] hover:border-white/60 hover:bg-white/90 hover:shadow-xl"
            style={{
              animationDelay: `${index * 100}ms`,
            }}
          >
            <div className={cn("pointer-events-none absolute inset-0 bg-gradient-to-br opacity-0 transition-opacity duration-500 group-hover:opacity-100", item.accent)} aria-hidden />
            <div className="relative space-y-4">
              <div className="inline-flex h-12 w-12 items-center justify-center rounded-2xl border border-border/30 bg-gradient-to-br from-white/80 to-white/60 text-foreground shadow-sm backdrop-blur-sm transition-transform group-hover:scale-110">
                <Icon className="h-5 w-5" aria-hidden />
              </div>
              <div className="space-y-2">
                <p className="text-base font-semibold tracking-tight text-foreground">{item.title}</p>
                <p className="text-sm leading-relaxed text-muted-foreground/90">{item.body}</p>
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
    <article className="space-y-8 rounded-[48px] border border-white/40 bg-gradient-to-br from-white/90 via-white/85 to-white/90 p-8 shadow-lg backdrop-blur-xl lg:p-12">
      <div className="space-y-4">
        <div className="inline-flex items-center gap-2 rounded-full border border-foreground/5 bg-white/60 px-4 py-1.5 text-xs font-medium tracking-wide text-muted-foreground/90 shadow-sm backdrop-blur-sm">
          <BookOpenText className="h-3.5 w-3.5" aria-hidden />
          {manifesto.badge}
        </div>

        <div className="space-y-3">
          <h2 className="text-3xl font-bold tracking-tight text-foreground sm:text-4xl lg:text-5xl">
            {manifesto.title}
          </h2>
          <p className="max-w-3xl text-base leading-relaxed text-muted-foreground/90 sm:text-lg">
            {manifesto.lede}
          </p>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-3">
        {manifesto.points.map((point, index) => (
          <div
            key={point.title}
            className="group relative overflow-hidden rounded-3xl border border-white/40 bg-white/60 p-6 backdrop-blur-sm transition-all hover:scale-[1.02] hover:border-white/60 hover:bg-white/80 hover:shadow-xl"
          >
            <div className="absolute left-6 top-6">
              <span
                className={cn("block h-3 w-3 rounded-full bg-gradient-to-r shadow-sm transition-transform group-hover:scale-125", point.accent)}
                aria-hidden
              />
            </div>
            <div className="space-y-3 pl-7">
              <p className="text-base font-semibold tracking-tight text-foreground">{point.title}</p>
              <p className="text-sm leading-relaxed text-muted-foreground/90">{point.body}</p>
            </div>
          </div>
        ))}
      </div>

      <div className="rounded-3xl border border-dashed border-amber-400/30 bg-gradient-to-br from-amber-50/80 to-orange-50/60 px-6 py-4 backdrop-blur-sm">
        <p className="text-center text-base font-semibold text-foreground sm:text-lg">{manifesto.closing}</p>
      </div>
    </article>
  );
}

function SlogPanel({ lang }: { lang: HomeLang }) {
  const slog = slogCopy[lang];
  return (
    <section className="space-y-6 rounded-[48px] border border-white/40 bg-gradient-to-br from-white/90 via-white/85 to-white/90 p-8 shadow-lg backdrop-blur-xl lg:p-10">
      <div className="space-y-3">
        <div className="inline-flex items-center gap-2 rounded-full border border-foreground/5 bg-white/60 px-4 py-1.5 text-xs font-medium tracking-wide text-muted-foreground/90 shadow-sm backdrop-blur-sm">
          <Layers className="h-3.5 w-3.5" aria-hidden />
          {slog.badge}
        </div>
        <h2 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl lg:text-4xl">
          {slog.title}
        </h2>
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        {slog.points.map((point, index) => (
          <div
            key={point.title}
            className="group relative overflow-hidden rounded-3xl border border-white/40 bg-white/60 p-6 backdrop-blur-sm transition-all hover:scale-[1.02] hover:border-white/60 hover:bg-white/80 hover:shadow-xl"
          >
            <div className="absolute right-4 top-4 text-6xl font-bold text-foreground/5">
              {index + 1}
            </div>
            <div className="relative space-y-2">
              <p className="text-base font-semibold tracking-tight text-foreground">{point.title}</p>
              <p className="text-sm leading-relaxed text-muted-foreground/90">{point.body}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function VideoPlaceholder({ lang }: { lang: HomeLang }) {
  const copy = videoCopy[lang];

  return (
    <div className="group relative overflow-hidden rounded-[48px] border border-dashed border-white/60 bg-gradient-to-br from-white/70 via-white/60 to-white/70 p-10 backdrop-blur-xl transition-all hover:border-white/80 hover:bg-white/80 hover:shadow-xl lg:p-14">
      <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(800px_circle_at_50%_50%,rgba(139,92,246,0.08),transparent_70%)]" />
      <div className="relative mx-auto max-w-2xl space-y-6 text-center">
        <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-3xl border border-white/40 bg-gradient-to-br from-white/80 to-white/60 text-primary shadow-lg backdrop-blur-sm transition-transform group-hover:scale-110">
          <PlayCircle className="h-10 w-10" aria-hidden />
        </div>
        <div className="space-y-3">
          <p className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">{copy.title}</p>
          <p className="text-base leading-relaxed text-muted-foreground/90 sm:text-lg">{copy.body}</p>
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground/70">
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
    <div className="relative min-h-screen bg-gradient-to-br from-[#fdfbf7] via-[#faf6f0] to-[#f8f3eb] text-foreground">
      {/* Animated gradient background */}
      <div className="pointer-events-none absolute inset-0 -z-10">
        <div className="absolute inset-0 animate-[gradient_12s_ease-in-out_infinite] bg-[radial-gradient(1200px_circle_at_20%_20%,rgba(52,211,153,0.12),transparent_60%),radial-gradient(1400px_circle_at_85%_15%,rgba(251,191,36,0.10),transparent_65%),radial-gradient(1100px_circle_at_50%_85%,rgba(45,212,191,0.08),transparent_60%)]" />
      </div>

      <PageContainer className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-16 px-4 pb-20 pt-6 sm:px-6 lg:gap-20 lg:px-12 lg:pb-32 lg:pt-10">
        <Suspense fallback={<div className="h-[60px]" />}>
          <Header
            title={heroCopy.title}
            subtitle={heroCopy.subtitle}
            actionsSlot={
              <div className="flex items-center gap-3">
                <LanguageToggle lang={lang} className="hidden sm:inline-flex" />
                <Link href="/conversation">
                  <Button size="sm" className="rounded-full shadow-md transition-all hover:scale-105 hover:shadow-lg">
                    <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
                    {heroCopy.actions.primary}
                  </Button>
                </Link>
              </div>
            }
          />
        </Suspense>

        <div className="flex flex-col gap-16 lg:gap-20">
          <Reveal>
            <Hero lang={lang} />
          </Reveal>
          <Reveal delay={100}>
            <Highlights lang={lang} />
          </Reveal>
          <Reveal delay={150}>
            <SlogPanel lang={lang} />
          </Reveal>
          <Reveal delay={200}>
            <ManifestoArticle lang={lang} />
          </Reveal>
          <Reveal delay={250}>
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
