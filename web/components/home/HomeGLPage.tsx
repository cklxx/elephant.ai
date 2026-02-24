"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import type { HomeLang } from "./types";

const GLCanvas = dynamic(
  () => import("./gl/GLCanvas").then((m) => m.GLCanvas),
  { ssr: false },
);

// ── Language toggle ─────────────────────────────────────────

function LanguageToggle({ lang }: { lang: HomeLang }) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <Link
        href="/zh"
        className="rounded-full px-3 py-1 transition-colors"
        style={{
          color: lang === "zh" ? "#fff" : "rgba(255,255,255,0.4)",
          backgroundColor: lang === "zh" ? "rgba(255,255,255,0.1)" : "transparent",
        }}
      >
        中文
      </Link>
      <span style={{ color: "rgba(255,255,255,0.2)" }} aria-hidden>
        ·
      </span>
      <Link
        href="/"
        className="rounded-full px-3 py-1 transition-colors"
        style={{
          color: lang === "en" ? "#fff" : "rgba(255,255,255,0.4)",
          backgroundColor: lang === "en" ? "rgba(255,255,255,0.1)" : "transparent",
        }}
      >
        EN
      </Link>
    </div>
  );
}

// ── Main page component ─────────────────────────────────────

export function HomeGLPage({ lang }: { lang: HomeLang }) {
  return (
    <div
      className="relative h-screen w-screen"
      style={{ background: "#080810" }}
    >
      {/* GL + scroll-driven scene */}
      <GLCanvas lang={lang} />

      {/* Language toggle — fixed top right, above everything */}
      <div className="fixed right-16 top-6 z-50">
        <LanguageToggle lang={lang} />
      </div>
    </div>
  );
}
