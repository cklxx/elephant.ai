import { AnyAgentEvent, WorkflowToolCompletedEvent } from '@/lib/types';
import { handleEnvironmentSnapshot } from '@/hooks/useDiagnostics';
import { handleAttachmentEvent } from './attachmentRegistry';
import { useSessionStore } from '@/hooks/useSessionStore';

type EventSideEffect = (event: AnyAgentEvent) => void;

type Registry = Map<AnyAgentEvent['event_type'], EventSideEffect[]>;

function normalizeSessionTitle(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    return '';
  }
  const chars = Array.from(trimmed);
  const maxChars = 32;
  if (chars.length <= maxChars) {
    return trimmed;
  }
  return `${chars.slice(0, maxChars).join('')}…`;
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

  const metadata = planEvent.metadata ?? {};
  const goalCandidate = [
    metadata.session_title,
    metadata.title,
    metadata.overall_goal_ui,
    metadata.overall_goal,
    metadata.internal_plan?.overall_goal,
    planEvent.result,
  ].find((value): value is string => typeof value === 'string' && value.trim().length > 0);

  if (!goalCandidate) {
    return;
  }

  const { renameSession } = useSessionStore.getState();
  renameSession(sessionId, normalizeSessionTitle(goalCandidate));
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
