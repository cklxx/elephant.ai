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
