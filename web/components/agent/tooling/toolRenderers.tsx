'use client';

import { ReactNode } from 'react';
import {
  WorkflowToolStartedEvent,
  WorkflowToolCompletedEvent,
} from '@/lib/types';
import { ToolArgumentsPanel, ToolResultPanel, ToolStreamPanel, SimplePanel, PanelHeader } from './ToolPanels';
import { MusicPlayerPanel } from './MusicPlayerPanel';

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
        <div className="max-w-[800px]">
          <p className="text-[12px] leading-relaxed text-foreground/80 break-all">
            {ctx.completeEvent.metadata.url}
          </p>
        </div>
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
        <div className="max-w-[900px]">
          <pre className="max-h-40 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/20 px-4 py-3 font-mono text-[12px] leading-relaxed text-foreground/90 shadow-sm">
            {ctx.startEvent.arguments.command}
          </pre>
        </div>
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
        <div className="max-w-[900px]">
          <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/20 px-4 py-3 font-mono text-[12px] leading-relaxed text-foreground/90 shadow-sm">
            {code}
          </pre>
        </div>
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
        <div className="max-w-[900px]">
          <p className="rounded-md border border-border/40 bg-muted/10 px-3 py-2 font-mono text-[12px] text-foreground/80 break-all">
            {ctx.startEvent.arguments.path}
          </p>
        </div>
      </SimplePanel>,
    );
  }
  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

const musicPlayRenderer: ToolRenderer = (ctx) => {
  const panels: ReactNode[] = [];
  const metadata = ctx.completeEvent?.metadata ?? null;
  const tracks = Array.isArray(metadata?.tracks) ? metadata.tracks : [];
  const query = typeof metadata?.query === 'string' ? metadata.query : '';

  if (tracks.length > 0) {
    panels.push(<MusicPlayerPanel key="music-player" query={query} tracks={tracks} />);
  }

  panels.push(...defaultRenderer(ctx).panels);
  return { panels };
};

export const resolveToolRenderer = (toolName: string): ToolRenderer => {
  const lower = toolName.toLowerCase();
  if (lower.includes('browser')) return browserRenderer;
  if (lower.includes('shell') || lower.includes('bash') || lower.includes('terminal')) return shellRenderer;
  if (lower.includes('code_execute')) return codeExecuteRenderer;
  if (lower === 'music_play') return musicPlayRenderer;
  if (lower.includes('file') || lower.includes('fs')) return fileRenderer;
  return defaultRenderer;
};
