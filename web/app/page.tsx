import Link from 'next/link';

const highlights = [
  {
    title: 'Sketch-first UX',
    description:
      'Everything in Alex Code starts as a hand-drawn idea. The interface keeps that playful energy with bold outlines and tactile hover states.',
  },
  {
    title: 'Conversational coding',
    description:
      'Move between natural language, structured prompts, and generated code in one continuous flow without breaking your train of thought.',
  },
  {
    title: 'Console-grade reliability',
    description:
      'Under the playful lines is a serious developer console with audited history, collaborative sessions, and reproducible builds.',
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
    <main className="min-h-screen bg-[hsl(var(--background))] text-[hsl(var(--foreground))]">
      <div className="mx-auto w-full flex max-w-none flex-col gap-16 px-6 py-12 lg:py-20">
        <header className="flex flex-col gap-10 lg:flex-row lg:items-center">
          <div className="flex-1 space-y-6">
            <p className="inline-flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.25em] text-gray-600">
              <span className="block h-[6px] w-[6px] rounded-full bg-gray-600" /> Alex Code Lab
            </p>
            <h1 className="text-4xl font-semibold leading-tight tracking-tight sm:text-5xl lg:text-6xl">
              A sketchbook-style operating system for thoughtful builders.
            </h1>
            <p className="max-w-2xl text-lg text-gray-600">
              This is the GitHub Pages home for Alex Code. Instead of dropping you directly into the
              conversation view, we wanted to pause for a beat—share the story, the craft, and the
              deliberate simplicity behind the console.
            </p>
            <div className="flex flex-wrap gap-4">
              <Link
                href="/login"
                className="inline-flex items-center justify-center rounded-xl border-2 border-[hsl(var(--foreground))] bg-[hsl(var(--foreground))] px-6 py-3 text-sm font-semibold uppercase tracking-wider text-[hsl(var(--background))] shadow-[8px_8px_0_rgba(0,0,0,0.75)] transition-transform hover:-translate-y-0.5"
              >
                Enter the lab
              </Link>
              <Link
                href="/conversation"
                className="inline-flex items-center justify-center rounded-xl border-2 border-dashed border-[hsl(var(--foreground))] px-6 py-3 text-sm font-semibold uppercase tracking-wider text-[hsl(var(--foreground))] transition-transform hover:-translate-y-0.5"
              >
                Continue to console →
              </Link>
            </div>
          </div>
          <div className="relative mx-auto w-full max-w-md overflow-hidden rounded-[32px] bg-[hsl(var(--card))] p-8 shadow-[16px_16px_0_rgba(0,0,0,0.8)]">
            <div className="absolute inset-4 rounded-3xl border-[3px] border-dashed border-[hsl(var(--foreground))]" aria-hidden="true" />
            <div className="relative flex flex-col gap-6">
              <div className="grid grid-cols-4 gap-3">
                {[...Array(8)].map((_, index) => (
                  <div
                    key={index}
                    className="aspect-square rounded-2xl border-[3px] border-[hsl(var(--foreground))] bg-gradient-to-br from-white via-gray-100 to-gray-200"
                  />
                ))}
              </div>
              <div className="space-y-3">
                <div className="h-3 rounded-full border-[3px] border-[hsl(var(--foreground))] bg-white" />
                <div className="h-3 w-4/5 rounded-full border-[3px] border-[hsl(var(--foreground))] bg-white" />
                <div className="flex gap-3">
                  <div className="h-20 flex-1 rounded-3xl border-[3px] border-[hsl(var(--foreground))]" />
                  <div className="h-20 flex-1 rounded-3xl border-[3px] border-[hsl(var(--foreground))] border-dashed" />
                </div>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-[0.2em] text-gray-500">simple lines</span>
                <span className="text-xs font-semibold uppercase tracking-[0.2em] text-gray-500">serious tools</span>
              </div>
            </div>
          </div>
        </header>

        <section className="grid gap-6 lg:grid-cols-3">
          {highlights.map((item) => (
            <article
              key={item.title}
              className="flex h-full flex-col gap-4 rounded-3xl border-[3px] border-[hsl(var(--foreground))] bg-[hsl(var(--card))] p-6 shadow-[10px_10px_0_rgba(0,0,0,0.7)]"
            >
              <h2 className="text-2xl font-semibold">{item.title}</h2>
              <p className="text-base text-gray-600">{item.description}</p>
            </article>
          ))}
        </section>

        <section className="rounded-[36px] border-[3px] border-[hsl(var(--foreground))] bg-[hsl(var(--card))] p-8 shadow-[14px_14px_0_rgba(0,0,0,0.75)]">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-center">
            <div className="flex-1 space-y-4">
              <p className="text-xs font-semibold uppercase tracking-[0.25em] text-gray-500">Workflow</p>
              <h2 className="text-3xl font-semibold">From notebook scribbles to production-grade runs</h2>
              <p className="text-gray-600">
                Alex Code pairs a playful surface with an opinionated developer stack. Each step keeps the
                hand-drawn aesthetic while surfacing the power tools you need when it is time to ship.
              </p>
            </div>
            <div className="flex-1 space-y-6">
              {timeline.map((item) => (
                <div key={item.label} className="flex gap-4">
                  <div className="flex h-12 w-12 items-center justify-center rounded-2xl border-[3px] border-[hsl(var(--foreground))] font-mono text-lg font-semibold">
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

        <footer className="rounded-3xl border-[3px] border-dashed border-[hsl(var(--foreground))] p-8 text-center">
          <p className="text-sm uppercase tracking-[0.3em] text-gray-500">Alex Code · Monochrome Playground · 2025</p>
        </footer>
      </div>
    </main>
  );
}
