export default function ConversationLoading() {
  return (
    <div className="min-h-[calc(100vh-6rem)] bg-app-canvas px-4 py-10 lg:px-8">
      <div className="mx-auto flex max-w-7xl flex-col gap-8">
        <div className="space-y-3">
          <p className="text-[11px] font-semibold text-muted-foreground">
            Loading console…
          </p>
          <h1 className="text-2xl font-semibold text-foreground">
            Preparing session
          </h1>
          <div className="h-4 w-80 rounded-full bg-muted/50" />
        </div>
        <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
          <div className="space-y-4">
            <div className="rounded-2xl border border-border/60 bg-background/70 p-6">
              <div className="mb-4 h-4 w-32 rounded-full bg-muted/40" />
              <div className="h-5 w-40 rounded-full bg-muted/60" />
            </div>
            {[1, 2, 3].map((key) => (
              <div
                key={key}
                className="rounded-2xl border border-border/60 bg-background/70 p-4"
              >
                <div className="mb-3 h-4 w-48 rounded-full bg-muted/40" />
                <div className="space-y-2">
                  <div className="h-3 w-full rounded-full bg-muted/40" />
                  <div className="h-3 w-5/6 rounded-full bg-muted/40" />
                </div>
              </div>
            ))}
          </div>
          <div className="space-y-4 rounded-2xl border border-dashed border-border/60 bg-background/80 p-6">
            <div className="space-y-2">
              <div className="h-5 w-2/3 rounded-full bg-muted/60" />
              <div className="h-4 w-full rounded-full bg-muted/40" />
            </div>
            {[1, 2, 3, 4].map((key) => (
              <div
                key={key}
                className="space-y-2 rounded-xl border border-border/40 bg-muted/10 p-4"
              >
                <div className="h-4 w-40 rounded-full bg-muted/50" />
                <div className="h-3 w-full rounded-full bg-muted/30" />
                <div className="h-3 w-5/6 rounded-full bg-muted/30" />
              </div>
            ))}
            <div className="flex flex-col gap-3 rounded-xl border border-border/40 bg-muted/10 p-4">
              <div className="h-4 w-32 rounded-full bg-muted/60" />
              <div className="h-10 rounded-lg bg-muted/40" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
