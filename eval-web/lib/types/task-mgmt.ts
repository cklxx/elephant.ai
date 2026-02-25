export type TaskStatus = "active" | "archived" | "draft";
export type RunStatus = "pending" | "running" | "completed" | "failed";

export interface EvalTaskDefinition {
  id: string;
  name: string;
  description?: string;
  status: TaskStatus;
  dataset_path?: string;
  dataset_type?: string;
  config: TaskConfig;
  tags?: string[];
  schedule?: Schedule;
  metadata?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export interface TaskConfig {
  instance_limit?: number;
  max_workers?: number;
  timeout_per_task?: number;
  agent_id?: string;
  enable_metrics: boolean;
  extract_rl: boolean;
}

export interface Schedule {
  cron_expr?: string;
  enabled: boolean;
}

export interface BatchRun {
  id: string;
  task_id: string;
  eval_job_id?: string;
  status: RunStatus;
  started_at: string;
  completed_at?: string;
  error?: string;
  result_count?: number;
}
