'use client';

import { Canvas } from '@react-three/fiber';
import { OrbitControls, Grid, Sky, PerspectiveCamera } from '@react-three/drei';
import { Suspense, useMemo } from 'react';
import { Buildings } from './Buildings';
import { CrabWorker } from './CrabWorker';
import { useVisualizerStream } from '@/hooks/useVisualizerStream';

export function ConstructionSite3D() {
  const { currentEvent } = useVisualizerStream();

  // Calculate crab target position based on current event
  const crabTargetPosition = useMemo((): [number, number, number] | null => {
    if (!currentEvent?.path) return null;

    // For now, use a simple hash to map folder path to position
    // In the future, we should query the actual building position
    const parts = currentEvent.path.split('/');
    const folderPath = parts.slice(0, -1).join('/') || '/';

    // Simple hash function for demo
    const hash = folderPath.split('').reduce((acc, char) => {
      return acc + char.charCodeAt(0);
    }, 0);

    const angle = (hash % 360) * (Math.PI / 180);
    const radius = ((hash % 20) + 5) * 0.5;
    const x = radius * Math.cos(angle);
    const z = radius * Math.sin(angle);

    // Height above the building
    const y = 3;

    return [x, y, z];
  }, [currentEvent]);

  return (
    <div className="w-full h-full">
      <Canvas
        shadows
        dpr={[1, 2]}
        gl={{ antialias: true, alpha: false }}
      >
        {/* Camera */}
        <PerspectiveCamera makeDefault position={[20, 15, 20]} fov={60} />

        {/* Lights */}
        <ambientLight intensity={0.4} />
        <directionalLight
          position={[10, 20, 5]}
          intensity={0.8}
          castShadow
          shadow-mapSize={[2048, 2048]}
          shadow-camera-left={-50}
          shadow-camera-right={50}
          shadow-camera-top={50}
          shadow-camera-bottom={-50}
        />

        {/* Environment */}
        <Sky
          distance={450000}
          sunPosition={[100, 20, 100]}
          inclination={0.5}
          azimuth={0.25}
        />

        {/* Ground */}
        <Grid
          infiniteGrid
          cellSize={1}
          cellThickness={0.5}
          sectionSize={5}
          sectionThickness={1}
          fadeDistance={50}
          fadeStrength={1}
          followCamera={false}
        />

        {/* Buildings */}
        <Suspense fallback={null}>
          <Buildings />
        </Suspense>

        {/* Crab Worker */}
        <CrabWorker currentEvent={currentEvent} targetPosition={crabTargetPosition} />

        {/* Controls */}
        <OrbitControls
          enablePan={true}
          enableZoom={true}
          enableRotate={true}
          minPolarAngle={0}
          maxPolarAngle={Math.PI / 2.1}
          minDistance={5}
          maxDistance={100}
          target={[0, 0, 0]}
        />
      </Canvas>
    </div>
  );
}
