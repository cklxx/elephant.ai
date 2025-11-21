import Link from 'next/link';

const highlights = [
  {
    title: 'Evidence weaving',
    description:
      'Spinner turns scattered facts, logs, and scratch notes into an actionable knowledge web with traceable connections.',
  },
  {
    title: 'Layered Go backbone',
    description:
      'It runs the same layered Go backend that powers ALEX, pairing a calm surface with durable infrastructure.',
  },
  {
    title: 'Operator-first framing',
    description:
      'Designed for analysts, engineers, and operators who need trustworthy provenance while moving quickly.',
  },
];

const timeline = [
  {
    label: '01',
    title: 'Observe',
    detail: 'Collect requirements, sketches, and constraints directly in the console.',
  },
  {
    label: '02',
    title: 'Compose',
    detail: 'Iterate on prompts, snippets, and responses in a single thread.',
  },
  {
    label: '03',
    title: 'Ship',
    detail: 'Jump into the login experience when you are ready to build for real.',
  },
];

const metrics = [
  { label: 'Surface kits', value: '6', detail: 'Hero shells, rails, cards, grids, pills, inputs' },
  { label: 'Interaction passes', value: '4', detail: 'Hover, focus, keyboard, disabled' },
  { label: 'Latent glow', value: 'Borderless', detail: 'Edges stay legible without heavy strokes' },
];

const componentDeck = [
  {
    title: 'Hero shells',
    detail: 'Layered gradients and gridlines keep the hero visible without forcing borders.',
  },
  {
    title: 'Rails & pills',
    detail: 'Inline rails and soft pills segment actions while staying airy.',
  },
  {
    title: 'Cards & stacks',
    detail: 'Card stacks rely on tint + blur to convey separation in a compact grid.',
  },
];

