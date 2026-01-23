export interface StartEvaluationRequest {
  dataset_path?: string;
  instance_limit?: number;
  max_workers?: number;
  timeout_seconds?: number;
  output_dir?: string;
  report_format?: string;
  enable_metrics?: boolean;
  agent_id?: string;
}

export interface EvaluationMetrics {
  performance: {
    success_rate: number;
    avg_execution_time: number;
    median_time: number;
    p95_time: number;
    timeout_rate: number;
    retry_rate: number;
  };
  quality: {
    solution_quality: number;
    error_recovery_rate: number;
    consistency_score: number;
    complexity_handling: number;
  };
  resources: {
    avg_tokens_used: number;
    total_tokens: number;
    avg_cost_per_task: number;
    total_cost: number;
    memory_usage_mb: number;
  };
  behavior: {
    avg_tool_calls: number;
    tool_usage_pattern: Record<string, number>;
    common_failures: Record<string, number>;
    error_patterns: string[];
  };
  timestamp?: string;
  total_tasks?: number;
  evaluation_id?: string;
}

export interface EvaluationAnalysisSummary {
  overall_score: number;
  performance_grade: string;
  key_strengths?: string[];
  key_weaknesses?: string[];
  risk_level?: string;
}

export interface EvaluationInsight {
  type?: string;
  title: string;
  description: string;
  impact?: string;
  confidence?: number;
}

export interface EvaluationRecommendation {
  title: string;
  description: string;
  priority?: string;
  action_items?: string[];
  expected_improvement?: string;
}

export interface EvaluationTrend {
  performance_trend?: string;
  quality_trend?: string;
  efficiency_trend?: string;
  predicted_score?: number;
  confidence_level?: number;
}

export interface EvaluationAlert {
  level?: string;
  title: string;
  description?: string;
  suggested_action?: string;
  timestamp?: string;
}

export interface EvaluationAnalysis {
  summary: EvaluationAnalysisSummary;
  insights?: EvaluationInsight[];
  recommendations?: EvaluationRecommendation[];
  trends?: EvaluationTrend;
  alerts?: EvaluationAlert[];
  timestamp?: string;
}

export interface AgentProfile {
  agent_id: string;
  config_hash?: string;
  created_at?: string;
  updated_at?: string;
  avg_success_rate?: number;
  avg_exec_time?: number;
  avg_cost_per_task?: number;
  avg_quality_score?: number;
  preferred_tools?: Record<string, number>;
  common_errors?: Record<string, number>;
  strengths?: string[];
  weaknesses?: string[];
  evaluation_count?: number;
  last_evaluated?: string;
  tags?: string[];
  description?: string;
  metadata?: Record<string, any>;
}

export interface EvaluationJobSummary {
  id: string;
  status: string;
  agent_id?: string;
  dataset_path?: string;
  instance_limit?: number;
  max_workers?: number;
  timeout_seconds?: number;
  started_at?: string;
  completed_at?: string;
  summary?: EvaluationAnalysisSummary;
  metrics?: EvaluationMetrics;
  agent?: AgentProfile;
}

export interface EvaluationWorkerResultSummary {
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
  started_at?: string;
  completed_at?: string;
}

export interface EvaluationDetailResponse {
  evaluation: EvaluationJobSummary;
  analysis?: EvaluationAnalysis;
  agent?: AgentProfile;
  results?: EvaluationWorkerResultSummary[];
}

export interface EvaluationListResponse {
  evaluations: EvaluationJobSummary[];
}
