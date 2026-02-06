"use client";

import { useRef, useMemo, useCallback, useEffect } from "react";
import { useFrame, useThree } from "@react-three/fiber";
import * as THREE from "three";
import {
  particleVertexShader,
  particleFragmentShader,
  lineVertexShader,
  lineFragmentShader,
} from "./shaders";

// ── Constants ────────────────────────────────────────────────

const SPREAD = 40; // spatial extent of particle cloud
const CONNECTION_DIST = 5; // max distance for connection lines
const MAX_LINES = 1200;
const MOUSE_RADIUS = 6;
const MOUSE_FORCE = 0.3;
const NOISE_SPEED = 0.08;
const NOISE_AMPLITUDE = 0.02;
const CELL_SIZE = CONNECTION_DIST; // spatial hash cell size
const LINE_UPDATE_INTERVAL = 3; // only rebuild lines every N frames

// Emerald (#34d399) → Teal (#2dd4bf) color range
const COLOR_A = new THREE.Color("#34d399");
const COLOR_B = new THREE.Color("#2dd4bf");

// ── Simple 3D noise (value noise via hash) ───────────────────

function hash(x: number, y: number, z: number): number {
  let h = x * 374761393 + y * 668265263 + z * 1274126177;
  h = ((h ^ (h >> 13)) * 1103515245) | 0;
  return ((h ^ (h >> 16)) & 0x7fffffff) / 0x7fffffff;
}

function noise3d(x: number, y: number, z: number): number {
  const ix = Math.floor(x);
  const iy = Math.floor(y);
  const iz = Math.floor(z);
  const fx = x - ix;
  const fy = y - iy;
  const fz = z - iz;

  // Smoothstep
  const sx = fx * fx * (3 - 2 * fx);
  const sy = fy * fy * (3 - 2 * fy);
  const sz = fz * fz * (3 - 2 * fz);

  const v000 = hash(ix, iy, iz);
  const v100 = hash(ix + 1, iy, iz);
  const v010 = hash(ix, iy + 1, iz);
  const v110 = hash(ix + 1, iy + 1, iz);
  const v001 = hash(ix, iy, iz + 1);
  const v101 = hash(ix + 1, iy, iz + 1);
  const v011 = hash(ix, iy + 1, iz + 1);
  const v111 = hash(ix + 1, iy + 1, iz + 1);

  const c00 = v000 + sx * (v100 - v000);
  const c10 = v010 + sx * (v110 - v010);
  const c01 = v001 + sx * (v101 - v001);
  const c11 = v011 + sx * (v111 - v011);

  const c0 = c00 + sy * (c10 - c00);
  const c1 = c01 + sy * (c11 - c01);

  return c0 + sz * (c1 - c0);
}

// ── Spatial hash grid for O(N*k) neighbor queries ────────────
// Uses numeric keys to avoid per-frame string allocation / GC pressure.

const HASH_PRIME_Y = 73856093;
const HASH_PRIME_Z = 83492791;

function cellKey(cx: number, cy: number, cz: number): number {
  return (cx * 92837111 + cy * HASH_PRIME_Y + cz * HASH_PRIME_Z) | 0;
}

function buildSpatialHash(positions: Float32Array, count: number) {
  const grid = new Map<number, number[]>();
  for (let i = 0; i < count; i++) {
    const cx = Math.floor(positions[i * 3] / CELL_SIZE);
    const cy = Math.floor(positions[i * 3 + 1] / CELL_SIZE);
    const cz = Math.floor(positions[i * 3 + 2] / CELL_SIZE);
    const key = cellKey(cx, cy, cz);
    const bucket = grid.get(key);
    if (bucket) {
      bucket.push(i);
    } else {
      grid.set(key, [i]);
    }
  }
  return grid;
}

/** Collect numeric keys for 27 neighbor cells. Reuses a static array. */
const _neighborKeys = new Int32Array(27);
function getNeighborKeys(cx: number, cy: number, cz: number): Int32Array {
  let idx = 0;
  for (let dx = -1; dx <= 1; dx++) {
    for (let dy = -1; dy <= 1; dy++) {
      for (let dz = -1; dz <= 1; dz++) {
        _neighborKeys[idx++] = cellKey(cx + dx, cy + dy, cz + dz);
      }
    }
  }
  return _neighborKeys;
}

// ── Particle data initialization (deterministic via hash, no Math.random) ──

interface ParticleData {
  offsets: Float32Array;
  baseOffsets: Float32Array;
  scales: Float32Array;
  colors: Float32Array;
}

/**
 * Generate particle data deterministically using hash-based pseudo-random.
 * Avoids Math.random() to satisfy React render purity rules.
 */
