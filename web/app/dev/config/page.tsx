"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { buildApiUrl } from "@/lib/api-base";
import { cn } from "@/lib/utils";
import {
  getRuntimeConfigSnapshot,
  updateRuntimeConfig,
} from "@/lib/api";
import type {
  ConfigReadinessTask,
  RuntimeConfigOverrides,
  RuntimeConfigSnapshot,
} from "@/lib/types";

const CONFIG_FIELDS: ConfigField[] = [
  { key: "llm_provider", label: "LLM Provider", type: "text" },
  { key: "llm_model", label: "LLM Model", type: "text" },
  { key: "base_url", label: "LLM Base URL", type: "text" },
  { key: "api_key", label: "Primary API Key", type: "secret" },
  { key: "ark_api_key", label: "Ark API Key", type: "secret" },
  { key: "tavily_api_key", label: "Tavily API Key", type: "secret" },
  { key: "seedream_text_endpoint_id", label: "Seedream Text Endpoint", type: "text" },
  { key: "seedream_image_endpoint_id", label: "Seedream Image Endpoint", type: "text" },
  { key: "seedream_text_model", label: "Seedream Text Model", type: "text" },
  { key: "seedream_image_model", label: "Seedream Image Model", type: "text" },
  { key: "seedream_vision_model", label: "Seedream Vision Model", type: "text" },
  { key: "seedream_video_model", label: "Seedream Video Model", type: "text" },
  { key: "sandbox_base_url", label: "Sandbox Base URL", type: "text" },
  { key: "environment", label: "Runtime Environment", type: "text" },
  { key: "agent_preset", label: "Agent Preset", type: "text" },
  { key: "tool_preset", label: "Tool Preset", type: "text" },
  { key: "session_dir", label: "Session Directory", type: "text" },
  { key: "cost_dir", label: "Cost Directory", type: "text" },
  { key: "max_tokens", label: "Max Tokens", type: "number", numericKind: "int" },
  { key: "max_iterations", label: "Max Iterations", type: "number", numericKind: "int" },
  { key: "temperature", label: "Temperature", type: "number", numericKind: "float" },
  { key: "top_p", label: "Top P", type: "number", numericKind: "float" },
  { key: "stop_sequences", label: "Stop Sequences", type: "stringList" },
  { key: "verbose", label: "Verbose Logging", type: "boolean" },
  { key: "disable_tui", label: "Disable TUI", type: "boolean" },
  { key: "follow_transcript", label: "Follow Transcript", type: "boolean" },
  { key: "follow_stream", label: "Follow Stream", type: "boolean" },
];

type FieldKey = (typeof CONFIG_FIELDS)[number]["key"];

type ConfigField = {
  key: keyof RuntimeConfigOverrides;
  label: string;
  type: "text" | "secret" | "number" | "boolean" | "stringList";
  numericKind?: "int" | "float";
};

type FormState = Record<string, string>;

type StreamState = "connecting" | "connected" | "error";

type ConfigTask = ConfigReadinessTask;

function deriveConfigTasks(snapshot: RuntimeConfigSnapshot | null): ConfigTask[] {
  if (!snapshot) return [];
  const effective = snapshot.effective ?? {};
  const tasks: ConfigTask[] = [];

  const provider = (effective.llm_provider ?? "").trim();
  const model = (effective.llm_model ?? "").trim();
  const apiKey = (effective.api_key ?? "").trim();
  const sandbox = (effective.sandbox_base_url ?? "").trim();
  const tavilyKey = (effective.tavily_api_key ?? "").trim();

  const providerNeedsKey =
    provider !== "" && provider !== "mock" && provider !== "ollama";

  if (!provider) {
    tasks.push({
      id: "llm-provider",
      label: "选择默认的 LLM 提供方",
      hint: "此项会影响所有任务的推理入口，请在保存前确保已确定供应商。",
      severity: "critical",
    });
  }

  if (!model) {
    tasks.push({
      id: "llm-model",
      label: "设置默认推理模型",
      hint: "例如 deepseek/deepseek-chat 或 gpt-4.1 等模型名称。",
      severity: "critical",
    });
  }

  if (providerNeedsKey && !apiKey) {
    tasks.push({
      id: "llm-api-key",
      label: "提供对应的 API Key",
      hint: "未配置密钥时所有请求都会失败，可以暂时切换为 mock/ollama 以继续调试。",
      severity: "critical",
    });
  }

  if (!sandbox) {
    tasks.push({
      id: "sandbox-url",
      label: "配置 Sandbox Base URL",
      hint: "未设置时将回退到本地执行模式，无法复用共享沙箱。",
      severity: "warning",
    });
  }

  if (!tavilyKey) {
    tasks.push({
      id: "tavily-key",
      label: "设置 Tavily API Key",
      hint: "缺少该密钥时外部搜索/检索能力会受限，可在 Tavily 控制台申请。",
      severity: "warning",
    });
  }

  return tasks;
}

