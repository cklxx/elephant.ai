export type LogFileSnippet = {
  path?: string;
  entries?: string[];
  truncated?: boolean;
  error?: string;
};

export type LogTraceBundle = {
  log_id: string;
  service: LogFileSnippet;
  llm: LogFileSnippet;
  latency: LogFileSnippet;
  requests: LogFileSnippet;
};

export type LogIndexEntry = {
  log_id: string;
  last_seen: string;
  service_count: number;
  llm_count: number;
  latency_count: number;
  request_count: number;
  error_count: number;
  last_error_class?: string;
  last_error_at?: string;
  total_count: number;
  sources?: string[];
};

export type LogIndexResponse = {
  entries: LogIndexEntry[];
  has_more?: boolean;
};

// Structured log types

export type ParsedTextLogEntry = {
  raw: string;
  timestamp: string;
  level: string;
  category: string;
  component: string;
  log_id?: string;
  source_file?: string;
  source_line?: number;
  message: string;
};

export type ParsedRequestLogEntry = {
  raw: string;
  timestamp: string;
  request_id: string;
  log_id?: string;
  entry_type: string;
  body_bytes: number;
  mode?: string;
  provider?: string;
  model?: string;
  intent?: string;
  stage?: string;
  error_class?: string;
  error?: string;
  latency_ms?: number;
  payload?: unknown;
  payload_text?: string;
};

export type StructuredLogSnippet = {
  path?: string;
  entries?: ParsedTextLogEntry[];
  truncated?: boolean;
  error?: string;
};

export type StructuredRequestSnippet = {
  path?: string;
  entries?: ParsedRequestLogEntry[];
  truncated?: boolean;
  error?: string;
};

export type StructuredLogBundle = {
  log_id: string;
  service: StructuredLogSnippet;
  llm: StructuredLogSnippet;
  latency: StructuredLogSnippet;
  requests: StructuredRequestSnippet;
  errors: StructuredRequestSnippet;
};
