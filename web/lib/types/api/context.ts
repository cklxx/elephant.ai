import type { AttachmentPayload } from '../ui/attachment';
import type { UserPersonaProfile } from './persona';

export type MessageSource =
  | 'system_prompt'
  | 'user_input'
  | 'user_history'
  | 'assistant_reply'
  | 'tool_result'
  | 'debug'
  | 'evaluation';

export interface ToolCall {
  id: string;
  name: string;
  arguments: Record<string, any>;
  session_id?: string;
  task_id?: string;
  parent_task_id?: string;
}

export interface ToolResult {
  call_id: string;
  content: string;
  error?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload> | null;
}

export interface Message {
  role: string;
  content: string;
  tool_calls?: ToolCall[];
  tool_results?: ToolResult[];
  tool_call_id?: string;
  metadata?: Record<string, any>;
  attachments?: Record<string, AttachmentPayload> | null;
  source?: MessageSource;
}

export interface PersonaProfile {
  id?: string;
  tone?: string;
  risk_profile?: string;
  decision_style?: string;
  voice?: string;
}

export interface GoalProfile {
  id?: string;
  long_term?: string[];
  mid_term?: string[];
  success_metrics?: string[];
}

export interface PolicyRule {
  id?: string;
  hard_constraints?: string[];
  soft_preferences?: string[];
  reward_hooks?: string[];
}

export interface KnowledgeReference {
  id?: string;
  description?: string;
  sop_refs?: string[];
  memory_keys?: string[];
}

export interface WorldProfile {
  id?: string;
  environment?: string;
  capabilities?: string[];
  limits?: string[];
  cost_model?: string[];
}

export interface PlanNode {
  id: string;
  title: string;
  status: string;
  description?: string;
  children?: PlanNode[];
}

export interface Belief {
  statement: string;
  confidence?: number;
  source?: string;
}

export interface FeedbackSignal {
  kind: string;
  message: string;
  value?: number;
  created_at?: string;
}

export interface MemoryFragment {
  key: string;
  content: string;
  created_at?: string;
  source?: string;
}

export interface StaticContext {
  persona?: PersonaProfile;
  goal?: GoalProfile;
  policies?: PolicyRule[];
  knowledge?: KnowledgeReference[];
  tools?: string[];
  world?: WorldProfile;
  user_persona?: UserPersonaProfile;
  environment_summary?: string;
  version?: string;
}

export interface DynamicContext {
  turn_id?: number;
  llm_turn_seq?: number;
  plans?: PlanNode[];
  beliefs?: Belief[];
  world_state?: Record<string, any>;
  feedback?: FeedbackSignal[];
  snapshot_timestamp?: string;
}

export interface MetaContext {
  memories?: MemoryFragment[];
  recommendations?: string[];
  persona_version?: string;
}

export interface ContextWindow {
  session_id: string;
  messages: Message[];
  system_prompt?: string;
  static?: StaticContext;
  dynamic?: DynamicContext;
  meta?: MetaContext;
}

export interface ContextWindowPreviewResponse {
  session_id: string;
  token_estimate?: number;
  token_limit?: number;
  persona_key?: string;
  tool_mode?: string;
  tool_preset?: string;
  window: ContextWindow;
}
