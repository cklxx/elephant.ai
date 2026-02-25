import { AnyAgentEvent, WorkflowToolCompletedEvent } from '@/lib/types';
import { handleEnvironmentSnapshot } from '@/hooks/useDiagnostics';
import { handleAttachmentEvent } from './attachmentRegistry';
import { useSessionStore } from '@/hooks/useSessionStore';
import { apiClient } from '@/lib/api';

type EventSideEffect = (event: AnyAgentEvent) => void;

type Registry = Map<AnyAgentEvent['event_type'], EventSideEffect[]>;

function normalizeSessionTitle(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }
  const firstLine = trimmed.split(/\r?\n/)[0]?.trim() ?? '';
  if (!firstLine) {
    return '';
  }
  const chars = Array.from(firstLine);
  const maxChars = 32;
  if (chars.length <= maxChars) {
    return firstLine;
  }
  return `${chars.slice(0, maxChars).join('')}…`;
}

function extractPlanTitle(metadata?: Record<string, any> | null): string {
  if (!metadata || typeof metadata !== 'object') {
    return '';
  }
  const rawTitle =
    typeof metadata.session_title === 'string'
      ? metadata.session_title
      : typeof metadata.overall_goal_ui === 'string'
        ? metadata.overall_goal_ui
        : '';
  return normalizeSessionTitle(rawTitle);
}

function handlePlanGoal(event: AnyAgentEvent) {
  if (event.event_type !== 'workflow.tool.completed' || event.tool_name !== 'plan') {
    return;
  }

  const planEvent = event as WorkflowToolCompletedEvent;
  const sessionId = planEvent.session_id?.trim();
  if (!sessionId) {
    return;
  }

  const { renameSession, sessionLabels } = useSessionStore.getState();
  if (sessionLabels?.[sessionId]?.trim()) {
    return;
  }

  const metadataTitle = extractPlanTitle(planEvent.metadata ?? null);
  if (metadataTitle) {
    renameSession(sessionId, metadataTitle);
    return;
  }

  void apiClient
    .getSessionTitle(sessionId)
    .then((title) => {
      const normalized = normalizeSessionTitle(title ?? '');
      if (!normalized) {
        return;
      }
      renameSession(sessionId, normalized);
    })
    .catch(() => {});
}

export class EventRegistry {
  private registry: Registry = new Map();

  register(type: AnyAgentEvent['event_type'], sideEffect: EventSideEffect) {
    if (!this.registry.has(type)) {
      this.registry.set(type, []);
    }
    this.registry.get(type)!.push(sideEffect);
  }

  run(event: AnyAgentEvent) {
    const effects = this.registry.get(event.event_type);
    effects?.forEach((effect) => effect(event));
  }

  clear() {
    this.registry.clear();
  }
}

export const defaultEventRegistry = new EventRegistry();

defaultEventRegistry.register('workflow.tool.completed', handlePlanGoal as EventSideEffect);
defaultEventRegistry.register('workflow.diagnostic.environment_snapshot', handleEnvironmentSnapshot as EventSideEffect);

defaultEventRegistry.register('workflow.tool.completed', handleAttachmentEvent as EventSideEffect);
defaultEventRegistry.register('workflow.input.received', handleAttachmentEvent as EventSideEffect);
defaultEventRegistry.register('workflow.result.final', handleAttachmentEvent as EventSideEffect);
