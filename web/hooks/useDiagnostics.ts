import { create } from 'zustand';
import { EnvironmentSnapshotEvent } from '@/lib/types';

interface EnvironmentSnapshotState {
  host: Record<string, string>;
  sandbox: Record<string, string>;
  captured: string;
}

interface DiagnosticsState {
  environments: EnvironmentSnapshotState | null;
  setEnvironment: (event: EnvironmentSnapshotEvent) => void;
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

export function handleEnvironmentSnapshot(event: EnvironmentSnapshotEvent) {
  useDiagnosticsStore.getState().setEnvironment(event);
}

export function resetDiagnostics() {
  useDiagnosticsStore.setState({ environments: null });
}
