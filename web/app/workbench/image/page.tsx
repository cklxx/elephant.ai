'use client';

import { useCallback, useMemo, useState } from 'react';
import { AlertCircle, Check, Clipboard, Image as ImageIcon, Loader2, Sparkles } from 'lucide-react';
import clsx from 'clsx';
import { generateImageConcepts, type GenerateImageConceptsPayload } from '@/lib/api';
import type { ImageConcept } from '@/lib/types';

const SANDBOX_URL =
  process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL ||
  (process.env.NODE_ENV === 'test' ? 'about:blank' : 'https://sandbox.alexapp.dev');

function normalizeReferences(value: string): string[] {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

export default function ImageWorkbenchPage(): JSX.Element {
  const [brief, setBrief] = useState('');
  const [style, setStyle] = useState('');
  const [referencesInput, setReferencesInput] = useState('');
  const [concepts, setConcepts] = useState<ImageConcept[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [copiedPrompt, setCopiedPrompt] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [taskId, setTaskId] = useState<string | undefined>();

  const references = useMemo(() => normalizeReferences(referencesInput), [referencesInput]);

  const submitDisabled = loading || brief.trim().length === 0;

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (submitDisabled) {
        return;
      }

      setLoading(true);
      setError(null);
      setSuccessMessage(null);
      setCopiedPrompt(null);

      const payload: GenerateImageConceptsPayload = {
        brief: brief.trim(),
      };
      if (style.trim().length > 0) {
        payload.style = style.trim();
      }
      if (references.length > 0) {
        payload.references = references;
      }

      try {
        const response = await generateImageConcepts(payload);
        setConcepts(response.concepts ?? []);
        setSessionId(response.session_id);
        setTaskId(response.task_id);
        if ((response.concepts ?? []).length > 0) {
          setSuccessMessage('已生成新的视觉方向');
        } else {
          setSuccessMessage('已请求 Agent，等待进一步指引');
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : '生成失败';
        setError(message);
      } finally {
        setLoading(false);
      }
    },
    [brief, style, references, submitDisabled]
  );

  const handleCopyPrompt = useCallback(async (prompt: string) => {
    try {
      await navigator.clipboard.writeText(prompt);
      setCopiedPrompt(prompt);
      setTimeout(() => {
        setCopiedPrompt((prev) => (prev === prompt ? null : prev));
      }, 2000);
    } catch (err) {
      console.warn('Failed to copy prompt', err);
      setError('无法复制提示词，请手动复制。');
    }
  }, []);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-10 px-6 py-12 lg:flex-row">
        <section className="w-full lg:w-2/5">
          <div className="rounded-3xl border border-cyan-500/30 bg-slate-900/70 p-8 shadow-2xl backdrop-blur">
            <div className="flex items-center gap-3 text-cyan-200">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-400/40 bg-cyan-500/15">
                <ImageIcon className="h-6 w-6" aria-hidden />
              </div>
              <div>
                <p className="text-sm font-medium text-cyan-300">Alex Workbench</p>
                <h1 className="text-2xl font-semibold">图片创作工作台</h1>
              </div>
            </div>

            <p className="mt-6 text-sm leading-relaxed text-slate-300">
              输入创作简报，Alex 将自动归纳风格重点并生成 Seedream 可用的提示词，帮助你快速探索视觉方向。
            </p>

            <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">创作简报</span>
                <textarea
                  value={brief}
                  onChange={(event) => setBrief(event.target.value)}
                  placeholder="例如：未来城市主题海报，突出霓虹灯与雨夜街景"
                  className="min-h-[120px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                  required
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">偏好风格（可选）</span>
                <input
                  value={style}
                  onChange={(event) => setStyle(event.target.value)}
                  placeholder="例如：赛博朋克、电影级构图、霓虹光效"
                  className="w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">参考素材或链接（每行一个，可选）</span>
                <textarea
                  value={referencesInput}
                  onChange={(event) => setReferencesInput(event.target.value)}
                  placeholder={'https://example.com/board/1\nPinterest: neon city moodboard'}
                  className="min-h-[96px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                />
              </label>

              <button
                type="submit"
                disabled={submitDisabled}
                className={clsx(
                  'flex w-full items-center justify-center gap-2 rounded-2xl px-4 py-3 text-sm font-medium transition',
                  submitDisabled
                    ? 'cursor-not-allowed border border-slate-700 bg-slate-800/70 text-slate-500'
                    : 'border border-cyan-400/60 bg-cyan-500/20 text-cyan-100 hover:border-cyan-300 hover:bg-cyan-500/30'
                )}
              >
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                    生成中…
                  </>
                ) : (
                  <>
                    <Sparkles className="h-4 w-4" aria-hidden />
                    生成视觉方向
                  </>
                )}
              </button>
            </form>

            {(successMessage || error) && (
              <div
                className={clsx(
                  'mt-6 flex items-start gap-3 rounded-2xl border px-4 py-3 text-sm',
                  error
                    ? 'border-rose-500/50 bg-rose-500/10 text-rose-100'
                    : 'border-emerald-500/40 bg-emerald-500/10 text-emerald-100'
                )}
              >
                {error ? <AlertCircle className="mt-0.5 h-4 w-4" aria-hidden /> : <Check className="mt-0.5 h-4 w-4" aria-hidden />}
                <p>{error ?? successMessage}</p>
              </div>
            )}

            {(sessionId || taskId) && (
              <div className="mt-6 rounded-2xl border border-slate-800 bg-slate-900/60 px-4 py-3 text-xs text-slate-400">
                <p className="font-medium text-slate-300">Agent 上下文</p>
                {sessionId && <p className="mt-1 break-all">Session: {sessionId}</p>}
                {taskId && <p className="mt-1 break-all">Task: {taskId}</p>}
              </div>
            )}
          </div>
        </section>

        <section className="w-full space-y-6 lg:w-3/5">
          <div className="rounded-3xl border border-slate-800 bg-slate-900/50 p-8">
            <header className="flex items-center justify-between gap-4">
              <div>
                <h2 className="text-xl font-semibold text-slate-50">生成的视觉方向</h2>
                <p className="mt-1 text-xs text-slate-400">
                  提示词已针对 Seedream 进行了语气与关键元素优化，可直接复制粘贴使用。
                </p>
              </div>
              <span className="rounded-full border border-cyan-500/30 bg-cyan-500/10 px-3 py-1 text-xs text-cyan-200">
                {concepts.length} 个方向
              </span>
            </header>

            <div className="mt-6 space-y-5">
              {concepts.length === 0 && !loading && (
                <div className="rounded-2xl border border-dashed border-slate-700/80 bg-slate-900/40 p-8 text-center text-sm text-slate-400">
                  <p>生成的提示词会显示在这里，可多次迭代优化。</p>
                </div>
              )}

              {concepts.map((concept) => (
                <article
                  key={`${concept.title}-${concept.prompt.slice(0, 16)}`}
                  className="space-y-3 rounded-2xl border border-slate-800/80 bg-slate-900/70 p-6 shadow-lg"
                >
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                      <h3 className="text-lg font-semibold text-slate-100">
                        {concept.title || '未命名方向'}
                      </h3>
                      {concept.mood && <p className="text-xs text-slate-400">氛围：{concept.mood}</p>}
                    </div>
                    <button
                      type="button"
                      onClick={() => handleCopyPrompt(concept.prompt)}
                      className="flex items-center gap-2 rounded-xl border border-cyan-400/40 bg-cyan-500/10 px-3 py-1.5 text-xs text-cyan-100 transition hover:border-cyan-300 hover:bg-cyan-500/20"
                    >
                      {copiedPrompt === concept.prompt ? (
                        <>
                          <Check className="h-3.5 w-3.5" aria-hidden />
                          已复制
                        </>
                      ) : (
                        <>
                          <Clipboard className="h-3.5 w-3.5" aria-hidden />
                          复制提示词
                        </>
                      )}
                    </button>
                  </div>

                  <div className="rounded-xl border border-slate-800 bg-slate-950/70 px-4 py-3 text-sm text-slate-200">
                    <p className="whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-cyan-100">
                      {concept.prompt}
                    </p>
                  </div>

                  <dl className="grid gap-3 text-xs text-slate-300 sm:grid-cols-2">
                    {concept.aspect_ratio && (
                      <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 px-3 py-2">
                        <dt className="text-[10px] font-medium text-slate-500">画幅</dt>
                        <dd className="mt-1 text-slate-200">{concept.aspect_ratio}</dd>
                      </div>
                    )}
                    {concept.seed_hint && (
                      <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 px-3 py-2">
                        <dt className="text-[10px] font-medium text-slate-500">Seed 建议</dt>
                        <dd className="mt-1 text-slate-200">{concept.seed_hint}</dd>
                      </div>
                    )}
                    {concept.style_notes && concept.style_notes.length > 0 && (
                      <div className="rounded-xl border border-slate-800/80 bg-slate-950/60 px-3 py-2 sm:col-span-2">
                        <dt className="text-[10px] font-medium text-slate-500">补充要点</dt>
                        <dd className="mt-1 space-y-1 text-slate-200">
                          {concept.style_notes.map((note) => (
                            <p key={note} className="leading-relaxed">
                              • {note}
                            </p>
                          ))}
                        </dd>
                      </div>
                    )}
                  </dl>
                </article>
              ))}

              {loading && (
                <div className="flex items-center justify-center gap-2 rounded-2xl border border-slate-800/80 bg-slate-900/60 p-6 text-sm text-slate-300">
                  <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                  Agent 正在推演视觉方案…
                </div>
              )}
            </div>
          </div>

          <div className="rounded-3xl border border-slate-800 bg-slate-900/40 p-4">
            <h2 className="text-sm font-semibold text-slate-200">Agent 沙箱实时视图</h2>
            <p className="mt-1 text-xs text-slate-500">
              右下角小窗展示了 Agent 在可视化沙箱中的操作，便于追踪查阅与工具调用。
            </p>
            <div className="mt-4 overflow-hidden rounded-2xl border border-slate-800/70 bg-black">
              <iframe
                title="Agent Sandbox Viewer"
                src={SANDBOX_URL}
                className="h-72 w-full"
                sandbox="allow-scripts allow-same-origin"
              />
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
