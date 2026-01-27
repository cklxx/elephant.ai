"use client";

import { useEffect, useMemo, useState } from "react";

import { AttachmentPayload } from "@/lib/types";
import { loadAttachmentText } from "@/lib/attachment-text";
import { A2UIMessage } from "@/lib/a2ui";
import { JsonRenderTree } from "@/lib/json-render-model";
import { parseUIPayload } from "@/lib/ui-payload";
import { A2UIRenderer } from "@/components/agent/A2UIRenderer";
import { JsonRenderRenderer } from "@/components/agent/JsonRenderRenderer";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LoadingDots } from "@/components/ui/loading-states";

export function A2UIAttachmentPreview({
  attachment,
}: {
  attachment: AttachmentPayload;
}) {
  const attachmentKey = useMemo(
    () => `${attachment.name}:${attachment.data ?? ""}:${attachment.uri ?? ""}`,
    [attachment.name, attachment.data, attachment.uri],
  );

  const attachmentSnapshot = useMemo<AttachmentPayload>(
    () => ({ ...attachment }),
    [attachment],
  );

  const [state, setState] = useState<{
    key: string;
    kind: "a2ui" | "json-render" | "unknown";
    payload: string | null;
    messages: A2UIMessage[] | null;
    tree: JsonRenderTree | null;
    error: string | null;
  }>(() => ({
    key: attachmentKey,
    kind: "unknown",
    payload: null,
    messages: null,
    tree: null,
    error: null,
  }));

  const [previewHtml, setPreviewHtml] = useState<{
    key: string;
    html: string | null;
    error: string | null;
  }>(() => ({
    key: attachmentKey,
    html: null,
    error: null,
  }));

  const [activeTab, setActiveTab] = useState<"preview" | "interactive">(
    "interactive",
  );

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    loadAttachmentText(attachmentSnapshot, controller.signal)
      .then((payload) => {
        if (cancelled) return;
        const parsed = parseUIPayload(payload);
        if (parsed.kind === "a2ui") {
          setState({
            key: attachmentKey,
            kind: "a2ui",
            payload,
            messages: parsed.messages,
            tree: null,
            error: null,
          });
          return;
        }
        if (parsed.kind === "json-render") {
          setState({
            key: attachmentKey,
            kind: "json-render",
            payload,
            messages: null,
            tree: parsed.tree,
            error: null,
          });
          return;
        }
        setState({
          key: attachmentKey,
          kind: "unknown",
          payload,
          messages: [],
          tree: null,
          error: parsed.error,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setState({
          key: attachmentKey,
          kind: "unknown",
          payload: null,
          messages: [],
          tree: null,
          error: err instanceof Error ? err.message : String(err),
        });
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [attachmentKey, attachmentSnapshot]);

  useEffect(() => {
    const isCurrent = state.key === attachmentKey;
    const kind = isCurrent ? state.kind : "unknown";
    const messages = isCurrent ? state.messages : null;
    const payload = isCurrent ? state.payload : null;
    if (kind === "a2ui" && (!messages || messages.length === 0)) {
      return;
    }
    if (kind === "json-render" && !payload) {
      return;
    }
    let cancelled = false;
    const body =
      kind === "a2ui"
        ? { messages }
        : kind === "json-render"
          ? { payload }
          : null;
    if (!body) {
      return;
    }
    fetch("/api/a2ui/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    })
      .then(async (response) => {
        if (!response.ok) {
          throw new Error(`SSR preview failed (${response.status})`);
        }
        return response.text();
      })
      .then((html) => {
        if (cancelled) return;
        setPreviewHtml({ key: attachmentKey, html, error: null });
        setActiveTab((current) =>
          current === "interactive" ? "preview" : current,
        );
      })
      .catch((err) => {
        if (cancelled) return;
        setPreviewHtml({
          key: attachmentKey,
          html: null,
          error: err instanceof Error ? err.message : String(err),
        });
      });

    return () => {
      cancelled = true;
    };
  }, [attachmentKey, state.key, state.kind, state.messages, state.payload]);

  const isCurrent = state.key === attachmentKey;
  const kind = isCurrent ? state.kind : "unknown";
  const messages = isCurrent ? state.messages : null;
  const tree = isCurrent ? state.tree : null;
  const error = isCurrent ? state.error : null;
  const title = attachment.description || attachment.name || "A2UI";
  const preview =
    previewHtml.key === attachmentKey ? previewHtml : undefined;
  const hasPreview =
    (kind === "a2ui" && messages && messages.length > 0) ||
    (kind === "json-render" && tree?.root);
  const previewPending = Boolean(
    hasPreview && !preview?.html && !preview?.error,
  );

  return (
    <Card className="border border-border/50 bg-background/80">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-semibold">{title}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {error ? (
          <div className="text-sm text-destructive">{error}</div>
        ) : messages === null ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoadingDots />
            <span>Loading UI...</span>
          </div>
        ) : kind === "unknown" ? (
          <div className="text-sm text-muted-foreground">
            No compatible UI content.
          </div>
        ) : hasPreview ? (
          <Tabs
            value={activeTab}
            onValueChange={(value) =>
              setActiveTab(value as "preview" | "interactive")
            }
          >
            <TabsList className="w-full justify-start">
              <TabsTrigger value="preview">Server Preview</TabsTrigger>
              <TabsTrigger value="interactive">Interactive</TabsTrigger>
            </TabsList>
            <TabsContent value="preview">
              {previewPending ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <LoadingDots />
                  <span>Rendering preview...</span>
                </div>
              ) : preview?.error ? (
                <div className="text-sm text-destructive">{preview.error}</div>
              ) : preview?.html ? (
                <div className="overflow-hidden rounded-xl border border-border/60 bg-white">
                  <iframe
                    title={`${title} preview`}
                    sandbox=""
                    srcDoc={preview.html}
                    className="h-[360px] w-full border-0"
                  />
                </div>
              ) : (
                <div className="text-sm text-muted-foreground">
                  Preview unavailable.
                </div>
              )}
            </TabsContent>
            <TabsContent value="interactive">
              {kind === "a2ui" && messages ? (
                <A2UIRenderer messages={messages} />
              ) : kind === "json-render" && tree ? (
                <JsonRenderRenderer tree={tree} />
              ) : null}
            </TabsContent>
          </Tabs>
        ) : (
          <>
            {kind === "a2ui" && messages ? (
              <A2UIRenderer messages={messages} />
            ) : kind === "json-render" && tree ? (
              <JsonRenderRenderer tree={tree} />
            ) : (
              <div className="text-sm text-muted-foreground">
                No compatible UI content.
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
