'use client';

import { useMemo, useEffect, useState } from 'react';
import { VisualizerEvent } from '@/hooks/useVisualizerStream';

interface FolderInfo {
  path: string;
  name: string;
  fileCount: number;
  totalLines: number;
  depth: number;
}

interface FolderStats extends FolderInfo {
  activityCount: number;
  lastActivity?: string;
  isActive: boolean;
}

interface TreemapRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

interface FolderMapProps {
  events: VisualizerEvent[];
  currentEvent: VisualizerEvent | null;
}

// Simple treemap layout algorithm (squarified)
function layoutTreemap(
  folders: FolderStats[],
  x: number,
  y: number,
  width: number,
  height: number
): Map<string, TreemapRect> {
  const layout = new Map<string, TreemapRect>();

  if (folders.length === 0) return layout;

  // Calculate total size
  const totalSize = folders.reduce((sum, f) => sum + f.fileCount + f.totalLines / 10, 0);

  let currentX = x;
  let currentY = y;
  let rowHeight = 0;
  let rowWidth = 0;

  folders.forEach((folder, index) => {
    const size = folder.fileCount + folder.totalLines / 10;
    const ratio = size / totalSize;
    const area = width * height * ratio;

    // Simple row-based layout
    let rectWidth: number;
    let rectHeight: number;

    if (width > height) {
      // Horizontal layout
      rectWidth = (area / height) * 0.95; // 5% gap
      rectHeight = height * 0.95;

      if (currentX + rectWidth > x + width) {
        // New row
        currentX = x;
        currentY += rectHeight + height * 0.05;
        rectHeight = height * 0.95;
      }
    } else {
      // Vertical layout
      rectHeight = (area / width) * 0.95;
      rectWidth = width * 0.95;

      if (currentY + rectHeight > y + height) {
        // New column
        currentY = y;
        currentX += rectWidth + width * 0.05;
        rectWidth = width * 0.95;
      }
    }

    layout.set(folder.path, {
      x: currentX,
      y: currentY,
      width: Math.max(50, rectWidth),
      height: Math.max(40, rectHeight),
    });

    if (width > height) {
      currentX += rectWidth + width * 0.05;
    } else {
      currentY += rectHeight + height * 0.05;
    }
  });

  return layout;
}

