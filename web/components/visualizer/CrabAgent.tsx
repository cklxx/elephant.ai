'use client';

import { useEffect, useRef, useState } from 'react';
import { VisualizerEvent } from '@/hooks/useVisualizerStream';

interface CrabAgentProps {
  currentEvent: VisualizerEvent | null;
}

export function CrabAgent({ currentEvent }: CrabAgentProps) {
  const crabRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ x: 100, y: 100 });
  const isThinking = Boolean(currentEvent && currentEvent.tool === 'Thinking');

  useEffect(() => {
    if (!currentEvent || typeof window === 'undefined') {
      return;
    }

    const frame = window.requestAnimationFrame(() => {
      if (currentEvent.tool === 'Thinking') {
        setPosition({ x: window.innerWidth / 2 - 24, y: 50 });
        return;
      }

      if (!currentEvent.path || typeof document === 'undefined') {
        return;
      }

      const parts = currentEvent.path.split('/');
      const folderPath = parts.slice(0, -1).join('/') || '/';
      const targetElement = document.querySelector(`[data-folder-path="${folderPath}"]`);
      if (!targetElement) {
        return;
      }

      const rect = targetElement.getBoundingClientRect();
      const scrollY = window.scrollY || document.documentElement.scrollTop;
      const scrollX = window.scrollX || document.documentElement.scrollLeft;
      setPosition({
        x: rect.left + scrollX + rect.width / 2 - 24,
        y: rect.top + scrollY + rect.height / 2 - 24,
      });
    });

    return () => window.cancelAnimationFrame(frame);
  }, [currentEvent]);

  if (!currentEvent) {
    // Idle state: crab in corner
    return (
      <div
        ref={crabRef}
        className="fixed bottom-8 right-8 pointer-events-none z-50 opacity-50"
      >
        <CrabSVG isThinking={false} isIdle={true} />
      </div>
    );
  }

  return (
    <>
      {/* Crab character */}
      <div
        ref={crabRef}
        className="fixed pointer-events-none z-50 transition-all duration-700 ease-out"
        style={{
          left: `${position.x}px`,
          top: `${position.y}px`,
        }}
      >
        <CrabSVG isThinking={isThinking} isIdle={false} />

        {/* Speech bubble */}
        <div
          className="absolute -top-16 left-1/2 -translate-x-1/2
                     bg-white rounded-lg shadow-xl px-3 py-2 min-w-[140px]
                     border-2 border-gray-200 animate-fadeInOut"
        >
          <div className="text-xs font-semibold text-gray-900 text-center">
            {getActionText(currentEvent)}
          </div>
          {currentEvent.path && (
            <div className="text-[10px] text-gray-600 truncate max-w-[120px] text-center">
              {getFileName(currentEvent.path)}
            </div>
          )}

          {/* Bubble tail */}
          <div
            className="absolute left-1/2 bottom-[-8px] -translate-x-1/2
                       w-0 h-0 border-l-[8px] border-l-transparent
                       border-r-[8px] border-r-transparent
                       border-t-[8px] border-t-white"
          />
        </div>
      </div>

      {/* Trail effect */}
      <svg
        className="fixed inset-0 pointer-events-none z-40"
        style={{ width: '100%', height: '100%' }}
      >
        <circle
          cx={position.x + 24}
          cy={position.y + 24}
          r="60"
          fill="rgba(230, 126, 34, 0.1)"
          className="animate-ping"
        />
      </svg>
    </>
  );
}

function CrabSVG({ isThinking, isIdle }: { isThinking: boolean; isIdle: boolean }) {
  return (
    <svg
      width="48"
      height="48"
      viewBox="0 0 48 48"
      className={`${isThinking ? 'animate-bounce' : ''} ${isIdle ? 'animate-pulse' : ''}`}
    >
      {/* Body */}
      <ellipse
        cx="24"
        cy="24"
        rx="14"
        ry="16"
        fill="#e67e22"
        className={!isIdle ? 'animate-pulse' : ''}
      />

      {/* Shell pattern */}
      <path
        d="M 18 20 Q 24 16 30 20"
        stroke="#d35400"
        strokeWidth="2"
        fill="none"
      />
      <path
        d="M 18 24 Q 24 20 30 24"
        stroke="#d35400"
        strokeWidth="2"
        fill="none"
      />

      {/* Eyes */}
      <circle cx="19" cy="19" r="3" fill="#fff" />
      <circle cx="29" cy="19" r="3" fill="#fff" />
      <circle
        cx="20"
        cy="19"
        r="1.5"
        fill="#000"
        className={isThinking ? 'animate-ping' : ''}
      />
      <circle
        cx="30"
        cy="19"
        r="1.5"
        fill="#000"
        className={isThinking ? 'animate-ping' : ''}
      />

      {/* Left claw */}
      <g className="origin-[12px_28px] animate-wave">
        <line x1="12" y1="28" x2="4" y2="24" stroke="#d35400" strokeWidth="3" />
        <circle cx="4" cy="24" r="2.5" fill="#d35400" />
        <path d="M 2 24 L 1 22 M 6 24 L 7 22" stroke="#c0392b" strokeWidth="1.5" />
      </g>

      {/* Right claw */}
      <g className="origin-[36px_28px] animate-wave-delayed">
        <line x1="36" y1="28" x2="44" y2="24" stroke="#d35400" strokeWidth="3" />
        <circle cx="44" cy="24" r="2.5" fill="#d35400" />
        <path d="M 42 24 L 41 22 M 46 24 L 47 22" stroke="#c0392b" strokeWidth="1.5" />
      </g>

      {/* Legs */}
      <g opacity="0.7">
        <line x1="16" y1="34" x2="14" y2="38" stroke="#d35400" strokeWidth="2" />
        <line x1="20" y1="36" x2="19" y2="40" stroke="#d35400" strokeWidth="2" />
        <line x1="28" y1="36" x2="29" y2="40" stroke="#d35400" strokeWidth="2" />
        <line x1="32" y1="34" x2="34" y2="38" stroke="#d35400" strokeWidth="2" />
      </g>
    </svg>
  );
}

// Tool to action text mapping
function getActionText(event: VisualizerEvent): string {
  const actionMap: Record<string, string> = {
    Read: '📖 正在阅读',
    Write: '✍️ 正在写入',
    Edit: '✏️ 正在编辑',
    Grep: '🔍 正在搜索',
    Glob: '🗂️ 正在查找',
    Bash: '💻 执行命令',
    WebFetch: '🌐 抓取网页',
    Thinking: '💭 正在思考',
  };
  return actionMap[event.tool] || `${event.tool} 中...`;
}

function getFileName(path: string): string {
  return path.split('/').pop() || path;
}
