import { ToolCallSummary } from './eventAggregation';

export type SandboxStrategy = 'required' | 'recommended';

export interface EnvironmentBlueprint {
  id: string;
  title: string;
  description: string;
  recommendedCapabilities: string[];
  persistenceHint: string;
}

export interface EnvironmentTodo {
  id: string;
  label: string;
  completed: boolean;
  manuallySet?: boolean;
}

export interface SessionEnvironmentPlan {
  sessionId: string;
  sandboxStrategy: SandboxStrategy;
  toolsUsed: string[];
  generatedAt: string;
  lastUpdatedAt: string;
  notes: string;
  blueprint: EnvironmentBlueprint;
  toolSummaries: ToolCallSummary[];
  todos: EnvironmentTodo[];
}

export interface SerializedEnvironmentPlan {
  sessionId: string;
  sandboxStrategy: SandboxStrategy;
  toolsUsed: string[];
  generatedAt: string;
  lastUpdatedAt: string;
  notes: string;
  blueprint: EnvironmentBlueprint;
  todos: Array<Pick<EnvironmentTodo, 'id' | 'label' | 'completed'>>;
}

function snapshotPlan(plan: SessionEnvironmentPlan) {
  const { lastUpdatedAt, ...rest } = plan;
  return rest;
}

function unique<T>(values: T[]): T[] {
  return Array.from(new Set(values));
}

function buildCapabilities(summaries: ToolCallSummary[]): string[] {
  const caps = new Set<string>([
    'network-isolation',
    'process-isolation',
    'sandbox-enforced',
  ]);

  if (summaries.some((summary) => summary.sandboxLevel === 'filesystem')) {
    caps.add('filesystem-proxy');
    caps.add('persistent-storage');
  }

  if (summaries.some((summary) => summary.sandboxLevel === 'system')) {
    caps.add('command-runner');
    caps.add('sandbox-auditing');
  }

  if (summaries.length > 0) {
    caps.add('tool-cache');
  }

  return Array.from(caps);
}

function buildNotes(sessionId: string, summaries: ToolCallSummary[]): string {
  if (summaries.length === 0) {
    return `Sandbox workspace ready for session ${sessionId}.`;
  }

  const elevatedTools = unique(
    summaries
      .filter((summary) => summary.sandboxLevel !== 'standard')
      .map((summary) => summary.toolName)
  );

  if (elevatedTools.length > 0) {
    return `All tool calls are sandboxed. Extra monitoring enabled for ${elevatedTools.join(', ')}.`;
  }

  return 'All tool calls are sandboxed. No elevated filesystem or system access detected yet.';
}

function buildTodos(
  sessionId: string,
  summaries: ToolCallSummary[],
  previousPlan?: SessionEnvironmentPlan
): EnvironmentTodo[] {
  const previousTodos = new Map(previousPlan?.todos?.map((todo) => [todo.id, todo]));
  const todos: EnvironmentTodo[] = [];

  const hasToolActivity = summaries.length > 0;

  const pushTodo = (id: string, label: string, completedByDefault: boolean) => {
    const previous = previousTodos.get(id);
    const manualOverride = previous?.manuallySet === true;
    const completed = manualOverride
      ? Boolean(previous?.completed)
      : completedByDefault;

    const todo: EnvironmentTodo = { id, label, completed };
    if (manualOverride) {
      todo.manuallySet = true;
    }
    todos.push(todo);
  };

  pushTodo(
    'confirm-sandbox',
    `Confirm sandbox workspace for session ${sessionId}.`,
    hasToolActivity
  );

  pushTodo(
    'persist-blueprint',
    `Save sandbox blueprint for session ${sessionId}.`,
    Boolean(previousPlan)
  );

  if (!hasToolActivity) {
    pushTodo('await-first-call', 'Await first tool call to tailor sandbox capabilities.', false);
    pushTodo(
      'monitor-running',
      'No active sandbox tools to monitor.',
      true
    );
    pushTodo('inspect-failures', 'No sandbox failures detected.', true);
    pushTodo('route-sandbox-tools', 'Sandbox routing verified for current tools.', true);
    pushTodo('review-files', 'No file-system tools executed yet.', true);
    pushTodo('audit-system', 'No system-level tools executed yet.', true);
    return todos;
  }

  pushTodo(
    'await-first-call',
    'First tool call observed; sandbox blueprint adjusted.',
    true
  );

  const allTools = unique(summaries.map((summary) => summary.toolName));
  const elevatedTools = unique(
    summaries
      .filter((summary) => summary.sandboxLevel !== 'standard')
      .map((summary) => summary.toolName)
  );

  if (allTools.length > 0) {
    const label =
      elevatedTools.length > 0
        ? `Route sandbox execution for ${elevatedTools.join(', ')} with elevated guards.`
        : 'Route all tool calls through sandbox runners.';
    pushTodo('route-sandbox-tools', label, elevatedTools.length === 0);
  } else {
    pushTodo('route-sandbox-tools', 'Sandbox routing verified for current tools.', true);
  }

  const fileTools = unique(
    summaries
      .filter((summary) => summary.sandboxLevel === 'filesystem')
      .map((summary) => summary.toolName)
  );

  if (fileTools.length > 0) {
    pushTodo(
      'review-files',
      `Review filesystem access for ${fileTools.join(', ')} within sandbox.`,
      false
    );
  } else {
    pushTodo('review-files', 'No file-system tools executed yet.', true);
  }

  const systemTools = unique(
    summaries
      .filter((summary) => summary.sandboxLevel === 'system')
      .map((summary) => summary.toolName)
  );

  if (systemTools.length > 0) {
    pushTodo(
      'audit-system',
      `Audit system-level actions from ${systemTools.join(', ')} logs.`,
      false
    );
  } else {
    pushTodo('audit-system', 'No system-level tools executed yet.', true);
  }

  const runningTools = summaries.filter((summary) => summary.status === 'running');
  if (runningTools.length > 0) {
    pushTodo(
      'monitor-running',
      `Monitor ongoing tools: ${runningTools.map((tool) => tool.toolName).join(', ')}.`,
      false
    );
  } else {
    pushTodo('monitor-running', 'No active sandbox tools to monitor.', true);
  }

  const erroredTools = summaries.filter((summary) => summary.status === 'error');
  if (erroredTools.length > 0) {
    pushTodo(
      'inspect-failures',
      `Inspect failures from ${erroredTools.map((tool) => tool.toolName).join(', ')}.`,
      false
    );
  } else {
    pushTodo('inspect-failures', 'No sandbox failures detected.', true);
  }

  return todos;
}

