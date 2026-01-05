import Link from "next/link";
import { Suspense } from "react";
import { BookOpenText, PlayCircle, Wand2 } from "lucide-react";

import { Header } from "@/components/layout";
import { PageContainer } from "@/components/layout/page-shell";
import {
  LiveChatShowcase,
  type ChatTurn,
  type StageCopy,
} from "@/components/home/LiveChatShowcase";
import { liveChatCopy, liveChatScript, liveChatStages } from "@/components/home/LiveChatCopy";
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
    badge: "elephant.ai · orchestration-first",
    title: "一只猛虎，不在纸上",
    subtitle: "首页与 Console 同框：一段实时对话，Plan · Clearify · ReAct。",
    actions: {
      primary: "进入控制台",
    },
  },
  en: {
    badge: "elephant.ai · orchestration-first",
    title: "A tiger is not on paper",
    subtitle: "Same frame as the console—one live chat to Plan · Clearify · ReAct.",
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
    title: "Plan · Clearify · ReAct 的纯净宣言",
    lede: "首页与控制台同框：以编排为先，少装饰，多留证据。",
    points: [
      {
        title: "先计划，不做堆料式提示",
        body: "Session 保持有状态，目标和约束并排，Agent 知道不能踩的线。",
        accent: "from-indigo-500 via-sky-500 to-emerald-500",
      },
      {
        title: "清晰拆解，可见的步骤",
        body: "任务拆成可调用的工具，证据同行，review 时能看见结果是怎么来的。",
        accent: "from-amber-500 via-orange-500 to-rose-500",
      },
      {
        title: "行动可追责",
        body: "每一步像控制台一样可停、可重放、易交接；记录先于包装。",
        accent: "from-emerald-500 via-teal-500 to-cyan-500",
      },
    ],
    closing: "慢即快：少一层包装，多一份真实状态。这就是我们保留的全部文案。",
  },
  en: {
    badge: "Plain text · why elephant.ai",
    title: "A clean manifesto for Plan · Clearify · ReAct",
    lede: "Homepage shares the console frame: orchestration first, minimal chrome, evidence always visible.",
    points: [
      {
        title: "Plan before prompting",
        body: "Sessions stay stateful with goals and guardrails side by side so agents know the lines they cannot cross.",
        accent: "from-indigo-500 via-sky-500 to-emerald-500",
      },
      {
        title: "Clearify into visible steps",
        body: "Work is sliced into callable tools with evidence inline, so reviewers see exactly how outputs are produced.",
        accent: "from-amber-500 via-orange-500 to-rose-500",
      },
      {
        title: "ReAct with accountability",
        body: "Every action mirrors the conversation view—stoppable, replayable, and ready to hand off without rework.",
        accent: "from-emerald-500 via-teal-500 to-cyan-500",
      },
    ],
    closing: "Slow is fast: fewer layers, truer state. That's the only marketing copy we keep.",
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

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  const liveCopy = liveChatCopy[lang];
  const liveStages: StageCopy[] = liveChatStages[lang];
  const liveScript: ChatTurn[] = liveChatScript[lang];
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

        <div className="grid flex-1 min-h-0 gap-5 lg:grid-cols-[0.95fr,1.05fr]">
          <div className="flex min-h-0 flex-col gap-4">
            <Hero lang={lang} />
            <ManifestoArticle lang={lang} />
          </div>
          <div className="flex min-h-0 flex-col">
            <LiveChatShowcase lang={lang} copy={liveCopy} stages={liveStages} script={liveScript} />
          </div>
        </div>
      </PageContainer>
    </div>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
