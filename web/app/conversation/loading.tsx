export default function ConversationLoading() {
  return (
    <div className="min-h-[calc(100vh-6rem)] bg-app-canvas px-4 py-10 lg:px-8">
      <div className="mx-auto flex max-w-7xl flex-col gap-8">
        <div className="space-y-3">
          <div className="h-4 w-24 rounded-full bg-slate-200" />
          <div className="h-8 w-64 rounded-full bg-slate-200" />
          <div className="h-4 w-80 rounded-full bg-slate-100" />
        </div>
        <div className="grid gap-6 lg:grid-cols-[280px_minmax(0,1fr)]">
          <div className="space-y-4">
            <div className="rounded-[28px] border border-slate-200 bg-white/70 p-6 shadow-soft">
              <div className="mb-4 h-4 w-32 rounded-full bg-slate-100" />
              <div className="h-5 w-40 rounded-full bg-slate-200" />
            </div>
            {[1, 2, 3].map((key) => (
              <div key={key} className="rounded-3xl border border-slate-200 bg-white/70 p-4">
                <div className="mb-3 h-4 w-48 rounded-full bg-slate-100" />
                <div className="space-y-2">
                  <div className="h-3 w-full rounded-full bg-slate-100" />
                  <div className="h-3 w-5/6 rounded-full bg-slate-100" />
                </div>
              </div>
            ))}
          </div>
          <div className="space-y-4 rounded-[32px] border border-dashed border-slate-200 bg-white/80 p-6">
            <div className="space-y-2">
              <div className="h-5 w-2/3 rounded-full bg-slate-200" />
              <div className="h-4 w-full rounded-full bg-slate-100" />
            </div>
            {[1, 2, 3, 4].map((key) => (
              <div key={key} className="space-y-2 rounded-2xl border border-slate-100 p-4">
                <div className="h-4 w-40 rounded-full bg-slate-100" />
                <div className="h-3 w-full rounded-full bg-slate-50" />
                <div className="h-3 w-5/6 rounded-full bg-slate-50" />
              </div>
            ))}
            <div className="flex flex-col gap-3 rounded-2xl border border-slate-100 p-4">
              <div className="h-4 w-32 rounded-full bg-slate-200" />
              <div className="h-10 rounded-xl bg-slate-100" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
