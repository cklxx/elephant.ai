"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import Image from "next/image";
type HomeLang = "en" | "zh";

/* ================================================================
   Copy – bilingual content
   ================================================================ */

const copy = {
  en: {
    nav: "elephant.ai",
    hero: {
      headline: "Your leader agent\nin Lark.",
      sub: "Tracks what's moving. Surfaces what matters. Follows up automatically. Only pulls you in when judgment is needed.",
      cta: "Get Started",
      ctaHref: "/conversation",
      ctaSec: "See how it works",
    },
    showcase: {
      label: "Project showcase",
      title: "One unified brand surface.",
      desc: "Homepage banner and promo video generated from this repository and deployed directly on GitHub Pages.",
      bannerAlt: "elephant.ai project banner",
      videoTitle: "Project promo video",
      videoDesc: "20-second overview of product value and architecture.",
    },
    how: {
      label: "How it works",
      title: "Hand it off. Stay in flow.",
      steps: [
        { num: "01", title: "Add to Lark", desc: "Invite elephant.ai into any group chat or DM. It becomes a persistent member — always listening, always ready." },
        { num: "02", title: "Delegate", desc: "Describe what you need. It breaks down the work, picks the right tools, and starts executing." },
        { num: "03", title: "Stay informed", desc: "Get progress updates, summaries, and escalations — only when they matter. Everything else runs in the background." },
      ],
    },
    features: {
      label: "Why a leader agent",
      title: "Less overhead. More momentum.",
      items: [
        { icon: "📌", title: "Continuous ownership", desc: "Hand off a task and it stays tracked. No more 'where did this go?' — the leader agent keeps working until it's done." },
        { icon: "🔇", title: "Attention gating", desc: "Compresses noise into what matters now, which risks are growing, and what needs your call. Everything else is handled." },
        { icon: "🔄", title: "Proactive follow-up", desc: "Chases status, summarizes results, escalates blockers. You don't have to ask 'any updates?' ever again." },
        { icon: "🤝", title: "Coordination on your behalf", desc: "The real work tax is track, ask, remind, align. The leader agent absorbs that coordination overhead." },
        { icon: "🧠", title: "Persistent memory", desc: "Remembers every conversation, decision, and preference across weeks and months. Context compounds over time." },
        { icon: "🛡️", title: "Approval gates", desc: "Knows when to ask before acting. Risky operations require explicit human sign-off." },
      ],
    },
    stats: [
      { value: "15+", label: "Built-in Skills" },
      { value: "8+", label: "LLM Providers" },
      { value: "6", label: "Delivery Channels" },
      { value: "100%", label: "Auditable" },
    ],
    arch: {
      label: "Architecture",
      title: "Built for production.",
      desc: "Clean layered architecture — delivery surfaces on top, domain logic in the middle, infrastructure adapters at the bottom. Every component is testable, swappable, and observable.",
      layers: [
        { title: "Delivery", items: ["CLI", "Lark", "WeChat", "Web Dashboard", "API Server"] },
        { title: "Application", items: ["Coordination", "Context Assembly", "Cost Control", "Preparation"] },
        { title: "Domain", items: ["ReAct Loop", "Typed Events", "Approval Gates", "Workflow Engine"] },
        { title: "Infrastructure", items: ["Multi-LLM", "Memory Store", "Tool Registry", "Observability"] },
      ],
    },
    llm: {
      label: "Multi-provider",
      title: "Your model, your choice.",
      desc: "Switch between providers with a single command. No vendor lock-in, no code changes.",
      providers: ["OpenAI", "Claude", "DeepSeek", "Doubao (ARK)", "Kimi", "Ollama", "Codex", "Qwen"],
    },
    cta: {
      title: "Ready for a leader agent?",
      desc: "Add elephant.ai to your Lark workspace. Start delegating in minutes.",
      button: "Get Started",
      href: "/conversation",
    },
  },
  zh: {
    nav: "elephant.ai",
    hero: {
      headline: "你的 Leader Agent，\n常驻飞书。",
      sub: "持续盯住在推进中的工作，帮你筛出真正重要的事，自动跟进、汇总、预警，只在需要你判断时才来打扰你。",
      cta: "开始使用",
      ctaHref: "/conversation",
      ctaSec: "了解工作方式",
    },
    showcase: {
      label: "项目展示",
      title: "统一风格的一体化主页素材。",
      desc: "首页 Banner 与宣传视频都来自本仓库，直接用于 GitHub Pages 部署首页。",
      bannerAlt: "elephant.ai 项目横幅",
      videoTitle: "项目宣传视频",
      videoDesc: "20 秒展示产品价值和分层架构。",
    },
    how: {
      label: "工作方式",
      title: "交出去，继续专注。",
      steps: [
        { num: "01", title: "加入飞书", desc: "把 elephant.ai 邀请到任何群聊或私信。它成为常驻成员——持续在线，随时待命。" },
        { num: "02", title: "委派任务", desc: "描述你的需求。它拆解工作、选择工具、开始执行。" },
        { num: "03", title: "按需关注", desc: "收到进展汇总、风险预警、需要拍板的决策——其余一切在后台运行。" },
      ],
    },
    features: {
      label: "为什么需要 Leader Agent",
      title: "少操心，多推进。",
      items: [
        { icon: "📌", title: "持续接手", desc: "交出去的事不会掉。不再问「这事到哪了？」——leader agent 一直盯着，直到做完。" },
        { icon: "🔇", title: "注意力守门", desc: "把信息压缩成：现在该看什么、哪个风险在变大、哪件事需要你拍板。其余全部消化。" },
        { icon: "🔄", title: "主动推进", desc: "自动催状态、汇总结果、卡住就升级。你再也不用问「进展呢？」" },
        { icon: "🤝", title: "代表你协调", desc: "很多工作本质上不是「做」，是「盯、问、催、对齐」。leader agent 替你扛住这部分。" },
        { icon: "🧠", title: "持续记忆", desc: "跨越数周数月，记住每一次对话、决策和偏好。上下文随时间积累，越用越懂你。" },
        { icon: "🛡️", title: "审批门控", desc: "知道什么时候该问你。敏感操作需要明确的人工确认。" },
      ],
    },
    stats: [
      { value: "15+", label: "内置技能" },
      { value: "8+", label: "LLM 供应商" },
      { value: "6", label: "交付通道" },
      { value: "100%", label: "全程可审计" },
    ],
    arch: {
      label: "架构",
      title: "为生产环境而生。",
      desc: "清晰的分层架构——交付层在上，领域逻辑在中，基础设施适配器在下。每个组件可测试、可替换、可观测。",
      layers: [
        { title: "交付层", items: ["CLI", "飞书", "微信", "Web 控制台", "API 服务器"] },
        { title: "应用层", items: ["协调", "上下文组装", "成本控制", "预处理"] },
        { title: "领域层", items: ["ReAct 循环", "类型化事件", "审批门控", "工作流引擎"] },
        { title: "基础设施", items: ["多 LLM", "记忆存储", "工具注册表", "可观测性"] },
      ],
    },
    llm: {
      label: "多供应商",
      title: "你的模型，你的选择。",
      desc: "一条命令切换供应商。无厂商锁定，无代码改动。",
      providers: ["OpenAI", "Claude", "DeepSeek", "豆包 (ARK)", "Kimi", "Ollama", "Codex", "通义千问"],
    },
    cta: {
      title: "准备好让 Leader Agent 上岗了吗？",
      desc: "把 elephant.ai 添加到飞书工作区，几分钟内开始委派任务。",
      button: "开始使用",
      href: "/conversation",
    },
  },
};

