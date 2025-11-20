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

export default function HomePage() {
  return (
    <main className="min-h-screen bg-transparent text-[hsl(var(--foreground))]">
      <div className="mx-auto w-full flex max-w-none flex-col gap-16 px-6 py-12 lg:py-20">
        <header className="flex flex-col gap-10 rounded-[28px] bg-white/5 p-6 shadow-none backdrop-blur-sm lg:flex-row lg:items-center">
          <div className="flex-1 space-y-6">
            <p className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.25em] text-gray-600">
              <span className="block h-[6px] w-[6px] rounded-full bg-gray-600" /> Spinner Loom Lab
            </p>
            <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-5xl lg:text-6xl">
              A calm console for weaving fragmented context into clarity.
            </h1>
            <p className="max-w-2xl text-lg text-gray-600">
              Spinner is an AI agent that turns scattered facts, logs, and scratch notes into an actionable knowledge web.
              It runs the same layered Go backend that powers ALEX, but the framing is focused on weaving together
              fragmented context for analysts, engineers, and operators.
            </p>
            <div className="flex flex-wrap gap-4">
              <Link
                href="/login"
                className="inline-flex items-center justify-center rounded-xl bg-white/20 px-6 py-3 text-sm font-semibold uppercase tracking-wider text-[hsl(var(--foreground))] shadow-sm backdrop-blur hover:-translate-y-0.5 hover:bg-white/30"
              >
                Enter the lab
              </Link>
              <Link
                href="/conversation"
                className="inline-flex items-center justify-center rounded-xl bg-white/10 px-6 py-3 text-sm font-semibold uppercase tracking-wider text-[hsl(var(--foreground))] shadow-sm backdrop-blur hover:-translate-y-0.5 hover:bg-white/20"
              >
                Continue to console →
              </Link>
            </div>
          </div>
          <div className="relative mx-auto w-full max-w-md overflow-hidden rounded-[32px] bg-white/10 p-8 backdrop-blur">
            <div className="absolute inset-4 rounded-3xl bg-white/10" aria-hidden="true" />
            <div className="relative flex flex-col gap-6">
              <div className="grid grid-cols-4 gap-3">
                {[...Array(8)].map((_, index) => (
                  <div
                    key={index}
                    className="aspect-square rounded-2xl bg-gradient-to-br from-white/60 via-white/40 to-white/30 shadow-sm"
                  />
                ))}
              </div>
              <div className="space-y-3">
                <div className="h-3 rounded-full bg-white/70" />
                <div className="h-3 w-4/5 rounded-full bg-white/70" />
                <div className="flex gap-3">
                  <div className="h-20 flex-1 rounded-3xl bg-white/25" />
                  <div className="h-20 flex-1 rounded-3xl bg-white/15" />
                </div>
              </div>
              <div className="flex items-center justify-between text-gray-500">
                <span className="text-xs font-semibold uppercase tracking-[0.2em]">simple lines</span>
                <span className="text-xs font-semibold uppercase tracking-[0.2em]">serious tools</span>
              </div>
            </div>
          </div>
        </header>

        <section className="grid gap-6 lg:grid-cols-3">
          {highlights.map((item) => (
            <article key={item.title} className="flex h-full flex-col gap-4 rounded-3xl bg-white/5 p-6 shadow-sm backdrop-blur-sm">
              <h2 className="text-2xl font-semibold">{item.title}</h2>
              <p className="text-base text-gray-600">{item.description}</p>
            </article>
          ))}
        </section>

        <section className="rounded-[36px] bg-white/5 p-8 shadow-sm backdrop-blur">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-center">
            <div className="flex-1 space-y-4">
              <p className="text-xs font-semibold uppercase tracking-[0.25em] text-gray-500">Workflow</p>
              <h2 className="text-3xl font-semibold">From loose notes to production-grade runs</h2>
              <p className="text-gray-600">
                Spinner pairs a calm, low-saturation surface with an opinionated Go-based stack. Each step keeps context
                traceable while surfacing the power tools you need when it is time to ship.
              </p>
            </div>
            <div className="flex-1 space-y-6">
              {timeline.map((item) => (
                <div key={item.label} className="flex gap-4 rounded-2xl bg-white/10 p-4 shadow-sm backdrop-blur">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl bg-white/30 font-mono text-lg font-semibold">
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

        <footer className="rounded-3xl bg-white/10 p-8 text-center shadow-sm backdrop-blur">
          <p className="text-sm uppercase tracking-[0.3em] text-gray-500">Spinner · Context Loom · 2025</p>
        </footer>
      </div>
    </main>
  );
}
