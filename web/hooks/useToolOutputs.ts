// Hook to convert agent events to tool outputs for WebViewport

import { useMemo } from 'react';
import {
  AnyAgentEvent,
  ToolCallStartEvent,
  ToolCallCompleteEvent,
  BrowserInfoEvent,
} from '@/lib/types';
import { ToolOutput, ToolOutputType } from '@/components/agent/WebViewport';

export function useToolOutputs(events: AnyAgentEvent[]): ToolOutput[] {
  return useMemo(() => {
    const outputs: ToolOutput[] = [];
    const toolCalls = new Map<string, Partial<ToolOutput>>();

    events.forEach((event) => {
      // Track tool call starts
      if (event.event_type === 'tool_call_start') {
        const e = event as ToolCallStartEvent;
        toolCalls.set(e.call_id, {
          id: e.call_id,
          toolName: e.tool_name,
          timestamp: new Date(event.timestamp).getTime(),
          type: mapToolNameToType(e.tool_name),
        });
      }

      // Complete tool calls
      if (event.event_type === 'tool_call_complete') {
        const e = event as ToolCallCompleteEvent;
        const existing = toolCalls.get(e.call_id);

        if (existing) {
          const output: ToolOutput = {
            id: e.call_id,
            toolName: e.tool_name,
            timestamp: existing.timestamp || new Date(event.timestamp).getTime(),
            type: existing.type || mapToolNameToType(e.tool_name),
            ...parseToolResult(e.tool_name, e.result, e.error),
          };

          outputs.push(output);
          toolCalls.delete(e.call_id);
        }
      }

      // Browser diagnostics
      if (event.event_type === 'browser_info') {
        const e = event as BrowserInfoEvent;
        const details: string[] = [];
        if (typeof e.success === 'boolean') {
          details.push(`Status: ${e.success ? 'available' : 'unavailable'}`);
        }
        if (e.message) {
          details.push(`Message: ${e.message}`);
        }
        if (e.user_agent) {
          details.push(`User-Agent: ${e.user_agent}`);
        }
        if (e.cdp_url) {
          details.push(`CDP URL: ${e.cdp_url}`);
        }
        if (e.vnc_url) {
          details.push(`VNC URL: ${e.vnc_url}`);
        }
        if (e.viewport_width && e.viewport_height) {
          details.push(`Viewport: ${e.viewport_width}x${e.viewport_height}`);
        }

        outputs.push({
          id: `browser-info-${event.timestamp}`,
          type: 'generic',
          toolName: 'browser_info',
          timestamp: new Date(event.timestamp).getTime(),
          result: details.length > 0 ? details.join('\n') : 'No browser diagnostics available.',
        });
      }
    });

    return outputs.sort((a, b) => a.timestamp - b.timestamp);
  }, [events]);
}

function mapToolNameToType(toolName: string): ToolOutputType {
  if (toolName.includes('web_fetch') || toolName.includes('browser')) {
    return 'web_fetch';
  }
  if (toolName.includes('bash') || toolName.includes('shell') || toolName.includes('execute')) {
    return 'bash';
  }
  if (toolName.includes('file_read') || toolName.includes('read')) {
    return 'file_read';
  }
  if (toolName.includes('file_write') || toolName.includes('write')) {
    return 'file_write';
  }
  if (toolName.includes('file_edit') || toolName.includes('edit')) {
    return 'file_edit';
  }
  return 'generic';
}

function parseToolResult(
  toolName: string,
  result: string,
  error?: string
): Partial<ToolOutput> {
  // Try to parse JSON result
  try {
    const parsed = JSON.parse(result);

    // Bash tool
    if (toolName.includes('bash') || toolName.includes('execute')) {
      return {
        command: parsed.command || parsed.input,
        stdout: parsed.stdout || parsed.output,
        stderr: parsed.stderr || error,
        exitCode: parsed.exit_code ?? parsed.exitCode,
      };
    }

    // Web fetch
    if (toolName.includes('web_fetch')) {
      return {
        url: parsed.url,
        screenshot: parsed.screenshot,
        htmlPreview: parsed.html || parsed.content,
      };
    }

    // File read
    if (toolName.includes('file_read')) {
      return {
        filePath: parsed.path || parsed.file_path,
        content: parsed.content || result,
      };
    }

    // File write
    if (toolName.includes('file_write')) {
      return {
        filePath: parsed.path || parsed.file_path,
        content: parsed.content || result,
      };
    }

    // File edit
    if (toolName.includes('file_edit')) {
      return {
        filePath: parsed.path || parsed.file_path,
        oldContent: parsed.old_content || parsed.before,
        newContent: parsed.new_content || parsed.after || parsed.content,
      };
    }
  } catch {
    // Not JSON or parsing failed, treat as plain text
  }

  // Fallback to generic result
  return {
    result: error || result,
  };
}
