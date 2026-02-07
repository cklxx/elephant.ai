export interface EvaluationJob {
  id: string;
  status: string;
  error?: string;
  agent_id?: string;
  dataset_path?: string;
  instance_limit?: number;
  max_workers?: number;
  timeout_seconds?: number;
  started_at?: string;
  completed_at?: string;
  summary?: AnalysisSummary;
  metrics?: EvaluationMetrics;
  agent?: AgentProfile;
}

export interface AnalysisSummary {
  overall_score: number;
  success_rate: number;
  total_tasks: number;
  completed_tasks: number;
  failed_tasks: number;
}

export interface EvaluationMetrics {
  performance?: Record<string, number>;
  quality?: Record<string, number>;
  resource?: Record<string, number>;
}

export interface AgentProfile {
  id: string;
  name?: string;
  model?: string;
  version?: string;
  evaluation_count?: number;
  average_score?: number;
  best_score?: number;
  last_evaluated?: string;
  tags?: string[];
}

export interface WorkerResultSummary {
  task_id: string;
  instance_id: string;
  status: string;
  duration_seconds?: number;
  tokens_used?: number;
  cost?: number;
  auto_score?: number;
  grade?: string;
  error?: string;
  files_changed?: number;
  tool_traces?: number;
}

export interface EvaluationDetail {
  evaluation: EvaluationJob;
  analysis?: any;
  results?: WorkerResultSummary[];
  agent?: AgentProfile;
}
