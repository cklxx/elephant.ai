import type { AttachmentUpload } from '../ui/attachment';

export interface LLMSelection {
  mode: "yaml" | "cli";
  provider: string;
  model: string;
  source: string;
}

export interface CreateTaskRequest {
  task: string;
  session_id?: string;
  parent_task_id?: string;
  attachments?: AttachmentUpload[];
  llm_selection?: LLMSelection;
}

export interface CreateTaskResponse {
  run_id: string;
  session_id: string;
  parent_run_id?: string | null;
  status?: string;
}

export interface TaskStatusResponse {
  run_id: string;
  session_id?: string;
  parent_run_id?: string | null;
  status: string;
  created_at?: string;
  completed_at?: string | null;
  updated_at?: string;
  final_answer?: string;
  error?: string;
}