/* ================================================================
   Scroll reveal hook — IntersectionObserver, fire-once
   ================================================================ */

function useScrollReveal() {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const prefersReduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const els = container.querySelectorAll<HTMLElement>("[data-anim]");

    if (prefersReduced) {
      els.forEach((el) => el.classList.add("is-visible"));
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add("is-visible");
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -60px 0px" },
    );

    els.forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  return containerRef;
}

/* ================================================================
   Language toggle
   ================================================================ */

function LangToggle({ lang }: { lang: HomeLang }) {
  return (
    <div className="flex items-center gap-1 text-sm">
      <Link
        href="/zh"
        className="rounded-full px-3 py-1 transition-colors"
        style={{
          color: lang === "zh" ? "#4f46e5" : "#94a3b8",
          backgroundColor: lang === "zh" ? "#eef2ff" : "transparent",
          fontWeight: lang === "zh" ? 600 : 400,
        }}
      >
        中文
      </Link>
      <Link
        href="/"
        className="rounded-full px-3 py-1 transition-colors"
        style={{
          color: lang === "en" ? "#4f46e5" : "#94a3b8",
          backgroundColor: lang === "en" ? "#eef2ff" : "transparent",
          fontWeight: lang === "en" ? 600 : 400,
        }}
      >
        EN
      </Link>
    </div>
  );
}

