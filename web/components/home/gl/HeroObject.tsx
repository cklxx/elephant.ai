"use client";

import { useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { useScroll, Float, MeshDistortMaterial } from "@react-three/drei";
import { MathUtils, Mesh } from "three";

// ── Scroll-range keyframes ──────────────────────────────────

const KEYFRAMES = [
  { at: 0.0, scale: 1.0, distort: 0.15, rotSpeed: 0.15, tiltX: 0 },
  { at: 0.2, scale: 1.2, distort: 0.25, rotSpeed: 0.3, tiltX: 0 },
  { at: 0.4, scale: 1.3, distort: 0.35, rotSpeed: 0.4, tiltX: 0.3 },
  { at: 0.6, scale: 1.5, distort: 0.5, rotSpeed: 0.6, tiltX: 0.5 },
  { at: 0.8, scale: 1.0, distort: 0.15, rotSpeed: 0.15, tiltX: 0 },
];

function lerpKeyframes(offset: number, key: "scale" | "distort" | "rotSpeed" | "tiltX"): number {
  for (let i = 0; i < KEYFRAMES.length - 1; i++) {
    const a = KEYFRAMES[i];
    const b = KEYFRAMES[i + 1];
    if (offset >= a.at && offset <= b.at) {
      const t = (offset - a.at) / (b.at - a.at);
      return MathUtils.lerp(a[key], b[key], t);
    }
  }
  return KEYFRAMES[KEYFRAMES.length - 1][key];
}

// ── Component ───────────────────────────────────────────────

export function HeroObject() {
  const meshRef = useRef<Mesh>(null);
  const scroll = useScroll();
  const smooth = useRef({ scale: 1, distort: 0.15, rotSpeed: 0.15, tiltX: 0 });

  useFrame((_, delta) => {
    if (!meshRef.current) return;

    const offset = scroll.offset;
    const s = smooth.current;
    const lambda = 4;

    s.scale = MathUtils.damp(s.scale, lerpKeyframes(offset, "scale"), lambda, delta);
    s.distort = MathUtils.damp(s.distort, lerpKeyframes(offset, "distort"), lambda, delta);
    s.rotSpeed = MathUtils.damp(s.rotSpeed, lerpKeyframes(offset, "rotSpeed"), lambda, delta);
    s.tiltX = MathUtils.damp(s.tiltX, lerpKeyframes(offset, "tiltX"), lambda, delta);

    meshRef.current.scale.setScalar(s.scale);
    meshRef.current.rotation.y += s.rotSpeed * delta;
    meshRef.current.rotation.x = s.tiltX;
  });

  return (
    <>
      {/* Key lighting: rim light from above-right, fill from left-below */}
      <pointLight position={[4, 4, 6]} intensity={80} color="#818cf8" />
      <pointLight position={[-3, -2, 4]} intensity={40} color="#c084fc" />
      <pointLight position={[0, 0, 8]} intensity={20} color="#e0e7ff" />
      <ambientLight intensity={0.15} />

      <Float speed={1.5} rotationIntensity={0.3} floatIntensity={0.5}>
        <mesh ref={meshRef}>
          <icosahedronGeometry args={[2.5, 12]} />
          <MeshDistortMaterial
            color="#4f46e5"
            emissive="#6366f1"
            emissiveIntensity={0.6}
            metalness={0.3}
            roughness={0.4}
            distort={0.15}
            speed={2}
          />
        </mesh>
      </Float>
    </>
  );
}
