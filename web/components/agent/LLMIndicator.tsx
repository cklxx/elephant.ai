"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  getRuntimeConfigSnapshot,
  getRuntimeModelCatalog,
  updateRuntimeConfig,
} from "@/lib/api";
import type {
  RuntimeConfigOverrides,
  RuntimeConfigSnapshot,
  RuntimeModelProvider,
} from "@/lib/types";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

const SOURCE_LABELS: Record<string, string> = {
  default: "default",
  file: "file",
  environment: "env",
  override: "override",
  codex_cli: "codex-cli",
  claude_cli: "claude-cli",
  antigravity_cli: "antigravity-cli",
};

type ModelsState = "idle" | "loading" | "error";

export function LLMIndicator() {
  const [snapshot, setSnapshot] = useState<RuntimeConfigSnapshot | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [modelsState, setModelsState] = useState<ModelsState>("idle");
  const [modelProviders, setModelProviders] = useState<RuntimeModelProvider[]>([]);

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

  const loadModels = useCallback(async () => {
    setModelsState("loading");
    try {
      const data = await getRuntimeModelCatalog();
      setModelProviders(data.providers ?? []);
      setModelsState("idle");
    } catch (error) {
      setModelsState("error");
    }
  }, []);

  const handleOpenChange = useCallback(
    (open: boolean) => {
      setMenuOpen(open);
      if (open) {
        void loadModels();
      }
    },
    [loadModels],
  );

  const handleSelectYaml = useCallback(async () => {
    if (!snapshot) return;
    const overrides: RuntimeConfigOverrides = { ...(snapshot.overrides ?? {}) };
    delete overrides.llm_provider;
    delete overrides.llm_model;
    delete overrides.base_url;
    try {
      const payload = await updateRuntimeConfig({ overrides });
      setSnapshot(payload);
    } catch (error) {
      console.error("Failed to reset runtime model overrides", error);
    }
  }, [snapshot]);

  const handleSelectModel = useCallback(
    async (provider: RuntimeModelProvider, modelId: string) => {
      if (!snapshot) return;
      const overrides: RuntimeConfigOverrides = {
        ...(snapshot.overrides ?? {}),
        llm_provider: provider.provider,
        llm_model: modelId,
      };
      if (provider.base_url) {
        overrides.base_url = provider.base_url;
      }
      try {
        const payload = await updateRuntimeConfig({ overrides });
        setSnapshot(payload);
      } catch (error) {
        console.error("Failed to update runtime model overrides", error);
      }
    },
    [snapshot],
  );

  const { provider, model, authSource, modelSource } = useMemo(() => {
    const effective = snapshot?.effective;
    const sources = snapshot?.sources ?? {};
    const rawAuthSource = sources.api_key ?? "default";
    const rawModelSource = sources.llm_model ?? "default";
    return {
      provider: effective?.llm_provider ?? "unknown",
      model: effective?.llm_model ?? "unknown",
      authSource: SOURCE_LABELS[rawAuthSource] ?? rawAuthSource,
      modelSource: SOURCE_LABELS[rawModelSource] ?? rawModelSource,
    };
  }, [snapshot]);

  const yamlActive = useMemo(() => {
    if (!snapshot?.sources) return true;
    const source = snapshot.sources.llm_model ?? "default";
    return source === "file" || source === "environment" || source === "default";
  }, [snapshot]);

  return (
    <DropdownMenu open={menuOpen} onOpenChange={handleOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="LLM indicator"
          className="fixed bottom-4 left-4 z-40 flex items-center gap-2 rounded-full border border-border/60 bg-background/80 px-3 py-2 text-xs text-muted-foreground shadow-sm backdrop-blur transition hover:text-foreground"
        >
          <span className="font-semibold text-foreground">{provider}</span>
          <span aria-hidden>·</span>
          <span>{model}</span>
          <span aria-hidden>·</span>
          <span>auth: {authSource}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        sideOffset={12}
        className="w-80 rounded-2xl p-2"
      >
        <DropdownMenuLabel className="text-[11px] uppercase tracking-wide text-muted-foreground">
          Current
        </DropdownMenuLabel>
        <div className="space-y-1 px-2 pb-2 text-xs text-muted-foreground">
          <div>Provider: {provider}</div>
          <div>Model: {model}</div>
          <div>Model source: {modelSource}</div>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-[11px] uppercase tracking-wide text-muted-foreground">
          YAML config
        </DropdownMenuLabel>
        <DropdownMenuItem
          onSelect={handleSelectYaml}
          className="flex items-center justify-between gap-2"
        >
          <div className="flex flex-col">
            <span className="text-sm font-medium">Use YAML config</span>
            <span className="text-xs text-muted-foreground">
              Revert to upstream runtime config
            </span>
          </div>
          {yamlActive && <span className="text-xs text-emerald-600">Active</span>}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-[11px] uppercase tracking-wide text-muted-foreground">
          CLI subscription models
        </DropdownMenuLabel>
        <div className="space-y-2 px-2 pb-2 text-xs text-muted-foreground">
          {modelsState === "loading" && <div>Loading models...</div>}
          {modelsState === "error" && <div>Failed to load models.</div>}
          {modelsState === "idle" && modelProviders.length === 0 && (
            <div>No CLI subscription models found.</div>
          )}
        </div>
        {modelProviders.map((providerEntry) => (
          <div key={providerEntry.provider} className="space-y-1">
            <div className="px-2 text-[11px] uppercase text-muted-foreground">
              {providerEntry.provider} · {providerEntry.source}
            </div>
            {providerEntry.error ? (
              <div className="px-2 pb-2 text-xs text-rose-500">
                {providerEntry.error}
              </div>
            ) : null}
            {(providerEntry.models ?? []).map((modelId) => (
              <DropdownMenuItem
                key={modelId}
                onSelect={() => handleSelectModel(providerEntry, modelId)}
                className="flex items-center justify-between gap-2"
              >
                <span className="text-sm">{modelId}</span>
                {provider === providerEntry.provider && model === modelId ? (
                  <span className="text-xs text-emerald-600">Active</span>
                ) : null}
              </DropdownMenuItem>
            ))}
          </div>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
