'use client';

import { useRef } from 'react';
import { Mesh } from 'three';
import { useSpring, animated, config } from '@react-spring/three';

interface BuildingProps {
  position: [number, number, number];
  height: number;
  width: number;
  color: string;
  opacity?: number;
  isBuilt?: boolean;
  folderPath: string;
}

export function Building({
  position,
  height,
  width,
  color,
  opacity = 1,
  isBuilt = false,
  folderPath,
}: BuildingProps) {
  const meshRef = useRef<Mesh>(null);

  // Build animation
  const { scale } = useSpring({
    scale: isBuilt ? 1 : 0,
    config: config.wobbly,
  });

  // Generate blocks for voxel stacking
  const numBlocks = Math.floor(height);
  const blocks = Array.from({ length: numBlocks }, (_, i) => i);

  return (
    <group position={position} userData={{ folderPath }}>
      {blocks.map((level) => (
        <animated.mesh
          key={level}
          ref={level === 0 ? meshRef : undefined}
          position={[0, level + 0.5, 0]}
          castShadow
          receiveShadow
          scale-y={scale.to((s) => Math.max(0, s - level * 0.05))}
        >
          <boxGeometry args={[width - 0.05, 1 - 0.05, width - 0.05]} />
          <meshStandardMaterial
            color={color}
            opacity={opacity}
            transparent={opacity < 1}
            roughness={0.7}
            metalness={0.2}
          />
        </animated.mesh>
      ))}

      {/* Base plate */}
      <mesh position={[0, -0.05, 0]} receiveShadow>
        <boxGeometry args={[width + 0.2, 0.1, width + 0.2]} />
        <meshStandardMaterial color="#333333" roughness={0.9} />
      </mesh>
    </group>
  );
}
