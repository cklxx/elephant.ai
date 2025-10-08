'use client';

import { Suspense } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { PlusCircle } from 'lucide-react';

import { SessionList } from '@/components/session/SessionList';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

import { SessionDetailsClient } from './SessionDetailsClient';

function SessionsPageContent() {
  const searchParams = useSearchParams();
  const activeSessionId = searchParams.get('sessionId');

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Sessions</h1>
          <p className="text-gray-600 mt-2">
            View and manage your ALEX agent sessions
          </p>
        </div>
        <Link href="/">
          <Button>
            <PlusCircle className="h-4 w-4 mr-2" />
            New Session
          </Button>
        </Link>
      </div>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,360px)_1fr]">
        <Card className="p-6 h-fit">
          <SessionList />
        </Card>

        <div className="space-y-6">
          {activeSessionId ? (
            <SessionDetailsClient sessionId={activeSessionId} />
          ) : (
            <Card className="p-12 text-center text-gray-500">
              <p>Select a session from the list to view its details.</p>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

export default function SessionsPage() {
  return (
    <Suspense fallback={<div className="p-12 text-center text-gray-500">Loading sessions…</div>}>
      <SessionsPageContent />
    </Suspense>
  );
}
