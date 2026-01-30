"use client";

import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

interface DeleteConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  targetId: string | null;
  targetLabel: string | null;
  targetBadge: string | null;
  deleteInProgress: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

export default function DeleteConfirmDialog({
  open,
  onOpenChange,
  targetId,
  targetLabel,
  targetBadge,
  deleteInProgress,
  onCancel,
  onConfirm,
}: DeleteConfirmDialogProps) {
  const { t } = useI18n();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md rounded-3xl">
        <DialogHeader className="space-y-3">
          <DialogTitle className="text-lg font-semibold">
            {t("sidebar.session.confirmDelete.title")}
          </DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground">
            {t("sidebar.session.confirmDelete.description")}
          </DialogDescription>
          {targetId && (
            <div className="flex items-center justify-between rounded-2xl border border-border/70 bg-muted/30 px-3 py-2">
              <div className="flex flex-col">
                <span className="text-sm font-semibold text-foreground">
                  {targetLabel}
                </span>
                <span className="text-xs text-muted-foreground">
                  {targetBadge}
                </span>
              </div>
            </div>
          )}
        </DialogHeader>
        <DialogFooter className="sm:justify-end">
          <Button
            variant="outline"
            onClick={onCancel}
            disabled={deleteInProgress}
          >
            {t("sidebar.session.confirmDelete.cancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={deleteInProgress}
          >
            {deleteInProgress ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : null}
            {t("sidebar.session.confirmDelete.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
