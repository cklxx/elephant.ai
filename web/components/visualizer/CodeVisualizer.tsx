'use client';

import { useVisualizerStream } from '@/hooks/useVisualizerStream';
import { FolderTreemap } from './FolderTreemap';
import { CrabAgent } from './CrabAgent';
import { EventLog } from './EventLog';

export function CodeVisualizer() {
  const { events, isConnected, currentEvent } = useVisualizerStream();

  return (
    <div className="fixed inset-0 bg-gradient-to-br from-gray-50 to-gray-100 overflow-hidden">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 shadow-sm">
        <div className="px-6 py-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="text-2xl">🦀</div>
              <div>
                <h1 className="text-xl font-bold text-gray-900">
                  Claude Code Visualizer
                </h1>
                <p className="text-xs text-gray-600">实时观察 AI 在代码库中的辛勤工作</p>
              </div>
            </div>

            {/* Connection status & stats */}
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-4 text-xs">
                <div className="flex items-center gap-1">
                  <span>📊</span>
                  <span className="font-semibold">{events.length}</span>
                  <span className="text-gray-500">事件</span>
                </div>
                <div className="flex items-center gap-1">
                  <span>📁</span>
                  <span className="font-semibold">
                    {
                      new Set(
                        events
                          .filter((e) => e.path)
                          .map((e) => e.path!.split('/').slice(0, -1).join('/'))
                      ).size
                    }
                  </span>
                  <span className="text-gray-500">活跃</span>
                </div>
                {currentEvent && (
                  <div className="flex items-center gap-1">
                    <span>🔧</span>
                    <span className="font-semibold">{currentEvent.tool}</span>
                  </div>
                )}
              </div>

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

      {/* Main content - fixed layout */}
      <main className="h-[calc(100vh-65px)] flex gap-4 p-4">
        {/* Treemap (main area) */}
        <div className="flex-1 bg-white rounded-lg shadow-lg overflow-hidden">
          <FolderTreemap events={events} currentEvent={currentEvent} />
        </div>

        {/* Event log (sidebar) */}
        <div className="w-80 flex-shrink-0">
          <div className="h-full bg-white rounded-lg shadow-lg overflow-hidden">
            <EventLog events={events} />
          </div>
        </div>
      </main>

      {/* Crab agent overlay */}
      <CrabAgent currentEvent={currentEvent} />
    </div>
  );
}
