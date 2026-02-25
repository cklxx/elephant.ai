export type QualityTier = "gold" | "silver" | "bronze" | "reject";

export interface RLTrajectory {
  id: string;
  eval_job_id: string;
  task_id: string;
  instance_id: string;
  quality_tier: QualityTier;
  auto_score: number;
  judge_score?: number;
  grade: string;
  steps: TrajectoryStep[];
  metadata: TrajectoryMeta;
  extracted_at: string;
}

export interface TrajectoryStep {
  step_index: number;
  thought?: string;
  action: string;
  observation?: string;
  tool_call?: ToolCall;
  timestamp: string;
  reward: number;
}

export interface ToolCall {
  name: string;
  arguments: Record<string, any>;
  result?: any;
  error?: string;
  duration: number;
}

export interface TrajectoryMeta {
  total_steps: number;
  duration: number;
  tokens_used: number;
  cost: number;
  tools_used?: string[];
  outcome: string;
}

export interface QualityConfig {
  gold_min_score: number;
  silver_min_score: number;
  bronze_min_score: number;
  judge_enabled: boolean;
  borderline_lower: number;
  borderline_upper: number;
  judge_provider: string;
  judge_model: string;
}

export interface RLStats {
  updated_at: string;
  tiers: Record<QualityTier, TierInfo>;
}

export interface TierInfo {
  files: FileInfo[];
  total_count: number;
  total_bytes: number;
}

export interface FileInfo {
  name: string;
  count: number;
  bytes: number;
}
