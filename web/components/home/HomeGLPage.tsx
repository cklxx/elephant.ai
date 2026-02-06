"use client";

import dynamic from "next/dynamic";
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import type { HomeLang } from "./types";
import { glCopy, glSections } from "./gl/copy";

const GLCanvas = dynamic(
  () => import("./gl/GLCanvas").then((m) => m.GLCanvas),
  { ssr: false },
);

// ── Cooldown (ms) between page transitions ──────────────────
const TRANSITION_COOLDOWN = 800;
const WHEEL_THRESHOLD = 50;
const TOUCH_THRESHOLD = 60;

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

  useEffect(() => {
    const el = spanRef.current;
    if (!el) return;
    el.style.opacity = "0";
    el.style.transform = "translateY(8px)";
    void el.offsetWidth;
    el.style.transition = "opacity 0.5s ease-out, transform 0.5s ease-out";
    el.style.opacity = "1";
    el.style.transform = "translateY(0)";
  }, [index]);

  return (
    <span
      ref={spanRef}
      className="inline-block"
      style={{ color: "#34d399" }}
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

// ── Page indicator dots ─────────────────────────────────────

function PageDots({
  total,
  current,
  onNavigate,
}: {
  total: number;
  current: number;
  onNavigate: (index: number) => void;
}) {
  return (
    <div className="fixed right-6 top-1/2 z-20 flex -translate-y-1/2 flex-col gap-3">
      {Array.from({ length: total }, (_, i) => (
        <button
          key={i}
          onClick={() => onNavigate(i)}
          className="group flex h-6 w-6 items-center justify-center"
          aria-label={`Go to section ${i + 1}`}
        >
          <span
            className="block rounded-full transition-all duration-300"
            style={{
              width: i === current ? 10 : 6,
              height: i === current ? 10 : 6,
              backgroundColor: i === current ? "#34d399" : "rgba(255,255,255,0.3)",
              boxShadow: i === current ? "0 0 12px rgba(52,211,153,0.5)" : "none",
            }}
          />
        </button>
      ))}
    </div>
  );
}

// ── Hero section (page 0) ───────────────────────────────────

function HeroSection({
  lang,
  active,
}: {
  lang: HomeLang;
  active: boolean;
}) {
  const t = glCopy[lang];

  return (
    <div
      className="absolute inset-0 flex flex-col items-center justify-center px-6 transition-all duration-700 ease-out"
      style={{
        opacity: active ? 1 : 0,
        transform: active ? "translateY(0)" : "translateY(-60px)",
        pointerEvents: active ? "auto" : "none",
      }}
    >
      <div className="max-w-2xl space-y-8 text-center">
        <h1
          className="font-mono text-6xl font-bold tracking-tight sm:text-7xl lg:text-8xl"
          style={{
            color: "#fff",
            textShadow: "0 0 40px rgba(52,211,153,0.3), 0 0 80px rgba(52,211,153,0.15)",
          }}
        >
          {t.title}
        </h1>

        <p
          className="font-sans text-lg sm:text-xl"
          style={{ color: "rgba(255,255,255,0.5)" }}
        >
          {t.tagline}
        </p>

        <div className="h-8 font-mono text-lg sm:text-xl">
          <CyclingKeywords keywords={t.keywords} />
        </div>

        <div className="pt-4">
          <a
            href={t.ctaHref}
            className="inline-block rounded-full px-8 py-3 font-sans text-sm font-semibold tracking-wide transition-all hover:scale-105"
            style={{
              color: "#fff",
              border: "1px solid rgba(52,211,153,0.4)",
              boxShadow:
                "0 0 20px rgba(52,211,153,0.15), inset 0 0 20px rgba(52,211,153,0.05)",
              background: "rgba(52,211,153,0.08)",
            }}
          >
            {t.cta}
          </a>
        </div>
      </div>
    </div>
  );
}

// ── Content section (page 1, 2, 3…) ────────────────────────

function ContentSection({
  section,
  active,
  direction,
}: {
  section: { title: string; description: string; points: { title: string; description: string }[] };
  active: boolean;
  direction: "up" | "down";
}) {
  const enterFrom = direction === "down" ? "translateY(80px)" : "translateY(-80px)";

  return (
    <div
      className="absolute inset-0 flex flex-col items-center justify-center px-6 transition-all duration-700 ease-out"
      style={{
        opacity: active ? 1 : 0,
        transform: active ? "translateY(0)" : enterFrom,
        pointerEvents: active ? "auto" : "none",
      }}
    >
      <div className="max-w-3xl space-y-10 text-center">
        <div className="space-y-4">
          <h2
            className="font-mono text-3xl font-bold tracking-tight sm:text-4xl lg:text-5xl"
            style={{
              color: "#fff",
              textShadow: "0 0 30px rgba(52,211,153,0.2)",
            }}
          >
            {section.title}
          </h2>
          <p
            className="mx-auto max-w-xl font-sans text-base sm:text-lg"
            style={{ color: "rgba(255,255,255,0.5)" }}
          >
            {section.description}
          </p>
        </div>

        <div className="grid gap-6 sm:grid-cols-3">
          {section.points.map((point, i) => (
            <div
              key={point.title}
              className="rounded-2xl p-6 text-left transition-all duration-500"
              style={{
                background: "rgba(255,255,255,0.05)",
                border: "1px solid rgba(52,211,153,0.15)",
                transitionDelay: active ? `${i * 100}ms` : "0ms",
                opacity: active ? 1 : 0,
                transform: active ? "translateY(0)" : "translateY(20px)",
              }}
            >
              <h3
                className="mb-2 font-mono text-sm font-semibold tracking-wide"
                style={{ color: "#34d399" }}
              >
                {point.title}
              </h3>
              <p
                className="font-sans text-sm leading-relaxed"
                style={{ color: "rgba(255,255,255,0.6)" }}
              >
                {point.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// ── Main page component ─────────────────────────────────────

export function HomeGLPage({ lang }: { lang: HomeLang }) {
  const sections = glSections[lang];
  const totalPages = 1 + sections.length; // hero + content sections

  const [currentPage, setCurrentPage] = useState(0);
  const [direction, setDirection] = useState<"up" | "down">("down");
  const lastTransition = useRef(0);
  const touchStartY = useRef(0);

  const navigate = useCallback(
    (target: number) => {
      const clamped = Math.max(0, Math.min(totalPages - 1, target));
      if (clamped === currentPage) return;

      const now = Date.now();
      if (now - lastTransition.current < TRANSITION_COOLDOWN) return;
      lastTransition.current = now;

      setDirection(clamped > currentPage ? "down" : "up");
      setCurrentPage(clamped);
    },
    [currentPage, totalPages],
  );

  // Wheel handler
  useEffect(() => {
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      if (Math.abs(e.deltaY) < WHEEL_THRESHOLD) return;
      navigate(currentPage + (e.deltaY > 0 ? 1 : -1));
    };

    window.addEventListener("wheel", onWheel, { passive: false });
    return () => window.removeEventListener("wheel", onWheel);
  }, [currentPage, navigate]);

  // Touch handler
  useEffect(() => {
    const onTouchStart = (e: TouchEvent) => {
      touchStartY.current = e.touches[0].clientY;
    };

    const onTouchEnd = (e: TouchEvent) => {
      const deltaY = touchStartY.current - e.changedTouches[0].clientY;
      if (Math.abs(deltaY) < TOUCH_THRESHOLD) return;
      navigate(currentPage + (deltaY > 0 ? 1 : -1));
    };

    window.addEventListener("touchstart", onTouchStart, { passive: true });
    window.addEventListener("touchend", onTouchEnd, { passive: true });
    return () => {
      window.removeEventListener("touchstart", onTouchStart);
      window.removeEventListener("touchend", onTouchEnd);
    };
  }, [currentPage, navigate]);

  // Keyboard handler
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown" || e.key === " ") {
        e.preventDefault();
        navigate(currentPage + 1);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        navigate(currentPage - 1);
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [currentPage, navigate]);

  return (
    <div
      className="relative h-screen overflow-hidden"
      style={{ background: "#080810" }}
    >
      {/* GL background layer */}
      <GLCanvas />

      {/* Language toggle — top right */}
      <div className="absolute right-16 top-6 z-20">
        <LanguageToggle lang={lang} />
      </div>

      {/* Page indicator dots */}
      <PageDots
        total={totalPages}
        current={currentPage}
        onNavigate={navigate}
      />

      {/* Sections */}
      <div className="relative z-10 h-full">
        <HeroSection lang={lang} active={currentPage === 0} />
        {sections.map((section, i) => (
          <ContentSection
            key={section.title}
            section={section}
            active={currentPage === i + 1}
            direction={direction}
          />
        ))}
      </div>
    </div>
  );
}
