import Link from "next/link";
import { ArrowRight, PlayCircle, Sparkles, Wand2 } from "lucide-react";

import { PageContainer, PageShell, SectionBlock } from "@/components/layout/page-shell";
import {
  LiveChatShowcase,
  type ChatTurn,
  type StageCopy,
} from "@/components/home/LiveChatShowcase";
import { liveChatCopy, liveChatScript, liveChatStages } from "@/components/home/LiveChatCopy";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

import type { HomeLang } from "./types";

type HomeCopy = {
  badge: string;
  title: string;
  subtitle: string;
  actions: {
    primary: string;
    secondary: string;
    evaluation: string;
    login: string;
  };
  nav: {
    console: string;
    sessions: string;
    evaluation: string;
    docs: string;
  };
  idea: {
    title: string;
    subtitle: string;
    bullets: string[];
    ribbon: string;
  };
  highlights: string[];
};

const copy: Record<HomeLang, HomeCopy> = {
  zh: {
    badge: "elephant.ai · Orchestration-first",
    title: "首页只保留一件事：以 chat demo 演示 Plan + Clearify + ReAct",
    subtitle:
      "实时对话框 + 理念卡片，展示 orchestrator 如何声明任务、调用工具并留存证据。其余噪声一概拿掉。",
    actions: {
      primary: "进入控制台",
      secondary: "查看会话",
      evaluation: "评估面板",
      login: "团队登录",
    },
    nav: {
      console: "控制台",
      sessions: "会话",
      evaluation: "评估",
      docs: "理念",
    },
    idea: {
      title: "理念框：把智能体当工程系统约束",
      subtitle: "Plan 对齐意图，Clearify 切成可验收任务，ReAct 在任务内交替推理/行动并留证据。",
      bullets: [
        "只公开目标与进度，完整计划作为内部控制状态，可随时重规划。",
        "每个任务先声明，再执行，粒度小到一句话就能验收。",
        "工具是契约：结构化 IO + 统一日志，方便回放、追责、复用。",
      ],
      ribbon: "为什么是 Plan + Clearify + ReAct？",
    },
    highlights: [
      "实时 chat demo 展示 orchestration",
      "任务、工具、证据三层同屏",
      "一键跳转控制台 / 会话 / 评估",
    ],
  },
  en: {
    badge: "elephant.ai · Orchestration-first",
    title: "One thing on the homepage: a chat demo for Plan + Clearify + ReAct",
    subtitle:
      "A live chat box and a single manifesto card show how the orchestrator declares tasks, calls tools, and leaves evidence. Everything else is stripped away.",
    actions: {
      primary: "Open console",
      secondary: "View sessions",
      evaluation: "Evaluation",
      login: "Team login",
    },
    nav: {
      console: "Console",
      sessions: "Sessions",
      evaluation: "Evaluation",
      docs: "Manifesto",
    },
    idea: {
      title: "The manifesto box: treat agents like software",
      subtitle:
        "Plan aligns intent, Clearify slices work into reviewable tasks, ReAct alternates reasoning and actions while keeping evidence tight.",
      bullets: [
        "Expose goals and progress only; keep the full plan as internal control state for fast replanning.",
        "Declare every task before acting—small enough to review in one sentence.",
        "Tools are contracts: structured IO with unified logging for replay, accountability, and reuse.",
      ],
      ribbon: "Why Plan + Clearify + ReAct?",
    },
    highlights: [
      "Live chat demo of orchestration",
      "Tasks, tools, evidence on one canvas",
      "Jump to console / sessions / evaluation",
    ],
  },
};

