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
  total_count: number;
  sources?: string[];
};

export type LogIndexResponse = {
  entries: LogIndexEntry[];
};
