import Link from "next/link";

import { PageContainer } from "@/components/layout/page-shell";
import { cn } from "@/lib/utils";

import type { HomeLang } from "@/components/home/types";

type BlogCopy = {
  label: string;
  title: string;
  date: string;
  intro: string;
  sections: { title: string; body: string }[];
  closing: string;
};

const blogCopy: Record<HomeLang, BlogCopy> = {
  zh: {
    label: "Blog · 理念",
    title: "主动式 Agent 的工作方式",
    date: "2026-01-09",
    intro:
      "我们把理念拆成可阅读的博客：主动 Agent 不是“会说话”的按钮，而是一套可追责的执行逻辑。",
    sections: [
      {
        title: "状态是前提",
        body: "Session 保持有状态，目标、约束、权限和时间线都清晰可见，这样才能判断哪些动作可做、哪些必须先确认。",
      },
      {
        title: "澄清优先于执行",
        body: "所有依赖都会在行动前暴露出来，问题越早问清楚，后续的动作越少返工。",
      },
      {
        title: "证据驱动交付",
        body: "日志、产出和步骤对齐在一起，让交付变成可复现的过程，而不是一句“已完成”。",
      },
    ],
    closing: "慢即快：减少来回沟通，把真实状态和证据放到台前。",
  },
  en: {
    label: "Blog · manifesto",
    title: "How proactive agents work",
    date: "2026-01-09",
    intro:
      "We wrote the manifesto as a readable blog post. A proactive agent is not a talking button—it is accountable execution.",
    sections: [
      {
        title: "State comes first",
        body: "Sessions stay stateful so goals, guardrails, permissions, and timelines are visible before any action is taken.",
      },
      {
        title: "Clarify before execution",
        body: "Dependencies surface early. The sooner we ask, the fewer loops we burn later.",
      },
      {
        title: "Evidence-led delivery",
        body: "Logs, artifacts, and steps line up so delivery is reproducible, not just “done.”",
      },
    ],
    closing: "Slow is fast: fewer back-and-forths, more real state and proof.",
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
        href="/zh/blog"
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
        href="/blog"
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

export function ManifestoBlogPage({ lang }: { lang: HomeLang }) {
  const blog = blogCopy[lang];

  return (
    <div className="min-h-screen bg-background text-foreground">
      <PageContainer className="mx-auto w-full px-4 pb-12 pt-6 sm:px-6 lg:px-10 lg:pb-16">
        <div className="flex items-center justify-between text-sm">
          <Link href={lang === "zh" ? "/zh" : "/"} className="font-semibold text-foreground">
            elephant.ai
          </Link>
          <LanguageToggle lang={lang} />
        </div>
        <article className="mt-6 rounded-3xl border border-border/60 bg-card/60 p-6 shadow-sm sm:p-8">
          <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {blog.label}
          </div>
          <div className="mt-3 space-y-2">
            <h1 className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
              {blog.title}
            </h1>
            <p className="text-xs text-muted-foreground">{blog.date}</p>
          </div>
          <p className="mt-4 text-sm leading-relaxed text-muted-foreground">{blog.intro}</p>
          <div className="mt-6 space-y-4">
            {blog.sections.map((section) => (
              <div key={section.title} className="space-y-2">
                <h2 className="text-sm font-semibold text-foreground">{section.title}</h2>
                <p className="text-sm leading-relaxed text-muted-foreground">{section.body}</p>
              </div>
            ))}
          </div>
          <p className="mt-6 text-sm font-semibold text-foreground">{blog.closing}</p>
        </article>
      </PageContainer>
    </div>
  );
}
