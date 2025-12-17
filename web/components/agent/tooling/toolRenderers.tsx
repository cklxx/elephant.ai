'use client';

import { ReactNode } from 'react';
import {
  WorkflowToolStartedEvent,
  WorkflowToolCompletedEvent,
} from '@/lib/types';
import { ToolArgumentsPanel, ToolResultPanel, ToolStreamPanel, SimplePanel, PanelHeader } from './ToolPanels';

export interface RendererContext {
  startEvent: WorkflowToolStartedEvent | null;
  completeEvent: WorkflowToolCompletedEvent | null;
  status: 'running' | 'done' | 'error';
  toolName: string;
  labels: {
    arguments: string;
    stream: string;
    result: string;
    error: string;
    copyArgs: string;
    copyResult: string;
    copyError: string;
    copied: string;
    metadataTitle: string;
  };
  streamContent?: string;
  streamTimestamp?: string;
}

export interface ToolRendererResult {
  panels: ReactNode[];
  metadata?: ReactNode;
}

export type ToolRenderer = (context: RendererContext) => ToolRendererResult;

const buildArguments = (ctx: RendererContext): ReactNode | null => {
  if (!ctx.startEvent?.arguments || Object.keys(ctx.startEvent.arguments).length === 0) {
    return null;
  }
  const formatted = JSON.stringify(ctx.startEvent.arguments, null, 2);
  return (
    <ToolArgumentsPanel
      args={formatted}
      label={ctx.labels.arguments}
      copyLabel={ctx.labels.copyArgs}
      copiedLabel={ctx.labels.copied}
    />
  );
};

const buildStream = (ctx: RendererContext): ReactNode | null => {
  if (!ctx.streamContent) return null;
  return <ToolStreamPanel title={ctx.labels.stream} content={ctx.streamContent} />;
};

const buildResult = (ctx: RendererContext): ReactNode | null => {
  return (
    <ToolResultPanel
      toolName={ctx.toolName}
      result={ctx.completeEvent?.result}
      error={ctx.completeEvent?.error}
      resultTitle={ctx.labels.result}
      errorTitle={ctx.labels.error}
      copyLabel={ctx.labels.copyResult}
      copyErrorLabel={ctx.labels.copyError}
      copiedLabel={ctx.labels.copied}
      attachments={ctx.completeEvent?.attachments ?? undefined}
      metadata={ctx.completeEvent?.metadata ?? null}
    />
  );
};

const defaultRenderer: ToolRenderer = (ctx) => {
  return {
    panels: [buildArguments(ctx), buildStream(ctx), buildResult(ctx)].filter(Boolean) as ReactNode[],
  };
};

const browserRenderer: ToolRenderer = (ctx) => {
  const panels: ReactNode[] = [];
  if (ctx.completeEvent?.metadata?.url) {
    panels.push(
      <SimplePanel key="browser-metadata">
        <PanelHeader title={ctx.labels.metadataTitle} />
        <p className="text-sm text-foreground/80">
          {ctx.completeEvent.metadata.url}
        </p>
      </SimplePanel>,
    );
  }
  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

const shellRenderer: ToolRenderer = (ctx) => {
  const panels: ReactNode[] = [];
  if (ctx.startEvent?.arguments?.command) {
    panels.push(
      <SimplePanel key="shell-command">
        <PanelHeader title="Command" />
        <pre className="max-h-32 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted/20 p-2 font-mono text-[10px] leading-snug text-foreground/90">
          {ctx.startEvent.arguments.command}
        </pre>
      </SimplePanel>,
    );
  }
  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

const codeExecuteRenderer: ToolRenderer = (ctx) => {
  const panels: ReactNode[] = [];
  const code =
    typeof ctx.startEvent?.arguments?.code === 'string'
      ? ctx.startEvent.arguments.code
      : undefined;
  if (code) {
    panels.push(
      <SimplePanel key="code-execute-source">
        <PanelHeader title="Code" />
        <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-muted/20 p-3 font-mono text-[11px] leading-relaxed text-foreground/90">
          {code}
        </pre>
      </SimplePanel>,
    );
  }
  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

const fileRenderer: ToolRenderer = (ctx) => {
  const panels: ReactNode[] = [];
  if (ctx.startEvent?.arguments?.path) {
    panels.push(
      <SimplePanel key="file-target">
        <PanelHeader title="File" />
        <p className="font-mono text-[11px] text-foreground/70">{ctx.startEvent.arguments.path}</p>
      </SimplePanel>,
    );
  }
  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

export const resolveToolRenderer = (toolName: string): ToolRenderer => {
  const lower = toolName.toLowerCase();
  if (lower.includes('browser')) return browserRenderer;
  if (lower.includes('shell') || lower.includes('bash') || lower.includes('terminal')) return shellRenderer;
  if (lower.includes('code_execute')) return codeExecuteRenderer;
  if (lower.includes('file') || lower.includes('fs')) return fileRenderer;
  return defaultRenderer;
};
