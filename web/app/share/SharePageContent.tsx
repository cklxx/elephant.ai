"use client";

import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Loader2 } from "lucide-react";

import { apiClient } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import type { AnyAgentEvent } from "@/lib/types";
import { Header, ContentArea } from "@/components/layout";
import { ConversationEventStream } from "@/components/agent/ConversationEventStream";
import {
  AttachmentPanel,
  collectAttachmentItems,
} from "@/components/agent/AttachmentPanel";
import { cn } from "@/lib/utils";
import { normalizeAgentEvents } from "@/lib/events/normalize";

export function SharePageContent() {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("session_id") ?? "";
  const token = searchParams.get("token") ?? "";

  const [events, setEvents] = useState<AnyAgentEvent[]>([]);
  const [title, setTitle] = useState<string | null>(null);
  const missingError = useMemo(() => {
    if (!sessionId) {
      return t("share.page.error.missingSession");
    }
    if (!token) {
      return t("share.page.error.missingToken");
    }
    return null;
  }, [sessionId, t, token]);

  const [loading, setLoading] = useState(() => !missingError);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (missingError) {
      return;
    }

    let cancelled = false;
    Promise.resolve().then(() => {
      if (cancelled) return;
      setLoading(true);
      setError(null);
    });

    apiClient
      .getSharedSession(sessionId, token)
      .then((response) => {
        if (cancelled) return;
        const normalized = normalizeAgentEvents(response.events ?? []);
        setEvents(normalized);
        setTitle(response.title?.trim() || null);
      })
      .catch((err) => {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : t("share.page.error.generic");
        setError(message);
      })
      .finally(() => {
        if (cancelled) return;
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [missingError, sessionId, t, token]);

  const hasAttachments = useMemo(
    () => collectAttachmentItems(events).length > 0,
    [events],
  );

  const effectiveError = missingError ?? error;
  const showLoading = !missingError && loading;
  const headerTitle = title || t("share.page.title");
  const headerSubtitle = t("share.page.subtitle");

  return (
    <div className="relative h-[100dvh] overflow-hidden bg-muted/10 text-foreground">
      <div className="relative mx-auto flex h-full min-h-0 w-full flex-col gap-6 overflow-hidden px-4 pb-10 pt-6 lg:px-8 2xl:px-12">
        <Header
          title={headerTitle}
          subtitle={headerSubtitle}
          showEnvironmentStrip={false}
        />

        <div className="flex flex-1 min-h-0 flex-col gap-5 overflow-hidden lg:flex-row">
          <div className="flex flex-1 min-h-0 min-w-0 flex-col overflow-hidden rounded-3xl">
            <ContentArea
              className="flex-1 min-h-0 min-w-0"
              fullWidth
              contentClassName="space-y-4"
            >
              {showLoading ? (
                <div className="flex min-h-[60vh] items-center justify-center">
                  <div className="flex flex-col items-center gap-3 rounded-3xl border border-border/60 bg-background/70 px-8 py-6 text-center">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                    <p className="text-sm text-muted-foreground">
                      {t("share.page.loading")}
                    </p>
                  </div>
                </div>
              ) : effectiveError ? (
                <div className="flex min-h-[60vh] items-center justify-center">
                  <div className="flex flex-col items-center gap-3 rounded-3xl border border-border/60 bg-background/70 px-8 py-6 text-center">
                    <p className="text-sm font-semibold text-foreground">
                      {t("share.page.error.title")}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {effectiveError}
                    </p>
                  </div>
                </div>
              ) : (
                <ConversationEventStream
                  events={events}
                  isConnected
                  isReconnecting={false}
                  error={null}
                  reconnectAttempts={0}
                  onReconnect={() => {}}
                  isRunning={false}
                />
              )}
            </ContentArea>
          </div>

          <div
            className={cn(
              "hidden lg:flex flex-none justify-end overflow-hidden transition-all duration-300",
              hasAttachments ? "w-[380px] xl:w-[440px]" : "w-0",
            )}
            aria-hidden={!hasAttachments}
          >
            {hasAttachments ? (
              <div className="sticky top-24 w-full max-w-[440px] space-y-4">
                <AttachmentPanel events={events} />
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}
