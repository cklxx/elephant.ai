"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { Canvas, useFrame } from "@react-three/fiber";
import { EffectComposer, Bloom } from "@react-three/postprocessing";
import { ScrollControls, Scroll, Sparkles, useScroll } from "@react-three/drei";
import { MathUtils, Group } from "three";
import { getGLCapabilities, type GLCapabilities } from "@/lib/webgl";
import { ParticleField } from "./ParticleField";
import { HeroObject } from "./HeroObject";
import { ScrollOverlay } from "./ScrollOverlay";
import { ScrollProgressIndicator } from "./ScrollProgressIndicator";
import type { HomeLang } from "../types";

// ── CSS fallback for no-WebGL environments ──────────────────

function CSSFallback() {
  return (
    <div
      className="fixed inset-0"
      style={{
        background: "#09090B",
      }}
    >
      <div
        className="absolute inset-0 animate-[gradient_12s_ease-in-out_infinite]"
        style={{
          background: [
            "radial-gradient(600px circle at 20% 30%, rgba(99,102,241,0.15), transparent 60%)",
            "radial-gradient(800px circle at 80% 20%, rgba(192,132,252,0.10), transparent 65%)",
            "radial-gradient(500px circle at 50% 70%, rgba(129,140,248,0.08), transparent 55%)",
          ].join(", "),
        }}
      />
    </div>
  );
}

// ── Parallax wrapper: offsets a group's Y based on scroll ────

function ParallaxGroup({
  children,
  factor,
}: {
  children: React.ReactNode;
  factor: number;
}) {
  const ref = useRef<Group>(null);
  const scroll = useScroll();
  const smoothY = useRef(0);

  useFrame((_, delta) => {
    if (!ref.current) return;
    const targetY = -scroll.offset * factor * 20;
    smoothY.current = MathUtils.damp(smoothY.current, targetY, 4, delta);
    ref.current.position.y = smoothY.current;
  });

  return <group ref={ref}>{children}</group>;
}

// ── Scene content inside ScrollControls ─────────────────────

function Scene({ caps, lang }: { caps: GLCapabilities; lang: HomeLang }) {
  const bgCount = caps.tier === "high" ? 200 : 80;

  return (
    <ScrollControls pages={5} damping={4}>
      {/* 3D scroll-synced layer */}
      <Scroll>
        {/* Background particles — slow parallax */}
        <ParallaxGroup factor={0.3}>
          <ParticleField count={bgCount} />
        </ParallaxGroup>

        {/* Hero 3D object — 1:1 with scroll */}
        <HeroObject />

        {/* Foreground sparkles — fast parallax */}
        <ParallaxGroup factor={1.5}>
          <Sparkles
            count={caps.tier === "high" ? 60 : 20}
            scale={30}
            size={2}
            speed={0.3}
            color="#818cf8"
            opacity={0.35}
          />
        </ParallaxGroup>
      </Scroll>

      {/* DOM overlay layer — scroll-synced */}
      <Scroll html>
        <ScrollOverlay lang={lang} />
      </Scroll>

      {/* Scroll progress indicator (DOM, inside ScrollControls for useScroll access) */}
      <Scroll html>
        <ScrollProgressIndicator />
      </Scroll>

      {/* Post-processing — inside ScrollControls but outside Scroll groups */}
      {caps.bloom && (
        <EffectComposer>
          <Bloom
            luminanceThreshold={0.15}
            luminanceSmoothing={0.8}
            intensity={1.0}
            mipmapBlur
          />
        </EffectComposer>
      )}
    </ScrollControls>
  );
}

// ── Visibility hook: pause Canvas when tab is hidden ─────────

function usePageVisible() {
  const [visible, setVisible] = useState(true);
  useEffect(() => {
    const onChange = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return visible;
}

// ── Main GLCanvas component ─────────────────────────────────

export function GLCanvas({ lang }: { lang: HomeLang }) {
  const caps = getGLCapabilities();
  const visible = usePageVisible();

  if (!caps.webgl) {
    return <CSSFallback />;
  }

  return (
    <div className="absolute inset-0" style={{ background: "#09090B" }}>
      <Canvas
        dpr={[1, caps.dpr]}
        camera={{ position: [0, 0, 30], fov: 60, near: 0.1, far: 100 }}
        gl={{
          alpha: false,
          antialias: false,
          stencil: false,
          powerPreference: "high-performance",
        }}
        frameloop={visible ? "always" : "never"}
        style={{ background: "#09090B" }}
      >
        <Suspense fallback={null}>
          <Scene caps={caps} lang={lang} />
        </Suspense>
      </Canvas>
    </div>
  );
}
