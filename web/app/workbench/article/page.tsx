'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Image from 'next/image';
import {
  Bold,
  Italic,
  List,
  Heading2,
  Quote,
  Undo,
  Redo,
  Link2,
  Sparkles,
  Lightbulb,
  Save,
  History,
  Image as ImageIcon,
  Copy,
  Download,
} from 'lucide-react';
import clsx from 'clsx';
import {
  generateArticleInsights,
  saveArticleDraft,
  APIError,
  listArticleDrafts,
  deleteArticleDraft,
} from '@/lib/api';
import type { ArticleInsightResponse, ArticleDraftSummary } from '@/lib/types';

const DEFAULT_CONTENT = `
  <h1>开启你的下一篇文章</h1>
  <p>写下主题、目标读者与期待的成果。Agent 会在后台帮你查阅可信资料，并在右侧提供补充建议。</p>
`;

const SANDBOX_VIEWER_URL = process.env.NEXT_PUBLIC_SANDBOX_VIEWER_URL ?? 'about:blank';

type ToolbarCommand = 'bold' | 'italic' | 'insertUnorderedList' | 'formatBlock' | 'undo' | 'redo' | 'createLink' | 'formatQuote';

function ToolbarButton({
  icon: Icon,
  label,
  onClick,
}: {
  icon: typeof Bold;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-9 w-9 items-center justify-center rounded-lg border border-slate-700/70 bg-slate-900/80 text-slate-100 transition-colors hover:border-cyan-400/60 hover:text-cyan-200"
      aria-label={label}
    >
      <Icon className="h-4 w-4" aria-hidden />
    </button>
  );
}

