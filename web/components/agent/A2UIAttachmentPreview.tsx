"use client";

import { useEffect, useMemo, useState } from "react";

import { AttachmentPayload } from "@/lib/types";
import { loadAttachmentText } from "@/lib/attachment-text";
import { JsonRenderTree } from "@/lib/json-render-model";
import { parseUIPayload } from "@/lib/ui-payload";
import { JsonRenderRenderer } from "@/components/agent/JsonRenderRenderer";
import { Card, CardContent } from "@/components/ui/card";
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
    payload: string | null;
    tree: JsonRenderTree | null;
    error: string | null;
  }>(() => ({
    key: attachmentKey,
    payload: null,
    tree: null,
    error: null,
  }));

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    loadAttachmentText(attachmentSnapshot, controller.signal)
      .then((payload) => {
        if (cancelled) return;
        const parsed = parseUIPayload(payload);
        if (parsed.kind === "json-render") {
          setState({
            key: attachmentKey,
            payload,
            tree: parsed.tree,
            error: null,
          });
          return;
        }
        setState({
          key: attachmentKey,
          payload,
          tree: null,
          error: parsed.error,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setState({
          key: attachmentKey,
          payload: null,
          tree: null,
          error: err instanceof Error ? err.message : String(err),
        });
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [attachmentKey, attachmentSnapshot]);

  const isCurrent = state.key === attachmentKey;
  const tree = isCurrent ? state.tree : null;
  const payload = isCurrent ? state.payload : null;
  const error = isCurrent ? state.error : null;

  return (
    <Card className="border border-border/50 bg-background/80">
      <CardContent className="space-y-3">
        {error ? (
          <div className="text-sm text-destructive">{error}</div>
        ) : payload === null ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <LoadingDots />
            <span>Loading UI...</span>
          </div>
        ) : !tree?.root ? (
          <div className="text-sm text-muted-foreground">
            No compatible UI content (json-render only).
          </div>
        ) : (
          <>
            {tree ? (
              <div className="min-h-[220px] max-h-[640px] overflow-auto pt-2">
                <JsonRenderRenderer tree={tree} />
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">
                No compatible UI content (json-render only).
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
