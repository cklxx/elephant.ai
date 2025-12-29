import Link from "next/link";
import {
  ArrowRight,
  CheckCircle2,
  Eye,
  FileCode2,
  GitBranch,
  ListChecks,
  ListTodo,
  PlayCircle,
  ScrollText,
  ShieldCheck,
  Wrench,
} from "lucide-react";

import {
  PageContainer,
  PageShell,
  SectionBlock,
} from "@/components/layout/page-shell";
import { FlowShowcase } from "@/components/home/FlowShowcase";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

import type { FlowStep } from "./FlowShowcase";
import type { HomeLang } from "./types";

const references = [
  {
    title: "ReAct: Synergizing Reasoning and Acting in Language Models",
    href: "https://arxiv.org/abs/2210.03629",
  },
  {
    title: "A practical guide to building agents (OpenAI)",
    href: "https://cdn.openai.com/business-guides-and-resources/a-practical-guide-to-building-agents.pdf",
  },
  {
    title: "Function calling (OpenAI API)",
    href: "https://platform.openai.com/docs/guides/function-calling",
  },
  {
    title: "Plan-and-Execute Agents (LangChain)",
    href: "https://blog.langchain.com/planning-agents/",
  },
  {
    title: "Claude Code: Best practices (Anthropic)",
    href: "https://www.anthropic.com/engineering/claude-code-best-practices",
  },
] as const;

const copy = {
  zh: {
    badge: "elephant.ai · Manifesto",
    title: "把 Agent 变成可控、可验收的工程系统",
    subtitle:
      "让 AI 像软件一样工作：可约束、可观测、可复盘，而不是像聊天一样凭运气。",
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
    section: {
      problem: {
        title: "问题不在智商，而在不稳定",
        description:
          "多数 Agent 失败不是模型不够强，而是缺少工程三要素：层级、分离、契约。",
        bullets: [
          "目标/任务层级不清：用户不知道系统现在到底在做什么",
          "计划和执行挤在同一段输出：难调试、易漂移、也难验收",
          "工具调用缺乏统一协议：状态不一致、日志不可追、问题不可复现",
        ],
      },
      beliefs: {
        title: "三条核心信念",
        items: [
          {
            icon: Eye,
            title: "目标可见，计划不可见",
            description:
              "对外只承诺意图与进度；完整计划作为内部控制结构保存，随时可重规划。",
          },
          {
            icon: ListChecks,
            title: "任务必须先声明，再开始行动",
            description:
              "把粒度压到可验收的最小单元：每个任务都有完成标准和证据入口。",
          },
          {
            icon: ShieldCheck,
            title: "工具是契约，不是提示词",
            description:
              "用结构化输入/输出和执行闭环取代“更长的提示词”，把系统变成可控的状态机。",
          },
        ],
      },
      method: {
        title: "Plan + Clearify + ReAct：让编排器掌控节奏",
        description:
          "Plan 对齐意图；Clearify 拆到最小可验收任务；ReAct 在任务内交替推理/行动并留证据。流程的实时演示见下方动态卡片。",
      },
      flow: {
        title: "实时演示：Plan → Clearify → ReAct",
        description:
          "把请求拆成可验收的任务链，直接展示“目标-任务-行动日志”三层证据的更新过程。",
        timelineLabel: "阶段进度",
        logLabel: "行动日志 / 证据",
        liveLabel: "实时播放",
      },
      engineering: {
        title: "工程化落点",
        bullets: [
          {
            icon: ScrollText,
            title: "可观测性",
            desc: "每次工具调用都带上下文和证据。",
          },
          {
            icon: GitBranch,
            title: "可追溯性",
            desc: "任务从声明到完成形成链路，方便复盘/回归。",
          },
          {
            icon: Wrench,
            title: "可重规划性",
            desc: "计划可在内部更新；界面只呈现当前任务与证据。",
          },
          {
            icon: CheckCircle2,
            title: "可协作性",
            desc: "工具定义与边界可文档化、版本化、复用。",
          },
        ],
      },
      oneLiner: {
        title: "一句话",
        description:
          "elephant.ai 不是让模型更会说，而是让智能体更像软件：目标清晰、任务明确、证据可展开、顺序可控、轨迹可复盘。",
      },
      refs: {
        title: "参考",
      },
    },
  },
  en: {
    badge: "elephant.ai · Manifesto",
    title: "Turn agents into a controllable, auditable system",
    subtitle:
      "Make AI behave like software: constrained, observable, and reviewable—rather than a chat that “usually works.”",
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
    section: {
      problem: {
        title: "What breaks agents is not intelligence—it’s instability",
        description:
          "Most failures come from missing engineering primitives: clear hierarchy, separation, and contracts.",
        bullets: [
          "Unclear goal/task hierarchy: users can’t tell what the system is doing",
          "Plan and execution mixed in one blob: hard to debug, easy to drift, hard to review",
          "No unified tool protocol: inconsistent state, untraceable logs, irreproducible issues",
        ],
      },
      beliefs: {
        title: "Three core beliefs",
        items: [
          {
            icon: Eye,
            title: "Show the goal, hide the full plan",
            description:
              "Expose intent and progress; keep the full plan as internal control state for replanning.",
          },
          {
            icon: ListChecks,
            title: "Declare tasks before acting",
            description:
              "Push work down to the smallest reviewable unit: every task has acceptance criteria and evidence.",
          },
          {
            icon: ShieldCheck,
            title: "Tools are contracts, not prompts",
            description:
              "Structured inputs/outputs + execution loop beat “longer prompts”—turn behavior into a state machine.",
          },
        ],
      },
      method: {
        title: "Plan + Clearify + ReAct: keep the orchestrator in control",
        description:
          "Plan aligns intent; Clearify defines the smallest acceptable tasks; ReAct alternates reasoning/actions inside a task and leaves evidence. Watch the live demo below instead of reading static prose.",
      },
      flow: {
        title: "Live demo: Plan → Clearify → ReAct",
        description:
          "Turn a request into a chain of reviewable tasks and show how goal, tasks, and evidence refresh together.",
        timelineLabel: "Timeline",
        logLabel: "Action log / evidence",
        liveLabel: "Live",
      },
      engineering: {
        title: "Engineering outcomes",
        bullets: [
          {
            icon: ScrollText,
            title: "Observability",
            desc: "Every tool call ships with context and evidence.",
          },
          {
            icon: GitBranch,
            title: "Traceability",
            desc: "A full chain from declaration → completion for replay/regression.",
          },
          {
            icon: Wrench,
            title: "Replanning",
            desc: "Plans can change internally; the UI stays on current task + proof.",
          },
          {
            icon: CheckCircle2,
            title: "Collaboration",
            desc: "Tool contracts can be documented, versioned, and reused.",
          },
        ],
      },
      oneLiner: {
        title: "One-liner for the homepage",
        description:
          "elephant.ai doesn’t make models talk better—it makes agents behave like software: clear goals, explicit tasks, expandable evidence, stable sequencing, and postmortem-friendly traces.",
      },
      refs: {
        title: "References",
      },
    },
  },
} as const satisfies Record<HomeLang, unknown>;

