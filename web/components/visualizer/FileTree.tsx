'use client';

import { useState } from 'react';

export interface FileNode {
  name: string;
  path: string;
  type: 'folder' | 'file';
  active?: boolean;
  children?: FileNode[];
}

interface FileTreeProps {
  node: FileNode;
  level?: number;
}

export function FileTree({ node, level = 0 }: FileTreeProps) {
  const [expanded, setExpanded] = useState(level < 2); // 默认展开前两层

  const hasChildren = node.children && node.children.length > 0;
  const isFolder = node.type === 'folder';

  return (
    <div className="select-none">
      <div
        className={`flex items-center gap-2 py-1.5 px-2 rounded cursor-pointer transition-colors ${
          node.active
            ? 'bg-yellow-100 border-l-4 border-yellow-500'
            : 'hover:bg-gray-50'
        }`}
        style={{ paddingLeft: `${level * 20 + 8}px` }}
        onClick={() => isFolder && setExpanded(!expanded)}
      >
        {isFolder && (
          <span className="text-gray-500 w-4">
            {expanded ? '📂' : '📁'}
          </span>
        )}
        {!isFolder && (
          <span className="text-gray-500 w-4">
            {getFileIcon(node.name)}
          </span>
        )}
        <span className={`text-sm ${node.active ? 'font-semibold text-gray-900' : 'text-gray-700'}`}>
          {node.name}
        </span>
        {node.active && (
          <span className="ml-auto">
            <ActivityIndicator />
          </span>
        )}
      </div>

      {isFolder && expanded && hasChildren && (
        <div>
          {node.children!.map((child, idx) => (
            <FileTree key={`${child.path}-${idx}`} node={child} level={level + 1} />
          ))}
        </div>
      )}
    </div>
  );
}

function ActivityIndicator() {
  return (
    <div className="flex gap-0.5">
      <div className="w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" style={{ animationDelay: '0ms' }} />
      <div className="w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" style={{ animationDelay: '150ms' }} />
      <div className="w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" style={{ animationDelay: '300ms' }} />
    </div>
  );
}

function getFileIcon(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase();
  const iconMap: Record<string, string> = {
    'ts': '📘',
    'tsx': '⚛️',
    'js': '📜',
    'jsx': '⚛️',
    'go': '🐹',
    'py': '🐍',
    'rs': '🦀',
    'md': '📝',
    'json': '📋',
    'yaml': '⚙️',
    'yml': '⚙️',
    'toml': '⚙️',
    'sh': '🔧',
    'css': '🎨',
    'html': '🌐',
  };
  return iconMap[ext || ''] || '📄';
}
