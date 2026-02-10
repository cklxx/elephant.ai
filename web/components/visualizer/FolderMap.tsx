'use client';

import { useMemo } from 'react';
import { VisualizerEvent } from '@/hooks/useVisualizerStream';

interface FolderStats {
  path: string;
  fileCount: number;
  lineCount: number;
  lastActivity?: string;
  isActive: boolean;
}

interface FolderMapProps {
  events: VisualizerEvent[];
  currentEvent: VisualizerEvent | null;
}

export function FolderMap({ events, currentEvent }: FolderMapProps) {
  // Calculate folder statistics from events
  const folderStats = useMemo(() => {
    const stats = new Map<string, FolderStats>();

    events.forEach((event) => {
      if (!event.path) return;

      // Extract folder path (remove filename)
      const parts = event.path.split('/');
      const folderPath = parts.slice(0, -1).join('/') || '/';

      if (!stats.has(folderPath)) {
        stats.set(folderPath, {
          path: folderPath,
          fileCount: 0,
          lineCount: 0,
          isActive: false,
        });
      }

      const folder = stats.get(folderPath)!;
      folder.fileCount += 1;
      folder.lastActivity = event.timestamp;

      // Estimate line count based on tool activity
      // (in real implementation, would query actual file stats)
      if (event.tool === 'Read' || event.tool === 'Write') {
        folder.lineCount += 100; // Rough estimate
      }
    });

    // Mark active folder
    if (currentEvent?.path) {
      const parts = currentEvent.path.split('/');
      const activeFolderPath = parts.slice(0, -1).join('/') || '/';
      const activeFolder = stats.get(activeFolderPath);
      if (activeFolder) {
        activeFolder.isActive = true;
      }
    }

    return Array.from(stats.values()).sort((a, b) => {
      // Sort by activity, then by file count
      if (a.isActive) return -1;
      if (b.isActive) return 1;
      return b.fileCount - a.fileCount;
    });
  }, [events, currentEvent]);

  // Calculate visual intensity (0-1 scale)
  const getIntensity = (folder: FolderStats): number => {
    const maxFileCount = Math.max(...folderStats.map((f) => f.fileCount), 1);
    const maxLineCount = Math.max(...folderStats.map((f) => f.lineCount), 1);

    const fileIntensity = folder.fileCount / maxFileCount;
    const lineIntensity = folder.lineCount / maxLineCount;

    return (fileIntensity * 0.4 + lineIntensity * 0.6); // Weight lines more
  };

  // Generate visual style based on intensity
  const getFolderStyle = (folder: FolderStats) => {
    const intensity = getIntensity(folder);

    // Color scale: light blue -> deep blue -> purple
    let bgColor: string;
    let borderColor: string;
    let scale = 1;
    let complexity = 1; // For future pattern complexity

    if (folder.isActive) {
      bgColor = 'bg-yellow-400';
      borderColor = 'border-yellow-600';
      scale = 1.1;
    } else if (intensity > 0.7) {
      bgColor = 'bg-purple-600';
      borderColor = 'border-purple-800';
      complexity = 3;
    } else if (intensity > 0.4) {
      bgColor = 'bg-blue-600';
      borderColor = 'border-blue-800';
      complexity = 2;
    } else if (intensity > 0.2) {
      bgColor = 'bg-blue-400';
      borderColor = 'border-blue-600';
      complexity = 1;
    } else {
      bgColor = 'bg-blue-200';
      borderColor = 'border-blue-400';
      complexity = 1;
    }

    return {
      bgColor,
      borderColor,
      scale,
      complexity,
      intensity,
    };
  };

  if (folderStats.length === 0) {
    return (
      <div className="flex items-center justify-center h-64 text-gray-500">
        等待 Claude Code 活动...
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 p-4">
      {folderStats.map((folder) => {
        const style = getFolderStyle(folder);
        const folderName = folder.path === '/' ? 'root' : folder.path.split('/').pop() || folder.path;

        return (
          <div
            key={folder.path}
            data-folder-path={folder.path}
            className={`
              relative rounded-lg border-2 p-4 transition-all duration-500
              ${style.bgColor} ${style.borderColor}
              ${folder.isActive ? 'ring-4 ring-yellow-300 animate-pulse' : ''}
              hover:scale-105 hover:shadow-xl
            `}
            style={{
              transform: `scale(${style.scale})`,
              opacity: 0.7 + style.intensity * 0.3,
            }}
          >
            {/* Folder icon with complexity */}
            <div className="text-4xl mb-2">
              {style.complexity === 3 ? '📚' : style.complexity === 2 ? '📁' : '📂'}
            </div>

            {/* Folder name */}
            <div className="text-sm font-semibold text-white truncate mb-1">
              {folderName}
            </div>

            {/* Stats */}
            <div className="text-xs text-white/80 space-y-0.5">
              <div>{folder.fileCount} 文件</div>
              <div>~{folder.lineCount} 行</div>
            </div>

            {/* Activity indicator */}
            {folder.isActive && (
              <div className="absolute -top-2 -right-2 w-6 h-6 bg-red-500 rounded-full animate-ping" />
            )}

            {/* Pattern overlay for high intensity */}
            {style.intensity > 0.6 && (
              <div
                className="absolute inset-0 opacity-20 rounded-lg"
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
  );
}
