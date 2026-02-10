'use client';

import { VisualizerEvent } from '@/hooks/useVisualizerStream';

interface EventLogProps {
  events: VisualizerEvent[];
}

export function EventLog({ events }: EventLogProps) {
  return (
    <div className="h-full flex flex-col p-4">
      <h2 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
        <span>📊</span>
        <span>事件日志</span>
        <span className="ml-auto text-xs font-normal text-gray-500">
          {events.length} 条
        </span>
      </h2>

      <div className="flex-1 overflow-auto space-y-1.5">
        {events.length === 0 && (
          <div className="text-center text-gray-500 text-xs py-8">
            <div className="text-3xl mb-2">⏳</div>
            <div>等待事件...</div>
          </div>
        )}

        {events
          .slice()
          .reverse()
          .map((event, idx) => (
            <EventLogItem key={`${event.timestamp}-${idx}`} event={event} />
          ))}
      </div>
    </div>
  );
}

function EventLogItem({ event }: { event: VisualizerEvent }) {
  const iconMap: Record<string, string> = {
    Read: '📖',
    Write: '✍️',
    Edit: '✏️',
    Grep: '🔍',
    Glob: '🗂️',
    Bash: '💻',
    WebFetch: '🌐',
    Thinking: '💭',
  };

  const statusColorMap: Record<string, string> = {
    started: 'bg-blue-100 text-blue-800 border-blue-300',
    completed: 'bg-green-100 text-green-800 border-green-300',
    error: 'bg-red-100 text-red-800 border-red-300',
    info: 'bg-gray-100 text-gray-800 border-gray-300',
  };

  const toolColorMap: Record<string, string> = {
    Read: 'border-l-blue-500',
    Write: 'border-l-green-500',
    Edit: 'border-l-yellow-500',
    Grep: 'border-l-purple-500',
    Glob: 'border-l-indigo-500',
    Bash: 'border-l-orange-500',
    WebFetch: 'border-l-cyan-500',
    Thinking: 'border-l-pink-500',
  };

  return (
    <div
      className={`
        flex items-start gap-2 p-2 rounded-lg border-l-2
        hover:bg-gray-50 transition-all duration-200
        ${toolColorMap[event.tool] || 'border-l-gray-500'}
        ${event.status === 'started' ? 'bg-blue-50/30' : ''}
      `}
    >
      {/* Icon */}
      <div className="text-lg flex-shrink-0">{iconMap[event.tool] || '⚙️'}</div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5 mb-0.5">
          <span className="font-medium text-xs text-gray-900">{event.tool}</span>
          <span
            className={`text-[10px] px-1.5 py-0.5 rounded-full border ${statusColorMap[event.status]}`}
          >
            {event.status}
          </span>
        </div>

        {event.path && (
          <div className="text-[10px] text-gray-600 font-mono truncate">
            {event.path.split('/').slice(-2).join('/')}
          </div>
        )}

        <div className="text-[9px] text-gray-400 mt-0.5">
          {new Date(event.timestamp).toLocaleTimeString()}
        </div>
      </div>

      {/* Visual indicator for active events */}
      {event.status === 'started' && (
        <div className="flex-shrink-0">
          <div className="w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse" />
        </div>
      )}
    </div>
  );
}
