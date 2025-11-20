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
    <div className="layout-input-bar px-6 py-4">
      <form
        onSubmit={handleSubmit}
        className="grid grid-cols-[auto,1fr,auto] items-start gap-3"
      >
        <div className="flex items-center gap-2 self-start">
          {showAttachment && (
            <button
              type="button"
              onClick={onAttachment}
              disabled={disabled || loading}
              className={cn(
                'console-icon-button h-10 w-10 !rounded-lg',
                'disabled:opacity-50'
              )}
              title={t('inputBar.actions.attach')}
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
                'console-icon-button h-10 w-10 !rounded-lg',
                'disabled:opacity-50'
              )}
              title={t('inputBar.actions.voice')}
            >
              <Mic className="h-4 w-4" />
            </button>
          )}
        </div>

        <div className="relative flex-1">
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
            className={cn(
              'w-full resize-none overflow-y-auto rounded-lg border border-input bg-background/80 px-4 py-3',
              'text-sm text-foreground placeholder:text-muted-foreground',
              'transition focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring',
              'disabled:cursor-not-allowed disabled:opacity-60',
              'max-h-32'
            )}
            style={{ fieldSizing: 'content' } as any}
          />
        </div>

        <button
          type="submit"
          disabled={disabled || loading || !text.trim()}
          className={cn(
            'console-icon-button console-icon-button-primary h-10 w-10 !rounded-lg',
            'flex-shrink-0 self-start'
          )}
          title={loading ? t('inputBar.actions.sending') : t('inputBar.actions.send')}
        >
          {loading ? (
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
          ) : (
            <Send className="h-4 w-4" />
          )}
        </button>
      </form>

      <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
        <span>{t('inputBar.hint.shortcut')}</span>
        {text.length > 0 && (
          <span className="font-mono">
            {text.length} / 4000
          </span>
        )}
      </div>
    </div>
  );
}
