import Link from "next/link";
import { PlayCircle } from "lucide-react";

import { PageContainer } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

import type { HomeLang } from "./types";

type DialogCopy = {
  title: string;
  placeholder: string;
  action: string;
  showcaseTitle: string;
  showcaseNote: string;
};

const dialogCopy: Record<HomeLang, DialogCopy> = {
  zh: {
    title: "对话框",
    placeholder: "描述你要完成的目标，或粘贴需求上下文。",
    action: "进入控制台",
    showcaseTitle: "Show case 展示",
    showcaseNote: "留白占位：分享回放功能完善后再补充。",
  },
  en: {
    title: "Dialogue",
    placeholder: "Describe your goal or paste the context you want handled.",
    action: "Open console",
    showcaseTitle: "Showcase",
    showcaseNote: "Placeholder for shareable replays once they are ready.",
  },
};

function LanguageToggle({ lang, className }: { lang: HomeLang; className?: string }) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-2 rounded-full border border-border/60 bg-background px-2 py-1 text-xs font-semibold text-muted-foreground",
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

function DialogPanel({ lang }: { lang: HomeLang }) {
  const copy = dialogCopy[lang];

  return (
    <section className="rounded-3xl border border-border/60 bg-card/60 p-6 shadow-sm sm:p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-foreground sm:text-xl">{copy.title}</h1>
        <LanguageToggle lang={lang} className="hidden sm:inline-flex" />
      </div>
      <div className="mt-4 space-y-4">
        <Textarea
          placeholder={copy.placeholder}
          className="min-h-[140px] resize-none rounded-2xl border-border/60 bg-background/70"
        />
        <div className="flex flex-wrap items-center gap-3">
          <Link href="/conversation">
            <Button className="rounded-full shadow-sm">
              <PlayCircle className="mr-2 h-5 w-5" aria-hidden />
              {copy.action}
            </Button>
          </Link>
          <LanguageToggle lang={lang} className="sm:hidden" />
        </div>
      </div>
    </section>
  );
}

function ShowcasePlaceholder({ lang }: { lang: HomeLang }) {
  const copy = dialogCopy[lang];

  return (
    <section className="rounded-3xl border border-dashed border-border/70 bg-muted/20 p-6 text-sm text-muted-foreground sm:p-8">
      <p className="text-base font-semibold text-foreground">{copy.showcaseTitle}</p>
      <p className="mt-2">{copy.showcaseNote}</p>
      <div className="mt-4 h-28 rounded-2xl border border-dashed border-border/60 bg-background/60" />
    </section>
  );
}

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <PageContainer className="mx-auto w-full px-4 pb-12 pt-6 sm:px-6 lg:px-10 lg:pb-16">
        <div className="flex flex-col gap-6">
          <DialogPanel lang={lang} />
          <ShowcasePlaceholder lang={lang} />
        </div>
      </PageContainer>
    </div>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
