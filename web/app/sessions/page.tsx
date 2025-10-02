'use client';

import { SessionList } from '@/components/session/SessionList';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { PlusCircle } from 'lucide-react';
import Link from 'next/link';

export default function SessionsPage() {
  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
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

      {/* Session list */}
      <Card className="p-6">
        <SessionList />
      </Card>
    </div>
  );
}
