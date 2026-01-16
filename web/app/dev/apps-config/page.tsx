"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/toast";
import { APIError, getAppsConfigSnapshot, updateAppsConfig } from "@/lib/api";
import type { AppPluginConfig, AppsConfigSnapshot } from "@/lib/types";
import { cn } from "@/lib/utils";

const emptyPlugin = (): AppPluginConfig => ({
  id: "",
  name: "",
  description: "",
  integration_note: "",
  capabilities: [],
  sources: [],
});

function formatList(values?: string[]) {
  if (!values || values.length === 0) return "";
  return values.join("\n");
}

function parseList(value: string) {
  return value
    .split(/[,\n]/)
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function getErrorDetails(error: unknown) {
  if (error instanceof APIError) {
    return error.details || error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Unknown error";
}

export default function AppsConfigPage() {
  const [snapshot, setSnapshot] = useState<AppsConfigSnapshot | null>(null);
  const [plugins, setPlugins] = useState<AppPluginConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const initialPlugins = snapshot?.apps?.plugins ?? [];
  const isDirty = useMemo(() => {
    return JSON.stringify(plugins) !== JSON.stringify(initialPlugins);
  }, [plugins, initialPlugins]);

  const loadSnapshot = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getAppsConfigSnapshot();
      setSnapshot(response);
      setPlugins(response.apps?.plugins ?? []);
    } catch (error) {
      toast.error("Failed to load apps config", getErrorDetails(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSnapshot();
  }, [loadSnapshot]);

  const updatePlugin = useCallback((index: number, patch: Partial<AppPluginConfig>) => {
    setPlugins((prev) =>
      prev.map((plugin, idx) => (idx === index ? { ...plugin, ...patch } : plugin)),
    );
  }, []);

  const handleAdd = useCallback(() => {
    setPlugins((prev) => [...prev, emptyPlugin()]);
  }, []);

  const handleRemove = useCallback((index: number) => {
    setPlugins((prev) => prev.filter((_, idx) => idx !== index));
  }, []);

  const handleReload = useCallback(() => {
    void loadSnapshot();
  }, [loadSnapshot]);

  const handleSave = useCallback(async () => {
    setSaving(true);
    try {
      const response = await updateAppsConfig({ apps: { plugins } });
      setSnapshot(response);
      setPlugins(response.apps?.plugins ?? []);
      toast.success("Apps config saved");
    } catch (error) {
      toast.error("Failed to save apps config", getErrorDetails(error));
    } finally {
      setSaving(false);
    }
  }, [plugins]);

  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-6xl flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <p className="text-[11px] font-semibold text-slate-400">Dev Tools</p>
            <h1 className="mt-2 text-xl font-semibold text-slate-900 lg:text-2xl">
              Apps config
            </h1>
            <p className="mt-2 text-sm text-slate-600">
              Manage custom app connector entries. Custom plugins override built-in IDs.
            </p>
            {snapshot?.path && (
              <p className="mt-3 text-xs text-slate-500">Config path: {snapshot.path}</p>
            )}
          </header>

          <div className="flex flex-wrap items-center gap-3">
            <Button variant="outline" onClick={handleReload} disabled={loading || saving}>
              <RefreshCw className="mr-2 h-4 w-4" />
              Reload
            </Button>
            <Button variant="outline" onClick={handleAdd} disabled={loading || saving}>
              <Plus className="mr-2 h-4 w-4" />
              Add plugin
            </Button>
            <Button onClick={handleSave} disabled={loading || saving || !isDirty}>
              {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Save className="mr-2 h-4 w-4" />}
              Save changes
            </Button>
            {isDirty && (
              <Badge className="bg-amber-100 text-amber-800 hover:bg-amber-100">Unsaved</Badge>
            )}
          </div>

          {loading ? (
            <Card>
              <CardContent className="flex items-center justify-center py-12 text-sm text-slate-500">
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Loading apps config…
              </CardContent>
            </Card>
          ) : plugins.length === 0 ? (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">No custom plugins yet</CardTitle>
                <CardDescription>
                  Add plugins to extend the built-in apps catalog.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <Button variant="outline" onClick={handleAdd}>
                  <Plus className="mr-2 h-4 w-4" />
                  Add plugin
                </Button>
              </CardContent>
            </Card>
          ) : (
            <div className="grid gap-4">
              {plugins.map((plugin, index) => {
                const title = plugin.id?.trim() || `Plugin ${index + 1}`;
                return (
                  <Card key={`${title}-${index}`} className={cn("border-slate-200")}>
                    <CardHeader className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                      <div>
                        <CardTitle className="text-base">{title}</CardTitle>
                        <CardDescription>Custom connector definition</CardDescription>
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleRemove(index)}
                        className="text-rose-600 hover:text-rose-700"
                      >
                        <Trash2 className="mr-2 h-4 w-4" />
                        Remove
                      </Button>
                    </CardHeader>
                    <CardContent className="grid gap-4">
                      <div className="grid gap-4 md:grid-cols-2">
                        <div className="grid gap-2">
                          <label className="text-xs font-semibold text-slate-600">ID</label>
                          <Input
                            value={plugin.id ?? ""}
                            onChange={(event) =>
                              updatePlugin(index, { id: event.target.value })
                            }
                            placeholder="wechat"
                          />
                        </div>
                        <div className="grid gap-2">
                          <label className="text-xs font-semibold text-slate-600">Name</label>
                          <Input
                            value={plugin.name ?? ""}
                            onChange={(event) =>
                              updatePlugin(index, { name: event.target.value })
                            }
                            placeholder="WeChat"
                          />
                        </div>
                      </div>
                      <div className="grid gap-2">
                        <label className="text-xs font-semibold text-slate-600">Description</label>
                        <Textarea
                          value={plugin.description ?? ""}
                          onChange={(event) =>
                            updatePlugin(index, { description: event.target.value })
                          }
                          placeholder="Short summary of what this connector does."
                          rows={2}
                        />
                      </div>
                      <div className="grid gap-2">
                        <label className="text-xs font-semibold text-slate-600">Integration note</label>
                        <Textarea
                          value={plugin.integration_note ?? ""}
                          onChange={(event) =>
                            updatePlugin(index, { integration_note: event.target.value })
                          }
                          placeholder="Auth, rate limits, or setup notes."
                          rows={2}
                        />
                      </div>
                      <div className="grid gap-4 md:grid-cols-2">
                        <div className="grid gap-2">
                          <label className="text-xs font-semibold text-slate-600">Capabilities</label>
                          <Textarea
                            value={formatList(plugin.capabilities)}
                            onChange={(event) =>
                              updatePlugin(index, { capabilities: parseList(event.target.value) })
                            }
                            placeholder="One capability per line."
                            rows={3}
                          />
                        </div>
                        <div className="grid gap-2">
                          <label className="text-xs font-semibold text-slate-600">Sources</label>
                          <Textarea
                            value={formatList(plugin.sources)}
                            onChange={(event) =>
                              updatePlugin(index, { sources: parseList(event.target.value) })
                            }
                            placeholder="GitHub URLs, one per line."
                            rows={3}
                          />
                        </div>
                      </div>
                    </CardContent>
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </RequireAuth>
  );
}
