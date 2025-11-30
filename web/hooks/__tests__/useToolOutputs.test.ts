import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useToolOutputs } from '../useToolOutputs';
import { AnyAgentEvent } from '@/lib/types';

describe('useToolOutputs', () => {
  describe('Tool Call Aggregation', () => {
    it('should aggregate tool call start and complete events', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: { command: 'ls -la' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:05Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: JSON.stringify({
            command: 'ls -la',
            stdout: 'file1.txt\nfile2.txt',
            exit_code: 0,
          }),
          duration: 5000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'call-1',
        toolName: 'bash',
        type: 'bash',
        command: 'ls -la',
        stdout: 'file1.txt\nfile2.txt',
        exitCode: 0,
      });
    });

    it('should handle tool calls with errors', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: { command: 'invalid-command' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: JSON.stringify({
            command: 'invalid-command',
            stderr: 'command not found',
            exit_code: 127,
          }),
          error: 'command not found',
          duration: 1000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'call-1',
        toolName: 'bash',
        type: 'bash',
        stderr: 'command not found',
        exitCode: 127,
      });
    });
  });

  describe('Tool Type Mapping', () => {
    it('should map web_fetch tool type', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'web_fetch',
          arguments: { url: 'https://example.com' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:05Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'web_fetch',
          result: JSON.stringify({
            url: 'https://example.com',
            content: '<html>...</html>',
          }),
          duration: 5000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].type).toBe('web_fetch');
      expect(result.current[0].url).toBe('https://example.com');
    });

    it('should map browser tool type and parse screenshot', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-browser',
          tool_name: 'browser',
          arguments: { url: 'https://example.com' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:04Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-browser',
          tool_name: 'browser',
          result: JSON.stringify({
            url: 'https://example.com',
            screenshot: 'data:image/png;base64,AAA',
            html: '<html>Example</html>',
          }),
          duration: 4000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0]).toMatchObject({
        type: 'web_fetch',
        url: 'https://example.com',
        screenshot: 'data:image/png;base64,AAA',
        htmlPreview: '<html>Example</html>',
      });
    });

    it('should map file_read tool type', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_read',
          arguments: { path: '/test/file.txt' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_read',
          result: JSON.stringify({
            path: '/test/file.txt',
            content: 'File contents here',
          }),
          duration: 1000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].type).toBe('file_read');
      expect(result.current[0].filePath).toBe('/test/file.txt');
      expect(result.current[0].content).toBe('File contents here');
    });

    it('should extract web metadata when JSON parsing fails', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'web_fetch',
          arguments: { url: 'https://example.com' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:05Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'web_fetch',
          result: 'Source: https://example.com',
          duration: 5000,
          metadata: {
            web: {
              url: 'https://example.com',
              screenshot: 'data:image/png;base64,AAA',
              html: '<html></html>',
            },
          },
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0]).toMatchObject({
        url: 'https://example.com',
        screenshot: 'data:image/png;base64,AAA',
        htmlPreview: '<html></html>',
      });
    });

    it('should extract browser metadata when JSON parsing fails', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-browser',
          tool_name: 'browser',
          arguments: { url: 'https://example.com' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:03Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-browser',
          tool_name: 'browser',
          result: 'Visit complete',
          duration: 3000,
          metadata: {
            browser: {
              url: 'https://example.com',
              screenshot: 'data:image/png;base64,BBB',
              html: '<html>Example</html>',
            },
          },
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0]).toMatchObject({
        url: 'https://example.com',
        screenshot: 'data:image/png;base64,BBB',
        htmlPreview: '<html>Example</html>',
        result: 'Visit complete',
      });
    });

    it('should map file_write tool type', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_write',
          arguments: { path: '/test/output.txt', content: 'Written content' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_write',
          result: JSON.stringify({
            path: '/test/output.txt',
            content: 'Written content',
          }),
          duration: 500,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].type).toBe('file_write');
      expect(result.current[0].filePath).toBe('/test/output.txt');
    });

    it('should map file_edit tool type', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_edit',
          arguments: { path: '/test/file.txt' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_edit',
          result: JSON.stringify({
            path: '/test/file.txt',
            old_content: 'old text',
            new_content: 'new text',
          }),
          duration: 300,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].type).toBe('file_edit');
      expect(result.current[0].filePath).toBe('/test/file.txt');
      expect(result.current[0].oldContent).toBe('old text');
      expect(result.current[0].newContent).toBe('new text');
    });

    it('should default to generic type for unknown tools', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'unknown_tool',
          arguments: {},
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'unknown_tool',
          result: 'Some result',
          duration: 100,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].type).toBe('generic');
      expect(result.current[0].result).toBe('Some result');
    });
  });

  describe('Browser Diagnostics', () => {
    it('should extract browser diagnostics', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.diagnostic.browser_info',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          captured: '2025-01-01T10:00:00Z',
          success: true,
          message: 'Browser ready',
          user_agent: 'AgentBrowser/1.0',
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        type: 'generic',
        toolName: 'workflow.diagnostic.browser_info',
        result: expect.stringContaining('Browser ready'),
      });
    });
  });

  describe('Non-JSON Result Handling', () => {
    it('should handle plain text results', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'custom_tool',
          arguments: {},
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'custom_tool',
          result: 'Plain text result',
          duration: 100,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].result).toBe('Plain text result');
    });

    it('should handle invalid JSON gracefully', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: {},
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: '{invalid json',
          duration: 100,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current[0].result).toBe('{invalid json');
    });
  });

  describe('Timestamp Ordering', () => {
    it('should sort outputs by timestamp', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:02:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-2',
          tool_name: 'bash',
          arguments: { command: 'pwd' },
        },
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: { command: 'ls' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: 'output1',
          duration: 1000,
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:02:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-2',
          tool_name: 'bash',
          result: 'output2',
          duration: 1000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      expect(result.current).toHaveLength(2);
      expect(result.current[0].id).toBe('call-1');
      expect(result.current[1].id).toBe('call-2');
    });
  });

  describe('Empty Events', () => {
    it('should return empty array for no events', () => {
      const { result } = renderHook(() => useToolOutputs([]));

      expect(result.current).toEqual([]);
    });
  });

  describe('Memoization', () => {
    it('should memoize results when events do not change', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: 'output',
          duration: 100,
        },
      ];

      const { result, rerender } = renderHook(
        ({ events }) => useToolOutputs(events),
        { initialProps: { events } }
      );

      const firstResult = result.current;

      rerender({ events }); // Same events

      expect(result.current).toBe(firstResult); // Should be same reference
    });

    it('should recompute when events change', () => {
      const events1: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: { command: 'ls' },
        },
      ];

      const events2: AnyAgentEvent[] = [
        ...events1,
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: 'output',
          duration: 1000,
        },
      ];

      const { result, rerender } = renderHook(
        ({ events }) => useToolOutputs(events),
        { initialProps: { events: events1 } }
      );

      const firstResult = result.current;
      expect(firstResult).toHaveLength(0); // No complete events yet

      rerender({ events: events2 });

      expect(result.current).not.toBe(firstResult); // Should be new reference
      expect(result.current).toHaveLength(1);
    });
  });

  describe('Edge Cases', () => {
    it('should handle complete without start event', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-orphan',
          tool_name: 'bash',
          result: 'orphan result',
          duration: 100,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      // Hook only processes complete events that had a start event
      // This is expected behavior to avoid partial data
      expect(result.current).toHaveLength(0);
    });

    it('should handle multiple complete events for same call_id', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          arguments: { command: 'ls' },
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: 'first result',
          duration: 1000,
        },
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T10:00:02Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'bash',
          result: 'second result',
          duration: 2000,
        },
      ];

      const { result } = renderHook(() => useToolOutputs(events));

      // Should only have first complete (call is removed from map after first complete)
      expect(result.current).toHaveLength(1);
      expect(result.current[0].result).toContain('first result');
    });
  });
});
