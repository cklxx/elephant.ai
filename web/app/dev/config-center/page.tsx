"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";

import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import {
  getServerConfigSnapshot,
  updateServerConfigSnapshot,
} from "@/lib/api";
import {
  ConfigCenterSnapshot,
  ServerConfigPayload,
} from "@/lib/types";

const emptyRuntimeList: string[] = [];

function normalizeConfig(config: ServerConfigPayload): ServerConfigPayload {
  return {
    ...config,
    runtime: {
      ...config.runtime,
      stop_sequences: Array.isArray(config.runtime.stop_sequences)
        ? [...config.runtime.stop_sequences]
        : [...emptyRuntimeList],
    },
    auth: { ...config.auth },
    analytics: { ...config.analytics },
  };
}

export default function ConfigCenterPage() {
  const [snapshot, setSnapshot] = useState<ConfigCenterSnapshot | null>(null);
  const [draft, setDraft] = useState<ServerConfigPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchSnapshot = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getServerConfigSnapshot();
      setSnapshot(response);
      setDraft(normalizeConfig(response.config));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load config");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchSnapshot();
  }, [fetchSnapshot]);

  const lastUpdated = useMemo(() => {
    if (!snapshot?.updated_at) {
      return "Unknown";
    }
    const date = new Date(snapshot.updated_at);
    return isNaN(date.getTime()) ? "Unknown" : date.toLocaleString();
  }, [snapshot]);

  const stopSequences = useMemo(() => {
    return (draft?.runtime.stop_sequences ?? []).join(", ");
  }, [draft]);

  const handleRuntimeChange = (
    key: keyof ServerConfigPayload["runtime"],
    value: string | number | boolean | string[],
  ) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        runtime: {
          ...prev.runtime,
          [key]: value,
        },
      };
    });
  };

  const handleAuthChange = (
    key: keyof ServerConfigPayload["auth"],
    value: string,
  ) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        auth: {
          ...prev.auth,
          [key]: value,
        },
      };
    });
  };

  const handleAnalyticsChange = (
    key: keyof ServerConfigPayload["analytics"],
    value: string,
  ) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        analytics: {
          ...prev.analytics,
          [key]: value,
        },
      };
    });
  };

  const handleServerChange = (
    key: keyof Omit<ServerConfigPayload, "runtime" | "auth" | "analytics">,
    value: string | boolean,
  ) => {
    setDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        [key]: value,
      } as ServerConfigPayload;
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!draft) return;
    setSaving(true);
    try {
      const response = await updateServerConfigSnapshot(draft);
      setSnapshot(response);
      setDraft(normalizeConfig(response.config));
      toast.success("Configuration updated", "All changes applied to the server");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      toast.error("Failed to update configuration", message);
    } finally {
      setSaving(false);
    }
  };

  if (loading && !draft) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50">
        <p className="rounded-full border border-slate-200 px-4 py-2 text-sm text-slate-500">
          Loading configuration…
        </p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-slate-50 px-4 py-10 text-slate-900 sm:px-8">
      <div className="mx-auto max-w-6xl space-y-8">
        <section className="flex flex-col gap-4 rounded-3xl border border-slate-200 bg-white/90 p-6 shadow-sm backdrop-blur">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-400">
              Internal Tools
            </p>
            <h1 className="text-3xl font-bold text-slate-900">Configuration Center</h1>
            <p className="mt-2 text-sm text-slate-500">
              Edit every server setting from one place. Changes are persisted to the
              shared config backend and broadcast to all connected clients.
            </p>
          </div>
          <div className="flex flex-wrap gap-6 text-sm text-slate-500">
            <div>
              <span className="font-semibold text-slate-700">Current version:</span>{" "}
              {snapshot?.version ?? "–"}
            </div>
            <div>
              <span className="font-semibold text-slate-700">Last updated:</span>{" "}
              {lastUpdated}
            </div>
            <div>
              <span className="font-semibold text-slate-700">Environment:</span>{" "}
              {draft?.runtime.environment ?? "unknown"}
            </div>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button
              type="button"
              variant="outline"
              disabled={loading || saving}
              onClick={fetchSnapshot}
            >
              Refresh snapshot
            </Button>
            <Button
              type="submit"
              form="config-form"
              disabled={!draft || saving}
            >
              {saving ? "Applying changes…" : "Apply all changes"}
            </Button>
          </div>
          {error && (
            <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700">
              {error}
            </p>
          )}
        </section>

        <form
          id="config-form"
          className="space-y-6"
          onSubmit={handleSubmit}
        >
          {draft && (
            <>
              <ConfigSection title="Runtime">
                <div className="grid gap-4 md:grid-cols-2">
                  <TextField
                    label="LLM Provider"
                    value={draft.runtime.llm_provider ?? ""}
                    onChange={(value) => handleRuntimeChange("llm_provider", value)}
                  />
                  <TextField
                    label="LLM Model"
                    value={draft.runtime.llm_model ?? ""}
                    onChange={(value) => handleRuntimeChange("llm_model", value)}
                  />
                  <TextField
                    label="Base URL"
                    value={draft.runtime.base_url ?? ""}
                    onChange={(value) => handleRuntimeChange("base_url", value)}
                  />
                  <TextField
                    label="Sandbox Base URL"
                    value={draft.runtime.sandbox_base_url ?? ""}
                    onChange={(value) => handleRuntimeChange("sandbox_base_url", value)}
                  />
                  <SecretField
                    label="API Key"
                    value={draft.runtime.api_key ?? ""}
                    onChange={(value) => handleRuntimeChange("api_key", value)}
                  />
                  <SecretField
                    label="Ark API Key"
                    value={draft.runtime.ark_api_key ?? ""}
                    onChange={(value) => handleRuntimeChange("ark_api_key", value)}
                  />
                <SecretField
                  label="Tavily API Key"
                  value={draft.runtime.tavily_api_key ?? ""}
                  onChange={(value) => handleRuntimeChange("tavily_api_key", value)}
                />
                <TextField
                  label="Seedream text endpoint ID"
                  value={draft.runtime.seedream_text_endpoint_id ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_text_endpoint_id", value)}
                />
                <TextField
                  label="Seedream image endpoint ID"
                  value={draft.runtime.seedream_image_endpoint_id ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_image_endpoint_id", value)}
                />
                <TextField
                  label="Seedream text model"
                  value={draft.runtime.seedream_text_model ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_text_model", value)}
                />
                <TextField
                  label="Seedream image model"
                  value={draft.runtime.seedream_image_model ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_image_model", value)}
                />
                <TextField
                  label="Seedream vision model"
                  value={draft.runtime.seedream_vision_model ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_vision_model", value)}
                />
                <TextField
                  label="Seedream video model"
                  value={draft.runtime.seedream_video_model ?? ""}
                  onChange={(value) => handleRuntimeChange("seedream_video_model", value)}
                />
                <NumberField
                  label="Max Tokens"
                  value={draft.runtime.max_tokens ?? 0}
                  onChange={(value) => handleRuntimeChange("max_tokens", value)}
                />
                  <NumberField
                    label="Max Iterations"
                    value={draft.runtime.max_iterations ?? 0}
                    onChange={(value) => handleRuntimeChange("max_iterations", value)}
                  />
                  <NumberField
                    label="Temperature"
                    step="0.1"
                    value={draft.runtime.temperature ?? 0}
                    onChange={(value) => handleRuntimeChange("temperature", value)}
                  />
                  <NumberField
                    label="Top P"
                    step="0.1"
                    value={draft.runtime.top_p ?? 0}
                    onChange={(value) => handleRuntimeChange("top_p", value)}
                  />
                  <TextField
                    label="Agent Preset"
                    value={draft.runtime.agent_preset ?? ""}
                    onChange={(value) => handleRuntimeChange("agent_preset", value)}
                  />
                  <TextField
                    label="Tool Preset"
                    value={draft.runtime.tool_preset ?? ""}
                    onChange={(value) => handleRuntimeChange("tool_preset", value)}
                  />
                  <TextField
                    label="Environment"
                    value={draft.runtime.environment ?? ""}
                    onChange={(value) => handleRuntimeChange("environment", value)}
                  />
                  <TextField
                    label="Session Directory"
                    value={draft.runtime.session_dir ?? ""}
                    onChange={(value) => handleRuntimeChange("session_dir", value)}
                  />
                  <TextField
                    label="Cost Directory"
                    value={draft.runtime.cost_dir ?? ""}
                    onChange={(value) => handleRuntimeChange("cost_dir", value)}
                  />
                  <div className="md:col-span-2">
                    <label className="text-sm font-semibold text-slate-600">
                      Stop sequences
                    </label>
                    <textarea
                      className="mt-2 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm shadow-inner focus:outline-none focus:ring-2 focus:ring-slate-900/10"
                      rows={2}
                      value={stopSequences}
                      onChange={(event) => {
                        const entries = event.target.value
                          .split(",")
                          .map((entry) => entry.trim())
                          .filter(Boolean);
                        handleRuntimeChange("stop_sequences", entries);
                      }}
                    />
                    <p className="mt-1 text-xs text-slate-400">
                      Comma-separated values applied to `stop_sequences`.
                    </p>
                  </div>
                  <ToggleField
                    label="Verbose logging"
                    checked={draft.runtime.verbose ?? false}
                    onChange={(checked) => handleRuntimeChange("verbose", checked)}
                  />
                  <ToggleField
                    label="Follow transcript"
                    checked={draft.runtime.follow_transcript ?? false}
                    onChange={(checked) => handleRuntimeChange("follow_transcript", checked)}
                  />
                <ToggleField
                  label="Follow stream"
                  checked={draft.runtime.follow_stream ?? false}
                  onChange={(checked) => handleRuntimeChange("follow_stream", checked)}
                />
                <ToggleField
                  label="Disable TUI"
                  checked={draft.runtime.disable_tui ?? false}
                  onChange={(checked) => handleRuntimeChange("disable_tui", checked)}
                />
              </div>
            </ConfigSection>

              <ConfigSection title="Server">
                <div className="grid gap-4 md:grid-cols-2">
                  <TextField
                    label="HTTP Port"
                    value={draft.port}
                    onChange={(value) => handleServerChange("port", value)}
                  />
                  <ToggleField
                    label="Enable MCP"
                    checked={draft.enable_mcp}
                    onChange={(checked) => handleServerChange("enable_mcp", checked)}
                  />
                  <div className="md:col-span-2">
                    <label className="text-sm font-semibold text-slate-600">
                      Environment summary
                    </label>
                    <textarea
                      className="mt-2 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm shadow-inner focus:outline-none focus:ring-2 focus:ring-slate-900/10"
                      rows={2}
                      value={draft.environment_summary ?? ""}
                      onChange={(event) =>
                        handleServerChange("environment_summary", event.target.value)
                      }
                    />
                  </div>
                </div>
              </ConfigSection>

              <ConfigSection title="Authentication">
                <div className="grid gap-4 md:grid-cols-2">
                  <SecretField
                    label="JWT Secret"
                    value={draft.auth.jwt_secret ?? ""}
                    onChange={(value) => handleAuthChange("jwt_secret", value)}
                  />
                  <TextField
                    label="Access token TTL (minutes)"
                    value={draft.auth.access_token_ttl_minutes ?? ""}
                    onChange={(value) => handleAuthChange("access_token_ttl_minutes", value)}
                  />
                  <TextField
                    label="Refresh token TTL (days)"
                    value={draft.auth.refresh_token_ttl_days ?? ""}
                    onChange={(value) => handleAuthChange("refresh_token_ttl_days", value)}
                  />
                  <TextField
                    label="State TTL (minutes)"
                    value={draft.auth.state_ttl_minutes ?? ""}
                    onChange={(value) => handleAuthChange("state_ttl_minutes", value)}
                  />
                  <TextField
                    label="Redirect base URL"
                    value={draft.auth.redirect_base_url ?? ""}
                    onChange={(value) => handleAuthChange("redirect_base_url", value)}
                  />
                  <TextField
                    label="Google Client ID"
                    value={draft.auth.google_client_id ?? ""}
                    onChange={(value) => handleAuthChange("google_client_id", value)}
                  />
                  <SecretField
                    label="Google Client Secret"
                    value={draft.auth.google_client_secret ?? ""}
                    onChange={(value) => handleAuthChange("google_client_secret", value)}
                  />
                  <TextField
                    label="Google Auth URL"
                    value={draft.auth.google_auth_url ?? ""}
                    onChange={(value) => handleAuthChange("google_auth_url", value)}
                  />
                  <TextField
                    label="Google Token URL"
                    value={draft.auth.google_token_url ?? ""}
                    onChange={(value) => handleAuthChange("google_token_url", value)}
                  />
                  <TextField
                    label="Google User Info URL"
                    value={draft.auth.google_userinfo_url ?? ""}
                    onChange={(value) => handleAuthChange("google_userinfo_url", value)}
                  />
                  <TextField
                    label="WeChat App ID"
                    value={draft.auth.wechat_app_id ?? ""}
                    onChange={(value) => handleAuthChange("wechat_app_id", value)}
                  />
                  <TextField
                    label="WeChat Auth URL"
                    value={draft.auth.wechat_auth_url ?? ""}
                    onChange={(value) => handleAuthChange("wechat_auth_url", value)}
                  />
                  <TextField
                    label="Database URL"
                    value={draft.auth.database_url ?? ""}
                    onChange={(value) => handleAuthChange("database_url", value)}
                  />
                  <TextField
                    label="Bootstrap email"
                    value={draft.auth.bootstrap_email ?? ""}
                    onChange={(value) => handleAuthChange("bootstrap_email", value)}
                  />
                  <SecretField
                    label="Bootstrap password"
                    value={draft.auth.bootstrap_password ?? ""}
                    onChange={(value) => handleAuthChange("bootstrap_password", value)}
                  />
                  <TextField
                    label="Bootstrap display name"
                    value={draft.auth.bootstrap_display_name ?? ""}
                    onChange={(value) => handleAuthChange("bootstrap_display_name", value)}
                  />
                </div>
              </ConfigSection>

              <ConfigSection title="Analytics">
                <div className="grid gap-4 md:grid-cols-2">
                  <SecretField
                    label="PostHog API key"
                    value={draft.analytics.posthog_api_key ?? ""}
                    onChange={(value) => handleAnalyticsChange("posthog_api_key", value)}
                  />
                  <TextField
                    label="PostHog host"
                    value={draft.analytics.posthog_host ?? ""}
                    onChange={(value) => handleAnalyticsChange("posthog_host", value)}
                  />
                </div>
              </ConfigSection>
            </>
          )}
        </form>
      </div>
    </main>
  );
}

interface TextFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
}

function TextField({ label, value, onChange }: TextFieldProps) {
  return (
    <label className="flex flex-col text-sm font-semibold text-slate-600">
      {label}
      <input
        className="mt-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm shadow-inner focus:outline-none focus:ring-2 focus:ring-slate-900/10"
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

interface SecretFieldProps extends TextFieldProps {}

function SecretField({ label, value, onChange }: SecretFieldProps) {
  return (
    <label className="flex flex-col text-sm font-semibold text-slate-600">
      {label}
      <input
        className="mt-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm shadow-inner focus:outline-none focus:ring-2 focus:ring-slate-900/10"
        type="password"
        value={value}
        autoComplete="off"
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

interface NumberFieldProps {
  label: string;
  value: number;
  step?: string;
  onChange: (value: number) => void;
}

function NumberField({ label, value, step, onChange }: NumberFieldProps) {
  return (
    <label className="flex flex-col text-sm font-semibold text-slate-600">
      {label}
      <input
        className="mt-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm shadow-inner focus:outline-none focus:ring-2 focus:ring-slate-900/10"
        type="number"
        value={Number.isFinite(value) ? value : 0}
        step={step}
        onChange={(event) => {
          const parsed = Number(event.target.value);
          onChange(Number.isFinite(parsed) ? parsed : 0);
        }}
      />
    </label>
  );
}

interface ToggleFieldProps {
  label: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}

function ToggleField({ label, checked, onChange }: ToggleFieldProps) {
  return (
    <label className="flex items-center gap-3 text-sm font-semibold text-slate-600">
      <input
        type="checkbox"
        className="h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-slate-900/30"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
      />
      {label}
    </label>
  );
}

interface ConfigSectionProps {
  title: string;
  children: ReactNode;
}

function ConfigSection({ title, children }: ConfigSectionProps) {
  return (
    <section className="rounded-3xl border border-slate-200 bg-white p-6 shadow-sm">
      <h2 className="text-lg font-semibold text-slate-900">{title}</h2>
      <div className="mt-4 space-y-4">{children}</div>
    </section>
  );
}
