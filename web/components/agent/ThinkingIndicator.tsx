'use client';

import { Brain, Sparkles } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

export function ThinkingIndicator() {
  const t = useTranslation();

  return (
    <>
      <div className="workflow.node.output.delta-indicator" data-testid="workflow.node.output.delta-event">
        <span className="workflow.node.output.delta-icon" aria-hidden>
          <Brain className="h-4 w-4" />
        </span>
        <div className="workflow.node.output.delta-copy">
          <span className="workflow.node.output.delta-title">{t('events.workflow.node.output.delta.title')}</span>
          <span className="workflow.node.output.delta-hint">{t('events.workflow.node.output.delta.hint')}</span>
        </div>
        <span className="workflow.node.output.delta-status" aria-live="polite">
          <Sparkles className="h-3.5 w-3.5" />
          <span>{t('events.workflow.node.output.delta.status')}</span>
        </span>
      </div>
      <style jsx>{`
        .workflow.node.output.delta-indicator {
          position: relative;
          display: flex;
          align-items: center;
          gap: 1rem;
          border-radius: 1.5rem;
          border: 1px solid hsl(222, 10%, 80%);
          background: radial-gradient(circle at top left, rgba(99, 102, 241, 0.08), transparent 50%);
          padding: 0.75rem 1rem;
        }
        .workflow.node.output.delta-icon {
          display: inline-flex;
          height: 2.5rem;
          width: 2.5rem;
          align-items: center;
          justify-content: center;
          border-radius: 9999px;
          border: 2px solid hsl(222, 13%, 75%);
          background: white;
          color: hsl(222, 40%, 40%);
        }
        .workflow.node.output.delta-copy {
          display: flex;
          flex-direction: column;
          gap: 0.15rem;
        }
        .workflow.node.output.delta-title {
          font-size: 0.95rem;
          font-weight: 600;
          text-transform: uppercase;
          letter-spacing: 0.18em;
          color: hsl(222, 15%, 20%);
        }
        .workflow.node.output.delta-hint {
          font-size: 0.65rem;
          text-transform: uppercase;
          letter-spacing: 0.24em;
          color: hsl(222, 10%, 50%);
        }
        .workflow.node.output.delta-status {
          margin-left: auto;
          position: relative;
          display: inline-flex;
          align-items: center;
          gap: 0.35rem;
          border-radius: 9999px;
          border: 1px solid hsl(260, 80%, 80%);
          background: linear-gradient(90deg, rgba(79, 70, 229, 0.15), rgba(59, 130, 246, 0.2));
          padding: 0.35rem 0.9rem;
          font-size: 0.75rem;
          font-weight: 600;
          color: hsl(222, 45%, 35%);
          overflow: hidden;
        }
        .workflow.node.output.delta-status span,
        .workflow.node.output.delta-status svg {
          position: relative;
          z-index: 1;
        }
        .workflow.node.output.delta-status::after {
          content: '';
          position: absolute;
          inset: 0;
          background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.9), transparent);
          transform: translateX(-100%);
          animation: shimmer 1.4s linear infinite;
        }
        @keyframes shimmer {
          100% {
            transform: translateX(100%);
          }
        }
        @media (prefers-reduced-motion: reduce) {
          .workflow.node.output.delta-status::after {
            animation: none;
          }
        }
      `}</style>
    </>
  );
}
