'use client';

import { useDiagnostics } from '@/hooks/useDiagnostics';
import { useSandboxProgress } from '@/hooks/useSandboxProgress';

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
  const { progress } = useSandboxProgress();

  const progressMessage = (() => {
    if (!progress) {
      return '';
    }

    const label = progress.message || progress.stage.replace(/_/g, ' ');
    const stepInfo = progress.total_steps > 0 ? `${progress.step}/${progress.total_steps}` : '';

    switch (progress.status) {
      case 'running':
      case 'pending':
        return stepInfo ? `Sandbox initializing (${stepInfo}): ${label}` : `Sandbox initializing: ${label}`;
      case 'error':
        return `Sandbox error: ${label}`;
      default:
        return '';
    }
  })();

  if (!environments) {
    if (!progressMessage) {
      return null;
    }
    return (
      <div
        className="mt-1 text-xs text-muted-foreground truncate"
        data-testid="environment-strip"
        aria-live="polite"
      >
        {progressMessage}
      </div>
    );
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

  if (!progressMessage && parts.length === 0) {
    return null;
  }

  return (
    <div
      className="mt-1 text-xs text-muted-foreground truncate"
      data-testid="environment-strip"
      aria-live="polite"
    >
      {progressMessage && <span>{progressMessage}{parts.length > 0 ? ' · ' : ''}</span>}
      {parts.join(' | ')}
    </div>
  );
}
