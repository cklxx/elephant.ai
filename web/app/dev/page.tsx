"use client";

import Link from "next/link";
import { RequireAuth } from "@/components/auth/RequireAuth";
import { Badge } from "@/components/ui/badge";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

const DEV_PAGES = [
  {
    href: "/dev/config",
    title: "Runtime config",
    description: "Review and override runtime configuration values.",
    tag: "Config",
  },
  {
    href: "/dev/apps-config",
    title: "Apps config",
    description: "Manage custom app connector definitions.",
    tag: "Config",
  },
  {
    href: "/dev/context-config",
    title: "Context config",
    description: "Inspect and tweak persona + goal context config.",
    tag: "Context",
  },
  {
    href: "/dev/context-window",
    title: "Context window",
    description: "Preview the full assembled context window for a session.",
    tag: "Context",
  },
  {
    href: "/dev/conversation-debug",
    title: "Conversation debugger",
    description: "Inspect raw SSE payloads and session snapshots.",
    tag: "SSE",
  },
  {
    href: "/dev/log-analyzer",
    title: "Log analyzer",
    description: "Browse recent log IDs and inspect correlated service/LLM/request logs.",
    tag: "Logs",
  },
  {
    href: "/dev/mock-console",
    title: "Mock console",
    description: "Replay a mocked agent console stream.",
    tag: "Mock",
  },
  {
    href: "/dev/mock-terminal-output",
    title: "Mock terminal output",
    description: "Preview terminal output streaming UI.",
    tag: "Mock",
  },
  {
    href: "/dev/plan-preview",
    title: "Plan preview",
    description: "Preview the timeline plan UI component.",
    tag: "Preview",
  },
] as const;

export default function DevHomePage() {
  return (
    <RequireAuth>
      <div className="min-h-screen bg-slate-50 px-4 py-8 lg:px-8">
        <div className="mx-auto flex max-w-6xl flex-col gap-6">
          <header className="rounded-2xl bg-white/90 p-6 ring-1 ring-slate-200/60">
            <p className="text-[11px] font-semibold text-slate-400">Dev Tools</p>
            <h1 className="mt-2 text-xl font-semibold text-slate-900 lg:text-2xl">
              Development routes
            </h1>
            <p className="mt-2 text-sm text-slate-600">
              Jump to individual dev pages for debugging, previews, and diagnostics.
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
    </RequireAuth>
  );
}
