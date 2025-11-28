"use client";

import Link from "next/link";

import { RequireAuth } from "@/components/auth/RequireAuth";
import { SessionList } from "@/components/session/SessionList";
import { PageContainer, PageShell, SectionBlock, SectionHeader } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/lib/i18n";

export default function SessionsPage() {
  const { t } = useI18n();
  return (
    <RequireAuth>
      <PageShell>
        <PageContainer>
          <SectionBlock>
            <SectionHeader
              overline={t("sessions.archiveLabel")}
              title={t("sessions.title")}
              description={t("sessions.description")}
              titleElement="h1"
              actions={
                <Link href="/" className="w-full sm:w-auto">
                  <Button className="w-full sm:w-auto">{t("sessions.newConversation")}</Button>
                </Link>
              }
            />
          </SectionBlock>

          <Card>
            <CardHeader className="p-4">
              <CardTitle>{t("sessions.title")}</CardTitle>
            </CardHeader>
            <CardContent className="p-4">
              <SessionList />
            </CardContent>
          </Card>
        </PageContainer>
      </PageShell>
    </RequireAuth>
  );
}
