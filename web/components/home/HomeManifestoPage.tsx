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
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export type HomeLang = "zh" | "en";

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
          "Plan 对齐意图；Clearify 拆到最小可验收任务；ReAct 在任务内交替推理/行动并留证据。编排器用 Gate 强制顺序：先 Plan → 再 Clearify → 再执行。",
      },
      showcase: {
        badge: "Mock showcase",
        title: "Showcase：合规发布的可验收样板",
        description:
          "主页直接模拟一次完整的发布任务，展示层级、证据与评审方式如何落地。",
        brief: {
          title: "任务简报",
          description:
            "先对齐目标、风险、交付物，再进入执行；所有步骤都可追溯。",
        },
        highlights: [
          { label: "目标", value: "v0.9 上线并可审计" },
          { label: "窗口", value: "48 小时，3 人复核" },
          { label: "入口", value: "CLI + Web 控制台" },
          { label: "风险", value: "10 分钟内可回滚" },
        ],
        deliverablesLabel: "交付物",
        deliverables: [
          "发布 checklist 与责任人",
          "审批记录与证据链接",
          "回归结果与风险摘要",
          "回滚与复盘计划",
        ],
        trace: {
          title: "执行轨迹（Mock）",
          description: "每条记录对应可验收任务，状态和证据一目了然。",
          steps: [
            { title: "Plan", detail: "冻结目标、回滚门槛、复核角色" },
            {
              title: "Clearify",
              detail: "拆分 12 个可验收子任务，绑定证据入口",
            },
            {
              title: "ReAct",
              detail: "按 gate 执行与审批，记录耗时和风险变化",
            },
          ],
        },
        metricsLabel: "评审指标",
        metrics: [
          { label: "任务关闭", value: "12", hint: "全部带证据" },
          { label: "审批次数", value: "5", hint: "含 2 次阻断" },
          { label: "响应时延", value: "1.8s", hint: "中位确认" },
        ],
      },
      ui: {
        title: "HTML UI 示例（可观测 + 可验收）",
        description:
          "UI 不是靠文案分层，而是靠“工具类型”分层：Plan/Task/Action Log 对应一级/二级/三级语义。",
        snippetLabel: "最小示例（HTML）",
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
          "Plan aligns intent once; Clearify defines the smallest acceptable tasks; ReAct alternates reasoning/actions inside a task and leaves evidence. The orchestrator gates order: Plan → Clearify → Execute.",
      },
      showcase: {
        badge: "Mock showcase",
        title: "Showcase: an audit-ready release cutover",
        description:
          "A mocked run on the homepage to demonstrate hierarchy, evidence, and review flow.",
        brief: {
          title: "Run brief",
          description:
            "Align the goal, risks, and deliverables before execution. Every step stays traceable.",
        },
        highlights: [
          { label: "Goal", value: "Ship v0.9 with audit trail" },
          { label: "Window", value: "48 hours, 3 reviewers" },
          { label: "Surface", value: "CLI + Web console" },
          { label: "Risk", value: "Rollback within 10 minutes" },
        ],
        deliverablesLabel: "Deliverables",
        deliverables: [
          "Release checklist with owners",
          "Approval log + evidence links",
          "Regression results and risk summary",
          "Rollback and postmortem plan",
        ],
        trace: {
          title: "Execution trace (mock)",
          description:
            "Each entry maps to a reviewable task with status and evidence.",
          steps: [
            { title: "Plan", detail: "Freeze goals, rollback gates, reviewers" },
            {
              title: "Clearify",
              detail:
                "Split into 12 reviewable tasks with evidence hooks",
            },
            {
              title: "ReAct",
              detail:
                "Run via gates, capture timing and risk changes",
            },
          ],
        },
        metricsLabel: "Review metrics",
        metrics: [
          { label: "Tasks closed", value: "12", hint: "All with evidence" },
          { label: "Approvals", value: "5", hint: "2 blocked" },
          { label: "Ack latency", value: "1.8s", hint: "Median step" },
        ],
      },
      ui: {
        title: "HTML UI example (observable + reviewable)",
        description:
          "UI hierarchy comes from tool types—not prose styling: Plan/Task/Action Log map to Level 1/2/3 semantics.",
        snippetLabel: "Minimal example (HTML)",
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
      </div>

      <MiniConsolePreview lang={lang} />
    </div>
  );
}

