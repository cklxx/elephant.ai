"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw, Save, RotateCcw } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/toast";
import { APIError, getContextConfig, getContextConfigPreview, updateContextConfig } from "@/lib/api";
import type { ContextConfigFile, ContextConfigSnapshot, ContextWindowPreviewResponse } from "@/lib/types";
import { cn } from "@/lib/utils";

const SECTION_ORDER = ["personas", "goals", "policies", "knowledge", "worlds"] as const;
const DEFAULT_PREVIEW_OVERRIDES = {
  personaKey: "",
  goalKey: "",
  worldKey: "",
};

type PreviewOverrides = typeof DEFAULT_PREVIEW_OVERRIDES;

function formatSectionLabel(section: string) {
  if (!section) return "Context";
  return section.slice(0, 1).toUpperCase() + section.slice(1);
}

function buildPreview(content: string) {
  const lines = content.split(/\r?\n/);
  const firstLine = lines.find((line) => line.trim().length > 0) ?? "";
  if (!firstLine) return "Empty file";
  if (firstLine.length > 120) {
    return `${firstLine.slice(0, 120)}…`;
  }
  return firstLine;
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

export default function ContextConfigPage() {
  const [snapshot, setSnapshot] = useState<ContextConfigSnapshot | null>(null);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [preview, setPreview] = useState<ContextWindowPreviewResponse | null>(null);
  const [previewOverrides, setPreviewOverrides] = useState<PreviewOverrides>(DEFAULT_PREVIEW_OVERRIDES);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const loadSnapshot = useCallback(async (options?: { preserveDrafts?: boolean }) => {
    const preserveDrafts = options?.preserveDrafts ?? false;
    setLoading(true);
    try {
      const response = await getContextConfig();
      setSnapshot(response);
      setDrafts((prev) => {
        const next: Record<string, string> = {};
        for (const file of response.files) {
          next[file.path] = preserveDrafts && prev[file.path] !== undefined ? prev[file.path] : file.content;
        }
        return next;
      });
      setSelectedPath((prev) => {
        if (preserveDrafts && prev && response.files.some((file) => file.path === prev)) {
          return prev;
        }
        return response.files[0]?.path ?? null;
      });
    } catch (error) {
      toast.error("Failed to load context files", getErrorDetails(error));
    } finally {
      setLoading(false);
    }
  }, []);

  const loadPreview = useCallback(async (overrides: PreviewOverrides) => {
    setLoadingPreview(true);
    setPreviewError(null);
    try {
      const response = await getContextConfigPreview({
        toolMode: "web",
        personaKey: overrides.personaKey || undefined,
        goalKey: overrides.goalKey || undefined,
        worldKey: overrides.worldKey || undefined,
      });
      setPreview(response);
    } catch (error) {
      setPreview(null);
      setPreviewError(getErrorDetails(error));
    } finally {
      setLoadingPreview(false);
    }
  }, []);

  useEffect(() => {
    void loadSnapshot();
  }, [loadSnapshot]);

  useEffect(() => {
    void loadPreview(DEFAULT_PREVIEW_OVERRIDES);
  }, [loadPreview]);

  const filesByPath = useMemo(() => {
    const map = new Map<string, ContextConfigFile>();
    if (!snapshot) return map;
    for (const file of snapshot.files) {
      map.set(file.path, file);
    }
    return map;
  }, [snapshot]);

  const dirtyPaths = useMemo(() => {
    if (!snapshot) return [];
    return snapshot.files
      .filter((file) => drafts[file.path] !== undefined && drafts[file.path] !== file.content)
      .map((file) => file.path);
  }, [snapshot, drafts]);

  const dirtySet = useMemo(() => new Set(dirtyPaths), [dirtyPaths]);

  const filteredFiles = useMemo(() => {
    if (!snapshot) return [];
    const query = search.trim().toLowerCase();
    if (!query) return snapshot.files;
    return snapshot.files.filter((file) => {
      const haystack = `${file.path}\n${file.content}`.toLowerCase();
      return haystack.includes(query);
    });
  }, [snapshot, search]);

  const sectionEntries = useMemo(() => {
    const grouped = new Map<string, ContextConfigFile[]>();
    for (const file of filteredFiles) {
      const section = file.section || file.path.split("/")[0] || "misc";
      const list = grouped.get(section) ?? [];
      list.push(file);
      grouped.set(section, list);
    }

    const entries: Array<[string, ContextConfigFile[]]> = [];
    const visited = new Set<string>();
    for (const section of SECTION_ORDER) {
      const files = grouped.get(section);
      if (files) {
        files.sort((a, b) => a.path.localeCompare(b.path));
        entries.push([section, files]);
        visited.add(section);
      }
    }
    for (const [section, files] of grouped.entries()) {
      if (visited.has(section)) continue;
      files.sort((a, b) => a.path.localeCompare(b.path));
      entries.push([section, files]);
    }
    return entries;
  }, [filteredFiles]);

  const selectedFile = selectedPath ? filesByPath.get(selectedPath) ?? null : null;
  const selectedDraft = selectedPath ? drafts[selectedPath] ?? "" : "";
  const selectedDirty = selectedPath ? dirtySet.has(selectedPath) : false;

  const handleSelect = useCallback((path: string) => {
    setSelectedPath(path);
  }, []);

  const handleDraftChange = useCallback(
    (value: string) => {
      if (!selectedPath) return;
      setDrafts((prev) => ({ ...prev, [selectedPath]: value }));
    },
    [selectedPath],
  );

  const handleReload = useCallback(async () => {
    if (dirtyPaths.length > 0) {
      const confirm = window.confirm("Discard unsaved changes and reload from disk?");
      if (!confirm) {
        return;
      }
    }
    await loadSnapshot();
  }, [dirtyPaths.length, loadSnapshot]);

  const handleRevert = useCallback(() => {
    if (!selectedFile || !selectedPath) return;
    setDrafts((prev) => ({ ...prev, [selectedPath]: selectedFile.content }));
  }, [selectedFile, selectedPath]);

  const handleSaveCurrent = useCallback(async () => {
    if (!selectedFile || !selectedPath) return;
    if (!selectedDirty) {
      toast.info("No changes to save");
      return;
    }
    setSaving(true);
    try {
      const response = await updateContextConfig({
        files: [{ path: selectedPath, content: drafts[selectedPath] ?? "" }],
      });
      const updatedMap = new Map(response.files.map((file) => [file.path, file]));
      const updated = updatedMap.get(selectedPath);
      setSnapshot((prev) => {
        if (!prev) return response;
        return {
          ...prev,
          root: response.root,
          files: prev.files.map((file) => updatedMap.get(file.path) ?? file),
        };
      });
      if (updated) {
        setDrafts((prev) => ({ ...prev, [selectedPath]: updated.content }));
      }
      toast.success("Context file saved", selectedPath);
      void loadPreview(previewOverrides);
    } catch (error) {
      toast.error("Failed to save context file", getErrorDetails(error));
    } finally {
      setSaving(false);
    }
  }, [selectedFile, selectedPath, selectedDirty, drafts, loadPreview, previewOverrides]);

  const handleSaveAll = useCallback(async () => {
    if (!snapshot) return;
    const dirtyFiles = snapshot.files.filter((file) => dirtySet.has(file.path));
    if (dirtyFiles.length === 0) {
      toast.info("No changes to save");
      return;
    }
    setSaving(true);
    try {
      const payload = {
        files: dirtyFiles.map((file) => ({
          path: file.path,
          content: drafts[file.path] ?? "",
        })),
      };
      const response = await updateContextConfig(payload);
      const updatedMap = new Map(response.files.map((file) => [file.path, file]));
      setSnapshot((prev) => {
        if (!prev) return response;
        return {
          ...prev,
          root: response.root,
          files: prev.files.map((file) => updatedMap.get(file.path) ?? file),
        };
      });
      setDrafts((prev) => {
        const next = { ...prev };
        for (const path of updatedMap.keys()) {
          if (next[path] !== undefined) {
            next[path] = updatedMap.get(path)?.content ?? next[path];
          }
        }
        return next;
      });
      toast.success("Context files saved", `${dirtyFiles.length} file(s) updated`);
      void loadPreview(previewOverrides);
    } catch (error) {
      toast.error("Failed to save context files", getErrorDetails(error));
    } finally {
      setSaving(false);
    }
  }, [snapshot, dirtySet, drafts, loadPreview, previewOverrides]);

  const selectedLineCount = useMemo(() => {
    if (!selectedDraft) return 0;
    return selectedDraft.split(/\r?\n/).length;
  }, [selectedDraft]);

  const previewTokenEstimate = useMemo(() => {
    if (preview && typeof preview.token_estimate === "number" && preview.token_estimate > 0) {
      return preview.token_estimate;
    }
    const prompt = preview?.window?.system_prompt?.trim();
    if (!prompt) return null;
    return Math.floor(prompt.length / 4);
  }, [preview]);

  return (
      <div className="min-h-screen bg-muted/40 p-6">
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-6">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="space-y-2">
              <h1 className="text-2xl font-semibold text-foreground">Context config editor</h1>
              <p className="text-sm text-muted-foreground">
                Edit personas, goals, policies, knowledge, and worlds. Static context refreshes on cache expiry (30 minutes)
                or process restart.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {dirtyPaths.length > 0 && (
                <Badge variant="warning">{dirtyPaths.length} unsaved</Badge>
              )}
              <Button variant="outline" onClick={handleReload} disabled={loading || saving}>
                <RefreshCw className="h-4 w-4" />
                Reload
              </Button>
              <Button onClick={handleSaveAll} disabled={saving || dirtyPaths.length === 0}>
                {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                Save all
              </Button>
            </div>
          </div>

          <Card>
            <CardHeader>
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div>
                  <CardTitle>System prompt preview</CardTitle>
                  <CardDescription>
                    Generated from the current context config in web mode (environment section omitted).
                  </CardDescription>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="info">web mode</Badge>
                  <Badge variant="secondary">
                    Tokens: {previewTokenEstimate ?? "—"}
                  </Badge>
                  <Button
                    variant="outline"
                    onClick={() => loadPreview(previewOverrides)}
                    disabled={loadingPreview}
                  >
                    <RefreshCw className={cn("h-4 w-4", loadingPreview && "animate-spin")} />
                    Refresh preview
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 md:grid-cols-3">
                <Input
                  placeholder="Persona key (optional)"
                  value={previewOverrides.personaKey}
                  onChange={(event) =>
                    setPreviewOverrides((prev) => ({ ...prev, personaKey: event.target.value }))
                  }
                />
                <Input
                  placeholder="Goal key (optional)"
                  value={previewOverrides.goalKey}
                  onChange={(event) =>
                    setPreviewOverrides((prev) => ({ ...prev, goalKey: event.target.value }))
                  }
                />
                <Input
                  placeholder="World key (optional)"
                  value={previewOverrides.worldKey}
                  onChange={(event) =>
                    setPreviewOverrides((prev) => ({ ...prev, worldKey: event.target.value }))
                  }
                />
              </div>
              {previewError && <p className="text-sm text-destructive">{previewError}</p>}
              <div className="grid gap-4 lg:grid-cols-2">
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">System prompt</p>
                  <ScrollArea className="h-[360px] rounded-lg border bg-muted/30" viewportClassName="p-4">
                    <pre className="whitespace-pre-wrap text-xs text-foreground">
                      {preview?.window?.system_prompt?.trim() || "No preview loaded yet."}
                    </pre>
                  </ScrollArea>
                </div>
                <div className="space-y-2">
                  <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Context window JSON</p>
                  <ScrollArea className="h-[360px] rounded-lg border bg-muted/30" viewportClassName="p-4">
                    <pre className="whitespace-pre-wrap text-xs text-foreground">
                      {preview ? JSON.stringify(preview.window, null, 2) : "No preview loaded yet."}
                    </pre>
                  </ScrollArea>
                </div>
              </div>
            </CardContent>
          </Card>

          <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
            <Card className="h-full">
              <CardHeader>
                <CardTitle>Context files</CardTitle>
                <CardDescription>
                  {snapshot ? `${snapshot.files.length} files` : "Loading..."}
                  {snapshot?.root ? ` · ${snapshot.root}` : ""}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <Input
                  placeholder="Filter files or content"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
                <ScrollArea className="h-[520px] pr-3" viewportClassName="pb-2">
                  {loading && (
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      Loading context files...
                    </div>
                  )}
                  {!loading && sectionEntries.length === 0 && (
                    <div className="text-sm text-muted-foreground">No files match the current filter.</div>
                  )}
                  {!loading &&
                    sectionEntries.map(([section, files]) => (
                      <div key={section} className="mb-4 space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                            {formatSectionLabel(section)}
                          </span>
                          <Badge variant="secondary">{files.length}</Badge>
                        </div>
                        <div className="space-y-2">
                          {files.map((file) => {
                            const isSelected = file.path === selectedPath;
                            const preview = buildPreview(drafts[file.path] ?? file.content);
                            return (
                              <button
                                type="button"
                                key={file.path}
                                onClick={() => handleSelect(file.path)}
                                className={cn(
                                  "w-full rounded-lg border px-3 py-2 text-left transition",
                                  isSelected
                                    ? "border-primary/40 bg-primary/5"
                                    : "border-border/60 hover:bg-muted/60",
                                )}
                              >
                                <div className="flex items-center justify-between gap-2">
                                  <span className="text-sm font-medium text-foreground">{file.name}</span>
                                  {dirtySet.has(file.path) && (
                                    <Badge variant="warning">Modified</Badge>
                                  )}
                                </div>
                                <p className="mt-1 truncate text-xs text-muted-foreground">{preview}</p>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    ))}
                </ScrollArea>
              </CardContent>
            </Card>

            <Card className="h-full">
              <CardHeader className="gap-3 border-b border-border/60">
                <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                  <div>
                    <CardTitle>Editor</CardTitle>
                    <CardDescription>
                      {selectedFile ? selectedFile.path : "Select a file to begin editing."}
                    </CardDescription>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    {selectedFile && (
                      <Badge variant="outline">{formatSectionLabel(selectedFile.section)}</Badge>
                    )}
                    {selectedDirty && <Badge variant="warning">Modified</Badge>}
                    <Button
                      variant="outline"
                      onClick={handleRevert}
                      disabled={!selectedDirty || saving || !selectedFile}
                    >
                      <RotateCcw className="h-4 w-4" />
                      Revert
                    </Button>
                    <Button onClick={handleSaveCurrent} disabled={!selectedDirty || saving || !selectedFile}>
                      {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                      Save
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-4 pt-6">
                {!selectedFile && (
                  <div className="rounded-lg border border-dashed border-border/70 bg-muted/40 px-4 py-12 text-center text-sm text-muted-foreground">
                    Choose a context file from the left to edit its YAML.
                  </div>
                )}
                {selectedFile && (
                  <>
                    <Textarea
                      value={selectedDraft}
                      onChange={(event) => handleDraftChange(event.target.value)}
                      className="min-h-[420px] font-mono text-xs leading-relaxed"
                      spellCheck={false}
                    />
                    <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                      <span>{selectedLineCount} lines</span>
                      {selectedFile.updated_at && (
                        <span>Updated {new Date(selectedFile.updated_at).toLocaleString()}</span>
                      )}
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
  );
}
