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

interface FolderMapProps {
  events: VisualizerEvent[];
  currentEvent: VisualizerEvent | null;
}

export function FolderMap({ events, currentEvent }: FolderMapProps) {
  const [baseFolders, setBaseFolders] = useState<FolderInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [workspace, setWorkspace] = useState<string>('');

  // Load initial folder structure on mount
  useEffect(() => {
    async function loadFolders() {
      try {
        const response = await fetch('/api/visualizer/folders?depth=3');
        const data = await response.json();

        if (data.folders) {
          setBaseFolders(data.folders);
          setWorkspace(data.workspace);
          console.log(`[FolderMap] Loaded ${data.folders.length} folders from ${data.workspace}`);
        }
      } catch (error) {
        console.error('[FolderMap] Failed to load folders:', error);
      } finally {
        setLoading(false);
      }
    }

    loadFolders();
  }, []);

  // Merge base folders with event activity
  const folderStats = useMemo(() => {
    // Count activity per folder
    const activityMap = new Map<string, { count: number; lastActivity?: string }>();

    events.forEach((event) => {
      if (!event.path) return;

      // Extract folder path (remove filename)
      const parts = event.path.split('/');
      const folderPath = parts.slice(0, -1).join('/') || '/';

      if (!activityMap.has(folderPath)) {
        activityMap.set(folderPath, { count: 0 });
      }

      const activity = activityMap.get(folderPath)!;
      activity.count += 1;
      activity.lastActivity = event.timestamp;
    });

    // Merge base folders with activity
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

    // Sort: active first, then by activity count, then by file count
    stats.sort((a, b) => {
      if (a.isActive) return -1;
      if (b.isActive) return 1;
      if (a.activityCount !== b.activityCount) {
        return b.activityCount - a.activityCount;
      }
      return b.fileCount - a.fileCount;
    });

    return stats;
  }, [baseFolders, events, currentEvent]);

  // Calculate visual intensity (0-1 scale)
  const getIntensity = (folder: FolderStats): number => {
    const maxFileCount = Math.max(...folderStats.map((f) => f.fileCount), 1);
    const maxLineCount = Math.max(...folderStats.map((f) => f.totalLines), 1);
    const maxActivity = Math.max(...folderStats.map((f) => f.activityCount), 1);

    const sizeIntensity = (folder.fileCount / maxFileCount) * 0.3 + (folder.totalLines / maxLineCount) * 0.3;
    const activityIntensity = (folder.activityCount / maxActivity) * 0.4;

    return sizeIntensity + activityIntensity;
  };

  // Generate visual style based on intensity
  const getFolderStyle = (folder: FolderStats) => {
    const intensity = getIntensity(folder);
    const hasActivity = folder.activityCount > 0;

    let bgColor: string;
    let borderColor: string;
    let textColor: string;
    let scale = 1;
    let complexity = 1;

    if (folder.isActive) {
      // Currently active folder
      bgColor = 'bg-yellow-400';
      borderColor = 'border-yellow-600';
      textColor = 'text-gray-900';
      scale = 1.1;
      complexity = 3;
    } else if (!hasActivity) {
      // No activity yet - show in gray
      bgColor = 'bg-gray-200';
      borderColor = 'border-gray-300';
      textColor = 'text-gray-700';
      complexity = 1;
    } else if (intensity > 0.7) {
      // High activity
      bgColor = 'bg-purple-600';
      borderColor = 'border-purple-800';
      textColor = 'text-white';
      complexity = 3;
    } else if (intensity > 0.4) {
      // Medium activity
      bgColor = 'bg-blue-600';
      borderColor = 'border-blue-800';
      textColor = 'text-white';
      complexity = 2;
    } else if (intensity > 0.2) {
      // Low activity
      bgColor = 'bg-blue-400';
      borderColor = 'border-blue-600';
      textColor = 'text-white';
      complexity = 1;
    } else {
      // Minimal activity
      bgColor = 'bg-blue-200';
      borderColor = 'border-blue-400';
      textColor = 'text-gray-900';
      complexity = 1;
    }

    return {
      bgColor,
      borderColor,
      textColor,
      scale,
      complexity,
      intensity,
      hasActivity,
    };
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <div className="flex flex-col items-center gap-3">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500" />
          <div>正在扫描代码库...</div>
        </div>
      </div>
    );
  }

  if (folderStats.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        <div className="flex flex-col items-center gap-3">
          <div className="text-4xl">📂</div>
          <div>未找到代码文件夹</div>
          <div className="text-xs text-gray-400">工作目录: {workspace}</div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Workspace info */}
      <div className="text-xs text-gray-500 flex items-center gap-2">
        <span>📍 工作目录:</span>
        <code className="bg-gray-100 px-2 py-1 rounded">{workspace}</code>
        <span>•</span>
        <span>{folderStats.length} 个文件夹</span>
      </div>

      {/* Folder grid */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
        {folderStats.map((folder) => {
          const style = getFolderStyle(folder);

          // Calculate size scale based on folder metrics
          const maxFiles = Math.max(...folderStats.map((f) => f.fileCount), 1);
          const maxLines = Math.max(...folderStats.map((f) => f.totalLines), 1);
          const sizeScore = (folder.fileCount / maxFiles) * 0.5 + (folder.totalLines / maxLines) * 0.5;

          // Visual size: 0.85x (small) to 1.15x (large)
          const visualScale = 0.85 + sizeScore * 0.3;

          return (
            <div
              key={folder.path}
              data-folder-path={folder.path}
              className={`
                relative rounded-lg border-2 p-4 transition-all duration-500
                ${style.bgColor} ${style.borderColor}
                ${folder.isActive ? 'ring-4 ring-yellow-300 animate-pulse' : ''}
                ${style.hasActivity ? 'hover:scale-105 hover:shadow-xl cursor-pointer' : 'opacity-60'}
              `}
              style={{
                transform: `scale(${folder.isActive ? style.scale : visualScale})`,
              }}
              title={folder.path}
            >
              {/* Folder icon with complexity */}
              <div className="text-4xl mb-2">
                {!style.hasActivity ? '📁' : style.complexity === 3 ? '📚' : style.complexity === 2 ? '📁' : '📂'}
              </div>

              {/* Folder name */}
              <div className={`text-sm font-semibold ${style.textColor} truncate mb-1`}>
                {folder.name}
              </div>

              {/* Stats */}
              <div className={`text-xs ${style.textColor} opacity-80 space-y-0.5`}>
                <div>{folder.fileCount} 文件</div>
                <div>{folder.totalLines} 行</div>
                {folder.activityCount > 0 && (
                  <div className="font-semibold">
                    {folder.activityCount} 次访问
                  </div>
                )}
              </div>

              {/* Activity indicator */}
              {folder.isActive && (
                <div className="absolute -top-2 -right-2 w-6 h-6 bg-red-500 rounded-full animate-ping" />
              )}

              {/* Pattern overlay for high intensity */}
              {style.intensity > 0.6 && style.hasActivity && (
                <div
                  className="absolute inset-0 opacity-20 rounded-lg pointer-events-none"
                  style={{
                    backgroundImage: `repeating-linear-gradient(
                      45deg,
                      transparent,
                      transparent 10px,
                      rgba(255,255,255,0.1) 10px,
                      rgba(255,255,255,0.1) 20px
                    )`,
                  }}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
