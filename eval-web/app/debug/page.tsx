"use client";

import { PageShell } from "@/components/layout/page-shell";

const debugTools = [
  {
    href: "/debug/conversation-debug",
    label: "Conversation Debug",
    description: "SSE inspector, session snapshots, timing breakdown",
  },
  {
    href: "/debug/config",
    label: "Runtime Config",
    description: "View and edit runtime configuration for LLM providers",
  },
  {
    href: "/debug/context-config",
    label: "Context Config",
    description: "Manage context window sections: personas, goals, policies",
  },
  {
    href: "/debug/context-window",
    label: "Context Window",
    description: "Preview context window composition and layered context",
  },
  {
    href: "/debug/apps-config",
    label: "Apps Config",
    description: "Multi-app plugin configuration and capabilities",
  },
];

export default function DebugPage() {
  return (
    <PageShell
      title="Debug Tools"
      description="Conversation debug, config inspection, and context window viewer."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {debugTools.map((tool) => (
          <a
            key={tool.href}
            href={tool.href}
            className="block rounded-lg border border-border bg-card p-4 transition-colors hover:bg-accent"
          >
            <p className="text-sm font-medium text-foreground">{tool.label}</p>
            <p className="mt-1 text-xs text-muted-foreground">{tool.description}</p>
          </a>
        ))}
      </div>
    </PageShell>
  );
}