/* ================================================================
   Page component
   ================================================================ */

export function HomeGLPage({ lang }: { lang: HomeLang }) {
  const t = copy[lang];
  const ref = useScrollReveal();
  const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";
  const withBasePath = (path: string) => `${basePath}${path}`;

  return (
    <div ref={ref} className="relative w-full overflow-x-hidden" style={{ background: "#fafbfc" }}>
      {/* ── Inline animation styles ────────────────────────── */}
      <style>{`
        [data-anim] {
          opacity: 0;
          will-change: opacity, transform;
          transition: opacity 0.8s cubic-bezier(0.28, 0.11, 0.32, 1),
                      transform 0.8s cubic-bezier(0.28, 0.11, 0.32, 1);
        }
        [data-anim="fade-up"] { transform: translateY(24px); }
        [data-anim="scale-in"] { transform: scale(0.95); }
        [data-anim="fade-in"] { transform: none; }
        [data-anim="stagger"] > * {
          opacity: 0;
          transform: translateY(16px);
          transition: opacity 0.6s cubic-bezier(0.28, 0.11, 0.32, 1),
                      transform 0.6s cubic-bezier(0.28, 0.11, 0.32, 1);
        }
        [data-anim].is-visible { opacity: 1; transform: translateY(0) scale(1); }
        [data-anim="stagger"].is-visible > * { opacity: 1; transform: translateY(0); }
        [data-anim="stagger"].is-visible > *:nth-child(1) { transition-delay: 0s; }
        [data-anim="stagger"].is-visible > *:nth-child(2) { transition-delay: 0.08s; }
        [data-anim="stagger"].is-visible > *:nth-child(3) { transition-delay: 0.16s; }
        [data-anim="stagger"].is-visible > *:nth-child(4) { transition-delay: 0.24s; }
        [data-anim="stagger"].is-visible > *:nth-child(5) { transition-delay: 0.32s; }
        [data-anim="stagger"].is-visible > *:nth-child(6) { transition-delay: 0.40s; }
        [data-anim="stagger"].is-visible > *:nth-child(7) { transition-delay: 0.48s; }
        [data-anim="stagger"].is-visible > *:nth-child(8) { transition-delay: 0.56s; }
      `}</style>

      {/* ── Gradient blobs (background decoration) ─────────── */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div
          className="absolute -left-40 -top-40 h-[600px] w-[600px] rounded-full opacity-[0.15] blur-[100px]"
          style={{ background: "#818cf8" }}
        />
        <div
          className="absolute -right-32 top-[40%] h-[500px] w-[500px] rounded-full opacity-[0.12] blur-[100px]"
          style={{ background: "#c084fc" }}
        />
        <div
          className="absolute -bottom-32 left-[30%] h-[400px] w-[400px] rounded-full opacity-[0.10] blur-[100px]"
          style={{ background: "#93c5fd" }}
        />
      </div>

      {/* ── Nav ────────────────────────────────────────────── */}
      <header className="relative z-10 flex items-center justify-between px-6 py-5 sm:px-12">
        <span className="text-base font-bold" style={{ color: "#0f172a" }}>
          {t.nav}
        </span>
        <LangToggle lang={lang} />
      </header>

      {/* ── Hero ───────────────────────────────────────────── */}
      <section className="relative z-10 mx-auto flex max-w-3xl flex-col items-center px-6 pb-28 pt-16 text-center sm:pt-24">
        <h1
          data-anim="fade-up"
          className="whitespace-pre-line text-[clamp(2.5rem,7vw,4.5rem)] font-extrabold leading-[1.05] tracking-tight"
          style={{ color: "#0f172a" }}
        >
          {t.hero.headline}
        </h1>
        <p
          data-anim="fade-up"
          className="mt-6 max-w-xl text-[clamp(1.0625rem,1.5vw,1.25rem)] leading-relaxed"
          style={{ color: "#64748b" }}
        >
          {t.hero.sub}
        </p>
        <div data-anim="fade-up" className="mt-10 flex items-center gap-4">
          <a
            href={t.hero.ctaHref}
            className="rounded-full px-8 py-3 text-sm font-semibold text-white transition-all hover:scale-105"
            style={{
              background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
              boxShadow: "0 4px 14px rgba(99,102,241,0.25)",
            }}
          >
            {t.hero.cta}
          </a>
          <a
            href="#how"
            className="text-sm font-medium transition-colors hover:underline"
            style={{ color: "#6366f1" }}
          >
            {t.hero.ctaSec} ↓
          </a>
        </div>
      </section>

      {/* ── Banner + Video Showcase ───────────────────────── */}
      <section className="relative z-10 mx-auto max-w-6xl px-6 pb-10 sm:pb-14">
        <div data-anim="fade-up" className="text-center">
          <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: "#6366f1" }}>
            {t.showcase.label}
          </span>
          <h2
            className="mt-3 text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
            style={{ color: "#0f172a" }}
          >
            {t.showcase.title}
          </h2>
          <p className="mx-auto mt-4 max-w-3xl text-sm leading-relaxed" style={{ color: "#64748b" }}>
            {t.showcase.desc}
          </p>
        </div>

        <div data-anim="stagger" className="mt-10 grid gap-6 lg:grid-cols-2">
          <article
            className="overflow-hidden rounded-3xl border bg-white/90 shadow-sm"
            style={{ borderColor: "#e2e8f0", boxShadow: "0 16px 40px rgba(15,23,42,0.08)" }}
          >
            <Image
              src={withBasePath("/media/home-banner.png")}
              alt={t.showcase.bannerAlt}
              width={1600}
              height={900}
              className="block h-full w-full object-cover"
              loading="lazy"
            />
          </article>

          <article
            className="rounded-3xl border bg-white/90 p-3 shadow-sm"
            style={{ borderColor: "#e2e8f0", boxShadow: "0 16px 40px rgba(15,23,42,0.08)" }}
          >
            <video
              controls
              playsInline
              preload="metadata"
              poster={withBasePath("/media/home-banner.png")}
              className="w-full rounded-2xl border"
              style={{ borderColor: "#e2e8f0" }}
            >
              <source src={withBasePath("/media/elephant-home-demo.mp4")} type="video/mp4" />
            </video>
            <div className="px-1 pb-1 pt-3">
              <h3 className="text-sm font-bold" style={{ color: "#0f172a" }}>
                {t.showcase.videoTitle}
              </h3>
              <p className="mt-1 text-sm leading-relaxed" style={{ color: "#64748b" }}>
                {t.showcase.videoDesc}
              </p>
            </div>
          </article>
        </div>
      </section>

      {/* ── Stats bar ──────────────────────────────────────── */}
      <section className="relative z-10 border-y" style={{ borderColor: "#e2e8f0" }}>
        <div
          data-anim="stagger"
          className="mx-auto grid max-w-4xl grid-cols-2 gap-px sm:grid-cols-4"
        >
          {t.stats.map((s) => (
            <div key={s.label} className="flex flex-col items-center py-8">
              <span
                className="text-3xl font-extrabold tracking-tight sm:text-4xl"
                style={{ color: "#4f46e5" }}
              >
                {s.value}
              </span>
              <span className="mt-1 text-xs font-medium uppercase tracking-wider" style={{ color: "#94a3b8" }}>
                {s.label}
              </span>
            </div>
          ))}
        </div>
      </section>

      {/* ── How it works ───────────────────────────────────── */}
      <section id="how" className="relative z-10 mx-auto max-w-4xl px-6 py-24 sm:py-32">
        <div data-anim="fade-up" className="text-center">
          <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: "#6366f1" }}>
            {t.how.label}
          </span>
          <h2
            className="mt-3 text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
            style={{ color: "#0f172a" }}
          >
            {t.how.title}
          </h2>
        </div>

        <div data-anim="stagger" className="mt-16 grid gap-8 sm:grid-cols-3">
          {t.how.steps.map((step) => (
            <div key={step.num} className="text-center sm:text-left">
              <span
                className="text-4xl font-extrabold"
                style={{ color: "#e0e7ff" }}
              >
                {step.num}
              </span>
              <h3 className="mt-3 text-lg font-bold" style={{ color: "#0f172a" }}>
                {step.title}
              </h3>
              <p className="mt-2 text-sm leading-relaxed" style={{ color: "#64748b" }}>
                {step.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* ── Features ───────────────────────────────────────── */}
      <section
        className="relative z-10 py-24 sm:py-32"
        style={{ background: "#f1f5f9" }}
      >
        <div className="mx-auto max-w-5xl px-6">
          <div data-anim="fade-up" className="text-center">
            <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: "#6366f1" }}>
              {t.features.label}
            </span>
            <h2
              className="mt-3 text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
              style={{ color: "#0f172a" }}
            >
              {t.features.title}
            </h2>
          </div>

          <div data-anim="stagger" className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {t.features.items.map((f) => (
              <div
                key={f.title}
                className="rounded-2xl border bg-white/70 px-6 py-5 backdrop-blur-sm"
                style={{ borderColor: "#e2e8f0" }}
              >
                <div className="text-2xl">{f.icon}</div>
                <h3 className="mt-3 text-sm font-bold" style={{ color: "#0f172a" }}>
                  {f.title}
                </h3>
                <p className="mt-1.5 text-sm leading-relaxed" style={{ color: "#64748b" }}>
                  {f.desc}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Architecture ───────────────────────────────────── */}
      <section className="relative z-10 mx-auto max-w-4xl px-6 py-24 sm:py-32">
        <div data-anim="fade-up" className="text-center">
          <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: "#6366f1" }}>
            {t.arch.label}
          </span>
          <h2
            className="mt-3 text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
            style={{ color: "#0f172a" }}
          >
            {t.arch.title}
          </h2>
          <p className="mx-auto mt-4 max-w-2xl text-sm leading-relaxed" style={{ color: "#64748b" }}>
            {t.arch.desc}
          </p>
        </div>

        <div data-anim="stagger" className="mt-12 space-y-3">
          {t.arch.layers.map((layer, i) => (
            <div
              key={layer.title}
              className="flex flex-col gap-3 rounded-xl border px-5 py-4 sm:flex-row sm:items-center"
              style={{
                borderColor: "#e2e8f0",
                background: i === 0 ? "#eef2ff" : i === 3 ? "#faf5ff" : "#fff",
              }}
            >
              <span
                className="w-28 shrink-0 text-xs font-bold uppercase tracking-wider"
                style={{ color: i === 0 ? "#4f46e5" : i === 3 ? "#7c3aed" : "#64748b" }}
              >
                {layer.title}
              </span>
              <div className="flex flex-wrap gap-2">
                {layer.items.map((item) => (
                  <span
                    key={item}
                    className="rounded-md border px-2.5 py-1 text-xs font-medium"
                    style={{ color: "#475569", borderColor: "#cbd5e1", background: "rgba(255,255,255,0.6)" }}
                  >
                    {item}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* ── LLM providers ──────────────────────────────────── */}
      <section
        className="relative z-10 py-24 sm:py-32"
        style={{ background: "#f1f5f9" }}
      >
        <div className="mx-auto max-w-4xl px-6 text-center">
          <div data-anim="fade-up">
            <span className="text-xs font-semibold uppercase tracking-widest" style={{ color: "#6366f1" }}>
              {t.llm.label}
            </span>
            <h2
              className="mt-3 text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
              style={{ color: "#0f172a" }}
            >
              {t.llm.title}
            </h2>
            <p className="mx-auto mt-4 max-w-xl text-sm leading-relaxed" style={{ color: "#64748b" }}>
              {t.llm.desc}
            </p>
          </div>

          <div data-anim="stagger" className="mt-10 flex flex-wrap justify-center gap-3">
            {t.llm.providers.map((p) => (
              <span
                key={p}
                className="rounded-full border bg-white/80 px-4 py-2 text-sm font-medium backdrop-blur-sm"
                style={{ color: "#334155", borderColor: "#e2e8f0" }}
              >
                {p}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* ── Final CTA ──────────────────────────────────────── */}
      <section className="relative z-10 mx-auto max-w-2xl px-6 py-28 text-center sm:py-36">
        <div data-anim="fade-up">
          <h2
            className="text-[clamp(1.75rem,4vw,2.75rem)] font-bold leading-tight tracking-tight"
            style={{ color: "#0f172a" }}
          >
            {t.cta.title}
          </h2>
          <p className="mx-auto mt-4 max-w-md text-base leading-relaxed" style={{ color: "#64748b" }}>
            {t.cta.desc}
          </p>
          <a
            href={t.cta.href}
            className="mt-8 inline-block rounded-full px-10 py-3.5 text-sm font-semibold text-white transition-all hover:scale-105"
            style={{
              background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
              boxShadow: "0 4px 14px rgba(99,102,241,0.25)",
            }}
          >
            {t.cta.button}
          </a>
        </div>
      </section>

      {/* ── Footer ─────────────────────────────────────────── */}
      <footer
        className="relative z-10 border-t px-6 py-8"
        style={{ borderColor: "#e2e8f0" }}
      >
        <div className="mx-auto flex max-w-5xl flex-col items-center gap-3 sm:flex-row sm:justify-between">
          <p className="text-xs" style={{ color: "#94a3b8" }}>
            © {new Date().getFullYear()} elephant.ai — Proactive AI Assistant
          </p>
          <div className="flex items-center gap-4">
            <a
              href="https://github.com/cklxx/elephant.ai"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs transition-colors hover:opacity-80"
              style={{ color: "#94a3b8" }}
            >
              GitHub
            </a>
            <a
              href="https://github.com/cklxx/elephant.ai/issues"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs transition-colors hover:opacity-80"
              style={{ color: "#94a3b8" }}
            >
              Issues
            </a>
            <a
              href="https://github.com/cklxx/elephant.ai/blob/main/README.md"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs transition-colors hover:opacity-80"
              style={{ color: "#94a3b8" }}
            >
              Docs
            </a>
            <a
              href="https://opensource.org/licenses/MIT"
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs transition-colors hover:opacity-80"
              style={{ color: "#94a3b8" }}
            >
              MIT License
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