export function buildEnvironmentPlan(
  sessionId: string,
  summaries: ToolCallSummary[],
  previousPlan?: SessionEnvironmentPlan
): SessionEnvironmentPlan {
  const generatedAt = previousPlan?.generatedAt ?? new Date().toISOString();
  const lastUpdatedAt = new Date().toISOString();
  const toolsUsed = unique(summaries.map((summary) => summary.toolName)).sort();
  const sandboxRequired = summaries.some((summary) => summary.requiresSandbox);
  const sandboxStrategy: SandboxStrategy = sandboxRequired ? 'required' : 'recommended';

  const blueprint: EnvironmentBlueprint = {
    id: previousPlan?.blueprint.id ?? `env-${sessionId}`,
    title: sandboxRequired ? 'Dedicated sandbox workspace' : 'Reusable sandbox workspace',
    description: `Automatically provisioned environment for session ${sessionId}.`,
    recommendedCapabilities: buildCapabilities(summaries),
    persistenceHint: 'Persist sandbox state per session to reuse tool context safely.',
  };

  const plan: SessionEnvironmentPlan = {
    sessionId,
    sandboxStrategy,
    toolsUsed,
    generatedAt,
    lastUpdatedAt,
    notes: buildNotes(sessionId, summaries),
    blueprint,
    toolSummaries: summaries,
    todos: buildTodos(sessionId, summaries, previousPlan),
  };

  if (previousPlan) {
    const previousSnapshot = JSON.stringify(snapshotPlan(previousPlan));
    const nextSnapshot = JSON.stringify(snapshotPlan(plan));

    if (previousSnapshot === nextSnapshot) {
      return { ...plan, lastUpdatedAt: previousPlan.lastUpdatedAt };
    }
  }

  return plan;
}

export function formatEnvironmentPlanShareText(plan: SessionEnvironmentPlan): string {
  const tools = plan.toolsUsed.length ? plan.toolsUsed.join(', ') : 'none';
  const capabilityLine = plan.blueprint.recommendedCapabilities.join(', ');
  const todos = plan.todos.map((todo) => `- [${todo.completed ? 'x' : ' '}] ${todo.label}`);

  const lines = [
    `Sandbox plan for session ${plan.sessionId}`,
    `Strategy: ${plan.sandboxStrategy}`,
    `Generated: ${plan.generatedAt}`,
    `Updated: ${plan.lastUpdatedAt}`,
    `Tools: ${tools}`,
    '',
    plan.blueprint.title,
    plan.blueprint.description,
    `Capabilities: ${capabilityLine}`,
    `Persistence: ${plan.blueprint.persistenceHint}`,
    '',
    'Todos:',
    ...todos,
  ];

  return lines.join('\n');
}

export function serializeEnvironmentPlan(plan: SessionEnvironmentPlan): SerializedEnvironmentPlan {
  return {
    sessionId: plan.sessionId,
    sandboxStrategy: plan.sandboxStrategy,
    toolsUsed: plan.toolsUsed,
    generatedAt: plan.generatedAt,
    lastUpdatedAt: plan.lastUpdatedAt,
    notes: plan.notes,
    blueprint: plan.blueprint,
    todos: plan.todos.map(({ id, label, completed }) => ({ id, label, completed })),
  };
}
