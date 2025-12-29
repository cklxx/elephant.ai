import Link from "next/link";
import {
  ArrowRight,
  FileCode2,
  GitBranch,
  ListChecks,
  ListTodo,
  Palette,
  PlayCircle,
  ScrollText,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import {
  PageContainer,
  PageShell,
  SectionBlock,
} from "@/components/layout/page-shell";
import { FlowShowcase, type FlowCopy } from "@/components/home/FlowShowcase";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

import type { FlowStep } from "./FlowShowcase";
import type { HomeLang } from "./types";

type Pillar = {
  icon: LucideIcon;
  title: string;
  description: string;
  detail: string;
};

type Snapshot = {
  title: string;
  description: string;
  metrics: { label: string; value: string; hint?: string }[];
  deliverablesLabel: string;
  deliverables: string[];
};

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
  section: {
    hero: {
      highlights: string[];
    };
    snapshot: Snapshot;
    flow: FlowCopy;
    pillars: {
      title: string;
      items: Pillar[];
    };
    proof: {
      title: string;
      bullets: string[];
    };
    cta: {
      title: string;
      description: string;
    };
    refs: {
      title: string;
    };
  };
};

const references = [
  {
    title: "ReAct: Synergizing Reasoning and Acting in Language Models",
    href: "https://arxiv.org/abs/2210.03629",
  },
  {
    title: "Structured generation for tool use (OpenAI)",
    href: "https://platform.openai.com/docs/guides/structured-outputs",
  },
  {
    title: "Plan-and-Execute Agents (LangChain)",
    href: "https://blog.langchain.com/planning-agents/",
  },
  {
    title: "Claude Code: Best practices",
    href: "https://www.anthropic.com/engineering/claude-code-best-practices",
  },
] as const;

