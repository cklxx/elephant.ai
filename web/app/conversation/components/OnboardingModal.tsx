"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "@/components/ui/toast";
import { getSubscriptionCatalog, updateOnboardingState } from "@/lib/api";
import { loadLLMSelection, saveLLMSelection } from "@/lib/llmSelection";
import type { RuntimeModelProvider, RuntimeModelRecommendation } from "@/lib/types";

type OnboardingModalProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCompleted: () => void;
};

export function OnboardingModal({
  open,
  onOpenChange,
  onCompleted,
}: OnboardingModalProps) {
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [providers, setProviders] = useState<RuntimeModelProvider[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const [selectedModel, setSelectedModel] = useState<string>("");

  const loadCatalog = useCallback(async () => {
    setLoading(true);
    try {
      const catalog = await getSubscriptionCatalog();
      const filtered = (catalog.providers ?? []).filter((provider) => {
        if (provider.selectable === false) {
          return false;
        }
        return orderedModels(provider).length > 0;
      });
      const sorted = [...filtered].sort((a, b) =>
        providerRank(a.provider) - providerRank(b.provider) ||
        a.provider.localeCompare(b.provider),
      );
      setProviders(sorted);

      const persisted = loadLLMSelection();
      const initialProvider = pickInitialProvider(sorted, persisted?.provider ?? "");
      setSelectedProvider(initialProvider?.provider ?? "");
      if (initialProvider) {
        const initialModel = pickInitialModel(initialProvider, persisted?.model ?? "");
        setSelectedModel(initialModel);
      } else {
        setSelectedModel("");
      }
    } catch (error) {
      setProviders([]);
      setSelectedProvider("");
      setSelectedModel("");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!open) {
      return;
    }
    void loadCatalog();
  }, [open, loadCatalog]);

  const activeProvider = useMemo(() => {
    return providers.find((provider) => provider.provider === selectedProvider) ?? null;
  }, [providers, selectedProvider]);

  const models = useMemo(() => {
    if (!activeProvider) {
      return [];
    }
    return orderedModels(activeProvider);
  }, [activeProvider]);

  useEffect(() => {
    if (!activeProvider) {
      setSelectedModel("");
      return;
    }
    if (selectedModel && models.includes(selectedModel)) {
      return;
    }
    setSelectedModel(pickInitialModel(activeProvider, ""));
  }, [activeProvider, models, selectedModel]);

  const completeWithSelection = useCallback(async () => {
    if (!activeProvider || !selectedModel) {
      return;
    }
    setSubmitting(true);
    try {
      saveLLMSelection({
        mode: "cli",
        provider: activeProvider.provider,
        model: selectedModel,
        source: activeProvider.source,
      });
      await updateOnboardingState({
        state: {
          selected_provider: activeProvider.provider,
          selected_model: selectedModel,
          used_source: activeProvider.source,
        },
      });
      toast.success("Setup completed", "Model selection saved.");
      onCompleted();
      onOpenChange(false);
    } catch (error) {
      toast.error("Setup failed", "Please try again.");
    } finally {
      setSubmitting(false);
    }
  }, [activeProvider, onCompleted, onOpenChange, selectedModel]);

  const completeWithYAML = useCallback(async () => {
    setSubmitting(true);
    try {
      await updateOnboardingState({
        state: {
          completed_at: new Date().toISOString(),
          used_source: "yaml",
          advanced_overrides_used: true,
        },
      });
      toast.info("Using YAML defaults", "You can switch model later.");
      onCompleted();
      onOpenChange(false);
    } catch (error) {
      toast.error("Setup failed", "Please try again.");
    } finally {
      setSubmitting(false);
    }
  }, [onCompleted, onOpenChange]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-xl" showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>Welcome to elephant.ai</DialogTitle>
          <DialogDescription>
            Complete first-run setup by choosing a provider and model. Base URL is configured automatically.
          </DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="rounded-lg border border-border/60 bg-muted/20 p-4 text-sm text-muted-foreground">
            Loading subscription catalog...
          </div>
        ) : providers.length === 0 ? (
          <div className="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-4 text-sm text-muted-foreground">
            <p>No subscription providers were detected from local CLI credentials.</p>
            <p>
              Sign in with `codex login` / Claude CLI and reopen setup, or continue with YAML defaults.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <label className="block space-y-2">
              <span className="text-sm font-medium text-foreground">Provider</span>
              <select
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-sm"
                value={selectedProvider}
                onChange={(event) => setSelectedProvider(event.target.value)}
                disabled={submitting}
              >
                {providers.map((provider) => (
                  <option key={provider.provider} value={provider.provider}>
                    {(provider.display_name ?? provider.provider)} ({provider.source})
                  </option>
                ))}
              </select>
            </label>
            <label className="block space-y-2">
              <span className="text-sm font-medium text-foreground">Model</span>
              <select
                className="w-full rounded-lg border border-border/70 bg-background px-3 py-2 text-sm"
                value={selectedModel}
                onChange={(event) => setSelectedModel(event.target.value)}
                disabled={submitting}
              >
                {models.map((model) => {
                  const suffix = activeProvider?.default_model === model ? " (default)" : "";
                  return (
                    <option key={model} value={model}>
                      {model + suffix}
                    </option>
                  );
                })}
              </select>
            </label>
            {activeProvider?.base_url ? (
              <div className="rounded-lg border border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
                Base URL: {activeProvider.base_url}
              </div>
            ) : null}
            {activeProvider?.setup_hint ? (
              <p className="text-xs text-muted-foreground">{activeProvider.setup_hint}</p>
            ) : null}
          </div>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => void completeWithYAML()}
            disabled={submitting}
          >
            Use YAML Defaults
          </Button>
          <Button
            type="button"
            onClick={() => void completeWithSelection()}
            disabled={submitting || !activeProvider || !selectedModel}
          >
            Complete Setup
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function providerRank(provider: string): number {
  switch (provider) {
    case "codex":
      return 0;
    case "anthropic":
    case "claude":
      return 1;
    case "llama_server":
      return 2;
    default:
      return 50;
  }
}

function pickInitialProvider(
  providers: RuntimeModelProvider[],
  preferredProvider: string,
): RuntimeModelProvider | null {
  if (providers.length === 0) {
    return null;
  }
  if (preferredProvider) {
    const matched = providers.find((provider) => provider.provider === preferredProvider);
    if (matched) {
      return matched;
    }
  }
  return providers[0];
}

function pickInitialModel(provider: RuntimeModelProvider, preferredModel: string): string {
  const models = orderedModels(provider);
  if (models.length === 0) {
    return "";
  }
  if (preferredModel && models.includes(preferredModel)) {
    return preferredModel;
  }
  if (provider.default_model && models.includes(provider.default_model)) {
    return provider.default_model;
  }
  return models[0];
}

function orderedModels(provider: RuntimeModelProvider): string[] {
  const merged: string[] = [];
  const seen = new Set<string>();
  const push = (model: string | undefined) => {
    const normalized = (model ?? "").trim();
    if (!normalized || seen.has(normalized)) {
      return;
    }
    seen.add(normalized);
    merged.push(normalized);
  };
  (provider.recommended_models ?? []).forEach((item: RuntimeModelRecommendation) => push(item.id));
  (provider.models ?? []).forEach((model) => push(model));
  push(provider.default_model);
  return merged;
}

