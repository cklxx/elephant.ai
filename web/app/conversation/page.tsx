"use client";

import { Suspense } from "react";
import dynamic from "next/dynamic";
import { useI18n } from "@/lib/i18n";

const ConversationPageContent = dynamic(
  () =>
    import("./ConversationPageContent").then((mod) => mod.ConversationPageContent),
  {
    loading: () => (
      <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
        Loading…
      </div>
    ),
    ssr: false,
  },
);

export default function ConversationPage() {
  const { t } = useI18n();

  return (
      <Suspense
        fallback={
          <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
            {t("app.loading")}
          </div>
        }
      >
        <ConversationPageContent />
      </Suspense>
  );
}
