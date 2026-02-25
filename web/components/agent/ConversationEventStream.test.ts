import { describe, it, expect } from 'vitest';
import type { AnyAgentEvent } from '@/lib/types';

// Import the functions to test (we'll need to export them from the component file)
// For now, we inline the key logic tests

describe('Subagent anchor logic', () => {
  describe('getSubagentAnchorId', () => {
    it('should extract anchor from call_id starting with subagent', () => {
      const event = {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        call_id: 'subagent_123',
      } as AnyAgentEvent;

      const anchorId = extractAnchorId(event);
      expect(anchorId).toBe('call:subagent_123');
    });

    it('should extract anchor from parent_run_id + run_id', () => {
      const event = {
        event_type: 'workflow.node.output.summary',
        agent_level: 'subagent',
        parent_run_id: 'run_parent',
        run_id: 'run_child',
      } as AnyAgentEvent;

      const anchorId = extractAnchorId(event);
      expect(anchorId).toBe('run:run_parent:run_child');
    });

    it('should extract anchor from subtask_index when run_id missing', () => {
      const event = {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        parent_run_id: 'run_parent',
        subtask_index: 2,
      } as unknown as AnyAgentEvent;

      const anchorId = extractAnchorId(event);
      expect(anchorId).toBe('subtask:run_parent:2');
    });

    it('should return undefined when no anchor info available', () => {
      const event = {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
      } as AnyAgentEvent;

      const anchorId = extractAnchorId(event);
      expect(anchorId).toBeUndefined();
    });
  });

  describe('buildInterleavedEntries ordering', () => {
    it('should place subagent group after its anchor event', () => {
      const displayEntries = [
        { kind: 'event' as const, event: createMainEvent(1, 'event1') },
        { kind: 'event' as const, event: createMainEvent(2, 'subagent_delegation') },
        { kind: 'event' as const, event: createMainEvent(3, 'event3') },
      ];

      const subagentThreads = [
        createSubagentThread('sub1', 'call:subagent_123', 2),
      ];

      const anchorMap = new Map([
        ['call:subagent_123', { timestamp: 2000, eventIndex: 1 }],
      ]);

      const result = buildInterleavedEntries(displayEntries, subagentThreads, anchorMap);

      // Find positions
      const posEvent1 = result.findIndex(e => e.kind === 'event' && e.event.timestamp === '1000');
      const posSubagent = result.findIndex(e => e.kind === 'subagentGroup');
      const posEvent3 = result.findIndex(e => e.kind === 'event' && e.event.timestamp === '3000');

      // Subagent should be after event1 and before event3
      expect(posEvent1).toBeLessThan(posSubagent);
      expect(posSubagent).toBeLessThan(posEvent3);
    });

    it('should sort multiple subagents by anchor index', () => {
      const displayEntries = [
        { kind: 'event' as const, event: createMainEvent(1, 'event1') },
        { kind: 'event' as const, event: createMainEvent(2, 'subagent_A_trigger') },
        { kind: 'event' as const, event: createMainEvent(3, 'event2') },
        { kind: 'event' as const, event: createMainEvent(4, 'subagent_B_trigger') },
      ];

      const subagentThreads = [
        createSubagentThread('sub_B', 'call:subagent_B', 3),
        createSubagentThread('sub_A', 'call:subagent_A', 1),
      ];

      const anchorMap = new Map([
        ['call:subagent_A', { timestamp: 2000, eventIndex: 1 }],
        ['call:subagent_B', { timestamp: 4000, eventIndex: 3 }],
      ]);

      const result = buildInterleavedEntries(displayEntries, subagentThreads, anchorMap);

      const subagentIndices = result
        .map((e, idx) => ({ kind: e.kind, idx }))
        .filter(e => e.kind === 'subagentGroup')
        .map(e => e.idx);

      // Subagent A should come before Subagent B
      expect(subagentIndices[0]).toBeLessThan(subagentIndices[1]);
    });

    it('should fallback to timestamp when no anchor', () => {
      const displayEntries = [
        { kind: 'event' as const, event: createMainEvent(1, 'early') },
        { kind: 'event' as const, event: createMainEvent(2, 'late') },
      ];

      const subagentThreads = [
        createSubagentThread('sub1', undefined, 0, 1500), // Timestamp between early and late
      ];

      const anchorMap = new Map();

      const result = buildInterleavedEntries(displayEntries, subagentThreads, anchorMap);

      const posEarly = result.findIndex(e => e.kind === 'event' && e.event.timestamp === '1000');
      const posSubagent = result.findIndex(e => e.kind === 'subagentGroup');
      const posLate = result.findIndex(e => e.kind === 'event' && e.event.timestamp === '2000');

      expect(posEarly).toBeLessThan(posSubagent);
      expect(posSubagent).toBeLessThan(posLate);
    });
  });
});

