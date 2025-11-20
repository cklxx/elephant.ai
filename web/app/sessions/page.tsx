"use client";

import { SessionList } from "@/components/session/SessionList";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PlusCircle } from "lucide-react";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { RequireAuth } from "@/components/auth/RequireAuth";

export default function SessionsPage() {
  const { t } = useI18n();
  return (
    <RequireAuth>
      <div className="console-shell">
        <div className="space-y-6">
          <section className="console-panel p-8">
            <div className="flex flex-col gap-6">
              <header className="flex flex-col gap-6">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <p className="console-pane-title">
                      {t("sessions.archiveLabel")}
                    </p>
                    <h1 className="text-3xl font-semibold text-foreground">
                      {t("sessions.title")}
                    </h1>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t("sessions.description")}
                    </p>
                  </div>
                </div>
                <Link href="/" className="inline-flex w-full sm:w-auto">
                  <Button className="w-full rounded-2xl bg-foreground/80 px-4 py-2 text-sm font-semibold text-primary-foreground shadow-none transition hover:bg-foreground sm:w-auto">
                    <PlusCircle className="mr-2 h-4 w-4" />
                    {t("sessions.newConversation")}
                  </Button>
                </Link>
              </header>

              <Card className="bg-white/5 p-0 shadow-none">
                <div className="rounded-2xl bg-white/5 p-6 backdrop-blur">
                  <SessionList />
                </div>
              </Card>
            </div>
          </section>
        </div>
      </div>
    </RequireAuth>
  );
}
