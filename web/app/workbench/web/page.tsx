'use client';

import { useCallback, useMemo, useState } from 'react';
import {
  AlertCircle,
  Check,
  Clipboard,
  LayoutDashboard,
  ListChecks,
  Loader2,
  Sparkles,
  Target,
} from 'lucide-react';
import clsx from 'clsx';
import { generateWebBlueprint, type GenerateWebBlueprintPayload } from '@/lib/api';
import type { WebBlueprintPlan } from '@/lib/types';

const SANDBOX_URL =
  process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL ||
  (process.env.NODE_ENV === 'test' ? 'about:blank' : 'https://sandbox.alexapp.dev');

function parseMustHaves(value: string): string[] {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

function SectionCard({
  section,
  onCopy,
}: {
  section: WebBlueprintPlan['sections'][number];
  onCopy: (content: string) => void;
}): JSX.Element {
  return (
    <article className="rounded-3xl border border-emerald-500/30 bg-slate-900/60 p-6 shadow-lg">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-lg font-semibold text-emerald-100">{section.title}</h3>
          <p className="mt-1 text-xs text-emerald-200/80">{section.purpose}</p>
        </div>
        <button
          type="button"
          onClick={() =>
            onCopy([
              section.title,
              section.purpose,
              ...(section.copy_suggestions ?? []),
            ]
              .filter(Boolean)
              .join('\n'))
          }
          className="inline-flex items-center gap-1 rounded-full border border-emerald-500/30 px-3 py-1 text-xs text-emerald-100 transition hover:border-emerald-400/70 hover:text-emerald-50"
        >
          <Clipboard className="h-3.5 w-3.5" aria-hidden />
          复制文案
        </button>
      </header>

      {section.components && section.components.length > 0 && (
        <div className="mt-4">
          <p className="text-xs font-medium text-slate-400">建议组件</p>
          <ul className="mt-2 space-y-1 text-sm text-slate-200">
            {section.components.map((component) => (
              <li key={component} className="flex items-center gap-2">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-400/70" aria-hidden />
                {component}
              </li>
            ))}
          </ul>
        </div>
      )}

      {section.copy_suggestions && section.copy_suggestions.length > 0 && (
        <div className="mt-4">
          <p className="text-xs font-medium text-slate-400">文案建议</p>
          <ul className="mt-2 space-y-1 text-sm text-slate-200">
            {section.copy_suggestions.map((suggestion) => (
              <li key={suggestion} className="rounded-xl border border-slate-800/60 bg-slate-900/60 px-3 py-2">
                {suggestion}
              </li>
            ))}
          </ul>
        </div>
      )}
    </article>
  );
}

export default function WebWorkbenchPage(): JSX.Element {
  const [goal, setGoal] = useState('');
  const [audience, setAudience] = useState('');
  const [tone, setTone] = useState('');
  const [mustHavesInput, setMustHavesInput] = useState('');
  const [blueprint, setBlueprint] = useState<WebBlueprintPlan | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [taskId, setTaskId] = useState<string | undefined>();

  const mustHaves = useMemo(() => parseMustHaves(mustHavesInput), [mustHavesInput]);
  const submitDisabled = loading || goal.trim().length === 0;

  const handleCopy = useCallback(async (content: string) => {
    try {
      await navigator.clipboard.writeText(content);
      setCopied(content);
      setTimeout(() => {
        setCopied((prev) => (prev === content ? null : prev));
      }, 2000);
    } catch (err) {
      console.warn('Failed to copy', err);
      setError('无法复制内容，请手动复制。');
    }
  }, []);

  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (submitDisabled) {
        return;
      }

      setLoading(true);
      setError(null);
      setSuccessMessage(null);
      setCopied(null);

      const payload: GenerateWebBlueprintPayload = {
        goal: goal.trim(),
      };
      if (audience.trim().length > 0) {
        payload.audience = audience.trim();
      }
      if (tone.trim().length > 0) {
        payload.tone = tone.trim();
      }
      if (mustHaves.length > 0) {
        payload.must_haves = mustHaves;
      }

      try {
        const response = await generateWebBlueprint(payload);
        setBlueprint(response.blueprint);
        setSessionId(response.session_id);
        setTaskId(response.task_id);
        if (response.blueprint) {
          setSuccessMessage('已生成新的页面蓝图');
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : '生成失败';
        setError(message);
      } finally {
        setLoading(false);
      }
    },
    [audience, goal, mustHaves, submitDisabled, tone]
  );

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-10 px-6 py-12 lg:flex-row">
        <section className="w-full lg:w-2/5">
          <div className="rounded-3xl border border-emerald-500/30 bg-slate-900/70 p-8 shadow-2xl backdrop-blur">
            <div className="flex items-center gap-3 text-emerald-200">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-emerald-400/40 bg-emerald-500/15">
                <LayoutDashboard className="h-6 w-6" aria-hidden />
              </div>
              <div>
                <p className="text-sm font-medium text-emerald-300">Alex Workbench</p>
                <h1 className="text-2xl font-semibold">网页构建工作台</h1>
              </div>
            </div>

            <p className="mt-6 text-sm leading-relaxed text-slate-300">
              描述你的网页目标与受众，Alex 将结合最佳实践给出结构化的落地页蓝图，包括模块划分、组件建议和可直接使用的文案要点。
            </p>

            <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">页面目标 *</span>
                <textarea
                  value={goal}
                  onChange={(event) => setGoal(event.target.value)}
                  placeholder="例如：发布全新产品的预热落地页，突出数据安全与客户案例"
                  className="min-h-[120px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-emerald-400 focus:outline-none focus:ring-2 focus:ring-emerald-500/40"
                  required
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">目标受众（可选）</span>
                <input
                  value={audience}
                  onChange={(event) => setAudience(event.target.value)}
                  placeholder="例如：成长型科技公司的市场负责人"
                  className="w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-emerald-400 focus:outline-none focus:ring-2 focus:ring-emerald-500/40"
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">品牌语气（可选）</span>
                <input
                  value={tone}
                  onChange={(event) => setTone(event.target.value)}
                  placeholder="例如：稳重、可信赖，同时保留创新感"
                  className="w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-emerald-400 focus:outline-none focus:ring-2 focus:ring-emerald-500/40"
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">必须包含的要素（每行一个，可选）</span>
                <textarea
                  value={mustHavesInput}
                  onChange={(event) => setMustHavesInput(event.target.value)}
                  placeholder={'客户证言\n报名表单\n产品功能概览'}
                  className="min-h-[96px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-emerald-400 focus:outline-none focus:ring-2 focus:ring-emerald-500/40"
                />
              </label>

              <button
                type="submit"
                disabled={submitDisabled}
                className={clsx(
                  'flex w-full items-center justify-center gap-2 rounded-2xl px-4 py-3 text-sm font-medium transition',
                  submitDisabled
                    ? 'cursor-not-allowed border border-slate-700 bg-slate-800/70 text-slate-500'
                    : 'border border-emerald-400/60 bg-emerald-500/20 text-emerald-100 hover:border-emerald-300 hover:bg-emerald-500/30'
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
                    生成页面蓝图
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

            {copied && !error && (
              <div className="mt-3 flex items-center gap-2 text-xs text-emerald-200">
                <Check className="h-3.5 w-3.5" aria-hidden />
                <span>已复制到剪贴板</span>
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
                <h2 className="text-xl font-semibold text-slate-50">页面蓝图</h2>
                <p className="mt-1 text-xs text-slate-400">结构、模块与文案建议将帮助设计与开发团队快速启动实现。</p>
              </div>
              {blueprint && (
                <button
                  type="button"
                  onClick={() => handleCopy(JSON.stringify(blueprint, null, 2))}
                  className="inline-flex items-center gap-2 rounded-full border border-slate-700/70 px-4 py-2 text-xs text-slate-200 transition hover:border-emerald-400/60 hover:text-emerald-100"
                >
                  <Clipboard className="h-3.5 w-3.5" aria-hidden />
                  复制 JSON
                </button>
              )}
            </header>

            {blueprint ? (
              <div className="mt-6 space-y-6">
                <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-6">
                  <h3 className="text-lg font-semibold text-slate-100">{blueprint.page_title}</h3>
                  <p className="mt-2 text-sm text-slate-300">{blueprint.summary}</p>
                </div>

                <div className="space-y-4">
                  <p className="text-sm font-medium text-emerald-300">页面模块</p>
                  <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
                    {blueprint.sections.map((section) => (
                      <SectionCard key={`${section.title}-${section.purpose}`} section={section} onCopy={handleCopy} />
                    ))}
                  </div>
                </div>

                {(blueprint.call_to_actions && blueprint.call_to_actions.length > 0) ||
                (blueprint.seo_keywords && blueprint.seo_keywords.length > 0) ? (
                  <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
                    {blueprint.call_to_actions && blueprint.call_to_actions.length > 0 && (
                      <div className="rounded-3xl border border-slate-800 bg-slate-900/60 p-6">
                        <div className="flex items-center gap-2 text-emerald-200">
                          <Target className="h-4 w-4" aria-hidden />
                          <p className="text-sm font-semibold">行动引导</p>
                        </div>
                        <ul className="mt-4 space-y-3 text-sm text-slate-200">
                          {blueprint.call_to_actions.map((cta) => (
                            <li key={`${cta.label}-${cta.destination}`} className="rounded-xl border border-slate-800/60 bg-slate-900/60 px-4 py-3">
                              <p className="font-medium text-slate-100">{cta.label}</p>
                              <p className="mt-1 text-xs text-slate-400">前往：{cta.destination}</p>
                              {cta.variant && <p className="mt-1 text-xs text-slate-500">样式：{cta.variant}</p>}
                              {cta.messaging && <p className="mt-1 text-xs text-slate-400">提示：{cta.messaging}</p>}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}

                    {blueprint.seo_keywords && blueprint.seo_keywords.length > 0 && (
                      <div className="rounded-3xl border border-slate-800 bg-slate-900/60 p-6">
                        <div className="flex items-center gap-2 text-emerald-200">
                          <ListChecks className="h-4 w-4" aria-hidden />
                          <p className="text-sm font-semibold">推荐关键词</p>
                        </div>
                        <div className="mt-4 flex flex-wrap gap-2 text-xs text-emerald-200">
                          {blueprint.seo_keywords.map((keyword) => (
                            <span key={keyword} className="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1">
                              {keyword}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="mt-10 rounded-3xl border border-dashed border-slate-700/70 bg-slate-900/40 p-10 text-center text-sm text-slate-400">
                <p>填写左侧信息并提交后，将在这里生成完整的页面结构建议。</p>
              </div>
            )}
          </div>

          <div className="relative overflow-hidden rounded-3xl border border-slate-800 bg-slate-900/60 p-6">
            <div className="flex items-center justify-between text-xs text-slate-400">
              <span>Agent Sandbox 预览</span>
              <span>只读模式</span>
            </div>
            <iframe
              title="Agent Sandbox Viewer"
              src={SANDBOX_URL}
              className="mt-4 h-[280px] w-full rounded-2xl border border-slate-800"
              sandbox="allow-scripts allow-same-origin"
            />
          </div>
        </section>
      </div>
    </div>
  );
}
