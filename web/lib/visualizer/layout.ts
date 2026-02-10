export interface FolderInfo {
  path: string;
  name: string;
  fileCount: number;
  totalLines: number;
  depth: number;
}

export interface BuildingLayout {
  id: string;
  position: [number, number, number];
  height: number;
  width: number;
  folderPath: string;
  color: string;
}

// Golden angle for spiral layout (137.5 degrees)
const GOLDEN_ANGLE = 137.5 * (Math.PI / 180);
const SPACING = 3;

/**
 * Generate spiral layout for buildings using golden angle
 * This creates a natural-looking distribution without overlap
 */
export function generateSpiralLayout(folders: FolderInfo[]): BuildingLayout[] {
  // Sort by importance (file count + line count)
  const sorted = [...folders].sort((a, b) => {
    const scoreA = a.fileCount + a.totalLines / 1000;
    const scoreB = b.fileCount + b.totalLines / 1000;
    return scoreB - scoreA;
  });

  return sorted.map((folder, index) => {
    // Spiral position using golden angle
    const angle = index * GOLDEN_ANGLE;
    const radius = Math.sqrt(index) * SPACING;
    const x = radius * Math.cos(angle);
    const z = radius * Math.sin(angle);

    // Height calculation (logarithmic scale to avoid too tall buildings)
    const height = Math.min(Math.log(folder.fileCount + 1) * 2 + 1, 10);

    // Width calculation (proportional to square root of file count)
    const width = Math.sqrt(folder.fileCount) * 0.3 + 0.5;

    // Default color (will be overridden by heatmap)
    const color = '#88aa77';

    return {
      id: folder.path,
      position: [x, 0, z],
      height,
      width,
      folderPath: folder.path,
      color,
    };
  });
}

/**
 * Calculate building color based on activity heat
 */
export function getHeatColor(activityScore: number): string {
  // activityScore ranges from 0 (cold) to 1 (hot)
  if (activityScore < 0.2) {
    return '#5588aa'; // Cold blue
  } else if (activityScore < 0.5) {
    return '#88aa77'; // Warm green
  } else if (activityScore < 0.8) {
    return '#ddaa44'; // Hot yellow
  } else {
    return '#ff5533'; // Very hot red
  }
}