function Showcase({ lang }: { lang: HomeLang }) {
  const showcase = copy[lang].section.showcase;

  return (
    <SectionBlock className="gap-4">
      <div className="flex flex-col gap-2">
        <div className="inline-flex w-fit items-center gap-2 rounded-full border border-border bg-background/70 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
          {showcase.badge}
        </div>
        <div className="space-y-2">
          <h2 className="text-xl font-semibold text-foreground sm:text-2xl">
            {showcase.title}
          </h2>
          <p className="text-sm text-muted-foreground sm:text-base">
            {showcase.description}
          </p>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[1.1fr,0.9fr]">
        <Card className="bg-card/70 backdrop-blur">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <ScrollText className="h-4 w-4" aria-hidden />
              {showcase.brief.title}
            </CardTitle>
            <p className="text-sm text-muted-foreground">
              {showcase.brief.description}
            </p>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-3 sm:grid-cols-2">
              {showcase.highlights.map((item) => (
                <div
                  key={item.label}
                  className="rounded-2xl border border-border/70 bg-background/60 p-3"
                >
                  <div className="text-xs font-semibold text-muted-foreground">
                    {item.label}
                  </div>
                  <div className="mt-1 text-sm font-semibold text-foreground">
                    {item.value}
                  </div>
                </div>
              ))}
            </div>
            <div className="rounded-2xl border border-border/70 bg-background/60 p-4">
              <div className="mb-2 text-xs font-semibold text-muted-foreground">
                {showcase.deliverablesLabel}
              </div>
              <ul className="space-y-2 text-sm text-foreground/90">
                {showcase.deliverables.map((item) => (
                  <li key={item} className="flex items-start gap-2">
                    <span className="mt-1.5 inline-block h-1.5 w-1.5 flex-none rounded-full bg-foreground/40" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card/70 backdrop-blur">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm">
              <ListChecks className="h-4 w-4" aria-hidden />
              {showcase.trace.title}
            </CardTitle>
            <p className="text-sm text-muted-foreground">
              {showcase.trace.description}
            </p>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-3">
              {showcase.trace.steps.map((step, index) => (
                <div
                  key={step.title}
                  className="rounded-2xl border border-border/70 bg-background/60 p-3"
                >
                  <div className="flex items-center gap-2 text-xs font-semibold text-foreground/80">
                    <span className="inline-flex h-6 w-6 items-center justify-center rounded-full border border-border bg-card text-[11px] text-foreground/80">
                      {index + 1}
                    </span>
                    {step.title}
                  </div>
                  <div className="mt-2 text-sm text-muted-foreground">
                    {step.detail}
                  </div>
                </div>
              ))}
            </div>
            <div className="rounded-2xl border border-border/70 bg-background/60 p-4">
              <div className="mb-2 text-xs font-semibold text-muted-foreground">
                {showcase.metricsLabel}
              </div>
              <div className="grid gap-3 sm:grid-cols-3">
                {showcase.metrics.map((metric) => (
                  <div
                    key={metric.label}
                    className="rounded-xl border border-border/70 bg-background/70 p-3"
                  >
                    <div className="text-xs text-muted-foreground">
                      {metric.label}
                    </div>
                    <div className="mt-1 text-lg font-semibold text-foreground">
                      {metric.value}
                    </div>
                    <div className="mt-1 text-xs text-muted-foreground">
                      {metric.hint}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </SectionBlock>
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

function HtmlUiExample({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  const snippet = `<!-- Level 1: Goal (Plan) -->
<section data-level="plan">
  <h2>Plan</h2>
  <p>Goal: controllable, auditable agent</p>
  <ol>
    <li>Align intent</li>
    <li>Declare tasks</li>
    <li>Execute with evidence</li>
  </ol>
</section>

<!-- Level 2: Task (Clearify) -->
<section data-level="clearify">
  <h3>Task: Replace homepage</h3>
  <ul>
    <li>Acceptance: bilingual (zh/en)</li>
    <li>Acceptance: includes UI examples</li>
    <li>Acceptance: low-saturation style</li>
  </ul>
</section>

<!-- Level 3: Action Log (ReAct) -->
<section data-level="react-log">
  <p>tool: file_read → web/app/page.tsx</p>
  <p>tool: apply_patch → web/app/page.tsx</p>
  <p>result: UI updated, ready to review</p>
</section>`;

  return (
    <Card className="overflow-hidden bg-card/70 backdrop-blur">
      <CardHeader className="space-y-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <FileCode2 className="h-4 w-4" aria-hidden />
          {t.section.ui.title}
        </CardTitle>
        <p className="text-sm text-muted-foreground">
          {t.section.ui.description}
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 lg:grid-cols-3">
            <ExampleCard
              title="Plan"
              icon={Eye}
              lines={[
              lang === "zh" ? "只呈现目标与进度" : "Expose goal + progress",
              lang === "zh" ? "完整计划放在内部" : "Keep full plan internal",
              ]}
            />
            <ExampleCard
              title="Clearify"
              icon={ListChecks}
              lines={[
              lang === "zh" ? "先声明任务" : "Declare tasks first",
              lang === "zh" ? "每项都有验收标准" : "Every task is reviewable",
              ]}
            />
          <ExampleCard
            title="ReAct"
            icon={Wrench}
            lines={[
              lang === "zh" ? "工具调用留证据" : "Evidence via tool calls",
              lang === "zh" ? "轨迹可复盘" : "Replayable traces",
            ]}
          />
        </div>

        <div>
          <div className="mb-2 text-xs font-semibold text-muted-foreground">
            {t.section.ui.snippetLabel}
          </div>
          <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-2xl border border-border bg-background/60 p-4 font-mono text-[11px] leading-relaxed text-foreground/90">
            {snippet}
          </pre>
        </div>
      </CardContent>
    </Card>
  );
}

function ExampleCard({
  title,
  icon: Icon,
  lines,
}: {
  title: string;
  icon: typeof Eye;
  lines: string[];
}) {
  return (
    <div className="rounded-2xl border border-border/70 bg-background/60 p-4">
      <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
        <Icon className="h-4 w-4 text-foreground/80" aria-hidden />
        {title}
      </div>
      <div className="mt-2 space-y-1 text-sm text-muted-foreground">
        {lines.map((line) => (
          <div key={line}>{line}</div>
        ))}
      </div>
    </div>
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

          <Showcase lang={lang} />

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
            <div className="grid gap-4 lg:grid-cols-2">
              <Card className="bg-card/70 backdrop-blur">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <GitBranch className="h-5 w-5" aria-hidden />
                    {t.section.method.title}
                  </CardTitle>
                  <p className="text-sm text-muted-foreground">
                    {t.section.method.description}
                  </p>
                </CardHeader>
                <CardContent className="space-y-3">
                  <WorkflowRow
                    items={[
                      { label: "Plan", icon: Eye },
                      { label: "Clearify", icon: ListChecks },
                      { label: "ReAct", icon: Wrench },
                    ]}
                  />
                  <div className="rounded-2xl border border-border/70 bg-background/60 p-4 text-sm text-muted-foreground">
                    <div className="flex items-center gap-2 font-semibold text-foreground">
                      <ScrollText className="h-4 w-4" aria-hidden />
                      {lang === "zh"
                        ? "验收标准建议"
                        : "Suggested acceptance criteria"}
                    </div>
                    <Bullets
                      items={[
                        lang === "zh"
                          ? "每个任务都能用一句话验收"
                          : "Every task is reviewable in one line",
                        lang === "zh"
                          ? "每次工具调用都能还原上下文"
                          : "Every tool call is replayable with context",
                        lang === "zh"
                          ? "UI 只呈现“当前任务 + 证据”"
                          : "UI focuses on “current task + evidence”",
                      ]}
                    />
                  </div>
                </CardContent>
              </Card>

              <HtmlUiExample lang={lang} />
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

function WorkflowRow({
  items,
}: {
  items: {
    label: string;
    icon: typeof Eye;
  }[];
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-2xl border border-border/70 bg-background/60 p-3">
      {items.map((item, index) => (
        <div key={item.label} className="flex items-center gap-2">
          <span className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-3 py-1 text-xs font-semibold text-foreground">
            <item.icon className="h-3.5 w-3.5 text-foreground/80" aria-hidden />
            {item.label}
          </span>
          {index < items.length - 1 ? (
            <ArrowRight className="h-4 w-4 text-muted-foreground" aria-hidden />
          ) : null}
        </div>
      ))}
    </div>
  );
}
