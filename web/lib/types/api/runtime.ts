export type ConfigReadinessSeverity = "critical" | "warning" | "info";

export interface ConfigReadinessTask {
  id: string;
  label: string;
  hint?: string;
  severity?: ConfigReadinessSeverity;
}

export type RuntimeConfigOverrides = Partial<{
  llm_provider: string;
  llm_model: string;
  base_url: string;
  api_key: string;
  ark_api_key: string;
  tavily_api_key: string;
  seedream_text_endpoint_id: string;
  seedream_image_endpoint_id: string;
  seedream_text_model: string;
  seedream_image_model: string;
  seedream_vision_model: string;
  seedream_video_model: string;
  environment: string;
  agent_preset: string;
  tool_preset: string;
  session_dir: string;
  cost_dir: string;
  max_tokens: number;
  max_iterations: number;
  temperature: number;
  top_p: number;
  stop_sequences: string[];
  verbose: boolean;
  disable_tui: boolean;
  follow_transcript: boolean;
  follow_stream: boolean;
}>;

export interface RuntimeConfigOverridesPayload {
  overrides: RuntimeConfigOverrides;
}

export interface RuntimeConfigSnapshot {
  effective?: RuntimeConfigOverrides;
  overrides?: RuntimeConfigOverrides;
  readiness?: ConfigReadinessTask[];
  sources?: Record<string, string>;
  tasks?: ConfigReadinessTask[];
  updated_at?: string;
}

export interface RuntimeModelProvider {
  provider: string;
  display_name?: string;
  source: string;
  auth_mode?: string;
  base_url?: string;
  models?: string[];
  default_model?: string;
  recommended_models?: RuntimeModelRecommendation[];
  key_create_url?: string;
  selectable?: boolean;
  setup_hint?: string;
  error?: string;
}

export interface RuntimeModelCatalog {
  providers: RuntimeModelProvider[];
}

export interface RuntimeModelRecommendation {
  id: string;
  tier?: string;
  default?: boolean;
  note?: string;
}

export interface SandboxBrowserInfo {
  user_agent: string;
  cdp_url: string;
  vnc_url: string;
  viewport: {
    width: number;
    height: number;
  };
}

export interface ContextConfigFile {
  path: string;
  section: string;
  name: string;
  content: string;
  updated_at?: string;
}

export interface ContextConfigSnapshot {
  root: string;
  files: ContextConfigFile[];
}

export interface ContextConfigUpdateFile {
  path: string;
  content: string;
}

export interface ContextConfigUpdatePayload {
  files: ContextConfigUpdateFile[];
}