const copy: Record<HomeLang, HomeCopy> = {
  zh: {
    badge: "elephant.ai · 前端编排",
    title: "让 Agent 直接交付可审计的首页体验",
    subtitle:
      "elephant.ai 把 Plan / Clearify / ReAct 作为 UI 编排器：版式、文案、状态、证据同频更新，可复盘、可验收、可合并。",
    actions: {
      primary: "开始一次演示",
      secondary: "查看运行记录",
      evaluation: "评估面板",
      login: "团队登录",
    },
    nav: {
      console: "控制台",
      sessions: "运行",
      evaluation: "评估",
      docs: "理念",
    },
    section: {
      hero: {
        highlights: [
          "明确三级结构：Goal → Task → Evidence",
          "多语言版式与文案保持语气一致",
          "每一步都有可追溯的 QA 记录与截图",
        ],
      },
      snapshot: {
        title: "一次 elephant.ai 运行长什么样",
        description: "从意图到交付在同一画布里：计划、执行、证据可随时回放与审计。",
        metrics: [
          { label: "意图到可交付 UI", value: "≈ 3 分钟", hint: "含文案、版式、状态" },
          { label: "并行语言", value: "中文 · English", hint: "语气/结构双向校对" },
          { label: "质量护栏", value: "a11y · 对比度 · 视口差异" },
        ],
        deliverablesLabel: "输出交付物",
        deliverables: [
          "可复盘的运行记录（含证据链接）",
          "页面规格说明 + 语气提示",
          "组件映射与状态定义",
          "可直接合并的 Next.js 片段",
        ],
      },
      flow: {
        title: "编排好的流程",
        description: "Plan / Clearify / ReAct 在同一轨迹上滚动播放，可随时暂停查看细节。",
        timelineLabel: "轨迹",
        logLabel: "事件 / 证据",
        liveLabel: "播放中",
      },
      pillars: {
        title: "elephant.ai 为首页设计做的事情",
        items: [
          {
            icon: Palette,
            title: "设计系统对齐",
            description: "先读取 Token、栅格、圆角与图标集，再生成页面。",
            detail: "样式不再漂移，品牌一致性内置。",
          },
          {
            icon: ScrollText,
            title: "语气与意图并行",
            description: "文案与 PRD 同步，双语互为校对。",
            detail: "不靠填充文本，而是可审计的语气提示。",
          },
          {
            icon: ShieldCheck,
            title: "带护栏的执行",
            description: "每一步自动跑 a11y / 对比度 / 响应式检查。",
            detail: "护栏和证据写进日志，可直接复盘。",
          },
          {
            icon: GitBranch,
            title: "交付无偏移",
            description: "导出的代码与批准版式一致，含组件映射和状态注释。",
            detail: "降低“手写落地”带来的偏差。",
          },
        ],
      },
      proof: {
        title: "让产品 / 设计 / 工程一起审阅",
        bullets: [
          "产品：先锁验收标准与风险门槛，再开始行动。",
          "设计：节奏、间距、态控来自预设，而不是提示词猜测。",
          "工程：拿到可复现的证据与可直接粘贴的代码片段。",
        ],
      },
      cta: {
        title: "把下一份首页需求交给 elephant.ai",
        description:
          "上传 PRD 或传入现有链接，elephant.ai 会生成可复盘、可审计的首页稿与代码。",
      },
      refs: {
        title: "灵感 / 参考",
      },
    },
  },
  en: {
    badge: "elephant.ai · Frontend orchestrator",
    title: "Ship a homepage that is reviewable from day zero",
    subtitle:
      "elephant.ai runs Plan / Clearify / ReAct as an orchestration loop: layout, copy, states, and evidence stay in sync and auditable.",
    actions: {
      primary: "Start a demo run",
      secondary: "View runs",
      evaluation: "QA dashboard",
      login: "Team login",
    },
    nav: {
      console: "Console",
      sessions: "Runs",
      evaluation: "QA",
      docs: "Manifesto",
    },
    section: {
      hero: {
        highlights: [
          "Explicit Goal → Task → Evidence hierarchy",
          "Copy + layout keep tone parity across locales",
          "Every step leaves QA receipts and screenshots automatically",
        ],
      },
      snapshot: {
        title: "What an elephant.ai run looks like",
        description: "Intent, execution, and evidence live on one canvas you can replay and audit.",
        metrics: [
          { label: "Intent → shippable UI", value: "~3 minutes", hint: "Layout + copy + states" },
          { label: "Locales in sync", value: "English · 中文", hint: "Tone-checked both ways" },
          { label: "Quality gates", value: "a11y · contrast · viewport diffs" },
        ],
        deliverablesLabel: "Deliverables",
        deliverables: [
          "Replayable transcript with evidence links",
          "Homepage spec with tone + messaging hooks",
          "Component mapping + state definitions",
          "Merge-ready Next.js snippet that matches the approved mock",
        ],
      },
      flow: {
        title: "Orchestrated flow",
        description: "Plan / Clearify / ReAct runs on a predictable track you can pause and inspect.",
        timelineLabel: "Timeline",
        logLabel: "Event stream",
        liveLabel: "Autoplay",
      },
      pillars: {
        title: "What elephant.ai brings to homepage work",
        items: [
          {
            icon: Palette,
            title: "Design-system aligned",
            description: "Reads your tokens, grid, radius, and iconography before composing.",
            detail: "Brand fidelity is enforced, not suggested.",
          },
          {
            icon: ScrollText,
            title: "Brief-aware copy",
            description: "Keeps PRD intent and tone aligned across locales.",
            detail: "No filler; each string is reviewable with context.",
          },
          {
            icon: ShieldCheck,
            title: "Guardrailed QA",
            description: "Accessibility, contrast, and responsive diffs run at every step.",
            detail: "Evidence is saved as part of the transcript.",
          },
          {
            icon: GitBranch,
            title: "Handoff without drift",
            description: "Exports code slices that match the approved layout.",
            detail: "Pairs component mapping with interaction notes.",
          },
        ],
      },
      proof: {
        title: "Built for product, design, and engineering to review together",
        bullets: [
          "Product: acceptance bar and risk gates are locked before execution.",
          "Design: rhythm, spacing, and states come from presets, not prompt roulette.",
          "Engineering: reproducible evidence plus copy-paste-ready code slices.",
        ],
      },
      cta: {
        title: "Let elephant.ai run your next homepage brief",
        description:
          "Drop in a PRD or the current URL—elephant.ai redraws it with QA receipts ready for review and merge.",
      },
      refs: {
        title: "Reading list",
      },
    },
  },
};

