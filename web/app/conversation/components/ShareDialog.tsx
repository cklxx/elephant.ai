"use client";

import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

interface ShareDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  shareLink: string | null;
  onCopyShareLink: () => void;
}

export default function ShareDialog({
  open,
  onOpenChange,
  shareLink,
  onCopyShareLink,
}: ShareDialogProps) {
  const { t } = useI18n();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md rounded-3xl">
        <DialogHeader className="space-y-2">
          <DialogTitle className="text-lg font-semibold">
            {t("share.dialog.title")}
          </DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground">
            {t("share.dialog.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Input readOnly value={shareLink ?? ""} />
          <Button
            type="button"
            variant="secondary"
            onClick={onCopyShareLink}
            disabled={!shareLink}
            className="w-full"
          >
            <Copy className="h-4 w-4" />
            {t("share.dialog.copy")}
          </Button>
        </div>
        <DialogFooter className="sm:justify-end">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("share.dialog.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
