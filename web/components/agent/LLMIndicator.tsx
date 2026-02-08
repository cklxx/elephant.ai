"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { getRuntimeConfigSnapshot, getSubscriptionCatalog } from "@/lib/api";
import {
  clearLLMSelection,
  loadLLMSelection,
  saveLLMSelection,
} from "@/lib/llmSelection";
import type {
  LLMSelection,
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
};

type ModelsState = "idle" | "loading" | "error";

export function LLMIndicator() {
  const [snapshot, setSnapshot] = useState<RuntimeConfigSnapshot | null>(null);
  const [selection, setSelection] = useState<LLMSelection | null>(() =>
    loadLLMSelection(),
  );
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
      const data = await getSubscriptionCatalog();
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

  const handleSelectYaml = useCallback(() => {
    clearLLMSelection();
    setSelection(null);
  }, []);

  const handleSelectModel = useCallback(
    (provider: RuntimeModelProvider, modelId: string) => {
      const next: LLMSelection = {
        mode: "cli",
        provider: provider.provider,
        model: modelId,
        source: provider.source,
      };
      saveLLMSelection(next);
      setSelection(next);
    },
    [],
  );

  const { provider, model, authSource, modelSource } = useMemo(() => {
    const effective = snapshot?.effective;
    const sources = snapshot?.sources ?? {};
    const rawAuthSource = sources.api_key ?? "default";
    const rawModelSource = sources.llm_model ?? "default";
    const selectionSource =
      selection?.source && (SOURCE_LABELS[selection.source] ?? selection.source);
    return {
      provider: selection?.provider ?? effective?.llm_provider ?? "unknown",
      model: selection?.model ?? effective?.llm_model ?? "unknown",
      authSource: selectionSource ?? SOURCE_LABELS[rawAuthSource] ?? rawAuthSource,
      modelSource: selectionSource ?? SOURCE_LABELS[rawModelSource] ?? rawModelSource,
    };
  }, [selection, snapshot]);

  const yamlActive = useMemo(() => {
    return !selection;
  }, [selection]);

  const statusMessage = useMemo(() => {
    if (modelsState === "loading") return "Loading models...";
    if (modelsState === "error") return "Failed to load models.";
    if (modelsState === "idle" && modelProviders.length === 0) {
      return "No models found.";
    }
    return "";
  }, [modelsState, modelProviders.length]);

  const sortedProviders = useMemo(() => {
    return [...modelProviders].sort((a, b) =>
      a.provider.localeCompare(b.provider),
    );
  }, [modelProviders]);

  const activeProvider = selection?.provider ?? provider;
  const activeModel = selection?.model ?? model;

  return (
    <DropdownMenu open={menuOpen} onOpenChange={handleOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="LLM indicator"
          className="fixed bottom-4 left-4 z-40 flex items-center gap-2 rounded-full border border-border/80 bg-background px-3 py-2 text-xs text-muted-foreground shadow-md transition hover:text-foreground"
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
        className="w-80 rounded-2xl border border-border/70 bg-background p-2 text-foreground shadow-xl"
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
          className="flex cursor-pointer items-center justify-between gap-2"
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
          Available models
        </DropdownMenuLabel>
        {statusMessage ? (
          <div className="px-2 pb-2 text-xs text-muted-foreground">
            {statusMessage}
          </div>
        ) : null}
        <div className="space-y-3">
          {sortedProviders.map((providerEntry) => {
            const models = providerEntry.models ?? [];
            const sourceLabel =
              SOURCE_LABELS[providerEntry.source] ?? providerEntry.source;
            return (
              <div key={providerEntry.provider} className="space-y-1">
                <div className="px-2 text-[11px] uppercase text-muted-foreground">
                  {providerEntry.provider} · {sourceLabel}
                </div>
                {providerEntry.error ? (
                  <div className="px-2 pb-2 text-xs text-rose-500">
                    {providerEntry.error}
                  </div>
                ) : null}
                {models.length === 0 && !providerEntry.error ? (
                  <div className="px-2 pb-2 text-xs text-muted-foreground">
                    No models reported.
                  </div>
                ) : null}
                {models.map((modelId) => {
                  const isActive =
                    providerEntry.provider === activeProvider &&
                    modelId === activeModel;
                  return (
                    <DropdownMenuItem
                      key={modelId}
                      onSelect={() => handleSelectModel(providerEntry, modelId)}
                      className="flex cursor-pointer items-center justify-between gap-2 data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground"
                    >
                      <span className="text-sm">{modelId}</span>
                      {isActive ? (
                        <span className="text-xs text-emerald-600">Active</span>
                      ) : null}
                    </DropdownMenuItem>
                  );
                })}
              </div>
            );
          })}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