function createParticleData(count: number): ParticleData {
  const offsets = new Float32Array(count * 3);
  const baseOffsets = new Float32Array(count * 3);
  const scales = new Float32Array(count);
  const colors = new Float32Array(count * 3);
  const tmpColor = new THREE.Color();

  for (let i = 0; i < count; i++) {
    // Deterministic pseudo-random values from particle index
    const r1 = hash(i, 0, 7919);
    const r2 = hash(i, 1, 7919);
    const r3 = hash(i, 2, 7919);
    const r4 = hash(i, 3, 7919);
    const r5 = hash(i, 4, 7919);

    const x = (r1 - 0.5) * SPREAD;
    const y = (r2 - 0.5) * SPREAD;
    const z = (r3 - 0.5) * SPREAD * 0.6;
    offsets[i * 3] = x;
    offsets[i * 3 + 1] = y;
    offsets[i * 3 + 2] = z;
    baseOffsets[i * 3] = x;
    baseOffsets[i * 3 + 1] = y;
    baseOffsets[i * 3 + 2] = z;

    scales[i] = 1.5 + r4 * 2.5;

    tmpColor.copy(COLOR_A).lerp(COLOR_B, r5);
    colors[i * 3] = tmpColor.r;
    colors[i * 3 + 1] = tmpColor.g;
    colors[i * 3 + 2] = tmpColor.b;
  }
  return { offsets, baseOffsets, scales, colors };
}

// ── Component ────────────────────────────────────────────────

interface ParticleFieldProps {
  count: number;
}

