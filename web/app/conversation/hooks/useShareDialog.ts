import { useCallback, useState } from "react";

import { apiClient } from "@/lib/api";
import { toast } from "@/components/ui/toast";
import { useI18n } from "@/lib/i18n";
import { formatParsedError, parseError } from "@/lib/errors";

interface UseShareDialogOptions {
  resolvedSessionId: string | null;
}

export function useShareDialog({ resolvedSessionId }: UseShareDialogOptions) {
  const { t } = useI18n();
  const [shareDialogOpen, setShareDialogOpen] = useState(false);
  const [shareLink, setShareLink] = useState<string | null>(null);
  const [shareInProgress, setShareInProgress] = useState(false);

  const handleShareRequest = useCallback(async () => {
    if (!resolvedSessionId || shareInProgress) return;
    setShareInProgress(true);
    try {
      const { share_token } = await apiClient.createSessionShare(
        resolvedSessionId,
      );
      const url = new URL("/share", window.location.origin);
      url.searchParams.set("session_id", resolvedSessionId);
      url.searchParams.set("token", share_token);
      setShareLink(url.toString());
      setShareDialogOpen(true);
    } catch (err) {
      const parsed = parseError(err, t("common.error.unknown"));
      toast.error(
        t("share.toast.createError.title"),
        t("share.toast.createError.description", {
          message: formatParsedError(parsed),
        }),
      );
    } finally {
      setShareInProgress(false);
    }
  }, [resolvedSessionId, shareInProgress, t]);

  const handleCopyShareLink = useCallback(async () => {
    if (!shareLink) return;
    try {
      await navigator.clipboard.writeText(shareLink);
      toast.success(
        t("share.toast.copied.title"),
        t("share.toast.copied.description"),
      );
    } catch (err) {
      const parsed = parseError(err, t("common.error.unknown"));
      toast.error(
        t("share.toast.copyError.title"),
        t("share.toast.copyError.description", {
          message: formatParsedError(parsed),
        }),
      );
    }
  }, [shareLink, t]);

  return {
    shareDialogOpen,
    setShareDialogOpen,
    shareLink,
    shareInProgress,
    handleShareRequest,
    handleCopyShareLink,
  };
}
