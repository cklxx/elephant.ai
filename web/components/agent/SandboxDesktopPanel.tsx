"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { ExternalLink, Eye, EyeOff, RefreshCw } from "lucide-react";
import { getSandboxBrowserInfo } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { SandboxBrowserInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { buildApiUrl } from "@/lib/api-base";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";

interface SandboxDesktopPanelProps {
  sessionId?: string | null;
  isRunning?: boolean;
}

function ensureAutoconnect(url: string): string {
  if (!url) return url;
  if (url.includes("autoconnect=")) {
    return url;
  }
  const separator = url.includes("?") ? "&" : "?";
  return `${url}${separator}autoconnect=true`;
}

export function SandboxDesktopPanel({
  sessionId,
  isRunning = false,
}: SandboxDesktopPanelProps) {
  const { t } = useI18n();
  const [isOpen, setIsOpen] = useState(false);
  const [status, setStatus] = useState<"idle" | "loading" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<SandboxBrowserInfo | null>(null);
  const [snapshotTick, setSnapshotTick] = useState(0);

  const canLoad = Boolean(sessionId);

  const snapshotUrl = useMemo(() => {
    if (!sessionId || !isOpen) {
      return "";
    }
    const query = new URLSearchParams({ session_id: sessionId, ts: String(snapshotTick) });
    return buildApiUrl(`/api/sandbox/browser-screenshot?${query.toString()}`);
  }, [isOpen, sessionId, snapshotTick]);

  const vncUrl = useMemo(() => {
    if (!info?.vnc_url) return "";
    return ensureAutoconnect(info.vnc_url);
  }, [info]);

  const loadInfo = useCallback(async () => {
    if (!sessionId) {
      return;
    }
    setStatus("loading");
    setError(null);
    try {
      const data = await getSandboxBrowserInfo(sessionId);
      setInfo(data);
      setStatus("idle");
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setError(message);
      setStatus("error");
    }
  }, [sessionId]);

  useEffect(() => {
    if (isOpen && sessionId) {
      void loadInfo();
    }
  }, [isOpen, loadInfo, sessionId]);

  const handleRefresh = useCallback(() => {
    if (!sessionId) {
      return;
    }
    setSnapshotTick(Date.now());
    void loadInfo();
  }, [loadInfo, sessionId]);

  return (
    <div className="mx-auto w-full max-w-4xl rounded-2xl border border-border/70 bg-card/60 px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
            {t("console.sandbox.title")}
            {isRunning && (
              <span className="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-700">
                {t("console.sandbox.running")}
              </span>
            )}
          </div>
          <p className="text-xs text-muted-foreground">
            {t("console.sandbox.subtitle")}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setIsOpen((prev) => !prev)}
            disabled={!canLoad}
          >
            {isOpen ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            {isOpen ? t("console.sandbox.hide") : t("console.sandbox.show")}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={handleRefresh}
            disabled={!canLoad || status === "loading"}
          >
            <RefreshCw className="h-4 w-4" />
            {t("console.sandbox.refresh")}
          </Button>
          {vncUrl && (
            <Button type="button" variant="ghost" size="sm" asChild>
              <a href={vncUrl} target="_blank" rel="noreferrer">
                <ExternalLink className="h-4 w-4" />
                {t("console.sandbox.external")}
              </a>
            </Button>
          )}
        </div>
      </div>

      {!canLoad && (
        <div className="mt-3 rounded-xl border border-dashed border-border/60 bg-background/60 px-4 py-3 text-xs text-muted-foreground">
          {t("console.sandbox.noSession")}
        </div>
      )}
      <Dialog open={isOpen} onOpenChange={setIsOpen}>
        <DialogContent
          unstyled
          showCloseButton={false}
          className="h-screen w-screen max-w-none max-h-none bg-background p-0"
        >
          <div className="flex h-full flex-col">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/60 px-6 py-4">
              <div className="flex flex-col gap-1">
                <DialogTitle className="text-base font-semibold">
                  {t("console.sandbox.title")}
                </DialogTitle>
                <DialogDescription className="text-xs text-muted-foreground">
                  {t("console.sandbox.subtitle")}
                </DialogDescription>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={handleRefresh}
                  disabled={!canLoad || status === "loading"}
                >
                  <RefreshCw className="h-4 w-4" />
                  {t("console.sandbox.refresh")}
                </Button>
                {vncUrl && (
                  <Button type="button" variant="ghost" size="sm" asChild>
                    <a href={vncUrl} target="_blank" rel="noreferrer">
                      <ExternalLink className="h-4 w-4" />
                      {t("console.sandbox.external")}
                    </a>
                  </Button>
                )}
                <DialogClose asChild>
                  <Button type="button" variant="outline" size="sm">
                    <EyeOff className="h-4 w-4" />
                    {t("console.sandbox.hide")}
                  </Button>
                </DialogClose>
              </div>
            </div>

            <div className="flex-1 overflow-auto px-6 py-4">
              <div className="space-y-3">
                {status === "loading" && (
                  <div className="rounded-xl border border-dashed border-border/60 bg-background/60 px-4 py-3 text-xs text-muted-foreground">
                    {t("console.sandbox.loading")}
                  </div>
                )}

                {status === "error" && (
                  <div className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700">
                    {t("console.sandbox.error")} {error ? `(${error})` : ""}
                  </div>
                )}

                {vncUrl ? (
                  <div className="overflow-hidden rounded-2xl border border-border/60 bg-black">
                    <iframe
                      title="Sandbox desktop"
                      src={vncUrl}
                      className="h-[65vh] w-full"
                    />
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed border-border/60 bg-background/60 px-4 py-3 text-xs text-muted-foreground">
                    {t("console.sandbox.noVnc")}
                  </div>
                )}

                {snapshotUrl && (
                  <div className="overflow-hidden rounded-2xl border border-border/60 bg-background">
                    <img
                      src={snapshotUrl}
                      alt={t("console.sandbox.snapshotAlt")}
                      className="h-[280px] w-full object-cover"
                    />
                  </div>
                )}
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