const flowSteps: Record<HomeLang, FlowStep[]> = {
  zh: [
    {
      stage: "Plan",
      headline: "Plan：确认目标与约束",
      summary: "抽取意图、约束和交付线索。",
      highlights: [
        "目标：让首页以动态流程为主角",
        "约束：保持可验收粒度、双语同步",
        "输出：编排器可重放的计划节点",
      ],
      log: [
        'plan.goal = "展示 Plan → Clearify → ReAct 的动态流程"',
        "plan.constraints = ['可观测', '可验收', '不靠长文案']",
        "emit(plan.contract)",
      ],
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      stage: "Clearify",
      headline: "Clearify：声明可验收任务",
      summary: "拆到可以一句话验收的粒度。",
      highlights: [
        "任务 1：流程卡片自动播放且可点击切换",
        "任务 2：行动日志面板同步更新证据",
        "任务 3：移除静态讲解，聚焦演示",
      ],
      log: [
        "task[1] = render_flow(autoPlay=true)",
        "task[2] = render_log(source=evidence.flow)",
        "acceptance = ['三步可切换', '日志随步骤刷新']",
      ],
      accent: "from-amber-500 via-orange-500 to-rose-500",
    },
    {
      stage: "ReAct",
      headline: "ReAct：执行并留痕",
      summary: "工具调用驱动界面更新，证据随步骤展开。",
      highlights: [
        "调用 tools.render → 更新流程高亮",
        "调用 tools.log → 记录工具入参/结果",
        "结果：首页成为动态可验收的轨迹",
      ],
      log: [
        "tool.render(target='flow.showcase', step=active)",
        "tool.log(event='action', proof='/evidence/flow')",
        "ui.commit(status='ready for review')",
      ],
      accent: "from-emerald-500 via-teal-500 to-cyan-500",
    },
  ],
  en: [
    {
      stage: "Plan",
      headline: "Plan: lock the goal",
      summary: "Extract intent, constraints, and acceptance hooks.",
      highlights: [
        "Goal: put the live flow front and center",
        "Constraints: reviewable steps, bilingual parity",
        "Output: a replayable contract for the orchestrator",
      ],
      log: [
        'plan.goal = "Show Plan → Clearify → ReAct as motion"',
        "plan.constraints = ['observable', 'reviewable', 'no long prose']",
        "emit(plan.contract)",
      ],
      accent: "from-indigo-500 via-sky-500 to-emerald-500",
    },
    {
      stage: "Clearify",
      headline: "Clearify: declare reviewable tasks",
      summary: "Cut work into one-line acceptance units.",
      highlights: [
        "Task 1: autoplayable flow card with manual switch",
        "Task 2: action log panel mirrors evidence updates",
        "Task 3: remove static explainer in favor of demo",
      ],
      log: [
        "task[1] = render_flow(autoPlay=true)",
        "task[2] = render_log(source=evidence.flow)",
        "acceptance = ['switch anytime', 'log follows step']",
      ],
      accent: "from-amber-500 via-orange-500 to-rose-500",
    },
    {
      stage: "ReAct",
      headline: "ReAct: execute with evidence",
      summary: "Tools drive the UI while proof is captured inline.",
      highlights: [
        "Call tools.render to move the flow highlight",
        "Call tools.log to persist inputs and outputs",
        "Outcome: homepage becomes a dynamic, auditable trace",
      ],
      log: [
        "tool.render(target='flow.showcase', step=active)",
        "tool.log(event='action', proof='/evidence/flow')",
        "ui.commit(status='ready for review')",
      ],
      accent: "from-emerald-500 via-teal-500 to-cyan-500",
    },
  ],
} as const;