const flowSteps: Record<HomeLang, FlowStep[]> = {
  zh: [
    {
      stage: "Plan",
      headline: "锁定目标与护栏",
      summary: "提取意图、语气、约束和交付物。",
      highlights: [
        "目标/受众/CTA 先对齐",
        "载入设计系统 Token 与栅格",
        "写下验收与风险门槛",
      ],
      log: [
        "plan.goal = \"清晰展示产品叙事\"",
        "plan.constraints = ['tone','grid','a11y']",
        "contract.acceptance = ['evidence','multi-locale']",
      ],
      accent: "from-emerald-400 via-teal-400 to-cyan-500",
    },
    {
      stage: "Clearify",
      headline: "声明可验收任务",
      summary: "把工作拆成可复盘的段落与状态。",
      highlights: [
        "Hero / Feature / Proof 映射组件",
        "双语文案保持语气一致",
        "状态与间距遵循预设",
      ],
      log: [
        "task.hero = layout.map(component='Hero')",
        "task.copy.parity(locale=['zh','en'])",
        "acceptance = ['状态一致','可溯源']",
      ],
      accent: "from-amber-400 via-orange-400 to-rose-500",
    },
    {
      stage: "ReAct",
      headline: "执行、留痕、交付",
      summary: "执行带 QA 的动作，导出代码与证据。",
      highlights: [
        "自动跑 a11y / contrast / viewport",
        "导出 Next.js 片段 + 组件映射",
        "证据（日志/截图）随任务保存",
      ],
      log: [
        "tool.qa(['a11y','contrast','viewport'])",
        "handoff.export('nextjs')",
        "evidence.attach(logs, screenshots)",
      ],
      accent: "from-indigo-400 via-sky-400 to-emerald-500",
    },
  ],
  en: [
    {
      stage: "Plan",
      headline: "Lock intent and guardrails",
      summary: "Extract goal, tone, constraints, and deliverables.",
      highlights: [
        "Goal / audience / CTA aligned up front",
        "Design system tokens + grid loaded first",
        "Acceptance and risk gates written down",
      ],
      log: [
        "plan.goal = \"Tell the product story\"",
        "plan.constraints = ['tone','grid','a11y']",
        "contract.acceptance = ['evidence','multi-locale']",
      ],
      accent: "from-emerald-400 via-teal-400 to-cyan-500",
    },
    {
      stage: "Clearify",
      headline: "Declare reviewable tasks",
      summary: "Map work to reviewable sections and states.",
      highlights: [
        "Hero / Feature / Proof mapped to components",
        "Copy parity across English / 中文",
        "States and spacing follow presets",
      ],
      log: [
        "task.hero = layout.map(component='Hero')",
        "task.copy.parity(locale=['en','zh'])",
        "acceptance = ['state parity','traceable']",
      ],
      accent: "from-amber-400 via-orange-400 to-rose-500",
    },
    {
      stage: "ReAct",
      headline: "Execute, log, deliver",
      summary: "Run guarded actions, export code and evidence.",
      highlights: [
        "Auto-run a11y / contrast / viewport checks",
        "Export Next.js slice with component mapping",
        "Logs + screenshots saved with the task",
      ],
      log: [
        "tool.qa(['a11y','contrast','viewport'])",
        "handoff.export('nextjs')",
        "evidence.attach(logs, screenshots)",
      ],
      accent: "from-indigo-400 via-sky-400 to-emerald-500",
    },
  ],
};

