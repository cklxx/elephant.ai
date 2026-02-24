"use client";

import Link from "next/link";
import dynamic from "next/dynamic";
import { Suspense } from "react";


import { PageContainer, PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useRequiredSearchParam } from "@/hooks/useRequiredSearchParam";
import { useI18n } from "@/lib/i18n";

function SessionDetailsFallback() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Loading...</CardTitle>
      </CardHeader>
    </Card>
  );
}

const SessionDetailsClient = dynamic(
  () =>
    import("../[id]/SessionDetailsClient").then((mod) => mod.SessionDetailsClient),
  {
    loading: SessionDetailsFallback,
    ssr: false,
  },
);

function SessionDetailsContent() {
  const { value: sessionId, missing: missingSessionId } = useRequiredSearchParam("id");
  const { t } = useI18n();

  if (!missingSessionId) {
    return <SessionDetailsClient sessionId={sessionId} />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("sessions.details.notFound")}</CardTitle>
      </CardHeader>
      <CardContent>
        <Button asChild variant="outline">
          <Link href="/sessions">{t("sessions.details.back")}</Link>
        </Button>
      </CardContent>
    </Card>
  );
}

export default function SessionDetailsQueryPage() {
  return (
      <PageShell>
        <PageContainer>
          <Suspense fallback={<SessionDetailsFallback />}>
            <SessionDetailsContent />
          </Suspense>
        </PageContainer>
      </PageShell>
  );
}
