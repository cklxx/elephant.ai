import Link from "next/link";
import { Suspense } from "react";
import { BookOpenText, PlayCircle, Wand2 } from "lucide-react";

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
    title: "一只猛虎，不在纸上",
    subtitle: "首页与 Console 同框：主动 Agent 先探路、先澄清、再执行。",
    actions: {
      primary: "进入控制台",
    },
  },
  en: {
    badge: "elephant.ai · proactive agent",
    title: "A tiger is not on paper",
    subtitle: "Proactive agents share the console frame—scouting, clarifying, then executing.",
    actions: {
      primary: "Open console",
    },
  },
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
    badge: "纯文本 · 理念",
    title: "主动式 Agent 的纯净宣言",
    lede: "首页与控制台同框：主动 Agent 持续感知、提前协同，编排先于回答。",
    points: [
      {
        title: "主动觉察，先于指令规划",
        body: "Session 保持有状态，主动 Agent 按目标和约束预判路径，标注不可踩的线。",
        accent: "from-indigo-500 via-sky-500 to-emerald-500",
      },
      {
        title: "少催促，多澄清",
        body: "行动前主动提问和暴露依赖，再把任务拆成可调用的工具，风险在执行前就被看见。",
        accent: "from-amber-500 via-orange-500 to-rose-500",
      },
      {
        title: "行动自证可追责",
        body: "每一步像控制台一样可停、可重放，连同证据输出，方便交接与复盘。",
        accent: "from-emerald-500 via-teal-500 to-cyan-500",
      },
    ],
    closing: "慢即快：主动 Agent 少一次往返催促，多一次真实状态与证据。",
  },
  en: {
    badge: "Plain text · why elephant.ai",
    title: "A clean manifesto for proactive agents",
    lede: "Homepage shares the console frame: proactive agents keep sensing, coordinating, and orchestrating before answering.",
    points: [
      {
        title: "Sense early, plan ahead",
        body: "Sessions stay stateful; proactive agents read goals and guardrails to anticipate paths and mark the lines they cannot cross.",
        accent: "from-indigo-500 via-sky-500 to-emerald-500",
      },
      {
        title: "Clarify before moving",
        body: "They ask first, surface dependencies, and slice work into callable tools so risks show up before execution.",
        accent: "from-amber-500 via-orange-500 to-rose-500",
      },
      {
        title: "Act with built-in evidence",
        body: "Every action mirrors the console view—pausable, replayable, and delivered with inline artifacts for handoffs and reviews.",
        accent: "from-emerald-500 via-teal-500 to-cyan-500",
      },
    ],
    closing: "Slow is fast: proactive agents cut nagging loops and keep real state with proof.",
  },
};

type VideoCopy = {
  title: string;
  body: string;
  note: string;
};

const videoCopy: Record<HomeLang, VideoCopy> = {
  zh: {
    title: "主动 Agent 演示视频稍后放在这里",
    body: "这一块预留给真实录屏：主动 Agent 如何先探路、澄清依赖，再行动并交付证据。",
    note: "无模拟对话框：留白给真实上下文与产出，方便直接替换成录屏。",
  },
  en: {
    title: "Proactive agent demo video will live here soon",
    body: "Reserved for a real recording: how a proactive agent scouts, clarifies dependencies, then acts with evidence.",
    note: "No simulated chat—a clean space to drop in real context and artifacts.",
  },
};

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
    <div className="space-y-5 rounded-3xl border border-border/60 bg-card/70 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-background/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
        <Wand2 className="h-3.5 w-3.5" aria-hidden />
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
  );
}

function ManifestoArticle({ lang }: { lang: HomeLang }) {
  const manifesto = manifestoCopy[lang];
  return (
    <article className="space-y-4 rounded-3xl border border-border/60 bg-background/80 p-6 shadow-sm">
      <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-card/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
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
            className="rounded-2xl border border-border/60 bg-card/60 px-4 py-3 shadow-sm"
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

function VideoPlaceholder({ lang }: { lang: HomeLang }) {
  const copy = videoCopy[lang];

  return (
    <div className="rounded-3xl border border-dashed border-border/80 bg-card/70 p-6 shadow-sm">
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
    <div className="relative min-h-screen bg-muted/10 text-foreground">
      <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(900px_circle_at_12%_20%,rgba(99,102,241,0.08),transparent_55%),radial-gradient(900px_circle_at_88%_10%,rgba(34,211,238,0.08),transparent_55%),radial-gradient(900px_circle_at_40%_92%,rgba(16,185,129,0.06),transparent_55%)]" />
      <PageContainer className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-6 px-4 pb-12 pt-6 sm:px-6 lg:px-10 lg:pb-16 lg:pt-10">
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

        <div className="flex min-h-0 flex-col gap-4">
          <Hero lang={lang} />
          <ManifestoArticle lang={lang} />
          <VideoPlaceholder lang={lang} />
        </div>
      </PageContainer>
    </div>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
