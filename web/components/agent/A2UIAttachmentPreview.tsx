"use client";

import { useEffect, useMemo, useState } from "react";

import { AttachmentPayload } from "@/lib/types";
import { A2UIMessage, loadA2UIAttachmentMessages } from "@/lib/a2ui";
import { A2UIRenderer } from "@/components/agent/A2UIRenderer";
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
    messages: A2UIMessage[] | null;
    error: string | null;
  }>(() => ({
    key: attachmentKey,
    messages: null,
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

    loadA2UIAttachmentMessages(attachmentSnapshot, controller.signal)
      .then((loaded) => {
        if (cancelled) return;
        setState({ key: attachmentKey, messages: loaded, error: null });
      })
      .catch((err) => {
        if (cancelled) return;
        setState({
          key: attachmentKey,
          messages: [],
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
    const messages = isCurrent ? state.messages : null;
    if (!messages || messages.length === 0) {
      return;
    }
    let cancelled = false;
    fetch("/api/a2ui/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messages }),
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
  }, [attachmentKey, state.key, state.messages]);

  const isCurrent = state.key === attachmentKey;
  const messages = isCurrent ? state.messages : null;
  const error = isCurrent ? state.error : null;
  const title = attachment.description || attachment.name || "A2UI";
  const preview =
    previewHtml.key === attachmentKey ? previewHtml : undefined;
  const hasPreview = Boolean(messages && messages.length > 0);
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
        ) : messages.length === 0 ? (
          <div className="text-sm text-muted-foreground">No A2UI content.</div>
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
              <A2UIRenderer messages={messages} />
            </TabsContent>
          </Tabs>
        ) : (
          <A2UIRenderer messages={messages} />
        )}
      </CardContent>
    </Card>
  );
}
