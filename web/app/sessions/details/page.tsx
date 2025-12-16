"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { RequireAuth } from "@/components/auth/RequireAuth";
import { PageContainer, PageShell } from "@/components/layout/page-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/lib/i18n";

import { SessionDetailsClient } from "../[id]/SessionDetailsClient";

function SessionDetailsContent() {
  const searchParams = useSearchParams();
  const sessionId = searchParams.get("id");
  const { t } = useI18n();

  if (sessionId) {
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

function SessionDetailsFallback() {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Loading...</CardTitle>
      </CardHeader>
    </Card>
  );
}

export default function SessionDetailsQueryPage() {
  return (
    <RequireAuth>
      <PageShell>
        <PageContainer>
          <Suspense fallback={<SessionDetailsFallback />}>
            <SessionDetailsContent />
          </Suspense>
        </PageContainer>
      </PageShell>
    </RequireAuth>
  );
}

