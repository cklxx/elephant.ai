'use client';

import { SessionList } from '@/components/session/SessionList';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { PlusCircle } from 'lucide-react';
import Link from 'next/link';
import { useI18n } from '@/lib/i18n';
import { LanguageSwitcher } from '@/components/LanguageSwitcher';

export default function SessionsPage() {
  const { t } = useI18n();
  return (
    <div className="console-shell">
      <div className="space-y-6">
        <section className="console-panel p-8">
          <div className="flex flex-col gap-6">
            <header className="flex flex-col gap-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
                <div>
                  <p className="console-pane-title">{t('sessions.archiveLabel')}</p>
                  <h1 className="text-3xl font-semibold text-slate-900">{t('sessions.title')}</h1>
                  <p className="mt-1 text-sm text-slate-500">{t('sessions.description')}</p>
                </div>
                <LanguageSwitcher variant="toolbar" />
              </div>
              <Link href="/" className="inline-flex w-full sm:w-auto">
                <Button className="w-full rounded-xl bg-sky-500 px-4 py-2 text-sm font-semibold text-white shadow-lg shadow-sky-500/20 hover:bg-sky-600 sm:w-auto">
                  <PlusCircle className="mr-2 h-4 w-4" />
                  {t('sessions.newConversation')}
                </Button>
              </Link>
            </header>

            <Card className="border-none bg-slate-50/40 p-0 shadow-none">
              <div className="rounded-2xl border border-slate-100 bg-white p-6">
                <SessionList />
              </div>
            </Card>
          </div>
        </section>
      </div>
    </div>
  );
}
