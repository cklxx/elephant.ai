'use client';

import Link from 'next/link';
import { Suspense } from 'react';
import { ArrowRight, MessageSquare, Sparkles } from 'lucide-react';
import { TranslationKey, useI18n } from '@/lib/i18n';

const highlightIcons = {
  chat: MessageSquare,
  glow: Sparkles,
};

function HomeContent() {
  const { t } = useI18n();
  const highlights: Array<{ title: TranslationKey; description: TranslationKey; icon: keyof typeof highlightIcons }> = [
    {
      title: 'home.feature.chat.title',
      description: 'home.feature.chat.description',
      icon: 'chat',
    },
    {
      title: 'home.feature.actions.title',
      description: 'home.feature.actions.description',
      icon: 'glow',
    },
  ];
  const summaryCards: Array<{
    label: TranslationKey;
    value: TranslationKey;
    description: TranslationKey;
  }> = [
    {
      label: 'home.summary.sessions.label',
      value: 'home.summary.sessions.value',
      description: 'home.summary.sessions.description',
    },
    {
      label: 'home.summary.timeline.label',
      value: 'home.summary.timeline.value',
      description: 'home.summary.timeline.description',
    },
    {
      label: 'home.summary.languages.label',
      value: 'home.summary.languages.value',
      description: 'home.summary.languages.description',
    },
  ];

  return (
    <div className="bg-app-canvas">
      <div className="console-shell py-16 sm:py-20">
        <header className="flex flex-wrap items-center justify-between gap-4">
          <span className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/80 px-4 py-1.5 text-xs font-semibold uppercase tracking-[0.35em] text-slate-500 shadow-sm">
            {t('console.brand')}
          </span>
        </header>

        <section className="mt-16 grid gap-12 lg:grid-cols-[minmax(0,1fr)_320px] lg:items-start">
          <div className="max-w-2xl space-y-6">
            <p className="text-xs font-semibold uppercase tracking-[0.35em] text-slate-400">
              {t('home.hero.label')}
            </p>
            <h1 className="text-4xl font-semibold text-slate-900 sm:text-5xl">
              {t('home.hero.title')}
            </h1>
            <p className="text-base text-slate-500 sm:text-lg">
              {t('home.hero.subtitle')}
            </p>

            <div className="flex flex-wrap gap-3 pt-4">
              <Link
                href="/conversation"
                className="inline-flex items-center gap-2 rounded-full bg-sky-500 px-5 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
              >
                <span>{t('home.hero.ctaPrimary')}</span>
                <ArrowRight className="h-4 w-4" />
              </Link>
              <Link
                href="/sessions"
                className="inline-flex items-center gap-2 rounded-full border border-slate-200 px-5 py-2 text-sm font-semibold text-slate-600 transition hover:border-sky-200 hover:text-sky-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
              >
                {t('home.hero.ctaSecondary')}
              </Link>
            </div>
          </div>

          <div className="space-y-4">
            {highlights.map(({ title, description, icon }) => {
              const Icon = highlightIcons[icon];
              return (
                <div
                  key={title}
                  className="flex items-start gap-3 rounded-3xl border border-slate-200/70 bg-white/80 p-5 shadow-[0_24px_80px_rgba(15,23,42,0.08)]"
                >
                  <span className="mt-1 inline-flex h-9 w-9 items-center justify-center rounded-2xl bg-sky-500/10 text-sky-500">
                    <Icon className="h-4 w-4" />
                  </span>
                  <div className="space-y-1">
                    <p className="text-sm font-semibold text-slate-800">{t(title)}</p>
                    <p className="text-sm text-slate-500">{t(description)}</p>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        <section className="mt-16 grid gap-6 rounded-[28px] border border-white/40 bg-white/70 p-6 shadow-[0_24px_120px_rgba(15,23,42,0.08)] sm:grid-cols-3">
          {summaryCards.map(({ label, value, description }) => (
            <div
              key={label}
              className="flex flex-col gap-2 rounded-2xl border border-slate-200/60 bg-white/90 p-4 text-sm text-slate-500"
            >
              <span className="text-xs font-semibold uppercase tracking-[0.35em] text-slate-300">
                {t(label)}
              </span>
              <span className="text-lg font-semibold text-slate-800">
                {t(value)}
              </span>
              <p className="console-microcopy text-slate-400">{t(description)}</p>
            </div>
          ))}
        </section>
      </div>
    </div>
  );
}

export default function HomePage() {
  const { t } = useI18n();
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
          {t('app.loading')}
        </div>
      }
    >
      <HomeContent />
    </Suspense>
  );
}
