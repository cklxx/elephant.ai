// Hook to convert agent events to tool outputs for WebViewport

import { useMemo } from 'react';
import {
  AnyAgentEvent,
  WorkflowToolStartedEvent,
  WorkflowToolCompletedEvent,
  AttachmentPayload,
} from '@/lib/types';
import { ToolOutput, ToolOutputType } from '@/components/agent/WebViewport';
import { eventMatches } from '@/lib/types';

export function useToolOutputs(events: AnyAgentEvent[]): ToolOutput[] {
  return useMemo(() => {
    const outputs: ToolOutput[] = [];
    const toolCalls = new Map<string, Partial<ToolOutput>>();

    events.forEach((event) => {
      // Track tool call starts
      if (eventMatches(event, 'workflow.tool.started', 'workflow.tool.started')) {
        const e = event as WorkflowToolStartedEvent;
        toolCalls.set(e.call_id, {
          id: e.call_id,
          toolName: e.tool_name,
          timestamp: new Date(e.timestamp).getTime(),
          type: mapToolNameToType(e.tool_name),
        });
      }

      // Complete tool calls
      if (eventMatches(event, 'workflow.tool.completed', 'workflow.tool.completed')) {
        const e = event as WorkflowToolCompletedEvent;
        const existing = toolCalls.get(e.call_id);

        if (existing) {
          const attachments = e.attachments as
            | Record<string, AttachmentPayload>
            | undefined;
          const output: ToolOutput = {
            id: e.call_id,
            toolName: e.tool_name,
            timestamp: existing.timestamp || new Date(e.timestamp).getTime(),
            type: existing.type || mapToolNameToType(e.tool_name),
            attachments,
            ...parseToolResult(e.tool_name, e.result, e.error, e.metadata),
          };

          outputs.push(output);
          toolCalls.delete(e.call_id);
        }
      }

    });

    return outputs.sort((a, b) => a.timestamp - b.timestamp);
  }, [events]);
}

function mapToolNameToType(toolName: string): ToolOutputType {
  if (toolName.includes('web_fetch')) {
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
  error?: string,
  metadata?: Record<string, any>
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
    // Not JSON or parsing failed, try metadata or treat as plain text
  }

  // Fallback to metadata when JSON result is unavailable
  if (metadata && toolName.includes('web_fetch')) {
    const web = extractWebMetadata(metadata);
    if (web) {
      return {
        url: web.url,
        screenshot: web.screenshot,
        htmlPreview: web.html ?? web.content,
        result: error || result,
      };
    }
  }

  // Fallback to generic result
  return {
    result: error || result,
  };
}

function extractWebMetadata(metadata: Record<string, any>):
  | { url?: string; screenshot?: string; html?: string; content?: string }
  | null {
  const candidate = metadata.browser ?? metadata.web ?? metadata.result ?? metadata;
  if (!candidate || typeof candidate !== 'object') {
    return null;
  }
  return {
    url: typeof candidate.url === 'string' ? candidate.url : undefined,
    screenshot:
      typeof candidate.screenshot === 'string' ? candidate.screenshot : undefined,
    html: typeof candidate.html === 'string' ? candidate.html : undefined,
    content:
      typeof candidate.content === 'string' ? candidate.content : undefined,
  };
}
