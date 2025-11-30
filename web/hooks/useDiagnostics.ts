import { create } from 'zustand';
import { WorkflowDiagnosticEnvironmentSnapshotEvent } from '@/lib/types';

interface EnvironmentSnapshotState {
  host: Record<string, string>;
  sandbox: Record<string, string>;
  captured: string;
}

interface DiagnosticsState {
  environments: EnvironmentSnapshotState | null;
  setEnvironment: (event: WorkflowDiagnosticEnvironmentSnapshotEvent) => void;
}

const useDiagnosticsStore = create<DiagnosticsState>((set) => ({
  environments: null,
  setEnvironment: (event) =>
    set({
      environments: {
        host: event.host ?? {},
        sandbox: event.sandbox ?? {},
        captured: event.captured,
      },
    }),
}));

export function useDiagnostics() {
  return useDiagnosticsStore((state) => state);
}

export function handleEnvironmentSnapshot(event: WorkflowDiagnosticEnvironmentSnapshotEvent) {
  useDiagnosticsStore.getState().setEnvironment(event);
}

export function resetDiagnostics() {
  useDiagnosticsStore.setState({ environments: null });
}
