'use client';

import { useState, useRef, useEffect } from 'react';
import { Send, Paperclip, Mic } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useI18n } from '@/lib/i18n';
import type { AttachmentUpload } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';

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
    <div className="px-6 py-4">
      <div className="mx-auto flex max-w-5xl flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
          <span className="rounded-full border border-border px-3 py-1 text-[11px] uppercase tracking-[0.18em]">
            {t('inputBar.placeholder')}
          </span>
          <span className="uppercase tracking-[0.22em]">{t('inputBar.hint.shortcut')}</span>
        </div>

        <form
          onSubmit={handleSubmit}
          className="grid grid-cols-[auto,1fr,auto] items-start gap-3 rounded-2xl border border-border bg-card p-3"
        >
          <div className="flex items-center gap-2 self-start">
            {showAttachment && (
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={onAttachment}
                disabled={disabled || loading}
                className="h-10 w-10 rounded-lg"
                title={t('inputBar.actions.attach')}
              >
                <Paperclip className="h-4 w-4" />
              </Button>
            )}
            {showVoice && (
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={onVoice}
                disabled={disabled || loading}
                className="h-10 w-10 rounded-lg"
                title={t('inputBar.actions.voice')}
              >
                <Mic className="h-4 w-4" />
              </Button>
            )}
          </div>

          <div className="relative flex-1">
            <Textarea
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
              className="min-h-[56px] max-h-32 resize-none"
              style={{ fieldSizing: 'content' } as any}
            />
          </div>

          <Button
            type="submit"
            disabled={disabled || loading || !text.trim()}
            className="h-10 w-10 rounded-lg"
            size="icon"
            title={loading ? t('inputBar.actions.sending') : t('inputBar.actions.send')}
          >
            {loading ? (
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-primary-foreground border-t-transparent" />
            ) : (
              <Send className="h-4 w-4" />
            )}
          </Button>
        </form>

        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{t('inputBar.placeholder')}</span>
          {text.length > 0 && (
            <span className="font-mono">
              {text.length} / 4000
            </span>
          )}
        </div>
      </div>
    </div>
  );
}
