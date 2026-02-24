"use client";

import { useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { useScroll } from "@react-three/drei";
import { MathUtils } from "three";
import type { HomeLang } from "../types";
import { glCopy, glSections } from "./copy";

// ── Hero overlay (0–20%) ────────────────────────────────────

function HeroOverlay({ lang }: { lang: HomeLang }) {
  const ref = useRef<HTMLDivElement>(null);
  const scroll = useScroll();
  const smooth = useRef({ opacity: 1, y: 0 });

  useFrame((_, delta) => {
    if (!ref.current) return;
    const s = smooth.current;
    // Visible in 0–15%, fades out 15–20%
    const progress = scroll.offset;
    const targetOpacity = progress < 0.15 ? 1 : 1 - MathUtils.clamp((progress - 0.15) / 0.05, 0, 1);
    const targetY = -progress * 100;

    s.opacity = MathUtils.damp(s.opacity, targetOpacity, 4, delta);
    s.y = MathUtils.damp(s.y, targetY, 4, delta);

    ref.current.style.opacity = String(s.opacity);
    ref.current.style.transform = `translateY(${s.y}px)`;
  });

  const t = glCopy[lang];

  return (
    <div
      ref={ref}
      className="absolute left-0 top-0 flex h-screen w-screen flex-col items-center justify-center px-6"
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

        <div className="flex flex-wrap justify-center gap-3">
          {t.keywords.map((kw) => (
            <span
              key={kw}
              className="rounded-full px-4 py-1.5 font-mono text-sm"
              style={{
                color: "#34d399",
                border: "1px solid rgba(52,211,153,0.3)",
                background: "rgba(52,211,153,0.06)",
              }}
            >
              {kw}
            </span>
          ))}
        </div>

        <div className="pt-4">
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
  );
}

// ── Content section ─────────────────────────────────────────

function ContentSectionOverlay({
  section,
  index,
}: {
  section: (typeof glSections)["en"][number];
  index: number;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const cardRefs = useRef<(HTMLDivElement | null)[]>([]);
  const scroll = useScroll();
  const smooth = useRef({ opacity: 0, y: 40 });
  const cardSmooth = useRef<{ opacity: number; y: number }[]>(
    section.points.map(() => ({ opacity: 0, y: 30 })),
  );

  useFrame((_, delta) => {
    if (!ref.current) return;
    const { from, distance } = section.scrollRange;
    const offset = scroll.offset;

    // Section visibility: fade in first 30% of range, fade out last 20%
    const localProgress = MathUtils.clamp((offset - from) / distance, 0, 1);
    let targetOpacity: number;
    let targetY: number;
    if (localProgress < 0.15) {
      targetOpacity = localProgress / 0.15;
      targetY = (1 - localProgress / 0.15) * 60;
    } else if (localProgress > 0.8) {
      targetOpacity = (1 - localProgress) / 0.2;
      targetY = -(localProgress - 0.8) / 0.2 * 60;
    } else {
      targetOpacity = 1;
      targetY = 0;
    }

    const s = smooth.current;
    s.opacity = MathUtils.damp(s.opacity, targetOpacity, 5, delta);
    s.y = MathUtils.damp(s.y, targetY, 5, delta);

    ref.current.style.opacity = String(Math.max(0, s.opacity));
    ref.current.style.transform = `translateY(${s.y}px)`;

    // Staggered card fade-in
    section.points.forEach((_, i) => {
      const card = cardRefs.current[i];
      if (!card) return;
      const cs = cardSmooth.current[i];
      const cardDelay = 0.05 * i;
      const cardLocal = MathUtils.clamp((localProgress - 0.15 - cardDelay) / 0.2, 0, 1);
      const cardFadeOut = localProgress > 0.8 ? (1 - localProgress) / 0.2 : 1;
      const cardTarget = Math.min(cardLocal, cardFadeOut);

      cs.opacity = MathUtils.damp(cs.opacity, cardTarget, 5, delta);
      cs.y = MathUtils.damp(cs.y, (1 - cardTarget) * 20, 5, delta);

      card.style.opacity = String(Math.max(0, cs.opacity));
      card.style.transform = `translateY(${cs.y}px)`;
    });
  });

  // Position: each section at its scroll range as % of total scroll height
  const topPercent = section.scrollRange.from * 100 * 5; // 5 pages total

  return (
    <div
      ref={ref}
      className="absolute left-0 flex h-screen w-screen flex-col items-center justify-center px-6"
      style={{ top: `${topPercent}vh` }}
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
              ref={(el) => { cardRefs.current[i] = el; }}
              className="rounded-2xl p-6 text-left"
              style={{
                background: "rgba(255,255,255,0.05)",
                border: "1px solid rgba(52,211,153,0.15)",
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

// ── Outro CTA (80–100%) ─────────────────────────────────────

function OutroOverlay({ lang }: { lang: HomeLang }) {
  const ref = useRef<HTMLDivElement>(null);
  const scroll = useScroll();
  const smooth = useRef({ opacity: 0, y: 40 });
  const t = glCopy[lang];

  useFrame((_, delta) => {
    if (!ref.current) return;
    const offset = scroll.offset;
    const localProgress = MathUtils.clamp((offset - 0.8) / 0.2, 0, 1);
    const targetOpacity = localProgress < 0.1 ? localProgress / 0.1 : 1;
    const targetY = (1 - localProgress) * 60;

    const s = smooth.current;
    s.opacity = MathUtils.damp(s.opacity, targetOpacity, 5, delta);
    s.y = MathUtils.damp(s.y, targetY, 5, delta);

    ref.current.style.opacity = String(Math.max(0, s.opacity));
    ref.current.style.transform = `translateY(${s.y}px)`;
  });

  return (
    <div
      ref={ref}
      className="absolute left-0 flex h-screen w-screen flex-col items-center justify-center px-6"
      style={{ top: `${0.8 * 100 * 5}vh` }}
    >
      <div className="max-w-lg space-y-8 text-center">
        <h2
          className="font-mono text-4xl font-bold tracking-tight sm:text-5xl"
          style={{
            color: "#fff",
            textShadow: "0 0 40px rgba(52,211,153,0.3)",
          }}
        >
          {t.title}
        </h2>
        <p
          className="font-sans text-lg"
          style={{ color: "rgba(255,255,255,0.5)" }}
        >
          {t.tagline}
        </p>
        <a
          href={t.ctaHref}
          className="inline-block rounded-full px-10 py-4 font-sans text-base font-semibold tracking-wide transition-all hover:scale-105"
          style={{
            color: "#fff",
            border: "1px solid rgba(52,211,153,0.5)",
            boxShadow: "0 0 30px rgba(52,211,153,0.2), inset 0 0 30px rgba(52,211,153,0.08)",
            background: "rgba(52,211,153,0.12)",
          }}
        >
          {t.cta}
        </a>
      </div>
    </div>
  );
}

// ── Combined overlay ────────────────────────────────────────

export function ScrollOverlay({ lang }: { lang: HomeLang }) {
  const sections = glSections[lang];

  return (
    <>
      <HeroOverlay lang={lang} />
      {sections.map((section, i) => (
        <ContentSectionOverlay key={section.title} section={section} index={i} />
      ))}
      <OutroOverlay lang={lang} />
    </>
  );
}