export function FolderTreemap({ events, currentEvent }: FolderMapProps) {
  const [baseFolders, setBaseFolders] = useState<FolderInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [workspace, setWorkspace] = useState<string>('');
  const [dimensions, setDimensions] = useState({ width: 1200, height: 600 });

  // Update dimensions on mount and resize
  useEffect(() => {
    const updateDimensions = () => {
      setDimensions({
        width: window.innerWidth - 100,
        height: window.innerHeight - 250,
      });
    };

    updateDimensions();
    window.addEventListener('resize', updateDimensions);
    return () => window.removeEventListener('resize', updateDimensions);
  }, []);

  // Load initial folder structure
  useEffect(() => {
    async function loadFolders() {
      try {
        const response = await fetch('/api/visualizer/folders?depth=4');
        const data = await response.json();

        if (data.folders) {
          setBaseFolders(data.folders);
          setWorkspace(data.workspace);
          console.log(`[Treemap] Loaded ${data.folders.length} folders`);
        }
      } catch (error) {
        console.error('[Treemap] Failed to load folders:', error);
      } finally {
        setLoading(false);
      }
    }

    loadFolders();
  }, []);

  // Merge base folders with event activity
  const folderStats = useMemo(() => {
    const activityMap = new Map<string, { count: number; lastActivity?: string }>();

    events.forEach((event) => {
      if (!event.path) return;
      const parts = event.path.split('/');
      const folderPath = parts.slice(0, -1).join('/') || '/';

      if (!activityMap.has(folderPath)) {
        activityMap.set(folderPath, { count: 0 });
      }

      const activity = activityMap.get(folderPath)!;
      activity.count += 1;
      activity.lastActivity = event.timestamp;
    });

    const stats: FolderStats[] = baseFolders.map((folder) => {
      const activity = activityMap.get(folder.path) || { count: 0 };
      return {
        ...folder,
        activityCount: activity.count,
        lastActivity: activity.lastActivity,
        isActive: false,
      };
    });

    // Mark active folder
    if (currentEvent?.path) {
      const parts = currentEvent.path.split('/');
      const activeFolderPath = parts.slice(0, -1).join('/') || '/';
      const activeFolder = stats.find((f) => f.path === activeFolderPath);
      if (activeFolder) {
        activeFolder.isActive = true;
      }
    }

    // Sort by size for better layout
    stats.sort((a, b) => {
      const sizeA = a.fileCount + a.totalLines / 10;
      const sizeB = b.fileCount + b.totalLines / 10;
      return sizeB - sizeA;
    });

    return stats;
  }, [baseFolders, events, currentEvent]);

  // Calculate treemap layout
  const layout = useMemo(() => {
    return layoutTreemap(folderStats, 0, 0, dimensions.width, dimensions.height);
  }, [folderStats, dimensions]);

  // Get folder style
  const getFolderStyle = (folder: FolderStats) => {
    const hasActivity = folder.activityCount > 0;
    const maxActivity = Math.max(...folderStats.map((f) => f.activityCount), 1);
    const activityRatio = folder.activityCount / maxActivity;

    let bgColor: string;
    let textColor: string;
    let borderColor: string;

    if (folder.isActive) {
      bgColor = 'rgb(251, 191, 36)'; // yellow-400
      textColor = 'rgb(17, 24, 39)'; // gray-900
      borderColor = 'rgb(217, 119, 6)'; // yellow-600
    } else if (!hasActivity) {
      bgColor = 'rgb(229, 231, 235)'; // gray-200
      textColor = 'rgb(75, 85, 99)'; // gray-600
      borderColor = 'rgb(209, 213, 219)'; // gray-300
    } else if (activityRatio > 0.7) {
      bgColor = 'rgb(147, 51, 234)'; // purple-600
      textColor = 'white';
      borderColor = 'rgb(107, 33, 168)'; // purple-700
    } else if (activityRatio > 0.4) {
      bgColor = 'rgb(37, 99, 235)'; // blue-600
      textColor = 'white';
      borderColor = 'rgb(29, 78, 216)'; // blue-700
    } else if (activityRatio > 0.2) {
      bgColor = 'rgb(96, 165, 250)'; // blue-400
      textColor = 'white';
      borderColor = 'rgb(59, 130, 246)'; // blue-500
    } else {
      bgColor = 'rgb(191, 219, 254)'; // blue-200
      textColor = 'rgb(17, 24, 39)'; // gray-900
      borderColor = 'rgb(147, 197, 253)'; // blue-300
    }

    return { bgColor, textColor, borderColor, hasActivity };
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="flex flex-col items-center gap-3">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500" />
          <div className="text-gray-600">扫描代码库...</div>
        </div>
      </div>
    );
  }

  if (folderStats.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-gray-500">未找到代码文件夹</div>
      </div>
    );
  }

  return (
    <div className="relative w-full h-full">
      {/* Info bar */}
      <div className="absolute top-0 left-0 right-0 bg-white/90 backdrop-blur-sm px-4 py-2 text-xs text-gray-600 flex items-center gap-4 z-10 border-b">
        <span>📍 {workspace}</span>
        <span>•</span>
        <span>{folderStats.length} 个文件夹</span>
        <span>•</span>
        <span className="text-gray-400">面积 = 文件数 + 代码行数</span>
      </div>

      {/* Treemap */}
      <svg
        className="absolute top-10 left-0"
        width={dimensions.width}
        height={dimensions.height}
        style={{ overflow: 'visible' }}
      >
        {folderStats.map((folder) => {
          const rect = layout.get(folder.path);
          if (!rect || rect.width < 30 || rect.height < 30) return null;

          const style = getFolderStyle(folder);
          const size = folder.fileCount + folder.totalLines / 10;

          // Font size based on rectangle size
          const area = rect.width * rect.height;
          const fontSize = area > 20000 ? 14 : area > 10000 ? 12 : area > 5000 ? 10 : 8;

          return (
            <g
              key={folder.path}
              data-folder-path={folder.path}
              className="transition-all duration-300 hover:opacity-90 cursor-pointer"
            >
              {/* Rectangle */}
              <rect
                x={rect.x}
                y={rect.y}
                width={rect.width}
                height={rect.height}
                fill={style.bgColor}
                stroke={style.borderColor}
                strokeWidth={folder.isActive ? 4 : 2}
                rx={4}
                className="transition-all duration-300"
              />

              {/* Pattern for high activity */}
              {folder.activityCount > 0 && (
                <rect
                  x={rect.x}
                  y={rect.y}
                  width={rect.width}
                  height={rect.height}
                  fill="url(#pattern-dots)"
                  opacity={0.15}
                  rx={4}
                  pointerEvents="none"
                />
              )}

              {/* Active indicator */}
              {folder.isActive && (
                <>
                  <circle
                    cx={rect.x + rect.width - 10}
                    cy={rect.y + 10}
                    r={6}
                    fill="rgb(239, 68, 68)"
                    className="animate-ping"
                  />
                  <circle
                    cx={rect.x + rect.width - 10}
                    cy={rect.y + 10}
                    r={4}
                    fill="rgb(239, 68, 68)"
                  />
                </>
              )}

              {/* Text content */}
              {rect.width > 60 && rect.height > 40 && (
                <g>
                  {/* Folder name */}
                  <text
                    x={rect.x + rect.width / 2}
                    y={rect.y + rect.height / 2 - fontSize / 2}
                    fill={style.textColor}
                    fontSize={fontSize}
                    fontWeight="600"
                    textAnchor="middle"
                    pointerEvents="none"
                  >
                    {folder.name.length > 20
                      ? folder.name.substring(0, 17) + '...'
                      : folder.name}
                  </text>

                  {/* Stats */}
                  {rect.height > 60 && (
                    <text
                      x={rect.x + rect.width / 2}
                      y={rect.y + rect.height / 2 + fontSize}
                      fill={style.textColor}
                      fontSize={Math.max(8, fontSize - 2)}
                      opacity={0.8}
                      textAnchor="middle"
                      pointerEvents="none"
                    >
                      {folder.fileCount}个 · {folder.totalLines}行
                    </text>
                  )}

                  {/* Activity count */}
                  {folder.activityCount > 0 && rect.height > 80 && (
                    <text
                      x={rect.x + rect.width / 2}
                      y={rect.y + rect.height / 2 + fontSize * 2.5}
                      fill={style.textColor}
                      fontSize={Math.max(8, fontSize - 2)}
                      fontWeight="700"
                      textAnchor="middle"
                      pointerEvents="none"
                    >
                      🔥 {folder.activityCount}次访问
                    </text>
                  )}
                </g>
              )}

              {/* Tooltip trigger */}
              <title>
                {folder.path}
                {'\n'}
                {folder.fileCount} 个文件，{folder.totalLines} 行代码
                {folder.activityCount > 0 && `\n🔥 ${folder.activityCount} 次访问`}
              </title>
            </g>
          );
        })}

        {/* Pattern definition */}
        <defs>
          <pattern id="pattern-dots" width="10" height="10" patternUnits="userSpaceOnUse">
            <circle cx="5" cy="5" r="1" fill="white" />
          </pattern>
        </defs>
      </svg>
    </div>
  );
}
