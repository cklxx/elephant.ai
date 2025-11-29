export type StepStatus = 'planned' | 'active' | 'done' | 'failed';

export interface TimelineStep {
  id: string;
  title: string;
  description?: string;
  status: StepStatus;
  startTime?: number;
  endTime?: number;
  duration?: number;
  toolsUsed?: string[];
  tokensUsed?: number;
  result?: string;
  error?: string;
  anchorEventIndex?: number;
}