function HomeTopBar({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-3">
        <div className="flex h-11 w-11 items-center justify-center rounded-2xl border border-border bg-card/70 shadow-sm backdrop-blur">
          <Sparkles className="h-5 w-5 text-foreground/90" aria-hidden />
        </div>
        <div>
          <div className="text-sm font-semibold text-foreground">elephant.ai</div>
          <div className="text-xs text-muted-foreground">{t.nav.docs}</div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-full border border-border bg-background/80 p-1 text-xs font-semibold backdrop-blur">
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

        <Link href="/conversation">
          <Button size="sm" className="rounded-full shadow-sm">
            <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
            {t.nav.console}
          </Button>
        </Link>
        <Link href="/sessions">
          <Button size="sm" variant="outline" className="rounded-full">
            {t.nav.sessions}
          </Button>
        </Link>
        <Link href="/evaluation">
          <Button size="sm" variant="outline" className="rounded-full">
            {t.nav.evaluation}
          </Button>
        </Link>
      </div>
    </div>
  );
}

function Hero({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="grid gap-6 lg:grid-cols-[1.05fr,0.95fr] lg:items-center">
      <div className="space-y-5">
        <div className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-card/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground backdrop-blur">
          <Wand2 className="h-3.5 w-3.5" aria-hidden />
          {t.badge}
        </div>

        <div className="space-y-3">
          <h1 className="text-3xl font-semibold leading-tight tracking-tight text-foreground sm:text-4xl">
            {t.title}
          </h1>
          <p className="max-w-3xl text-base leading-relaxed text-muted-foreground sm:text-lg">
            {t.subtitle}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Link href="/conversation">
            <Button className="group rounded-xl shadow-sm">
              <PlayCircle className="mr-2 h-5 w-5" aria-hidden />
              {t.actions.primary}
              <ArrowRight
                className="ml-2 h-4 w-4 transition-transform group-hover:translate-x-0.5"
                aria-hidden
              />
            </Button>
          </Link>
          <Link href="/sessions">
            <Button variant="outline" className="rounded-xl">
              {t.actions.secondary}
            </Button>
          </Link>
          <Link href="/evaluation">
            <Button variant="outline" className="rounded-xl">
              {t.actions.evaluation}
            </Button>
          </Link>
          <Link
            href="/login"
            className="text-sm font-semibold text-muted-foreground hover:text-foreground"
          >
            {t.actions.login}
          </Link>
        </div>

        <div className="flex flex-wrap gap-2">
          {t.highlights.map((item) => (
            <Badge
              key={item}
              variant="secondary"
              className="border border-border/60 bg-background/70 text-foreground"
            >
              {item}
            </Badge>
          ))}
        </div>
      </div>

      <Card className="bg-card/70 shadow-sm backdrop-blur">
        <CardHeader className="space-y-2">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Sparkles className="h-4 w-4" aria-hidden />
            {t.nav.docs}
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            {t.idea.subtitle}
          </p>
        </CardHeader>
        <CardContent className="grid gap-2">
          {t.idea.bullets.map((line) => (
            <div
              key={line}
              className="flex items-start gap-2 rounded-xl border border-border/70 bg-background/70 p-3 text-sm text-muted-foreground"
            >
              <span className="mt-1 inline-flex h-2 w-2 flex-none rounded-full bg-foreground/50" />
              <span>{line}</span>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  const liveCopy = liveChatCopy[lang];
  const liveStages: StageCopy[] = liveChatStages[lang];
  const liveScript: ChatTurn[] = liveChatScript[lang];

  return (
    <PageShell padding="none">
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(900px_circle_at_12%_20%,rgba(99,102,241,0.08),transparent_55%),radial-gradient(900px_circle_at_88%_10%,rgba(34,211,238,0.08),transparent_55%),radial-gradient(900px_circle_at_40%_92%,rgba(16,185,129,0.06),transparent_55%)]" />

        <PageContainer className="px-4 py-10 sm:px-6 lg:px-10 lg:py-14">
          <SectionBlock className="gap-8">
            <HomeTopBar lang={lang} />
            <Hero lang={lang} />
          </SectionBlock>

          <SectionBlock className="gap-6">
            <LiveChatShowcase lang={lang} copy={liveCopy} stages={liveStages} script={liveScript} />
          </SectionBlock>
        </PageContainer>
      </div>
    </PageShell>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
