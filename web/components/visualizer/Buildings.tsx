'use client';

import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Building } from './Building';
import { generateSpiralLayout, type BuildingLayout } from '@/lib/visualizer/layout';
import { useVisualizerStream } from '@/hooks/useVisualizerStream';
import { HeatmapManager } from '@/lib/visualizer/heatmap';

// Global heatmap manager
const heatmapManager = new HeatmapManager();

export function Buildings() {
  // Fetch folder data
  const { data: foldersData } = useQuery({
    queryKey: ['visualizer-folders'],
    queryFn: async () => {
      const res = await fetch('/api/visualizer/folders?depth=4');
      if (!res.ok) throw new Error('Failed to fetch folders');
      return res.json();
    },
    refetchInterval: 30000, // Refresh every 30 seconds
  });

  // Subscribe to events for heatmap
  const { events } = useVisualizerStream();

  // Generate building layouts
  const buildings = useMemo<BuildingLayout[]>(() => {
    if (!foldersData?.folders) return [];

    // Limit to top 200 folders to maintain performance
    const topFolders = foldersData.folders.slice(0, 200);

    return generateSpiralLayout(topFolders);
  }, [foldersData]);

  // Update heatmap based on events
  useMemo(() => {
    events.forEach((event) => {
      if (event.path) {
        // Extract folder path from file path
        const parts = event.path.split('/');
        const folderPath = parts.slice(0, -1).join('/') || '/';
        heatmapManager.recordActivity(folderPath);
      }
    });
  }, [events]);

  // Apply heatmap colors to buildings
  const buildingsWithHeat = useMemo(() => {
    return buildings.map((building) => {
      const heatScore = heatmapManager.getHeatScore(building.folderPath);
      const color = heatmapManager.getHeatColor(heatScore);
      return {
        ...building,
        color,
        isBuilt: heatScore > 0, // Build animation triggered by first activity
      };
    });
  }, [buildings, events]); // Re-calculate when events change

  if (!foldersData) {
    return (
      <group>
        {/* Loading placeholder */}
        <mesh position={[0, 0.5, 0]}>
          <boxGeometry args={[1, 1, 1]} />
          <meshStandardMaterial color="#888888" />
        </mesh>
      </group>
    );
  }

  return (
    <group>
      {buildingsWithHeat.map((building) => (
        <Building
          key={building.id}
          position={building.position}
          height={building.height}
          width={building.width}
          color={building.color}
          opacity={1}
          isBuilt={building.isBuilt}
          folderPath={building.folderPath}
        />
      ))}
    </group>
  );
}
