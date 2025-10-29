import { create } from 'zustand';
import { SandboxProgressEvent } from '@/lib/types';

type SandboxProgressSnapshot = {
  status: SandboxProgressEvent['status'];
  stage: string;
  message?: string;
  step: number;
  total_steps: number;
  error?: string;
  updated: string;
};

interface SandboxProgressState {
  progress: SandboxProgressSnapshot | null;
  setProgress: (event: SandboxProgressEvent) => void;
}

const useSandboxProgressStore = create<SandboxProgressState>((set) => ({
  progress: null,
  setProgress: (event) =>
    set({
      progress: {
        status: event.status,
        stage: event.stage,
        message: event.message,
        step: event.step,
        total_steps: event.total_steps,
        error: event.error,
        updated: event.updated,
      },
    }),
}));

export function useSandboxProgress() {
  return useSandboxProgressStore((state) => state);
}

export function handleSandboxProgress(event: SandboxProgressEvent) {
  useSandboxProgressStore.getState().setProgress(event);
}

export function resetSandboxProgress() {
  useSandboxProgressStore.setState({ progress: null });
}
