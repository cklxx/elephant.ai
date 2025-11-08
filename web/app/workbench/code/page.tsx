'use client';

import { useCallback, useMemo, useState } from 'react';
import {
  Braces,
  Check,
  Clipboard,
  Cpu,
  Layers,
  Loader2,
  Rocket,
  ServerCog,
} from 'lucide-react';
import clsx from 'clsx';
import { generateCodePlan, type GenerateCodePlanPayload } from '@/lib/api';
import type { CodeServicePlan } from '@/lib/types';

const SANDBOX_URL =
  process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL ||
  (process.env.NODE_ENV === 'test' ? 'about:blank' : 'https://sandbox.alexapp.dev');

const LANGUAGE_PRESETS: { label: string; value: string }[] = [
  { label: 'Go', value: 'go' },
  { label: 'Node.js', value: 'nodejs' },
  { label: 'TypeScript', value: 'typescript' },
  { label: 'Python', value: 'python' },
];

function parseMultiline(value: string): string[] {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);
}

function SectionHeading({ icon, title }: { icon: JSX.Element; title: string }): JSX.Element {
  return (
    <header className="mb-4 flex items-center gap-2">
      <span className="flex h-8 w-8 items-center justify-center rounded-xl border border-cyan-500/40 bg-cyan-500/10 text-cyan-200">
        {icon}
      </span>
      <h3 className="text-lg font-semibold text-cyan-100">{title}</h3>
    </header>
  );
}

