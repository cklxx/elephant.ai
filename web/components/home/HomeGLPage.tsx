"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import type { HomeLang } from "./types";
import { glCopy } from "./gl/copy";

// Dynamic import: Three.js is client-only, excluded from SSR and main bundle
const GLCanvas = dynamic(
  () => import("./gl/GLCanvas").then((m) => m.GLCanvas),
  { ssr: false },
);

// ── Cycling keyword display ─────────────────────────────────

function CyclingKeywords({ keywords }: { keywords: string[] }) {
  const [index, setIndex] = useState(0);
  const spanRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    const interval = setInterval(() => {
      setIndex((prev) => (prev + 1) % keywords.length);
    }, 2400);
    return () => clearInterval(interval);
  }, [keywords.length]);

  // Trigger fade-in animation on index change
  useEffect(() => {
    const el = spanRef.current;
    if (!el) return;
    el.style.opacity = "0";
    el.style.transform = "translateY(8px)";
    // Force reflow
    void el.offsetWidth;
    el.style.transition = "opacity 0.5s ease-out, transform 0.5s ease-out";
    el.style.opacity = "1";
    el.style.transform = "translateY(0)";
  }, [index]);

  return (
    <span
      ref={spanRef}
      className="inline-block"
      style={{
        color: "#34d399",
      }}
    >
      {keywords[index]}
    </span>
  );
}

// ── Language toggle ─────────────────────────────────────────

function LanguageToggle({ lang }: { lang: HomeLang }) {
  return (
    <div className="flex items-center gap-2 text-sm">
      <Link
        href="/zh"
        className="px-3 py-1 rounded-full transition-colors"
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
        className="px-3 py-1 rounded-full transition-colors"
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
  const t = glCopy[lang];

  return (
    <div
      className="relative min-h-screen overflow-hidden"
      style={{ background: "#080810" }}
    >
      {/* GL background layer */}
      <GLCanvas />

      {/* Language toggle — top right */}
      <div className="absolute right-6 top-6 z-10">
        <LanguageToggle lang={lang} />
      </div>

      {/* Content overlay */}
      <div className="pointer-events-none relative z-10 flex min-h-screen flex-col items-center justify-center px-6">
        <div className="max-w-2xl space-y-8 text-center">
          {/* Title */}
          <h1
            className="font-mono text-6xl font-bold tracking-tight sm:text-7xl lg:text-8xl"
            style={{
              color: "#fff",
              textShadow: "0 0 40px rgba(52,211,153,0.3), 0 0 80px rgba(52,211,153,0.15)",
            }}
          >
            {t.title}
          </h1>

          {/* Tagline */}
          <p
            className="font-sans text-lg sm:text-xl"
            style={{ color: "rgba(255,255,255,0.5)" }}
          >
            {t.tagline}
          </p>

          {/* Cycling keywords */}
          <div className="h-8 font-mono text-lg sm:text-xl">
            <CyclingKeywords keywords={t.keywords} />
          </div>

          {/* CTA */}
          <div className="pointer-events-auto pt-4">
            <a
              href={t.ctaHref}
              className="inline-block rounded-full px-8 py-3 font-sans text-sm font-semibold tracking-wide transition-all hover:scale-105"
              style={{
                color: "#fff",
                border: "1px solid rgba(52,211,153,0.4)",
                boxShadow: "0 0 20px rgba(52,211,153,0.15), inset 0 0 20px rgba(52,211,153,0.05)",
                background: "rgba(52,211,153,0.08)",
              }}
            >
              {t.cta}
            </a>
          </div>
        </div>
      </div>

    </div>
  );
}
