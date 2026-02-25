"use client";

import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

const DEV_PAGES = [
  {
    href: "/dev/diagnostics",
    title: "Diagnostics workbench",
    description: "Inspect SSE payloads, snapshots, memory, and structured log chains.",
    tag: "Diagnostics",
  },
  {
    href: "/dev/configuration",
    title: "Configuration workbench",
    description: "Manage runtime overrides, app connectors, and context configuration tools.",
    tag: "Configuration",
  },
] as const;

export default function DevHomePage() {
  return (
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-6xl flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <p className="text-[11px] font-semibold text-slate-400">Dev Tools</p>
            <h1 className="mt-2 text-xl font-semibold text-slate-900 lg:text-2xl">
              Development routes
            </h1>
            <p className="mt-2 text-sm text-slate-600">
              Consolidated workbenches for diagnostics and configuration.
            </p>
          </header>

          <div className="grid gap-4 md:grid-cols-2">
            {DEV_PAGES.map((page) => (
              <Link key={page.href} href={page.href} className="group">
                <Card className="h-full transition hover:-translate-y-0.5 hover:shadow-md">
                  <CardHeader className="space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <CardTitle className="text-base">{page.title}</CardTitle>
                      <Badge variant="outline">{page.tag}</Badge>
                    </div>
                    <CardDescription>{page.description}</CardDescription>
                  </CardHeader>
                </Card>
              </Link>
            ))}
          </div>
        </div>
      </div>
  );
}
