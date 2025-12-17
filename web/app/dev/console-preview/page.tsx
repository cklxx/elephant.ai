'use client';

import dynamic from 'next/dynamic';
import { useSearchParams } from 'next/navigation';
import { useState } from 'react';
import { Button } from '@/components/ui/button';

const ConsolePreviewContent = dynamic(() => import('./ConsolePreviewContent'), {
  ssr: false,
  loading: () => (
    <div className="min-h-screen bg-slate-100 px-6 py-10">
      <div className="mx-auto flex max-w-2xl flex-col gap-6">
        <div className="rounded-3xl bg-white/80 p-6 ring-1 ring-white/70">
          <p className="text-sm text-slate-600">Loading preview…</p>
        </div>
      </div>
    </div>
  ),
});

export default function ConsolePreviewPage() {
  const searchParams = useSearchParams();
  const autoPreview = searchParams.get('auto') === '1';
  const [manualPreview, setManualPreview] = useState(false);
  const showPreview = manualPreview || autoPreview;

  if (showPreview) {
    return <ConsolePreviewContent />;
  }

  return (
    <div className="min-h-screen bg-slate-100 px-6 py-10">
      <div className="mx-auto flex max-w-2xl flex-col gap-6">
        <header className="space-y-2">
          <p className="text-[10px] font-semibold text-slate-400">Dev Preview</p>
          <h1 className="text-2xl font-semibold text-slate-900">Console preview</h1>
          <p className="text-sm text-slate-600">
            The full preview UI is loaded on demand to keep the initial bundle small (and Lighthouse-friendly).
          </p>
        </header>

        <div className="rounded-3xl bg-white/80 p-6 ring-1 ring-white/70">
          <div className="flex flex-wrap items-center gap-3">
            <Button type="button" onClick={() => setManualPreview(true)}>
              Load preview
            </Button>
            <p className="text-xs text-slate-500">Tip: append ?auto=1 to auto-load.</p>
          </div>
        </div>
      </div>
    </div>
  );
}
