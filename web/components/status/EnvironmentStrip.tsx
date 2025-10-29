'use client';

import { useDiagnostics } from '@/hooks/useDiagnostics';

const KEY_WHITELIST = ['HOSTNAME', 'USER', 'SANDBOX_BASE_URL'];

function formatEnv(env?: Record<string, string>) {
  if (!env) {
    return '';
  }

  return KEY_WHITELIST
    .map((key) => {
      const value = env[key];
      return value ? `${key}=${value}` : null;
    })
    .filter((value): value is string => Boolean(value))
    .join(' · ');
}

export function EnvironmentStrip() {
  const { environments } = useDiagnostics();

  if (!environments) {
    return null;
  }

  const hostSummary = formatEnv(environments.host);
  const sandboxSummary = formatEnv(environments.sandbox);

  const parts: string[] = [];
  if (hostSummary) {
    parts.push(`Host: ${hostSummary}`);
  }
  if (sandboxSummary) {
    parts.push(`Sandbox: ${sandboxSummary}`);
  }

  if (parts.length === 0) {
    return null;
  }

  return (
    <div
      className="mt-1 text-xs text-muted-foreground truncate"
      data-testid="environment-strip"
      aria-live="polite"
    >
      {parts.join(' | ')}
    </div>
  );
}