function toDisplayValue(field: ConfigField, snapshot: RuntimeConfigSnapshot | null) {
  if (!snapshot) return "";
  const overrides = snapshot.overrides ?? {};
  const effective = snapshot.effective ?? {};
  const rawValue = (overrides[field.key as keyof RuntimeConfigOverrides] ??
    (effective as Record<string, unknown>)[field.key as string]) as unknown;

  if (field.type === "boolean") {
    if (typeof rawValue === "boolean") return rawValue ? "true" : "false";
    return "";
  }

  if (field.type === "stringList") {
    if (Array.isArray(rawValue)) {
      return rawValue.join(", ");
    }
    if (typeof rawValue === "string") return rawValue;
    return "";
  }

  if (rawValue === undefined || rawValue === null) {
    return "";
  }

  return String(rawValue);
}

function buildFormState(snapshot: RuntimeConfigSnapshot | null): FormState {
  const initial: FormState = {};
  for (const field of CONFIG_FIELDS) {
    initial[field.key as string] = toDisplayValue(field, snapshot);
  }
  return initial;
}

function createOverridesFromForm(values: FormState): RuntimeConfigOverrides {
  // Build overrides using a generic record first; the metadata-driven loop
  // makes it difficult for TypeScript to infer the precise key/value mapping.
  // We'll cast it back to the RuntimeConfigOverrides shape once populated.
  const overrides: Record<string, unknown> = {};
  for (const field of CONFIG_FIELDS) {
    const raw = values[field.key as string];
    if (raw === undefined) {
      continue;
    }
    const trimmed = raw.trim();
    if (trimmed.length === 0) {
      continue;
    }

    switch (field.type) {
      case "boolean": {
        overrides[field.key] = trimmed === "true";
        break;
      }
      case "number": {
        const parsed = Number(trimmed);
        if (!Number.isNaN(parsed)) {
          overrides[field.key] = field.numericKind === "int" ? Math.trunc(parsed) : parsed;
        }
        break;
      }
      case "stringList": {
        const list = trimmed
          .split(/[,\n]/)
          .map((value) => value.trim())
          .filter(Boolean);
        overrides[field.key] = list;
        break;
      }
      default: {
        overrides[field.key] = raw;
      }
    }
  }
  return overrides as RuntimeConfigOverrides;
}

function describeSource(key: FieldKey, snapshot: RuntimeConfigSnapshot | null) {
  if (!snapshot?.sources) return "default";
  return snapshot.sources[key] ?? "default";
}

