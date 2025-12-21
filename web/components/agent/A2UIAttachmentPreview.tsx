"use client";

import { useEffect, useMemo, useState } from "react";

import { AttachmentPayload } from "@/lib/types";
import { A2UIMessage, loadA2UIAttachmentMessages } from "@/lib/a2ui";
import { A2UIRenderer } from "@/components/agent/A2UIRenderer";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { LoadingDots } from "@/components/ui/loading-states";

export function A2UIAttachmentPreview({
  attachment,
}: {
  attachment: AttachmentPayload;
}) {
  const [messages, setMessages] = useState<A2UIMessage[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const attachmentKey = useMemo(
    () => `${attachment.name}:${attachment.data ?? ""}:${attachment.uri ?? ""}`,
    [attachment.name, attachment.data, attachment.uri],
  );

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    setMessages(null);
    setError(null);

    loadA2UIAttachmentMessages(attachment, controller.signal)
      .then((loaded) => {
        if (cancelled) return;
        setMessages(loaded);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : String(err));
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [attachment, attachmentKey]);

  const title = attachment.description || attachment.name || "A2UI";

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
        ) : (
          <A2UIRenderer messages={messages} />
        )}
      </CardContent>
    </Card>
  );
}
