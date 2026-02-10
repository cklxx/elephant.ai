'use client';

import { useVisualizerStream } from '@/hooks/useVisualizerStream';
import { FolderMap } from './FolderMap';
import { CrabAgent } from './CrabAgent';
import { EventLog } from './EventLog';

export function CodeVisualizer() {
  const { events, isConnected, currentEvent } = useVisualizerStream();

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 sticky top-0 z-30 shadow-sm">
        <div className="max-w-[1920px] mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="text-3xl">🦀</div>
              <div>
                <h1 className="text-2xl font-bold text-gray-900">
                  Claude Code Visualizer
                </h1>
                <p className="text-sm text-gray-600">实时观察 AI 在代码库中的工作</p>
              </div>
            </div>

            {/* Connection status */}
            <div className="flex items-center gap-2">
              <div
                className={`w-3 h-3 rounded-full ${
                  isConnected ? 'bg-green-500 animate-pulse' : 'bg-red-500'
                }`}
              />
              <span className="text-sm font-medium text-gray-700">
                {isConnected ? '已连接' : '未连接'}
              </span>
            </div>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-[1920px] mx-auto px-6 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Folder map (main area) */}
          <div className="lg:col-span-2">
            <div className="bg-white rounded-lg shadow-lg p-6 min-h-[600px]">
              <h2 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
                <span>📁</span>
                <span>代码库文件夹</span>
                <span className="ml-auto text-sm font-normal text-gray-500">
                  颜色深度反映活动强度
                </span>
              </h2>
              <FolderMap events={events} currentEvent={currentEvent} />
            </div>
          </div>

          {/* Event log (sidebar) */}
          <div className="lg:col-span-1">
            <EventLog events={events} />
          </div>
        </div>

        {/* Stats footer */}
        <div className="mt-6 bg-white rounded-lg shadow-lg p-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
            <StatCard
              icon="📊"
              label="总事件数"
              value={events.length.toString()}
            />
            <StatCard
              icon="📁"
              label="活跃文件夹"
              value={
                new Set(
                  events
                    .filter((e) => e.path)
                    .map((e) => e.path!.split('/').slice(0, -1).join('/'))
                ).size.toString()
              }
            />
            <StatCard
              icon="🔧"
              label="当前工具"
              value={currentEvent?.tool || '-'}
            />
            <StatCard
              icon="⏱️"
              label="最后活动"
              value={
                events.length > 0
                  ? new Date(events[events.length - 1].timestamp).toLocaleTimeString()
                  : '-'
              }
            />
          </div>
        </div>
      </main>

      {/* Crab agent overlay */}
      <CrabAgent currentEvent={currentEvent} />
    </div>
  );
}

function StatCard({ icon, label, value }: { icon: string; label: string; value: string }) {
  return (
    <div className="flex flex-col items-center">
      <div className="text-2xl mb-1">{icon}</div>
      <div className="text-xs text-gray-600 mb-1">{label}</div>
      <div className="text-lg font-bold text-gray-900">{value}</div>
    </div>
  );
}