export default function ConfigAdminPage() {
  const [snapshot, setSnapshot] = useState<RuntimeConfigSnapshot | null>(null);
  const [formState, setFormState] = useState<FormState>({});
  const [initialState, setInitialState] = useState<FormState>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [streamState, setStreamState] = useState<StreamState>("connecting");
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);
  const dirtyRef = useRef(false);

  const isDirty = useMemo(() => {
    return JSON.stringify(formState) !== JSON.stringify(initialState);
  }, [formState, initialState]);

  useEffect(() => {
    dirtyRef.current = isDirty;
  }, [isDirty]);

  const applySnapshot = useCallback((next: RuntimeConfigSnapshot, force = false) => {
    setSnapshot(next);
    const derived = buildFormState(next);
    setInitialState(derived);
    if (!dirtyRef.current || force) {
      setFormState(derived);
    }
    setLastUpdated(next.updated_at ?? new Date().toISOString());
  }, []);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      const data = await getRuntimeConfigSnapshot();
      applySnapshot(data, true);
    } catch (error) {
      console.error("Failed to load config snapshot", error);
      toast.error("无法获取配置", "请检查服务器日志");
    } finally {
      setLoading(false);
    }
  }, [applySnapshot]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const remainingTasks = useMemo(() => {
    if (snapshot?.tasks) {
      return snapshot.tasks;
    }
    return deriveConfigTasks(snapshot);
  }, [snapshot]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const url = buildApiUrl("/api/internal/config/runtime/stream");
    const source = new EventSource(url, { withCredentials: true });
    source.onopen = () => setStreamState("connected");
    source.onerror = () => setStreamState("error");
    source.onmessage = (event) => {
      try {
        const data: RuntimeConfigSnapshot = JSON.parse(event.data);
        applySnapshot(data);
        setStreamState("connected");
      } catch (error) {
        console.error("Failed to parse config stream", error);
        setStreamState("error");
      }
    };
    return () => source.close();
  }, [applySnapshot]);

  const handleChange = useCallback((key: string, value: string) => {
    setFormState((prev) => ({ ...prev, [key]: value }));
  }, []);

  const handleResetField = useCallback((key: string) => {
    setFormState((prev) => ({ ...prev, [key]: initialState[key] ?? "" }));
  }, [initialState]);

  const handleSubmit = useCallback(async () => {
    try {
      setSaving(true);
      const overrides = createOverridesFromForm(formState);
      const payload = await updateRuntimeConfig({ overrides });
      applySnapshot(payload, true);
      toast.success("配置已更新", "所有服务将使用最新配置");
    } catch (error) {
      console.error("Failed to update config", error);
      toast.error("更新失败", "请重试或查看日志");
    } finally {
      setSaving(false);
    }
  }, [formState, applySnapshot]);

  const streamBadge = useMemo(() => {
    switch (streamState) {
      case "connected":
        return { label: "实时同步", className: "bg-emerald-100 text-emerald-700" };
      case "error":
        return { label: "监听中断", className: "bg-rose-100 text-rose-700" };
      default:
        return { label: "连接中", className: "bg-amber-100 text-amber-700" };
    }
  }, [streamState]);

  return (
    <RequireAuth>
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-8">
        <Card className="border-primary/30 bg-white/80">
          <CardHeader>
            <CardTitle>配置后台（内部专用）</CardTitle>
            <CardDescription>
              集中管理所有运行配置。修改后将立即持久化并广播给所有监听者。
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap items-center gap-4 text-sm text-slate-600">
            <div className="flex items-center gap-2">
              <span className={cn("rounded-full px-3 py-1 text-xs font-semibold", streamBadge.className)}>
                {streamBadge.label}
              </span>
              {lastUpdated && (
                <span>最近同步：{new Date(lastUpdated).toLocaleString()}</span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-slate-500">状态</span>
              {loading ? "加载中..." : isDirty ? "存在未保存修改" : "已同步"}
            </div>
            <div className="flex flex-1 justify-end gap-3">
              <Button variant="outline" disabled={loading} onClick={refresh}>
                重新拉取
              </Button>
              <Button
                variant="ghost"
                disabled={!isDirty || !snapshot}
                onClick={() => snapshot && applySnapshot(snapshot, true)}
              >
                放弃修改
              </Button>
              <Button onClick={handleSubmit} disabled={!isDirty || saving}>
                {saving ? "保存中..." : "一键应用配置"}
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="bg-white/90">
          <CardHeader>
            <CardTitle>就绪检查</CardTitle>
            <CardDescription>帮助快速定位仍需完善的关键配置项。</CardDescription>
          </CardHeader>
          <CardContent>
            {remainingTasks.length === 0 ? (
              <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
                所有关键配置均已准备就绪，随时可以运行最新环境。
              </div>
            ) : (
              <ul className="space-y-3">
                {remainingTasks.map((task) => (
                  <li key={task.id} className="rounded-lg border border-slate-200 px-4 py-3">
                    <div className="flex items-center gap-3">
                      <span
                        className={cn(
                          "rounded-full border px-2 py-0.5 text-xs font-semibold",
                          task.severity === "critical"
                            ? "border-rose-200 bg-rose-50 text-rose-700"
                            : "border-amber-200 bg-amber-50 text-amber-700",
                        )}
                      >
                        {task.severity === "critical" ? "必填" : "建议"}
                      </span>
                      <span className="text-sm font-medium text-slate-900">{task.label}</span>
                    </div>
                    {task.hint && <p className="mt-1 text-xs text-slate-500">{task.hint}</p>}
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        <Card className="bg-white/90">
          <CardHeader>
            <CardTitle>运行配置</CardTitle>
            <CardDescription>所有字段均会覆盖环境变量，清空以恢复默认来源。</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
              {CONFIG_FIELDS.map((field) => (
                <div key={field.key as string} className="space-y-2 rounded-lg border border-slate-200 p-4">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-slate-900" htmlFor={field.key as string}>
                      {field.label}
                    </label>
                    <span className="text-xs text-slate-400">
                      来源：{describeSource(field.key as FieldKey, snapshot)}
                    </span>
                  </div>
                  {field.type === "boolean" ? (
                    <select
                      id={field.key as string}
                      className="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm focus:border-primary focus:outline-none"
                      value={formState[field.key as string] ?? ""}
                      onChange={(event) => handleChange(field.key as string, event.target.value)}
                    >
                      <option value="">使用上游配置</option>
                      <option value="true">true</option>
                      <option value="false">false</option>
                    </select>
                  ) : field.type === "stringList" ? (
                    <textarea
                      id={field.key as string}
                      className="h-24 w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-primary focus:outline-none"
                      placeholder="使用逗号或换行分隔多个条目"
                      value={formState[field.key as string] ?? ""}
                      onChange={(event) => handleChange(field.key as string, event.target.value)}
                    />
                  ) : (
                    <input
                      id={field.key as string}
                      type={field.type === "number" ? "number" : field.type === "secret" ? "password" : "text"}
                      className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-primary focus:outline-none"
                      value={formState[field.key as string] ?? ""}
                      onChange={(event) => handleChange(field.key as string, event.target.value)}
                    />
                  )}
                  <div className="flex items-center justify-between text-xs text-slate-500">
                    <button
                      type="button"
                      className="text-primary hover:underline"
                      onClick={() => handleResetField(field.key as string)}
                    >
                      恢复当前来源
                    </button>
                    {snapshot?.overrides && snapshot.overrides[field.key] !== undefined ? (
                      <span className="text-emerald-600">已覆盖</span>
                    ) : (
                      <span className="text-slate-400">沿用来源</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      </div>
    </RequireAuth>
  );
}
