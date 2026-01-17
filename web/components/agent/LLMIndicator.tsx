"use client";

import { useEffect, useMemo, useState } from "react";
import { getRuntimeConfigSnapshot } from "@/lib/api";
import type { RuntimeConfigSnapshot } from "@/lib/types";

const SOURCE_LABELS: Record<string, string> = {
  default: "default",
  file: "file",
  environment: "env",
  override: "override",
  codex_cli: "codex-cli",
  claude_cli: "claude-cli",
  antigravity_cli: "antigravity-cli",
};

export function LLMIndicator() {
  const [snapshot, setSnapshot] = useState<RuntimeConfigSnapshot | null>(null);

  useEffect(() => {
    let active = true;
    const load = async () => {
      try {
        const data = await getRuntimeConfigSnapshot();
        if (active) {
          setSnapshot(data);
        }
      } catch (error) {
        if (active) {
          setSnapshot(null);
        }
      }
    };
    load();
    return () => {
      active = false;
    };
  }, []);

  const { provider, model, authSource } = useMemo(() => {
    const effective = snapshot?.effective;
    const sources = snapshot?.sources ?? {};
    const rawSource = sources.api_key ?? "default";
    return {
      provider: effective?.llm_provider ?? "unknown",
      model: effective?.llm_model ?? "unknown",
      authSource: SOURCE_LABELS[rawSource] ?? rawSource,
    };
  }, [snapshot]);

  return (
    <div className="pointer-events-none fixed bottom-4 left-4 z-40 flex items-center gap-2 rounded-full border border-border/60 bg-background/80 px-3 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur">
      <span className="font-semibold text-foreground">{provider}</span>
      <span aria-hidden>·</span>
      <span>{model}</span>
      <span aria-hidden>·</span>
      <span>auth: {authSource}</span>
    </div>
  );
}