function HomeTopBar({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-2xl border border-border bg-card/70 backdrop-blur">
          <Sparkles className="h-5 w-5 text-foreground/90" aria-hidden />
        </div>
        <div>
          <div className="text-sm font-semibold text-foreground">Raphael</div>
          <div className="text-xs text-muted-foreground">{t.nav.docs}</div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="inline-flex rounded-full border border-border bg-background/60 p-1 text-xs font-semibold backdrop-blur">
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
          <Button size="sm" className="rounded-full">
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

function HeroHighlights({ items }: { items: string[] }) {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      {items.map((item) => (
        <div
          key={item}
          className="rounded-xl border border-border/70 bg-background/70 p-3 text-sm text-muted-foreground"
        >
          <span className="mr-2 inline-flex h-5 w-5 items-center justify-center rounded-full bg-emerald-500/10 text-[11px] font-semibold text-emerald-500">
            •
          </span>
          {item}
        </div>
      ))}
    </div>
  );
}

function Hero({ lang }: { lang: HomeLang }) {
  const t = copy[lang];

  return (
    <div className="grid gap-8 lg:grid-cols-[1.1fr,0.9fr] lg:items-center">
      <div className="space-y-6">
        <div className="inline-flex items-center gap-2 rounded-full border border-border bg-card/70 px-3 py-1 text-[11px] font-semibold text-muted-foreground backdrop-blur">
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
            <Button className="group rounded-xl">
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
        <HeroHighlights items={t.section.hero.highlights} />
      </div>

      <MiniConsolePreview lang={lang} />
    </div>
  );
}

function Snapshot({ lang }: { lang: HomeLang }) {
  const snapshot = copy[lang].section.snapshot;

  return (
    <SectionBlock className="gap-4">
      <div className="flex flex-col gap-2">
        <div className="inline-flex w-fit items-center gap-2 rounded-full border border-border bg-background/70 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
          <ListChecks className="h-4 w-4" aria-hidden />
          {snapshot.title}
        </div>
        <p className="text-sm text-muted-foreground sm:text-base">{snapshot.description}</p>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.1fr,0.9fr]">
        <Card className="bg-card/70 backdrop-blur">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <Palette className="h-4 w-4" aria-hidden />
              {lang === "zh" ? "运行速览" : "Run snapshot"}
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            {snapshot.metrics.map((item) => (
              <div
                key={item.label}
                className="rounded-2xl border border-border/70 bg-background/60 p-4"
              >
                <div className="text-xs font-semibold text-muted-foreground">{item.label}</div>
                <div className="mt-1 text-lg font-semibold text-foreground">{item.value}</div>
                {item.hint ? (
                  <div className="mt-1 text-xs text-muted-foreground">{item.hint}</div>
                ) : null}
              </div>
            ))}
          </CardContent>
        </Card>

        <Card className="bg-card/70 backdrop-blur">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <FileCode2 className="h-4 w-4" aria-hidden />
              {snapshot.deliverablesLabel}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-foreground/90">
            {snapshot.deliverables.map((item) => (
              <div
                key={item}
                className="flex items-start gap-3 rounded-2xl border border-border/70 bg-background/60 p-3"
              >
                <span className="mt-1 inline-flex h-2.5 w-2.5 flex-none items-center justify-center rounded-full bg-emerald-400" />
                <span>{item}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </SectionBlock>
  );
}

function Pillars({ lang }: { lang: HomeLang }) {
  const { title, items } = copy[lang].section.pillars;

  return (
    <SectionBlock className="gap-4">
      <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
        <Sparkles className="h-4 w-4" aria-hidden />
        {title}
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        {items.map((item) => (
          <Card key={item.title} className="bg-card/70 backdrop-blur">
            <CardContent className="space-y-2 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <item.icon className="h-4 w-4 text-foreground/80" aria-hidden />
                {item.title}
              </div>
              <div className="text-sm text-muted-foreground">{item.description}</div>
              <div className="text-xs font-medium text-foreground/70">{item.detail}</div>
            </CardContent>
          </Card>
        ))}
      </div>
    </SectionBlock>
  );
}

function Proof({ lang }: { lang: HomeLang }) {
  const proof = copy[lang].section.proof;

  return (
    <SectionBlock className="gap-3">
      <Card className="bg-card/70 backdrop-blur">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm">
            <ListTodo className="h-4 w-4" aria-hidden />
            {proof.title}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          {proof.bullets.map((item) => (
            <div
              key={item}
              className="flex items-start gap-3 rounded-xl border border-border/60 bg-background/70 p-3"
            >
              <span className="mt-1 inline-flex h-2 w-2 flex-none rounded-full bg-foreground/50" />
              <span>{item}</span>
            </div>
          ))}
        </CardContent>
      </Card>
    </SectionBlock>
  );
}

function CTA({ lang }: { lang: HomeLang }) {
  const cta = copy[lang].section.cta;

  return (
    <SectionBlock className="gap-3">
      <Card className="overflow-hidden bg-gradient-to-r from-foreground via-foreground to-foreground text-background">
        <CardContent className="flex flex-col gap-3 p-6 sm:flex-row sm:items-center sm:justify-between">
          <div className="space-y-1">
            <div className="text-lg font-semibold">{cta.title}</div>
            <div className="text-sm text-background/80">{cta.description}</div>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href="/conversation">
              <Button variant="secondary" className="rounded-full bg-background text-foreground">
                <PlayCircle className="mr-2 h-4 w-4" aria-hidden />
                {lang === "zh" ? "立即生成" : "Start a run"}
              </Button>
            </Link>
            <Link href="/evaluation">
              <Button variant="outline" className="rounded-full border-background/40 text-background">
                {lang === "zh" ? "查看质量报告" : "See QA report"}
              </Button>
            </Link>
          </div>
        </CardContent>
      </Card>
    </SectionBlock>
  );
}

function References({ lang }: { lang: HomeLang }) {
  return (
    <SectionBlock className="gap-3">
      <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
        <ScrollText className="h-4 w-4" aria-hidden />
        {copy[lang].section.refs.title}
      </div>
      <div className="grid gap-2 lg:grid-cols-2">
        {references.map((ref) => (
          <a
            key={ref.href}
            href={ref.href}
            target="_blank"
            rel="noreferrer"
            className="group flex items-start justify-between gap-3 rounded-2xl border border-border/60 bg-card/60 px-4 py-3 text-sm text-muted-foreground backdrop-blur transition hover:border-border hover:bg-card/80 hover:text-foreground"
          >
            <span className="leading-snug">{ref.title}</span>
            <ArrowRight
              className="mt-0.5 h-4 w-4 flex-none opacity-50 transition group-hover:translate-x-0.5 group-hover:opacity-80"
              aria-hidden
            />
          </a>
        ))}
      </div>
    </SectionBlock>
  );
}

function MiniConsolePreview({ lang }: { lang: HomeLang }) {
  const t = copy[lang];

  return (
    <Card className="overflow-hidden bg-card/70 backdrop-blur">
      <CardHeader className="space-y-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <Sparkles className="h-4 w-4" aria-hidden />
          {lang === "zh" ? "Raphael 运行预览" : "Raphael run preview"}
        </CardTitle>
        <p className="text-sm text-muted-foreground">
          {lang === "zh"
            ? "简报、组件库、QA 护栏在一张画布里，随时可回放。"
            : "Brief, system tokens, and QA guardrails live in one canvas you can replay."}
        </p>
      </CardHeader>
      <CardContent className="grid gap-3">
        <MiniPanel
          title={lang === "zh" ? "Brief" : "Brief"}
          lines={[
            lang === "zh"
              ? "hero.goal = \"提升产品认知\""
              : "hero.goal = \"Tell the product story\"",
            lang === "zh"
              ? "tone = \"清晰、可信、可落地\""
              : "tone = \"clear, credible, shippable\"",
          ]}
        />
        <MiniPanel
          title={lang === "zh" ? "Layout" : "Layout"}
          lines={[
            "grid = brand.grid",
            "components = ['Hero', 'FeatureGrid', 'Proof']",
            lang === "zh"
              ? "states = ['idle', 'hover', 'loading']"
              : "states = ['idle', 'hover', 'loading']",
          ]}
        />
        <MiniPanel
          title="QA"
          lines={[
            "qa.check(['a11y','contrast','viewport'])",
            lang === "zh" ? "evidence.attach(logs, screenshots)" : "evidence.attach(logs, screenshots)",
            t.actions.secondary,
          ]}
        />
      </CardContent>
    </Card>
  );
}

function MiniPanel({ title, lines }: { title: string; lines: string[] }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-background/60 p-3 font-mono text-[11px] leading-relaxed text-foreground/90">
      <div className="mb-1 font-sans text-xs font-semibold text-foreground/80">
        {title}
      </div>
      {lines.map((line) => (
        <div key={line} className="whitespace-pre-wrap">
          {line}
        </div>
      ))}
    </div>
  );
}

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  const t = copy[lang];

  return (
    <PageShell padding="none">
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(900px_circle_at_12%_20%,rgba(16,185,129,0.07),transparent_55%),radial-gradient(900px_circle_at_88%_10%,rgba(59,130,246,0.08),transparent_55%),radial-gradient(900px_circle_at_40%_92%,rgba(14,165,233,0.06),transparent_55%)]" />

        <PageContainer className="px-4 py-10 sm:px-6 lg:px-10 lg:py-14">
          <SectionBlock className="gap-8">
            <HomeTopBar lang={lang} />
            <Hero lang={lang} />
          </SectionBlock>

          <Snapshot lang={lang} />

          <SectionBlock className="gap-4">
            <FlowShowcase
              lang={lang}
              copy={t.section.flow}
              steps={flowSteps[lang]}
            />
          </SectionBlock>

          <Pillars lang={lang} />

          <Proof lang={lang} />

          <CTA lang={lang} />

          <References lang={lang} />
        </PageContainer>
      </div>
    </PageShell>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