// Helper functions for test data creation
function createMainEvent(index: number, label: string): AnyAgentEvent {
  return {
    event_type: 'workflow.node.output.summary',
    agent_level: 'core',
    timestamp: `${index * 1000}`,
    node_id: label,
    content: `Content ${label}`,
  } as AnyAgentEvent;
}

function createSubagentThread(
  key: string,
  anchorId: string | undefined,
  anchorIndex: number,
  timestamp?: number,
): any {
  return {
    key,
    groupKey: key,
    context: { preview: `Subagent ${key}` },
    events: [],
    subtaskIndex: 0,
    firstSeenAt: timestamp ?? (anchorIndex * 1000 + 500),
    firstArrival: anchorIndex,
    anchorEventId: anchorId,
  };
}

// Placeholder functions - these should be imported from the component
function extractAnchorId(event: AnyAgentEvent): string | undefined {
  const callId = 'call_id' in event && typeof event.call_id === 'string'
    ? event.call_id
    : undefined;
  if (callId?.startsWith('subagent')) {
    return `call:${callId}`;
  }

  const parentRunId = 'parent_run_id' in event && typeof event.parent_run_id === 'string'
    ? event.parent_run_id
    : undefined;
  const runId = 'run_id' in event && typeof event.run_id === 'string'
    ? event.run_id
    : undefined;

  if (parentRunId && runId) {
    return `run:${parentRunId}:${runId}`;
  }

  const subtaskIndex = 'subtask_index' in event && typeof event.subtask_index === 'number'
    ? event.subtask_index
    : undefined;
  if (parentRunId && typeof subtaskIndex === 'number') {
    return `subtask:${parentRunId}:${subtaskIndex}`;
  }

  return undefined;
}

function buildInterleavedEntries(
  displayEntries: any[],
  subagentThreads: any[],
  anchorMap: Map<string, { timestamp: number | null; eventIndex: number }>,
): any[] {
  // Simplified version for testing
  const result: any[] = [];
  const insertedGroups = new Set<string>();

  displayEntries.forEach((entry, currentIndex) => {
    // Find groups to insert after this position
    const groupsToInsert = subagentThreads
      .filter(thread => {
        if (insertedGroups.has(thread.groupKey)) return false;
        if (thread.anchorEventId) {
          const anchor = anchorMap.get(thread.anchorEventId);
          if (anchor) {
            return anchor.eventIndex <= currentIndex;
          }
        }
        // Fallback to timestamp
        return thread.firstSeenAt <= (entry.event?.timestamp ?? 0);
      })
      .sort((a, b) => {
        const aAnchor = a.anchorEventId ? anchorMap.get(a.anchorEventId)?.eventIndex ?? Infinity : Infinity;
        const bAnchor = b.anchorEventId ? anchorMap.get(b.anchorEventId)?.eventIndex ?? Infinity : Infinity;
        return aAnchor - bAnchor;
      });

    groupsToInsert.forEach((thread) => {
      if (!insertedGroups.has(thread.groupKey)) {
        insertedGroups.add(thread.groupKey);
        result.push({
          kind: 'subagentGroup',
          groupKey: thread.groupKey,
          threads: [thread],
          ts: thread.firstSeenAt,
          order: currentIndex,
        });
      }
    });

    result.push({ ...entry, ts: Date.parse(entry.event?.timestamp ?? '') || 0, order: currentIndex });
  });

  // Add remaining groups at end
  subagentThreads.forEach((thread) => {
    if (!insertedGroups.has(thread.groupKey)) {
      insertedGroups.add(thread.groupKey);
      result.push({
        kind: 'subagentGroup',
        groupKey: thread.groupKey,
        threads: [thread],
        ts: thread.firstSeenAt,
        order: displayEntries.length,
      });
    }
  });

  return result;
}