export default function CodeWorkbenchPage(): JSX.Element {
  const [serviceName, setServiceName] = useState('');
  const [objective, setObjective] = useState('');
  const [language, setLanguage] = useState('');
  const [featureInput, setFeatureInput] = useState('');
  const [integrationInput, setIntegrationInput] = useState('');
  const [plan, setPlan] = useState<CodeServicePlan | null>(null);
  const [sessionId, setSessionId] = useState<string | undefined>();
  const [taskId, setTaskId] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const features = useMemo(() => parseMultiline(featureInput), [featureInput]);
  const integrations = useMemo(() => parseMultiline(integrationInput), [integrationInput]);
  const submitDisabled = loading || serviceName.trim().length === 0 || objective.trim().length === 0;

  const handleCopy = useCallback(async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(value);
      setTimeout(() => {
        setCopied((prev) => (prev === value ? null : prev));
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

      const payload: GenerateCodePlanPayload = {
        service_name: serviceName.trim(),
        objective: objective.trim(),
      };
      if (language.trim().length > 0) {
        payload.language = language.trim();
      }
      if (features.length > 0) {
        payload.features = features;
      }
      if (integrations.length > 0) {
        payload.integrations = integrations;
      }

      try {
        const response = await generateCodePlan(payload);
        setPlan(response.plan);
        setSessionId(response.session_id);
        setTaskId(response.task_id);
        setSuccessMessage('已生成微服务蓝图');
      } catch (err) {
        const message = err instanceof Error ? err.message : '生成失败，请稍后重试。';
        setError(message);
      } finally {
        setLoading(false);
      }
    },
    [features, integrations, language, objective, serviceName, submitDisabled]
  );

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-10 px-6 py-12 lg:flex-row">
        <section className="w-full lg:w-2/5">
          <div className="rounded-3xl border border-cyan-500/30 bg-slate-900/70 p-8 shadow-2xl backdrop-blur">
            <div className="flex items-center gap-3 text-cyan-200">
              <div className="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-400/40 bg-cyan-500/15">
                <Cpu className="h-6 w-6" aria-hidden />
              </div>
              <div>
                <p className="text-sm font-medium text-cyan-300">Alex Workbench</p>
                <h1 className="text-2xl font-semibold">代码微服务工作台</h1>
              </div>
            </div>

            <p className="mt-6 text-sm leading-relaxed text-slate-300">
              描述目标与关键特性，Alex 将生成可执行的微服务蓝图，包括组件划分、API 设计、开发步骤与运行建议，同时保持与 crafts 和 sandbox 的联动。
            </p>

            <form className="mt-8 space-y-6" onSubmit={handleSubmit}>
              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">服务名称 *</span>
                <input
                  value={serviceName}
                  onChange={(event) => setServiceName(event.target.value)}
                  placeholder="例如：订单状态聚合服务"
                  className="w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                  required
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">核心目标 *</span>
                <textarea
                  value={objective}
                  onChange={(event) => setObjective(event.target.value)}
                  placeholder="说明业务背景、需要解决的问题以及成功标准"
                  className="min-h-[140px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                  required
                />
              </label>

              <div className="space-y-2 text-sm">
                <span className="text-slate-200">首选技术栈（可选）</span>
                <div className="flex flex-wrap gap-2">
                  {LANGUAGE_PRESETS.map((preset) => (
                    <button
                      key={preset.value}
                      type="button"
                      onClick={() => setLanguage(preset.label)}
                      className={clsx(
                        'rounded-xl border px-3 py-1 text-xs transition',
                        language.toLowerCase().startsWith(preset.value)
                          ? 'border-cyan-400 bg-cyan-500/20 text-cyan-100'
                          : 'border-slate-700/60 bg-slate-900/60 text-slate-300 hover:border-cyan-400/60 hover:text-cyan-100'
                      )}
                    >
                      {preset.label}
                    </button>
                  ))}
                </div>
                <input
                  value={language}
                  onChange={(event) => setLanguage(event.target.value)}
                  placeholder="例如：Go + chi / Python + FastAPI"
                  className="w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                />
              </div>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">关键功能（每行一项，可选）</span>
                <textarea
                  value={featureInput}
                  onChange={(event) => setFeatureInput(event.target.value)}
                  placeholder={'示例：\n- 聚合多渠道订单状态\n- 推送异常告警到 Slack'}
                  className="min-h-[120px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                />
              </label>

              <label className="block space-y-2 text-sm">
                <span className="text-slate-200">外部依赖或集成（每行一项，可选）</span>
                <textarea
                  value={integrationInput}
                  onChange={(event) => setIntegrationInput(event.target.value)}
                  placeholder={'示例：\n- PostgreSQL / Prisma\n- 内部 CRM GraphQL API'}
                  className="min-h-[100px] w-full rounded-2xl border border-slate-700/60 bg-slate-900/70 px-4 py-3 text-sm text-slate-100 placeholder:text-slate-500 focus:border-cyan-400 focus:outline-none focus:ring-2 focus:ring-cyan-500/40"
                />
              </label>

              <button
                type="submit"
                disabled={submitDisabled}
                className="inline-flex w-full items-center justify-center gap-2 rounded-2xl bg-cyan-500/90 px-5 py-3 text-sm font-medium text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:bg-slate-700/70 disabled:text-slate-400"
              >
                {loading ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden /> : <Rocket className="h-4 w-4" aria-hidden />}
                {loading ? '生成中...' : '生成微服务蓝图'}
              </button>

              {error && <p className="text-sm text-red-400">{error}</p>}
              {successMessage && <p className="text-sm text-cyan-300">{successMessage}</p>}
            </form>
          </div>
        </section>

        <section className="w-full lg:w-3/5 space-y-6">
          <div className="rounded-3xl border border-slate-800/70 bg-slate-900/60 p-6 shadow-xl">
            {plan ? (
              <div className="space-y-8">
                <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-xl font-semibold text-slate-50">{plan.service_name}</h2>
                      <p className="mt-2 text-sm text-slate-300">{plan.summary}</p>
                    </div>
                    <div className="rounded-xl border border-cyan-500/30 bg-cyan-500/10 px-3 py-2 text-xs text-cyan-200">
                      {plan.language || '自动建议'} {plan.runtime ? `• ${plan.runtime}` : null}
                    </div>
                  </div>
                  {(sessionId || taskId) && (
                    <dl className="mt-4 grid grid-cols-1 gap-3 text-xs text-slate-400 sm:grid-cols-2">
                      {sessionId ? (
                        <div>
                          <dt className="font-medium text-slate-300">Session</dt>
                          <dd className="mt-1 break-all text-slate-400">{sessionId}</dd>
                        </div>
                      ) : null}
                      {taskId ? (
                        <div>
                          <dt className="font-medium text-slate-300">Task</dt>
                          <dd className="mt-1 break-all text-slate-400">{taskId}</dd>
                        </div>
                      ) : null}
                    </dl>
                  )}
                </div>

                {plan.architecture && plan.architecture.length > 0 && (
                  <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                    <SectionHeading icon={<Layers className="h-4 w-4" aria-hidden />} title="架构要点" />
                    <ul className="space-y-2 text-sm text-slate-200">
                      {plan.architecture.map((item) => (
                        <li key={item} className="flex items-center gap-2">
                          <span className="h-1.5 w-1.5 rounded-full bg-cyan-400/70" aria-hidden />
                          {item}
                        </li>
                      ))}
                    </ul>
                    {(() => {
                      const architectureText = plan.architecture?.join('\n') ?? '';
                      return (
                        <button
                          type="button"
                          onClick={() => handleCopy(architectureText)}
                          className="mt-3 inline-flex items-center gap-1 rounded-full border border-cyan-500/40 px-3 py-1 text-xs text-cyan-100 transition hover:border-cyan-300"
                        >
                          {copied === architectureText ? (
                            <Check className="h-3.5 w-3.5" aria-hidden />
                          ) : (
                            <Clipboard className="h-3.5 w-3.5" aria-hidden />
                          )}
                          {copied === architectureText ? '已复制' : '复制架构要点'}
                        </button>
                      );
                    })()}
                  </div>
                )}

                {plan.components.length > 0 && (
                  <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                    <SectionHeading icon={<Braces className="h-4 w-4" aria-hidden />} title="组件划分" />
                    <div className="grid gap-4 sm:grid-cols-2">
                      {plan.components.map((component) => (
                        <div key={component.name} className="rounded-2xl border border-slate-800/60 bg-slate-950/60 p-4">
                          <h4 className="text-sm font-semibold text-slate-100">{component.name}</h4>
                          <p className="mt-2 text-xs text-slate-300">{component.responsibility}</p>
                          {component.tech_notes && component.tech_notes.length > 0 && (
                            <ul className="mt-3 space-y-1 text-xs text-slate-400">
                              {component.tech_notes.map((note) => (
                                <li key={note} className="rounded-lg border border-slate-800/60 px-3 py-1">
                                  {note}
                                </li>
                              ))}
                            </ul>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {plan.api_endpoints && plan.api_endpoints.length > 0 && (
                  <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                    <SectionHeading icon={<ServerCog className="h-4 w-4" aria-hidden />} title="API 端点" />
                    <div className="space-y-3">
                      {plan.api_endpoints.map((endpoint) => {
                        const toCopy = `${endpoint.method.toUpperCase()} ${endpoint.path}`;
                        return (
                          <div
                            key={`${endpoint.method}-${endpoint.path}`}
                            className="rounded-2xl border border-slate-800/60 bg-slate-950/60 p-4"
                          >
                            <div className="flex flex-wrap items-center justify-between gap-3">
                              <div className="flex items-center gap-3">
                                <span className="rounded-full bg-cyan-500/20 px-2 py-0.5 text-xs font-semibold text-cyan-200">
                                  {endpoint.method.toUpperCase()}
                                </span>
                                <span className="text-sm text-slate-100">{endpoint.path}</span>
                              </div>
                              <button
                                type="button"
                                onClick={() => handleCopy(toCopy)}
                                className="inline-flex items-center gap-1 rounded-full border border-cyan-500/40 px-3 py-1 text-xs text-cyan-100 transition hover:border-cyan-300"
                              >
                                {copied === toCopy ? (
                                  <Check className="h-3.5 w-3.5" aria-hidden />
                                ) : (
                                  <Clipboard className="h-3.5 w-3.5" aria-hidden />
                                )}
                                {copied === toCopy ? '已复制' : '复制'}
                              </button>
                            </div>
                            <p className="mt-2 text-xs text-slate-300">{endpoint.description}</p>
                            {(endpoint.request_schema || endpoint.response_schema) && (
                              <dl className="mt-2 grid grid-cols-1 gap-2 text-xs text-slate-400 sm:grid-cols-2">
                                {endpoint.request_schema ? (
                                  <div>
                                    <dt className="font-medium text-slate-300">请求</dt>
                                    <dd className="mt-1 whitespace-pre-wrap break-words">{endpoint.request_schema}</dd>
                                  </div>
                                ) : null}
                                {endpoint.response_schema ? (
                                  <div>
                                    <dt className="font-medium text-slate-300">响应</dt>
                                    <dd className="mt-1 whitespace-pre-wrap break-words">{endpoint.response_schema}</dd>
                                  </div>
                                ) : null}
                              </dl>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                <div className="grid gap-4 md:grid-cols-3">
                  {plan.dev_tasks && plan.dev_tasks.length > 0 && (
                    <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                      <SectionHeading icon={<Layers className="h-4 w-4" aria-hidden />} title="开发步骤" />
                      <ul className="space-y-2 text-xs text-slate-300">
                        {plan.dev_tasks.map((task) => (
                          <li key={task} className="rounded-xl border border-slate-800/60 bg-slate-950/60 px-3 py-2">
                            {task}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {plan.operations && plan.operations.length > 0 && (
                    <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                      <SectionHeading icon={<ServerCog className="h-4 w-4" aria-hidden />} title="运行建议" />
                      <ul className="space-y-2 text-xs text-slate-300">
                        {plan.operations.map((item) => (
                          <li key={item} className="rounded-xl border border-slate-800/60 bg-slate-950/60 px-3 py-2">
                            {item}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                  {plan.testing && plan.testing.length > 0 && (
                    <div className="rounded-2xl border border-slate-800/60 bg-slate-900/80 p-5">
                      <SectionHeading icon={<Braces className="h-4 w-4" aria-hidden />} title="测试策略" />
                      <ul className="space-y-2 text-xs text-slate-300">
                        {plan.testing.map((item) => (
                          <li key={item} className="rounded-xl border border-slate-800/60 bg-slate-950/60 px-3 py-2">
                            {item}
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="flex min-h-[360px] flex-col items-center justify-center rounded-2xl border border-dashed border-slate-800/60 bg-slate-900/60 text-sm text-slate-400">
                <Layers className="mb-3 h-6 w-6 text-slate-600" aria-hidden />
                <p className="max-w-sm text-center">
                  填写左侧表单后，这里将展示结构化的微服务蓝图、组件划分、API 列表以及与 sandbox 的联动建议。
                </p>
              </div>
            )}
          </div>

          <div className="rounded-3xl border border-slate-800/70 bg-slate-900/40 p-4">
            <div className="flex items-center gap-2 text-sm text-slate-300">
              <ServerCog className="h-4 w-4" aria-hidden />
              <span>Sandbox 运行视图</span>
            </div>
            <iframe
              title="Agent Sandbox Viewer"
              src={SANDBOX_URL}
              className="mt-4 h-72 w-full rounded-2xl border border-slate-800/60 bg-black"
              allow="clipboard-read; clipboard-write"
            />
          </div>
        </section>
      </div>
    </div>
  );
}