export default function HomePage() {
  return (
    <main className="relative min-h-screen bg-transparent text-[hsl(var(--foreground))]">
      <div
        className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_12%_6%,rgba(255,255,255,0.08),transparent_36%),radial-gradient(circle_at_88%_2%,rgba(0,0,0,0.05),transparent_38%)]"
        aria-hidden
      />
      <div className="pointer-events-none absolute inset-0 -z-10 console-gridlines" aria-hidden />

      <div className="mx-auto flex w-full max-w-6xl flex-col gap-14 px-6 py-12 lg:gap-20 lg:py-20">
        <header className="console-surface-strong relative flex flex-col gap-10 overflow-hidden p-8 lg:flex-row lg:items-center lg:gap-12">
          <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[hsla(var(--foreground)/0.08)] to-transparent" aria-hidden />
          <div className="relative flex-1 space-y-6">
            <div className="flex flex-wrap items-center gap-3">
              <p className="console-kicker">Spinner Loom Lab</p>
              <span className="console-pill">Borderless surfaces</span>
              <span className="console-pill console-pill-quiet">Visible delineation</span>
            </div>

            <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-5xl lg:text-6xl">
              A calm console that keeps edges visible while everything stays borderless. <span className="console-gradient-text">Context stays intact.</span>
            </h1>
            <p className="max-w-2xl text-lg text-gray-600">
              Spinner turns scattered facts, logs, and scratch notes into an actionable knowledge web. The same layered Go
              backbone that powers ALEX keeps provenance strong while the UI leans on tint, blur, and glow instead of strokes.
            </p>

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {metrics.map((item) => (
                <div key={item.label} className="console-card flex flex-col gap-1 p-4">
                  <span className="text-xs uppercase tracking-[0.22em] text-muted-foreground">{item.label}</span>
                  <span className="text-2xl font-semibold">{item.value}</span>
                  <span className="text-sm text-gray-600">{item.detail}</span>
                </div>
              ))}
            </div>

            <div className="flex flex-wrap gap-3">
              <Link
                href="/login"
                className="console-primary-action inline-flex items-center justify-center rounded-2xl px-6 py-3 text-sm uppercase tracking-[0.14em]"
              >
                Enter the lab
              </Link>
              <Link
                href="/conversation"
                className="console-button console-button-ghost inline-flex items-center justify-center rounded-2xl px-6 py-3 text-sm uppercase tracking-[0.14em] !bg-white/5"
              >
                Continue to console →
              </Link>
            </div>
          </div>

          <div className="relative mx-auto w-full max-w-md space-y-4">
            <div className="console-card relative overflow-hidden p-6">
              <div className="absolute inset-0 bg-[radial-gradient(circle_at_30%_-10%,rgba(255,255,255,0.18),transparent_45%),radial-gradient(circle_at_80%_10%,rgba(255,255,255,0.08),transparent_40%)]" aria-hidden />
              <div className="relative flex flex-col gap-5">
                <div className="console-rail">
                  <span className="font-semibold uppercase tracking-[0.18em]">Component Pulse</span>
                  <span className="console-pill console-pill-quiet">Live preview</span>
                </div>
                <div className="grid grid-cols-4 gap-3">
                  {[...Array(8)].map((_, index) => (
                    <div
                      key={index}
                      className="aspect-square rounded-2xl bg-gradient-to-br from-white/60 via-white/35 to-white/10 shadow-[0_20px_45px_-30px_rgba(15,23,42,0.45)]"
                    />
                  ))}
                </div>
                <div className="grid grid-cols-2 gap-3 text-sm text-gray-600">
                  <div className="console-stack">
                    <span className="text-xs uppercase tracking-[0.22em] text-muted-foreground">Hero shell</span>
                    <span className="text-base font-semibold">Tint + glow</span>
                    <span className="text-xs text-gray-500">Readable without borders</span>
                  </div>
                  <div className="console-stack">
                    <span className="text-xs uppercase tracking-[0.22em] text-muted-foreground">Input rail</span>
                    <span className="text-base font-semibold">Soft capsule</span>
                    <span className="text-xs text-gray-500">Inset focus, subtle lift</span>
                  </div>
                </div>
              </div>
            </div>
            <div className="grid grid-cols-3 gap-3 text-center text-xs font-semibold uppercase tracking-[0.22em] text-muted-foreground">
              <span className="console-pill">Stacked gradients</span>
              <span className="console-pill console-pill-quiet">Gridline anchors</span>
              <span className="console-pill">Soft shadows</span>
            </div>
          </div>
        </header>

        <section className="grid gap-6 lg:grid-cols-3">
          {highlights.map((item) => (
            <article key={item.title} className="console-card flex h-full flex-col gap-4 p-6">
              <div className="flex items-center justify-between">
                <h2 className="text-2xl font-semibold">{item.title}</h2>
                <span className="console-pill console-pill-quiet">calm</span>
              </div>
              <p className="text-base text-gray-600">{item.description}</p>
            </article>
          ))}
        </section>

        <section className="console-surface space-y-6 p-8">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-3">
              <p className="console-kicker">Component system</p>
              <h2 className="text-3xl font-semibold">Borderless components that still read as distinct</h2>
              <p className="max-w-2xl text-gray-600">
                Each surface uses tint, gradient, and blur instead of strokes. Pills, rails, and stacks add hierarchy so the UI stays
                clear while keeping the no-border promise.
              </p>
            </div>
            <div className="console-rail max-w-md">
              <span className="text-muted-foreground">Design direction</span>
              <span className="font-semibold uppercase tracking-[0.18em]">Glow-first · Grid anchored</span>
            </div>
          </div>

          <div className="grid gap-4 lg:grid-cols-3">
            {componentDeck.map((item) => (
              <div key={item.title} className="console-stack">
                <div className="flex items-center justify-between">
                  <h3 className="text-xl font-semibold">{item.title}</h3>
                  <span className="console-pill console-pill-quiet">Preview</span>
                </div>
                <p className="text-sm text-gray-600">{item.detail}</p>
                <div className="console-card-interactive grid gap-2 p-3">
                  <div className="h-2 rounded-full bg-[hsla(var(--foreground)/0.18)]" />
                  <div className="h-2 w-4/5 rounded-full bg-[hsla(var(--foreground)/0.12)]" />
                  <div className="grid grid-cols-3 gap-2">
                    <div className="h-14 rounded-2xl bg-[hsla(var(--foreground)/0.08)]" />
                    <div className="h-14 rounded-2xl bg-[hsla(var(--foreground)/0.06)]" />
                    <div className="h-14 rounded-2xl bg-[hsla(var(--foreground)/0.04)]" />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="console-surface p-8">
          <div className="flex flex-col gap-8 lg:flex-row lg:items-start">
            <div className="flex-1 space-y-4">
              <p className="console-kicker">Workflow</p>
              <h2 className="text-3xl font-semibold">From loose notes to production-grade runs</h2>
              <p className="text-gray-600">
                Spinner pairs a calm, low-saturation surface with an opinionated Go-based stack. Each step keeps context traceable
                while surfacing the power tools you need when it is time to ship.
              </p>
            </div>
            <div className="flex-1 space-y-4">
              {timeline.map((item) => (
                <div key={item.label} className="console-card-interactive flex gap-4 p-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/30 font-mono text-lg font-semibold text-[hsl(var(--foreground))]">
                    {item.label}
                  </div>
                  <div className="space-y-1">
                    <h3 className="text-xl font-semibold">{item.title}</h3>
                    <p className="text-gray-600">{item.detail}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        <footer className="console-card p-6 text-center">
          <p className="text-sm uppercase tracking-[0.3em] text-muted-foreground">Spinner · Context Loom · 2025</p>
        </footer>
      </div>
    </main>
  );
}
