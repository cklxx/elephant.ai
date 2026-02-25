import { QuickPromptButtons, type QuickPromptItem } from "./QuickPromptButtons";

interface EmptyStateViewProps {
  badge: string;
  title: string;
  quickstartTitle: string;
  hotkeyHint: string;
  items: QuickPromptItem[];
  onSelect: (prompt: string) => void;
}

export function EmptyStateView({
  badge,
  title,
  quickstartTitle,
  hotkeyHint,
  items,
  onSelect,
}: EmptyStateViewProps) {
  return (
    <div className="w-full max-w-md" data-testid="conversation-empty-state">
      <div className="rounded-3xl p-6 text-center">
        <div className="mx-auto inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/60 px-3 py-1 text-[11px] font-semibold text-muted-foreground">
          <span className="h-2 w-2 animate-pulse rounded-full bg-emerald-400/70" />
          {badge}
        </div>

        <div className="mt-4 space-y-2">
          <p
            className="text-lg font-semibold tracking-tight text-foreground"
            data-testid="conversation-empty-title"
          >
            {title}
          </p>
        </div>

        <div className="mt-6">
          <p className="text-[11px] font-semibold tracking-wide text-muted-foreground">
            {quickstartTitle}
          </p>
          <QuickPromptButtons items={items} onSelect={onSelect} />

          <p className="mt-4 text-xs text-muted-foreground">{hotkeyHint}</p>
        </div>
      </div>
    </div>
  );
}
