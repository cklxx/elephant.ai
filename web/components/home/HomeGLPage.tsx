"use client";

import Link from "next/link";
import type { HomeLang } from "./types";

// ── Copy ──────────────────────────────────────────────────────

const copy = {
  en: {
    title: "elephant.ai",
    tagline: "Your AI teammate, always on.",
    sub: "Lives in Lark. Remembers everything. Acts autonomously.",
    cta: "Get Started",
    ctaHref: "/conversation",
    features: [
      { title: "Lives where you work", desc: "Runs inside Lark groups and DMs — nothing to install." },
      { title: "Ships real work", desc: "Research, code, documents — from a message to deliverable." },
      { title: "Remembers everything", desc: "Persistent memory across sessions, weeks, and months." },
      { title: "Safe by design", desc: "Approval gates for risky actions. Full audit trail." },
    ],
  },
  zh: {
    title: "elephant.ai",
    tagline: "你的 AI 队友，永远在线。",
    sub: "住在飞书里。记住一切。自主行动。",
    cta: "开始使用",
    ctaHref: "/conversation",
    features: [
      { title: "住在你的工作流里", desc: "在飞书群聊和私信里直接使用——无需安装。" },
      { title: "交付真实成果", desc: "搜索、写代码、生成文档——从消息到交付物。" },
      { title: "记住一切", desc: "跨会话的持续记忆，跨越数周数月。" },
      { title: "安全可控", desc: "高风险操作需审批。全程可审计。" },
    ],
  },
};

// ── Language toggle ──────────────────────────────────────────

function LangToggle({ lang }: { lang: HomeLang }) {
  return (
    <div className="flex items-center gap-1.5 text-sm">
      <Link
        href="/zh"
        className="rounded-full px-3 py-1 transition-colors"
        style={{
          color: lang === "zh" ? "#4f46e5" : "#94a3b8",
          backgroundColor: lang === "zh" ? "#eef2ff" : "transparent",
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
        }}
      >
        EN
      </Link>
    </div>
  );
}

// ── Main page ────────────────────────────────────────────────

export function HomeGLPage({ lang }: { lang: HomeLang }) {
  const t = copy[lang];

  return (
    <div className="relative min-h-screen w-full" style={{ background: "#fafbfc" }}>
      {/* Subtle gradient blobs */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div
          className="absolute -left-32 -top-32 h-[500px] w-[500px] rounded-full opacity-30 blur-3xl"
          style={{ background: "radial-gradient(circle, #c7d2fe 0%, transparent 70%)" }}
        />
        <div
          className="absolute -right-24 top-1/3 h-[400px] w-[400px] rounded-full opacity-20 blur-3xl"
          style={{ background: "radial-gradient(circle, #ddd6fe 0%, transparent 70%)" }}
        />
        <div
          className="absolute -bottom-24 left-1/3 h-[350px] w-[350px] rounded-full opacity-20 blur-3xl"
          style={{ background: "radial-gradient(circle, #bfdbfe 0%, transparent 70%)" }}
        />
      </div>

      {/* Nav */}
      <header className="relative z-10 flex items-center justify-between px-6 py-5 sm:px-12">
        <span className="font-sans text-lg font-bold" style={{ color: "#1e293b" }}>
          elephant.ai
        </span>
        <LangToggle lang={lang} />
      </header>

      {/* Hero */}
      <section className="relative z-10 flex flex-col items-center px-6 pb-24 pt-20 text-center sm:pt-28">
        <h1
          className="font-sans text-5xl font-extrabold tracking-tight sm:text-6xl lg:text-7xl"
          style={{ color: "#0f172a" }}
        >
          {t.title}
        </h1>

        <p
          className="mt-6 max-w-lg font-sans text-xl sm:text-2xl"
          style={{ color: "#475569" }}
        >
          {t.tagline}
        </p>

        <p
          className="mt-3 max-w-md font-sans text-base"
          style={{ color: "#94a3b8" }}
        >
          {t.sub}
        </p>

        <a
          href={t.ctaHref}
          className="mt-10 inline-block rounded-full px-8 py-3 font-sans text-sm font-semibold tracking-wide text-white transition-all hover:scale-105 hover:shadow-lg"
          style={{
            background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
            boxShadow: "0 4px 14px rgba(99,102,241,0.3)",
          }}
        >
          {t.cta}
        </a>
      </section>

      {/* Features */}
      <section className="relative z-10 mx-auto max-w-4xl px-6 pb-32">
        <div className="grid gap-5 sm:grid-cols-2">
          {t.features.map((f) => (
            <div
              key={f.title}
              className="rounded-2xl border px-6 py-5"
              style={{
                background: "rgba(255,255,255,0.7)",
                borderColor: "#e2e8f0",
                backdropFilter: "blur(8px)",
              }}
            >
              <h3
                className="font-sans text-sm font-semibold uppercase tracking-wider"
                style={{ color: "#6366f1" }}
              >
                {f.title}
              </h3>
              <p className="mt-2 font-sans text-sm leading-relaxed" style={{ color: "#64748b" }}>
                {f.desc}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Footer */}
      <footer className="relative z-10 pb-8 text-center">
        <p className="font-sans text-xs" style={{ color: "#cbd5e1" }}>
          {new Date().getFullYear()} elephant.ai
        </p>
      </footer>
    </div>
  );
}
