import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { VirtualizedEventList } from '../VirtualizedEventList';
import { LanguageProvider } from '@/lib/i18n';
import { AnyAgentEvent } from '@/lib/types';

vi.mock('@tanstack/react-virtual', () => ({
  useVirtualizer: ({ count }: { count: number }) => ({
    getTotalSize: () => count * 100,
    getVirtualItems: () =>
      Array.from({ length: count }, (_, index) => ({
        key: index,
        index,
        start: index * 100,
        size: 100,
      })),
    scrollToIndex: vi.fn(),
    measureElement: () => {},
  }),
}));

describe('VirtualizedEventList subagent tool rendering', () => {
  it('renders compact tool cards for subagent tool events', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        session_id: 'session-1',
        run_id: 'task-sub',
        parent_run_id: 'parent-1',
        timestamp: new Date().toISOString(),
        call_id: 'call-sub-1',
        tool_name: 'bash',
        result: 'ok',
        duration: 1200,
        is_subtask: true,
      } as AnyAgentEvent,
      {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-core',
        timestamp: new Date().toISOString(),
        call_id: 'call-core-1',
        tool_name: 'web_fetch',
        result: 'done',
        duration: 420,
      } as AnyAgentEvent,
    ];

    render(
      <LanguageProvider>
        <VirtualizedEventList events={events} autoScroll={false} />
      </LanguageProvider>,
    );

    expect(screen.getByTestId('compact-tool-call-bash')).toBeInTheDocument();
    expect(screen.queryByTestId('tool-call-card-bash')).not.toBeInTheDocument();
    expect(screen.getByTestId('tool-call-card-web_fetch')).toBeInTheDocument();
  });
});
