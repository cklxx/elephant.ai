import type { AnyAgentEvent } from '../events/workflow';

export interface Session {
  id: string;
  title?: string | null;
  created_at: string;
  updated_at: string;
  task_count: number;
  last_task?: string | null;
}

export interface SessionTaskSummary {
  task_id: string;
  parent_task_id?: string | null;
  status: string;
  created_at: string;
  updated_at?: string;
  final_answer?: string | null;
}

export interface SessionListResponse {
  sessions: Session[];
}

export interface SessionDetailsResponse {
  session: Session;
  tasks: SessionTaskSummary[];
}

export interface ShareTokenResponse {
  session_id: string;
  share_token: string;
}

export interface SharedSessionResponse {
  session_id: string;
  title?: string | null;
  created_at?: string;
  updated_at?: string;
  events: AnyAgentEvent[];
}