export default function ArticleWorkbenchPage() {
  const editorRef = useRef<HTMLDivElement | null>(null);
  const [html, setHtml] = useState<string>(DEFAULT_CONTENT);
  const [selectionLink, setSelectionLink] = useState<string>('');
  const [insights, setInsights] = useState<ArticleInsightResponse | null>(null);
  const [insightStatus, setInsightStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [insightError, setInsightError] = useState<string | null>(null);
  const [insightUpdatedAt, setInsightUpdatedAt] = useState<Date | null>(null);
  const pendingRequestRef = useRef(0);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'success' | 'error'>('idle');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [recentCraftName, setRecentCraftName] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<ArticleDraftSummary[]>([]);
  const [draftStatus, setDraftStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [draftError, setDraftError] = useState<string | null>(null);
  const [draftLoadError, setDraftLoadError] = useState<string | null>(null);
  const [loadingDraftId, setLoadingDraftId] = useState<string | null>(null);
  const [loadedDraftName, setLoadedDraftName] = useState<string | null>(null);
  const [deletingDraftId, setDeletingDraftId] = useState<string | null>(null);
  const [draftDeleteError, setDraftDeleteError] = useState<string | null>(null);
  const [deletedDraftName, setDeletedDraftName] = useState<string | null>(null);
  const [copiedIllustrationIndex, setCopiedIllustrationIndex] = useState<number | null>(null);

  useEffect(() => {
    if (editorRef.current) {
      editorRef.current.innerHTML = DEFAULT_CONTENT;
    }
  }, []);

  const runCommand = useCallback(
    (command: ToolbarCommand, value?: string) => {
      if (typeof document === 'undefined') return;

      if (command === 'formatBlock') {
        document.execCommand('formatBlock', false, value ?? 'P');
      } else if (command === 'formatQuote') {
        document.execCommand('formatBlock', false, 'blockquote');
      } else if (command === 'createLink') {
        const url = value || window.prompt('请输入要插入的链接地址', selectionLink || 'https://');
        if (url) {
          document.execCommand('createLink', false, url);
        }
      } else {
        document.execCommand(command, false, value ?? '');
      }

      if (editorRef.current) {
        setHtml(editorRef.current.innerHTML);
      }
    },
    [selectionLink]
  );

  const handleInput = useCallback(() => {
    if (editorRef.current) {
      setHtml(editorRef.current.innerHTML);
    }
  }, []);

  const handleSelectionChange = useCallback(() => {
    if (typeof document === 'undefined') return;
    const selection = document.getSelection();
    if (!selection || selection.rangeCount === 0) {
      setSelectionLink('');
      return;
    }
    const container = selection.getRangeAt(0).commonAncestorContainer;
    const element = container instanceof Element ? container : container.parentElement;
    const linkElement = element?.closest?.('a');
    if (linkElement instanceof HTMLAnchorElement) {
      setSelectionLink(linkElement.href);
    } else {
      setSelectionLink('');
    }
  }, []);

  const handleInsertText = useCallback(
    (text: string) => {
      const payload = text.trim();
      if (!payload) return;
      if (typeof document === 'undefined') return;
      if (editorRef.current) {
        editorRef.current.focus();
      }
      document.execCommand('insertText', false, payload);
      if (editorRef.current) {
        setHtml(editorRef.current.innerHTML);
      }
    },
    []
  );

  useEffect(() => {
    if (typeof document === 'undefined') return;
    document.addEventListener('selectionchange', handleSelectionChange);
    return () => document.removeEventListener('selectionchange', handleSelectionChange);
  }, [handleSelectionChange]);

  const refreshDrafts = useCallback(() => {
    setDraftStatus('loading');
    setDraftError(null);
    listArticleDrafts()
      .then((response) => {
        setDrafts(response.drafts ?? []);
        setDraftStatus('success');
      })
      .catch((error: unknown) => {
        setDraftStatus('error');
        let message = '无法获取草稿列表，请稍后再试。';
        if (error instanceof APIError) {
          if (error.status === 401) {
            message = '请登录后查看保存的草稿。';
          } else if (error.details) {
            message = error.details;
          } else if (error.message) {
            message = error.message;
          }
        } else if (error instanceof Error && error.message) {
          message = error.message;
        }
        setDraftError(message);
      });
  }, []);

  useEffect(() => {
    refreshDrafts();
  }, [refreshDrafts]);

  useEffect(() => {
    if (saveStatus !== 'success') {
      return;
    }
    const timer = window.setTimeout(() => {
      setSaveStatus('idle');
      setSaveError(null);
    }, 4000);
    return () => window.clearTimeout(timer);
  }, [saveStatus]);

  useEffect(() => {
    if (!loadedDraftName) {
      return;
    }
    const timer = window.setTimeout(() => {
      setLoadedDraftName(null);
    }, 4000);
    return () => window.clearTimeout(timer);
  }, [loadedDraftName]);

  useEffect(() => {
    if (!draftLoadError) {
      return;
    }
    const timer = window.setTimeout(() => {
      setDraftLoadError(null);
    }, 5000);
    return () => window.clearTimeout(timer);
  }, [draftLoadError]);

  useEffect(() => {
    if (!draftDeleteError) {
      return;
    }
    const timer = window.setTimeout(() => {
      setDraftDeleteError(null);
    }, 5000);
    return () => window.clearTimeout(timer);
  }, [draftDeleteError]);

  useEffect(() => {
    if (!deletedDraftName) {
      return;
    }
    const timer = window.setTimeout(() => {
      setDeletedDraftName(null);
    }, 4000);
    return () => window.clearTimeout(timer);
  }, [deletedDraftName]);

  useEffect(() => {
    const normalized = html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
    if (!normalized) {
      pendingRequestRef.current += 1;
      setInsights(null);
      setInsightStatus('idle');
      setInsightError(null);
      setInsightUpdatedAt(null);
      setSessionId(null);
      return;
    }

    const requestId = pendingRequestRef.current + 1;
    pendingRequestRef.current = requestId;
    setInsightStatus('loading');
    setInsightError(null);

    const truncated = Array.from(html).slice(0, 5000).join('');

    const timer = window.setTimeout(() => {
      generateArticleInsights(truncated)
        .then((response) => {
          if (pendingRequestRef.current !== requestId) {
            return;
          }
          setInsights(response);
          setInsightStatus('success');
          setInsightUpdatedAt(new Date());
          setInsightError(null);
          if (response.session_id) {
            setSessionId(response.session_id);
          }
        })
        .catch((error: unknown) => {
          if (pendingRequestRef.current !== requestId) {
            return;
          }
          setInsightStatus('error');
          let message = '获取 AI 辅助信息失败';
          if (error instanceof APIError) {
            if (error.status === 401) {
              message = '请登录后使用 AI 辅助功能。';
            } else if (error.message) {
              message = error.message;
            }
          } else if (error instanceof Error && error.message) {
            message = error.message;
          }
          setInsightError(message);
        });
    }, 1200);

    return () => {
      window.clearTimeout(timer);
    };
  }, [html]);

  const plainText = useMemo(() => {
    return html.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
  }, [html]);

  const derivedTitle = useMemo(() => {
    const headingMatch = html.match(/<h[1-6][^>]*>(.*?)<\/h[1-6]>/i);
    if (headingMatch && headingMatch[1]) {
      const headingText = headingMatch[1].replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
      if (headingText) {
        return headingText.slice(0, 60);
      }
    }
    if (plainText) {
      return plainText.slice(0, 60);
    }
    return '文章草稿';
  }, [html, plainText]);

  const derivedSummary = useMemo(() => {
    if (insights?.summary && insights.summary.trim().length > 0) {
      return insights.summary.trim();
    }
    if (plainText) {
      return plainText.slice(0, 160);
    }
    return '';
  }, [insights, plainText]);

  const wordCount = useMemo(() => {
    if (!plainText) return 0;
    return plainText.split(' ').filter(Boolean).length;
  }, [plainText]);

  const hasContent = plainText.length > 0;

  const lastUpdatedLabel = useMemo(() => {
    if (!insightUpdatedAt) return '';
    try {
      return insightUpdatedAt.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch (error) {
      return insightUpdatedAt.toISOString();
    }
  }, [insightUpdatedAt]);

  const keyPoints = insights?.key_points ?? [];
  const suggestions = insights?.suggestions ?? [];
  const citations = insights?.citations ?? [];
  const illustrations = insights?.illustrations ?? [];
  const isLoadingInsights = insightStatus === 'loading';
  const isSavingDraft = saveStatus === 'saving';
  const showSaveSuccess = saveStatus === 'success' && recentCraftName;
  const showSaveError = saveStatus === 'error' && saveError;
  const canSaveDraft = hasContent && !isSavingDraft;

  const handleCopyIllustrationPrompt = useCallback(async (prompt: string, index: number) => {
    const trimmed = prompt.trim();
    if (!trimmed) return;
    try {
      await navigator.clipboard.writeText(trimmed);
      setCopiedIllustrationIndex(index);
      window.setTimeout(() => {
        setCopiedIllustrationIndex((prev) => (prev === index ? null : prev));
      }, 3000);
    } catch (error) {
      window.prompt('复制失败，请手动复制以下内容：', trimmed);
    }
  }, []);

  const handleSaveDraft = useCallback(() => {
    if (!plainText) {
      setSaveStatus('error');
      setSaveError('请先输入文章内容后再保存。');
      return;
    }
    setSaveStatus('saving');
    setSaveError(null);
    saveArticleDraft({
      session_id: sessionId ?? undefined,
      title: derivedTitle,
      content: html,
      summary: derivedSummary || undefined,
    })
      .then((response) => {
        setSaveStatus('success');
        setRecentCraftName(response.craft.name);
        setSessionId(response.session_id);
        refreshDrafts();
        setLoadedDraftName(null);
      })
      .catch((error: unknown) => {
        let message = '保存草稿失败，请稍后再试。';
        if (error instanceof APIError) {
          if (error.status === 401) {
            message = '请登录后保存草稿。';
          } else if (error.details) {
            message = error.details;
          } else if (error.message) {
            message = error.message;
          }
        } else if (error instanceof Error && error.message) {
          message = error.message;
        }
        setSaveStatus('error');
        setSaveError(message);
      });
  }, [plainText, sessionId, derivedTitle, html, derivedSummary, refreshDrafts]);

  const handleLoadDraft = useCallback(
    (draft: ArticleDraftSummary) => {
      if (!draft.download_url) {
        setDraftLoadError('该草稿缺少下载链接，请在 Crafts 页面查看原文件。');
        return;
      }
      setLoadingDraftId(draft.craft.id);
      setDraftLoadError(null);
      setLoadedDraftName(null);
      fetch(draft.download_url)
        .then((response) => {
          if (!response.ok) {
            throw new Error(`下载草稿失败（HTTP ${response.status}）`);
          }
          return response.text();
        })
        .then((content) => {
          if (editorRef.current) {
            editorRef.current.innerHTML = content;
          }
          setHtml(content);
          setSessionId(draft.craft.session_id || null);
          setLoadedDraftName(draft.craft.name);
        })
        .catch((error: unknown) => {
          let message = '载入草稿失败，请稍后再试。';
          if (error instanceof Error && error.message) {
            message = error.message;
          }
          setDraftLoadError(message);
        })
        .finally(() => {
          setLoadingDraftId(null);
        });
    },
    [setHtml, setSessionId]
  );

  const handleDeleteDraft = (draft: ArticleDraftSummary) => {
    setDeletingDraftId(draft.craft.id);
    setDraftDeleteError(null);
    deleteArticleDraft(draft.craft.id)
      .then(() => {
        setDrafts((previous) => previous.filter((item) => item.craft.id !== draft.craft.id));
        setDeletedDraftName(draft.craft.name);
      })
      .catch((error: unknown) => {
        let message = '删除草稿失败，请稍后再试。';
        if (error instanceof APIError) {
          if (error.status === 401) {
            message = '请登录后删除草稿。';
          } else if (error.details) {
            message = error.details;
          } else if (error.message) {
            message = error.message;
          }
        } else if (error instanceof Error && error.message) {
          message = error.message;
        }
        setDraftDeleteError(message);
      })
      .finally(() => {
        setDeletingDraftId(null);
      });
  };

  return (
    <div className="relative min-h-screen bg-slate-950 pb-32 text-slate-100">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-6 py-10 lg:flex-row">
        <section className="flex-1">
          <header className="mb-6 flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-sm font-medium text-cyan-300/80">Article Studio</p>
              <h1 className="mt-2 text-3xl font-semibold">所见即所得的文章工作台</h1>
              <p className="mt-2 text-sm text-slate-400">
                随手记录想法，使用工具栏快速排版。Agent 会自动补充经过核实的资料与引用建议。
              </p>
            </div>
            <div className="flex flex-col items-start gap-2 md:items-end">
              <button
                type="button"
                onClick={handleSaveDraft}
                disabled={!canSaveDraft}
                className={clsx(
                  'inline-flex items-center gap-2 rounded-xl border px-4 py-2 text-sm transition focus:outline-none focus:ring-2 focus:ring-cyan-400/60',
                  isSavingDraft
                    ? 'border-cyan-500/60 bg-cyan-500/20 text-cyan-100'
                    : 'border-cyan-400/40 bg-cyan-500/10 text-cyan-100 hover:border-cyan-300 hover:bg-cyan-500/20',
                  !canSaveDraft && 'cursor-not-allowed border-slate-700 bg-slate-800/40 text-slate-400'
                )}
              >
                <Save className="h-4 w-4" aria-hidden />
                {isSavingDraft ? '保存中…' : '保存到 Crafts'}
              </button>
              {showSaveSuccess && recentCraftName && (
                <span className="text-xs text-cyan-200" role="status">
                  已保存到 Crafts：{recentCraftName}
                </span>
              )}
              {showSaveError && saveError && (
                <span className="text-xs text-rose-300" role="alert">
                  {saveError}
                </span>
              )}
              {!hasContent && !showSaveError && (
                <span className="text-xs text-slate-500">输入内容后即可保存到 Crafts。</span>
              )}
            </div>
          </header>

          <div className="rounded-2xl border border-slate-800/70 bg-slate-900/60 shadow-xl">
            <div className="flex flex-wrap gap-2 border-b border-slate-800/70 p-4">
              <ToolbarButton icon={Heading2} label="标题" onClick={() => runCommand('formatBlock', 'H2')} />
              <ToolbarButton icon={Bold} label="加粗" onClick={() => runCommand('bold')} />
              <ToolbarButton icon={Italic} label="斜体" onClick={() => runCommand('italic')} />
              <ToolbarButton icon={List} label="无序列表" onClick={() => runCommand('insertUnorderedList')} />
              <ToolbarButton icon={Quote} label="引用" onClick={() => runCommand('formatQuote')} />
              <ToolbarButton icon={Link2} label="插入链接" onClick={() => runCommand('createLink')} />
              <ToolbarButton icon={Undo} label="撤销" onClick={() => runCommand('undo')} />
              <ToolbarButton icon={Redo} label="重做" onClick={() => runCommand('redo')} />
            </div>
            <div className="prose prose-invert max-w-none p-6">
              <div
                ref={editorRef}
                className={clsx(
                  'min-h-[420px] w-full rounded-xl border border-transparent bg-slate-950/40 p-6 text-base leading-relaxed shadow-inner',
                  'focus:border-cyan-400/70 focus:outline-none'
                )}
                contentEditable
                suppressContentEditableWarning
                onInput={handleInput}
                aria-label="文章内容编辑器"
              />
            </div>
          </div>

          <footer className="mt-4 flex items-center justify-between text-xs text-slate-500">
            <span>字数统计：{wordCount}</span>
            {selectionLink && (
              <span className="truncate">当前链接：{selectionLink}</span>
            )}
          </footer>
        </section>

        <aside className="w-full max-w-sm rounded-2xl border border-cyan-500/10 bg-cyan-500/5 p-6">
          <div className="flex items-center gap-3 text-cyan-200">
            <Sparkles className="h-5 w-5" aria-hidden />
            <h2 className="text-lg font-semibold">AI 辅助面板</h2>
          </div>
          <p className="mt-3 text-sm text-cyan-100/80">
            {isLoadingInsights
              ? 'Agent 正在根据当前文稿检索可信资料…'
              : 'Agent 会自动去重资料并筛选可执行的建议，随时点击插入文稿。'}
          </p>
          {insightStatus === 'success' && lastUpdatedLabel && (
            <p className="mt-2 text-xs text-cyan-200/70">最近更新：{lastUpdatedLabel}</p>
          )}
          {insightError && (
            <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 p-3 text-xs text-rose-200">
              {insightError}
            </p>
          )}
          <div className="mt-4 space-y-4 text-sm text-cyan-50/90">
            <div className="rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
              <div className="flex items-center gap-2 text-cyan-200">
                <Sparkles className="h-4 w-4" aria-hidden />
                <span className="font-medium">摘要提炼</span>
              </div>
              <p
                className={clsx(
                  'mt-2 leading-relaxed text-cyan-100/80',
                  isLoadingInsights && 'animate-pulse text-cyan-100/60'
                )}
              >
                {insightStatus === 'success' && insights?.summary
                  ? insights.summary
                  : '正在为草稿整理关键摘要与论据…'}
              </p>
            </div>
            <div className="rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
              <div className="flex items-center gap-2 text-cyan-200">
                <Lightbulb className="h-4 w-4" aria-hidden />
                <span className="font-medium">重点与结构</span>
              </div>
              {keyPoints.length > 0 ? (
                <ul className="mt-2 list-disc space-y-1 pl-5">
                  {keyPoints.map((point, index) => (
                    <li key={index} className="text-cyan-100/80">
                      {point}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-cyan-100/70">等待最新内容以提炼重点…</p>
              )}
            </div>
            <div className="rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
              <div className="flex items-center gap-2 text-cyan-200">
                <Sparkles className="h-4 w-4" aria-hidden />
                <span className="font-medium">下一步建议</span>
              </div>
              {suggestions.length > 0 ? (
                <ul className="mt-3 space-y-2">
                  {suggestions.map((suggestion, index) => (
                    <li
                      key={index}
                      className="flex items-start justify-between gap-2 rounded-lg border border-cyan-500/20 bg-slate-950/40 p-3"
                    >
                      <span className="text-xs text-cyan-100/90">{suggestion}</span>
                      <button
                        type="button"
                        onClick={() => handleInsertText(suggestion)}
                        className="shrink-0 rounded-md border border-cyan-500/40 px-2 py-1 text-[11px] text-cyan-200 transition hover:border-cyan-300 hover:text-cyan-100"
                      >
                        插入
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-cyan-100/70">整理建议中…稍后即可一键插入。</p>
              )}
            </div>
            <div className="rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
              <div className="flex items-center gap-2 text-cyan-200">
                <Lightbulb className="h-4 w-4" aria-hidden />
                <span className="font-medium">引用资料</span>
              </div>
              {citations.length > 0 ? (
                <ul className="mt-3 space-y-3 text-xs text-cyan-100/80">
                  {citations.map((citation, index) => (
                    <li key={`${citation.url}-${index}`} className="space-y-1">
                      <a
                        href={citation.url}
                        target="_blank"
                        rel="noreferrer"
                        className="font-medium text-cyan-200 hover:text-cyan-100 hover:underline"
                      >
                        {citation.title}
                      </a>
                      {citation.snippet && <p className="text-cyan-100/70">{citation.snippet}</p>}
                      {citation.source && <p className="text-cyan-100/50">来源：{citation.source}</p>}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="mt-2 text-cyan-100/70">暂无可信引用，Agent 正在继续检索。</p>
              )}
            </div>
            <div className="rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
              <div className="flex items-center gap-2 text-cyan-200">
                <ImageIcon className="h-4 w-4" aria-hidden />
                <span className="font-medium">段落插图建议</span>
              </div>
              {illustrations.length > 0 ? (
                <ul className="mt-3 space-y-3 text-xs text-cyan-100/85">
                  {illustrations.map((item, index) => {
                    const hasImage = Boolean(item.image_url);
                    return (
                      <li
                        key={`${item.prompt}-${index}`}
                        className="space-y-2 rounded-lg border border-cyan-500/20 bg-slate-950/60 p-3"
                      >
                        {item.paragraph_summary && (
                          <p className="text-[13px] font-medium text-cyan-200/90">
                            {item.paragraph_summary}
                          </p>
                        )}
                        <p className="text-cyan-100/80">{item.image_idea}</p>
                        {hasImage ? (
                          <div className="overflow-hidden rounded-lg border border-cyan-500/20 bg-slate-900/60">
                            <Image
                              src={item.image_url}
                              alt={item.image_idea || '文章插图'}
                              width={640}
                              height={256}
                              className="h-44 w-full object-cover"
                              unoptimized
                            />
                            <div className="flex items-center justify-between gap-2 border-t border-cyan-500/10 bg-slate-900/70 px-3 py-2">
                              <div className="flex flex-col text-[11px] text-cyan-100/80">
                                <span className="font-medium text-cyan-100">
                                  {item.name || '自动生成插图'}
                                </span>
                                <span className="text-cyan-100/60">
                                  {item.media_type ? item.media_type : 'image/png'}
                                </span>
                              </div>
                              <a
                                href={item.image_url}
                                target="_blank"
                                rel="noreferrer"
                                className="inline-flex items-center gap-1 rounded-md border border-cyan-500/40 px-2 py-1 text-[11px] text-cyan-200 transition hover:border-cyan-300 hover:text-cyan-100"
                              >
                                <Download className="h-3.5 w-3.5" aria-hidden />
                                查看 / 下载
                              </a>
                            </div>
                          </div>
                        ) : (
                          <div className="rounded-md border border-dashed border-cyan-500/30 bg-slate-950/40 px-3 py-4 text-center text-[11px] text-cyan-100/60">
                            插图生成中…完成后会自动展示并同步到 Crafts。
                          </div>
                        )}
                        {item.keywords && item.keywords.length > 0 && (
                          <p className="text-cyan-100/60">
                            关键词：{item.keywords.join(' · ')}
                          </p>
                        )}
                        <div className="flex items-start justify-between gap-2 rounded-md border border-cyan-500/20 bg-slate-950/40 p-2">
                          <code className="block flex-1 whitespace-pre-wrap break-words text-[11px] leading-relaxed text-cyan-100/70">
                            {item.prompt}
                          </code>
                          <button
                            type="button"
                            onClick={() => handleCopyIllustrationPrompt(item.prompt, index)}
                            className="flex shrink-0 items-center gap-1 rounded-md border border-cyan-500/40 px-2 py-1 text-[11px] text-cyan-200 transition hover:border-cyan-300 hover:text-cyan-100"
                          >
                            <Copy className="h-3.5 w-3.5" aria-hidden />
                            {copiedIllustrationIndex === index ? '已复制' : '复制插图提示词'}
                          </button>
                        </div>
                      </li>
                    );
                  })}
                </ul>
              ) : (
                <p className="mt-2 text-cyan-100/70">Agent 会为重点段落生成合适的视觉提示词。</p>
              )}
            </div>
          </div>
          <div className="mt-6 rounded-xl border border-cyan-500/20 bg-slate-950/40 p-4">
            <div className="flex items-center gap-2 text-cyan-200">
              <History className="h-4 w-4" aria-hidden />
              <span className="font-medium">草稿历史</span>
            </div>
            {draftStatus === 'loading' ? (
              <p className="mt-2 text-xs text-cyan-100/70">载入草稿列表…</p>
            ) : drafts.length === 0 ? (
              <p className="mt-2 text-xs text-cyan-100/60">暂无保存的草稿，完成编辑后即可保存并显示在此处。</p>
            ) : (
              <ul className="mt-3 space-y-3 text-xs text-cyan-100/80">
                {drafts.map((draft) => {
                  let createdLabel = draft.craft.created_at;
                  try {
                    const parsed = new Date(draft.craft.created_at);
                    if (!Number.isNaN(parsed.getTime())) {
                      createdLabel = parsed.toLocaleString();
                    }
                  } catch (error) {
                    createdLabel = draft.craft.created_at;
                  }
                  const isActiveSession = sessionId && draft.craft.session_id === sessionId;
                  const isLoading = loadingDraftId === draft.craft.id;
                  const isDeleting = deletingDraftId === draft.craft.id;
                  return (
                    <li
                      key={draft.craft.id}
                      className="rounded-lg border border-cyan-500/20 bg-slate-950/30 p-3 shadow-sm"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 space-y-1">
                          <p className="truncate text-sm text-cyan-100">{draft.craft.name}</p>
                          <p className="text-[11px] text-cyan-100/60">{createdLabel}</p>
                          {isActiveSession && (
                            <p className="text-[10px] text-cyan-200/80">当前工作会话</p>
                          )}
                        </div>
                        <div className="flex flex-col items-end gap-2">
                          <button
                            type="button"
                            onClick={() => handleLoadDraft(draft)}
                            disabled={isLoading || isDeleting}
                            className={clsx(
                              'shrink-0 rounded-md border px-3 py-1 text-[11px] transition focus:outline-none focus:ring-2 focus:ring-cyan-400/60',
                              isLoading
                                ? 'border-cyan-500/40 bg-cyan-500/20 text-cyan-100'
                                : 'border-cyan-500/30 bg-slate-950/60 text-cyan-200 hover:border-cyan-300 hover:text-cyan-100',
                              isDeleting && 'cursor-not-allowed opacity-70'
                            )}
                          >
                            {isLoading ? '载入中…' : '载入'}
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDeleteDraft(draft)}
                            disabled={isDeleting}
                            className={clsx(
                              'shrink-0 rounded-md border px-3 py-1 text-[11px] transition focus:outline-none focus:ring-2 focus:ring-rose-400/60',
                              isDeleting
                                ? 'border-rose-500/40 bg-rose-500/20 text-rose-100'
                                : 'border-rose-500/30 bg-slate-950/60 text-rose-200 hover:border-rose-400 hover:text-rose-100'
                            )}
                          >
                            {isDeleting ? '删除中…' : '删除'}
                          </button>
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
            {draftError && (
              <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 p-2 text-[11px] text-rose-200" role="alert">
                {draftError}
              </p>
            )}
            {draftLoadError && (
              <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 p-2 text-[11px] text-rose-200" role="alert">
                {draftLoadError}
              </p>
            )}
            {draftDeleteError && (
              <p className="mt-3 rounded-lg border border-rose-500/40 bg-rose-500/10 p-2 text-[11px] text-rose-200" role="alert">
                {draftDeleteError}
              </p>
            )}
            {loadedDraftName && (
              <p className="mt-3 rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-2 text-[11px] text-cyan-100" role="status">
                已载入草稿：{loadedDraftName}
              </p>
            )}
            {deletedDraftName && (
              <p className="mt-3 rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-2 text-[11px] text-cyan-100" role="status">
                已删除草稿：{deletedDraftName}
              </p>
            )}
          </div>
        </aside>
      </div>

      <div className="pointer-events-none fixed bottom-6 right-6 hidden w-96 max-w-full overflow-hidden rounded-xl border border-slate-800/80 bg-slate-950/90 shadow-2xl md:block">
        <div className="flex items-center justify-between border-b border-slate-800/80 px-4 py-2">
          <span className="text-xs font-medium text-slate-300">Agent Sandbox</span>
          <span className="text-[10px] text-slate-500">实时操作轨迹</span>
        </div>
        <iframe
          title="Agent Sandbox Viewer"
          src={SANDBOX_VIEWER_URL}
          className="pointer-events-auto h-64 w-full border-0"
          allow="clipboard-read; clipboard-write"
        />
      </div>
    </div>
  );
}
