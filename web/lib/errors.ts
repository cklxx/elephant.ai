import type { APIError } from './api';

export interface ParsedError {
  message: string;
  details?: string;
  status?: number;
  statusText?: string;
  raw?: unknown;
}

function normalizeText(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim().length > 0
    ? value.trim()
    : undefined;
}

export function isAPIError(error: unknown): error is APIError {
  if (!(error instanceof Error)) {
    return false;
  }

  const candidate = error as Partial<APIError> & { name?: unknown };

  return (
    candidate.name === 'APIError' &&
    typeof candidate.status === 'number' &&
    typeof candidate.statusText === 'string'
  );
}

export function parseError(
  error: unknown,
  fallbackMessage: string
): ParsedError {
  if (isAPIError(error)) {
    const message = normalizeText(error.message) ?? fallbackMessage;
    const details = normalizeText(error.details);

    return {
      message,
      details: details && details !== message ? details : undefined,
      status: error.status,
      statusText: normalizeText(error.statusText),
      raw: error.payload ?? error.rawBody,
    };
  }

  if (error instanceof Error) {
    return {
      message: normalizeText(error.message) ?? fallbackMessage,
    };
  }

  return { message: fallbackMessage };
}

export function formatParsedError(parsed: ParsedError): string {
  if (parsed.details) {
    return `${parsed.message}: ${parsed.details}`;
  }
  return parsed.message;
}

export function getErrorLogPayload(error: unknown): Record<string, unknown> {
  if (isAPIError(error)) {
    return {
      type: 'APIError',
      status: error.status,
      statusText: error.statusText,
      message: error.message,
      details: error.details,
      payload: error.payload,
      rawBody: error.rawBody,
    };
  }

  if (error instanceof Error) {
    return {
      type: error.name || 'Error',
      message: error.message,
      stack: error.stack,
    };
  }

  return { type: typeof error, value: error };
}
