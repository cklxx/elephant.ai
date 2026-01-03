import Link from "next/link";
import { PlayCircle, Wand2 } from "lucide-react";

import { PageContainer, PageShell, SectionBlock } from "@/components/layout/page-shell";
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
    subtitle: "一段实时对话，Plan · Clearify · ReAct。",
    actions: {
      primary: "进入控制台",
    },
  },
  en: {
    badge: "elephant.ai · orchestration-first",
    title: "A tiger is not on paper",
    subtitle: "One live chat: Plan · Clearify · ReAct.",
    actions: {
      primary: "Open console",
    },
  },
};

function Hero({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="grid gap-6 lg:grid-cols-[1.05fr,0.95fr] lg:items-center">
      <div className="space-y-4">
        <div className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-card/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground backdrop-blur">
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
            <Button className="rounded-xl shadow-sm">
              <PlayCircle className="mr-2 h-5 w-5" aria-hidden />
              {t.actions.primary}
            </Button>
          </Link>
          <div className="inline-flex items-center gap-2 text-sm text-muted-foreground">
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
        </div>
      </div>
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