export function ParticleField({ count }: ParticleFieldProps) {
  const pointsRef = useRef<THREE.Points>(null);
  const linesRef = useRef<THREE.LineSegments>(null);
  const mouseRef = useRef(new THREE.Vector2(9999, 9999));
  const mouse3d = useRef(new THREE.Vector3(9999, 9999, 0));
  const { camera, size } = useThree();

  // Deterministic initialization — pure function, safe in useMemo
  const { offsets, baseOffsets, scales, colors } = useMemo(
    () => createParticleData(count),
    [count],
  );

  // Shader material for particles
  const shaderMaterial = useMemo(
    () =>
      new THREE.ShaderMaterial({
        vertexShader: particleVertexShader,
        fragmentShader: particleFragmentShader,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
      }),
    [],
  );

  // Line material
  const lineMaterial = useMemo(
    () =>
      new THREE.ShaderMaterial({
        vertexShader: lineVertexShader,
        fragmentShader: lineFragmentShader,
        transparent: true,
        depthWrite: false,
        blending: THREE.AdditiveBlending,
      }),
    [],
  );

  // Geometry for particles
  const pointsGeometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    geo.setAttribute("instanceOffset", new THREE.BufferAttribute(offsets, 3));
    geo.setAttribute("instanceScale", new THREE.BufferAttribute(scales, 1));
    geo.setAttribute("instanceColor", new THREE.BufferAttribute(colors, 3));
    geo.setAttribute(
      "position",
      new THREE.BufferAttribute(new Float32Array(count * 3), 3),
    );
    return geo;
  }, [offsets, scales, colors, count]);

  // Geometry for connection lines (pre-allocated buffer)
  const lineGeometry = useMemo(() => {
    const geo = new THREE.BufferGeometry();
    const positions = new Float32Array(MAX_LINES * 6);
    const lineColors = new Float32Array(MAX_LINES * 6);
    geo.setAttribute("position", new THREE.BufferAttribute(positions, 3));
    geo.setAttribute("color", new THREE.BufferAttribute(lineColors, 3));
    geo.setDrawRange(0, 0);
    return geo;
  }, []);

  // Track mouse in normalized device coords
  const handlePointerMove = useCallback(
    (e: PointerEvent) => {
      mouseRef.current.x = (e.clientX / size.width) * 2 - 1;
      mouseRef.current.y = -(e.clientY / size.height) * 2 + 1;

      // Unproject to world space at z=0 plane
      mouse3d.current.set(mouseRef.current.x, mouseRef.current.y, 0.5);
      mouse3d.current.unproject(camera);
      const dir = mouse3d.current.sub(camera.position).normalize();
      const distance = -camera.position.z / dir.z;
      mouse3d.current
        .copy(camera.position)
        .add(dir.multiplyScalar(distance));
    },
    [camera, size],
  );

  // Attach/detach pointer listener
  useEffect(() => {
    window.addEventListener("pointermove", handlePointerMove);
    return () => window.removeEventListener("pointermove", handlePointerMove);
  }, [handlePointerMove]);

  // Frame counter for throttled updates
  const frameCount = useRef(0);

  // Animation loop
  useFrame(({ clock }) => {
    const time = clock.getElapsedTime();
    const frame = frameCount.current++;
    const offsetAttr = pointsGeometry.getAttribute("instanceOffset") as THREE.BufferAttribute;
    const posArray = offsetAttr.array as Float32Array;

    // Update particle positions (every frame — cheap)
    for (let i = 0; i < count; i++) {
      const i3 = i * 3;

      // Noise-based drift
      const nx = noise3d(
        baseOffsets[i3] * 0.1 + time * NOISE_SPEED,
        baseOffsets[i3 + 1] * 0.1,
        baseOffsets[i3 + 2] * 0.1,
      );
      const ny = noise3d(
        baseOffsets[i3] * 0.1,
        baseOffsets[i3 + 1] * 0.1 + time * NOISE_SPEED,
        baseOffsets[i3 + 2] * 0.1 + 100,
      );
      const nz = noise3d(
        baseOffsets[i3] * 0.1 + 200,
        baseOffsets[i3 + 1] * 0.1,
        baseOffsets[i3 + 2] * 0.1 + time * NOISE_SPEED,
      );

      posArray[i3] = baseOffsets[i3] + (nx - 0.5) * NOISE_AMPLITUDE * SPREAD;
      posArray[i3 + 1] = baseOffsets[i3 + 1] + (ny - 0.5) * NOISE_AMPLITUDE * SPREAD;
      posArray[i3 + 2] = baseOffsets[i3 + 2] + (nz - 0.5) * NOISE_AMPLITUDE * SPREAD;

      // Mouse repulsion
      const dx = posArray[i3] - mouse3d.current.x;
      const dy = posArray[i3 + 1] - mouse3d.current.y;
      const dist = Math.sqrt(dx * dx + dy * dy);
      if (dist < MOUSE_RADIUS && dist > 0.01) {
        const force = (1 - dist / MOUSE_RADIUS) * MOUSE_FORCE;
        posArray[i3] += (dx / dist) * force;
        posArray[i3 + 1] += (dy / dist) * force;
      }
    }
    offsetAttr.needsUpdate = true;

    // Update connection lines only every N frames (expensive)
    if (frame % LINE_UPDATE_INTERVAL !== 0) return;

    const grid = buildSpatialHash(posArray, count);
    const linePos = lineGeometry.getAttribute("position") as THREE.BufferAttribute;
    const lineCol = lineGeometry.getAttribute("color") as THREE.BufferAttribute;
    const lp = linePos.array as Float32Array;
    const lc = lineCol.array as Float32Array;
    let lineCount = 0;

    // Use numeric pair key (i * count + j) instead of string to avoid GC
    const visited = new Set<number>();

    for (let i = 0; i < count && lineCount < MAX_LINES; i++) {
      const cx = Math.floor(posArray[i * 3] / CELL_SIZE);
      const cy = Math.floor(posArray[i * 3 + 1] / CELL_SIZE);
      const cz = Math.floor(posArray[i * 3 + 2] / CELL_SIZE);
      const neighborKeys = getNeighborKeys(cx, cy, cz);

      for (let n = 0; n < 27; n++) {
        const bucket = grid.get(neighborKeys[n]);
        if (!bucket) continue;
        for (const j of bucket) {
          if (j <= i) continue;
          const pairKey = i * count + j;
          if (visited.has(pairKey)) continue;

          const ddx = posArray[i * 3] - posArray[j * 3];
          const ddy = posArray[i * 3 + 1] - posArray[j * 3 + 1];
          const ddz = posArray[i * 3 + 2] - posArray[j * 3 + 2];
          const d = Math.sqrt(ddx * ddx + ddy * ddy + ddz * ddz);

          if (d < CONNECTION_DIST) {
            visited.add(pairKey);
            const idx = lineCount * 6;
            lp[idx] = posArray[i * 3];
            lp[idx + 1] = posArray[i * 3 + 1];
            lp[idx + 2] = posArray[i * 3 + 2];
            lp[idx + 3] = posArray[j * 3];
            lp[idx + 4] = posArray[j * 3 + 1];
            lp[idx + 5] = posArray[j * 3 + 2];

            const fade = 1 - d / CONNECTION_DIST;
            const r = COLOR_A.r * fade;
            const g = COLOR_A.g * fade;
            const b = COLOR_A.b * fade;
            lc[idx] = r;
            lc[idx + 1] = g;
            lc[idx + 2] = b;
            lc[idx + 3] = r;
            lc[idx + 4] = g;
            lc[idx + 5] = b;

            lineCount++;
            if (lineCount >= MAX_LINES) break;
          }
        }
        if (lineCount >= MAX_LINES) break;
      }
    }

    lineGeometry.setDrawRange(0, lineCount * 2);
    linePos.needsUpdate = true;
    lineCol.needsUpdate = true;
  });

  return (
    <group>
      <points ref={pointsRef} geometry={pointsGeometry} material={shaderMaterial} />
      <lineSegments ref={linesRef} geometry={lineGeometry} material={lineMaterial} />
    </group>
  );
}
