import { PageShell } from "@/components/layout/page-shell";

export default function DebugPage() {
  return (
    <PageShell
      title="Debug Tools"
      description="Conversation debug, config inspection, and context window viewer."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <DebugCard href="/debug/conversation-debug" label="Conversation Debug" description="SSE inspector, session snapshots, timing breakdown" />
        <DebugCard href="/debug/config" label="Runtime Config" description="View and edit runtime configuration" />
        <DebugCard href="/debug/context-config" label="Context Config" description="Context window configuration" />
        <DebugCard href="/debug/context-window" label="Context Window" description="Preview context window assembly" />
        <DebugCard href="/debug/apps-config" label="Apps Config" description="Multi-app configuration" />
      </div>
    </PageShell>
  );
}

function DebugCard({ href, label, description }: { href: string; label: string; description: string }) {
  return (
    <a
      href={href}
      className="block rounded-lg border border-border bg-card p-4 transition-colors hover:bg-accent"
    >
      <p className="text-sm font-medium text-foreground">{label}</p>
      <p className="mt-1 text-xs text-muted-foreground">{description}</p>
    </a>
  );
}