function HomeTopBar({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-2xl border border-border bg-card/70 backdrop-blur">
          <FileCode2 className="h-5 w-5 text-foreground/90" aria-hidden />
        </div>
        <div>
          <div className="text-sm font-semibold text-foreground">elephant.ai</div>
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

function Hero({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  const problem = t.section.problem;
  const beliefs = t.section.beliefs;
  const flow = t.section.flow;

  return (
    <div className="grid gap-6 lg:grid-cols-[1.05fr,0.95fr] lg:items-center">
      <div className="space-y-5">
        <div className="flex flex-wrap items-center gap-3">
          <div className="inline-flex items-center gap-2 rounded-full border border-border bg-card/70 px-3 py-1 text-[11px] font-semibold text-muted-foreground backdrop-blur">
            {t.badge}
          </div>
          <div className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
            <ScrollText className="h-3.5 w-3.5" aria-hidden />
            {flow.title}
          </div>
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

        <div className="grid gap-3 lg:grid-cols-2">
          <HighlightCard
            title={problem.title}
            description={problem.description}
            lines={problem.bullets}
            icon={ShieldCheck}
          />
          <HighlightCard
            title={beliefs.title}
            description=""
            lines={beliefs.items.map((item) => `${item.title} · ${item.description}`)}
            icon={ListChecks}
          />
        </div>

        <HeroHighlights items={t.section.hero.highlights} />
      </div>

      <div className="space-y-4">
        <MiniConsolePreview lang={lang} />
        <Card className="bg-card/70 backdrop-blur">
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-sm">
              <PlayCircle className="h-4 w-4" aria-hidden />
              {flow.description}
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-2 sm:grid-cols-3">
            {t.section.hero.highlights.map((item) => (
              <div
                key={item}
                className="rounded-xl border border-border/70 bg-background/60 p-3 text-sm text-muted-foreground"
              >
                {item}
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function MiniConsolePreview({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  const planTitle =
    lang === "zh" ? "Plan（一级：目标）" : "Plan (Level 1: Goal)";
  const taskTitle =
    lang === "zh" ? "Clearify（二级：任务）" : "Clearify (Level 2: Task)";
  const logTitle =
    lang === "zh" ? "ReAct（三级：行动日志）" : "ReAct (Level 3: Action log)";

  return (
    <Card className="overflow-hidden bg-card/70 backdrop-blur">
      <CardHeader className="space-y-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <ListTodo className="h-4 w-4" aria-hidden />
          {t.section.method.title}
        </CardTitle>
        <p className="text-sm text-muted-foreground">
          {t.section.method.description}
        </p>
      </CardHeader>
      <CardContent className="grid gap-3 lg:grid-cols-1">
        <MiniPanel
          title={planTitle}
          lines={[
            lang === "zh"
              ? "Goal: 让 Agent 可控且可验收"
              : "Goal: controllable & auditable agents",
            lang === "zh"
              ? "Steps: 层级清晰 → 契约明确 → 证据留存"
              : "Steps: hierarchy → contracts → evidence",
          ]}
        />
        <MiniPanel
          title={taskTitle}
          lines={[
            lang === "zh"
              ? "Task: 重写首页，更新双语理念文案"
              : "Task: replace homepage with bilingual manifesto",
            lang === "zh"
              ? "Done: 中/英可切换；含 UI 示例；低饱和样式"
              : "Done: zh/en toggle; UI examples; low-sat style",
          ]}
        />
        <MiniPanel
          title={logTitle}
          lines={[
            "tool: file_read web/app/page.tsx",
            "tool: apply_patch web/app/page.tsx",
            lang === "zh"
              ? "evidence: UI 与文案可直接验收"
              : "evidence: UI + copy are reviewable",
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

function HighlightCard({
  title,
  description,
  lines,
  icon: Icon,
}: {
  title: string;
  description?: string;
  lines: string[];
  icon: LucideIcon;
}) {
  return (
    <Card className="bg-card/70 backdrop-blur">
      <CardContent className="space-y-3 p-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <Icon className="h-4 w-4 text-foreground/80" aria-hidden />
          {title}
        </div>
        {description ? (
          <div className="text-sm text-muted-foreground">{description}</div>
        ) : null}
        <ul className="space-y-2 text-sm text-muted-foreground">
          {lines.map((line) => (
            <li key={line} className="flex items-start gap-2">
              <span className="mt-1 inline-flex h-1.5 w-1.5 flex-none rounded-full bg-foreground/50" />
              <span>{line}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

function Bullets({ items }: { items: readonly string[] }) {
  return (
    <ul className="space-y-2 text-sm leading-relaxed text-muted-foreground">
      {items.map((item) => (
        <li key={item} className="flex gap-2">
          <span className="mt-1.5 inline-block h-1.5 w-1.5 flex-none rounded-full bg-foreground/30" />
          <span>{item}</span>
        </li>
      ))}
    </ul>
  );
}

function HomePage({ lang = "en" }: { lang?: HomeLang }) {
  const t = copy[lang];

  return (
    <PageShell padding="none">
      <div className="relative overflow-hidden">
        <div className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(900px_circle_at_12%_20%,rgba(99,102,241,0.08),transparent_55%),radial-gradient(900px_circle_at_88%_10%,rgba(34,211,238,0.08),transparent_55%),radial-gradient(900px_circle_at_40%_92%,rgba(16,185,129,0.06),transparent_55%)]" />

        <PageContainer className="px-4 py-10 sm:px-6 lg:px-10 lg:py-14">
          <SectionBlock className="gap-8">
            <HomeTopBar lang={lang} />
            <Hero lang={lang} />
          </SectionBlock>

          <SectionBlock className="gap-4">
            <FlowShowcase
              lang={lang}
              copy={t.section.flow}
              steps={flowSteps[lang]}
            />
          </SectionBlock>

          <SectionBlock className="gap-4">
            <div className="grid gap-4 lg:grid-cols-2 lg:items-start">
              <Card className="bg-card/70 backdrop-blur">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <ShieldCheck className="h-5 w-5" aria-hidden />
                    {t.section.problem.title}
                  </CardTitle>
                  <p className="text-sm text-muted-foreground">
                    {t.section.problem.description}
                  </p>
                </CardHeader>
                <CardContent>
                  <Bullets items={t.section.problem.bullets} />
                </CardContent>
              </Card>

              <Card className="bg-card/70 backdrop-blur">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <ListChecks className="h-5 w-5" aria-hidden />
                    {t.section.beliefs.title}
                  </CardTitle>
                </CardHeader>
                <CardContent className="grid gap-3">
                  {t.section.beliefs.items.map((item) => (
                    <div
                      key={item.title}
                      className="rounded-2xl border border-border/70 bg-background/60 p-4"
                    >
                      <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                        <item.icon
                          className="h-4 w-4 text-foreground/80"
                          aria-hidden
                        />
                        {item.title}
                      </div>
                      <div className="mt-2 text-sm leading-relaxed text-muted-foreground">
                        {item.description}
                      </div>
                    </div>
                  ))}
                </CardContent>
              </Card>
            </div>
          </SectionBlock>

          <SectionBlock className="gap-4">
            <Card className="bg-card/70 backdrop-blur">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Wrench className="h-5 w-5" aria-hidden />
                  {t.section.engineering.title}
                </CardTitle>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2">
                {t.section.engineering.bullets.map((item) => (
                  <div
                    key={item.title}
                    className="rounded-2xl border border-border/70 bg-background/60 p-4"
                  >
                    <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                      <item.icon
                        className="h-4 w-4 text-foreground/80"
                        aria-hidden
                      />
                      {item.title}
                    </div>
                    <div className="mt-1 text-sm text-muted-foreground">
                      {item.desc}
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>
          </SectionBlock>

          <SectionBlock className="gap-4">
            <Card className="bg-card/70 backdrop-blur">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <ScrollText className="h-5 w-5" aria-hidden />
                  {t.section.oneLiner.title}
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-relaxed text-muted-foreground">
                {t.section.oneLiner.description}
              </CardContent>
            </Card>
          </SectionBlock>

          <SectionBlock className="gap-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <ScrollText className="h-4 w-4" aria-hidden />
              {t.section.refs.title}
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
        </PageContainer>
      </div>
    </PageShell>
  );
}

export function HomeManifestoPage({ lang }: { lang: HomeLang }) {
  return <HomePage lang={lang} />;
}
