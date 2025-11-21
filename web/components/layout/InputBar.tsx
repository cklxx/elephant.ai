'use client';

import { useState, useRef, useEffect } from 'react';
import { Send, Paperclip, Mic } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useI18n } from '@/lib/i18n';
import type { AttachmentUpload } from '@/lib/types';

interface InputBarProps {
  onSubmit: (text: string, attachments: AttachmentUpload[]) => void;
  placeholder?: string;
  disabled?: boolean;
  loading?: boolean;
  prefill?: string | null;
  onPrefillApplied?: () => void;
  showAttachment?: boolean;
  showVoice?: boolean;
  onAttachment?: () => void;
  onVoice?: () => void;
}

export function InputBar({
  onSubmit,
  placeholder,
  disabled = false,
  loading = false,
  prefill = null,
  onPrefillApplied,
  showAttachment = false,
  showVoice = false,
  onAttachment,
  onVoice,
}: InputBarProps) {
  const { t } = useI18n();
  const [text, setText] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const resolvedPlaceholder = placeholder ?? t('inputBar.placeholder');

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = textareaRef.current.scrollHeight + 'px';
    }
  }, [text]);

  // Handle prefill
  useEffect(() => {
    if (typeof prefill !== 'string') return;
    const nextValue = prefill.trim();
    if (!nextValue) return;

    setText(prefill);

    const focusField = () => {
      if (!textareaRef.current) return;
      textareaRef.current.focus();
      const length = prefill.length;
      textareaRef.current.setSelectionRange(length, length);
    };

    if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
      window.requestAnimationFrame(focusField);
    } else {
      setTimeout(focusField, 0);
    }

    onPrefillApplied?.();
  }, [prefill, onPrefillApplied]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (text.trim() && !loading && !disabled) {
      onSubmit(text.trim(), []);
      setText('');
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto';
      }
    }
  };

  return (
    <div className="layout-input-bar border-t border-border/70 bg-gradient-to-b from-background to-muted/40 px-4 py-4">
      <form onSubmit={handleSubmit} className="mx-auto flex w-full max-w-5xl flex-col gap-2">
        <div className="flex items-end gap-3 rounded-2xl border border-border/70 bg-card/90 p-3 shadow-lg shadow-black/5 backdrop-blur">
          <div className="flex items-center gap-2 self-stretch">
            {showAttachment && (
              <button
                type="button"
                onClick={onAttachment}
                disabled={disabled || loading}
                className={cn(
                  'inline-flex h-11 w-11 items-center justify-center rounded-xl border border-border/70 bg-background/90 text-foreground shadow-sm transition',
                  'hover:-translate-y-[1px] hover:bg-muted/70 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30',
                  'disabled:pointer-events-none disabled:translate-y-0 disabled:opacity-50'
                )}
                title={t('inputBar.actions.attach')}
                aria-label={t('inputBar.actions.attach')}
              >
                <Paperclip className="h-4 w-4" />
              </button>
            )}
            {showVoice && (
              <button
                type="button"
                onClick={onVoice}
                disabled={disabled || loading}
                className={cn(
                  'inline-flex h-11 w-11 items-center justify-center rounded-xl border border-border/70 bg-background/90 text-foreground shadow-sm transition',
                  'hover:-translate-y-[1px] hover:bg-muted/70 hover:shadow-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/30',
                  'disabled:pointer-events-none disabled:translate-y-0 disabled:opacity-50'
                )}
                title={t('inputBar.actions.voice')}
                aria-label={t('inputBar.actions.voice')}
              >
                <Mic className="h-4 w-4" />
              </button>
            )}
          </div>

          <div className="relative flex-1">
            <div className="group flex min-h-[68px] items-start gap-3 rounded-xl border border-border/80 bg-gradient-to-br from-background/95 via-background/90 to-muted/50 px-4 py-3 shadow-inner transition focus-within:border-primary/60 focus-within:ring-2 focus-within:ring-primary/20">
              <textarea
                ref={textareaRef}
                value={text}
                onChange={(e) => setText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    handleSubmit(e);
                  }
                }}
                placeholder={resolvedPlaceholder}
                disabled={disabled || loading}
                rows={1}
                maxLength={4000}
                className={cn(
                  'peer w-full resize-none overflow-y-auto border-none bg-transparent text-base leading-6 text-foreground outline-none',
                  'placeholder:text-muted-foreground/90',
                  'disabled:cursor-not-allowed disabled:opacity-60',
                  'max-h-36'
                )}
                style={{ fieldSizing: 'content' } as any}
              />

              <span className="absolute bottom-2 right-3 text-[11px] font-mono text-muted-foreground/90">
                {text.length} / 4000
              </span>
            </div>
          </div>

          <button
            type="submit"
            disabled={disabled || loading || !text.trim()}
            className={cn(
              'inline-flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-primary to-indigo-500 text-white shadow-lg transition',
              'hover:scale-105 hover:shadow-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/40',
              'disabled:scale-100 disabled:bg-muted disabled:text-muted-foreground disabled:shadow-none'
            )}
            title={loading ? t('inputBar.actions.sending') : t('inputBar.actions.send')}
            aria-label={loading ? t('inputBar.actions.sending') : t('inputBar.actions.send')}
          >
            {loading ? (
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/80 border-t-transparent" />
            ) : (
              <Send className="h-4 w-4" />
            )}
          </button>
        </div>

        <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-1 text-[11px] leading-tight text-muted-foreground/90">
          <span>{t('inputBar.hint.shortcut')}</span>
          <span className="rounded-full bg-muted px-2 py-1 text-[10px] font-medium uppercase tracking-wide text-foreground/80">
            {t('inputBar.placeholder')}
          </span>
        </div>
      </form>
    </div>
  );
}
