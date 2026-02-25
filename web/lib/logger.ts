import { isDebugModeEnabled } from "./debugMode";

export type LogLevel = "debug" | "info" | "warn" | "error";

type LogContext = Record<string, unknown>;

export interface Logger {
  debug(message: string, context?: LogContext): void;
  info(message: string, context?: LogContext): void;
  warn(message: string, context?: LogContext): void;
  error(message: string, context?: LogContext): void;
  child(namespace: string): Logger;
}

const LEVEL_PRIORITY: Record<LogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
};

function shouldLog(level: LogLevel): boolean {
  if (level === "error" || level === "warn") return true;
  return isDebugModeEnabled();
}

function formatPrefix(namespaces: string[]): string {
  return namespaces.map((ns) => `[${ns}]`).join("");
}

function createLoggerImpl(namespaces: string[]): Logger {
  const prefix = formatPrefix(namespaces);

  function log(level: LogLevel, message: string, context?: LogContext): void {
    if (!shouldLog(level)) return;

    const method = level === "debug" ? "log" : level;
    const fn = console[method] as (...args: unknown[]) => void;

    if (context && Object.keys(context).length > 0) {
      fn(`${prefix} ${message}`, context);
    } else {
      fn(`${prefix} ${message}`);
    }
  }

  return {
    debug: (message, context) => log("debug", message, context),
    info: (message, context) => log("info", message, context),
    warn: (message, context) => log("warn", message, context),
    error: (message, context) => log("error", message, context),
    child(namespace: string): Logger {
      return createLoggerImpl([...namespaces, namespace]);
    },
  };
}

export function createLogger(namespace: string): Logger {
  return createLoggerImpl([namespace]);
}
