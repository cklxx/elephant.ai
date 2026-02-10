'use client';

import { useMemo } from 'react';
import { useVisualizerStream } from '@/hooks/useVisualizerStream';
import { ConstructionSite3D } from './ConstructionSite3D';
import { EventLog } from './EventLog';

export function ConstructionSiteVisualizer() {
  const { events, isConnected, currentEvent } = useVisualizerStream();

  // Calculate stats
  const stats = useMemo(() => {
    const activeFolders = new Set(
      events
        .filter((e) => e.path)
        .map((e) => e.path!.split('/').slice(0, -1).join('/'))
    );

    return {
      totalEvents: events.length,
      activeFolders: activeFolders.size,
      currentTool: currentEvent?.tool || 'Idle',
    };
  }, [events, currentEvent]);

  return (
    <div className="fixed inset-0 bg-gradient-to-b from-sky-300 to-sky-500">
      {/* Header */}
      <header className="absolute top-0 left-0 right-0 z-10 bg-white/90 backdrop-blur-sm border-b border-gray-200 shadow-sm">
        <div className="px-6 py-3">
          <div className="flex items-center justify-between">
            {/* Title */}
            <div className="flex items-center gap-3">
              <div className="text-3xl">🏗️</div>
              <div>
                <h1 className="text-xl font-bold text-gray-900">
                  Claude Code Construction Site
                </h1>
                <p className="text-xs text-gray-600">实时观察 AI 建造代码城市</p>
              </div>
            </div>

            {/* Stats */}
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-4 text-xs">
                <div className="flex items-center gap-1">
                  <span>📊</span>
                  <span className="font-semibold">{stats.totalEvents}</span>
                  <span className="text-gray-500">操作</span>
                </div>
                <div className="flex items-center gap-1">
                  <span>🏢</span>
                  <span className="font-semibold">{stats.activeFolders}</span>
                  <span className="text-gray-500">活跃</span>
                </div>
                {currentEvent && (
                  <div className="flex items-center gap-1">
                    <span>🔧</span>
                    <span className="font-semibold">{stats.currentTool}</span>
                  </div>
                )}
              </div>

              {/* Connection status */}
              <div className="flex items-center gap-2">
                <div
                  className={`w-2 h-2 rounded-full ${
                    isConnected ? 'bg-green-500 animate-pulse' : 'bg-red-500'
                  }`}
                />
                <span className="text-xs font-medium text-gray-700">
                  {isConnected ? '已连接' : '未连接'}
                </span>
              </div>
            </div>
          </div>
        </div>
      </header>

      {/* 3D Scene */}
      <div className="absolute inset-0 pt-16">
        <ConstructionSite3D />
      </div>

      {/* Event Log Sidebar */}
      <div className="absolute right-4 top-20 bottom-4 w-80 pointer-events-auto">
        <div className="h-full bg-white/95 backdrop-blur-sm rounded-lg shadow-2xl overflow-hidden border border-gray-200">
          <EventLog events={events} />
        </div>
      </div>

      {/* Controls hint */}
      <div className="absolute bottom-4 left-4 bg-white/90 backdrop-blur-sm rounded-lg shadow-lg px-4 py-2 text-xs text-gray-700 pointer-events-none">
        <div className="font-semibold mb-1">🎮 控制提示</div>
        <div className="space-y-0.5 text-[10px]">
          <div>• 左键拖动：旋转视角</div>
          <div>• 右键拖动：平移视角</div>
          <div>• 滚轮：缩放</div>
        </div>
      </div>
    </div>
  );
}
