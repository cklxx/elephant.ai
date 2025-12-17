import { useEffect, useMemo, useState } from 'react';

function parseTimestampMs(timestamp?: string | null): number | null {
  if (!timestamp) return null;
  const ms = new Date(timestamp).getTime();
  return Number.isNaN(ms) ? null : ms;
}

export function useElapsedDurationMs(options: {
  startTimestamp?: string | null;
  running: boolean;
  tickMs?: number;
}): number | null {
  const startMs = useMemo(
    () => parseTimestampMs(options.startTimestamp),
    [options.startTimestamp],
  );
  const [elapsedMs, setElapsedMs] = useState<number | null>(() => {
    if (!options.running || startMs == null) return null;
    return Math.max(0, Date.now() - startMs);
  });

  useEffect(() => {
    if (!options.running || startMs == null) {
      return;
    }

    setElapsedMs(Math.max(0, Date.now() - startMs));
    const interval = window.setInterval(() => {
      setElapsedMs(Math.max(0, Date.now() - startMs));
    }, options.tickMs ?? 250);

    return () => {
      window.clearInterval(interval);
    };
  }, [options.running, options.tickMs, startMs]);

  return elapsedMs;
}

