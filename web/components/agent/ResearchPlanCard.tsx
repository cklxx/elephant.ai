'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Check,
  X,
  Edit3,
  ChevronDown,
  ChevronUp,
  Lightbulb,
  ArrowUp,
  ArrowDown,
  Ban,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';

export interface ResearchPlan {
  goal: string;
  steps: string[];
  estimated_tools: string[];
  estimated_iterations: number;
}

interface ResearchPlanCardProps {
  plan: ResearchPlan | null;
  loading?: boolean;
  onApprove?: () => void;
  onModify?: (updatedPlan: ResearchPlan) => void;
  onReject?: (reason: string) => void;
  readonly?: boolean;
}

export function ResearchPlanCard({
  plan,
  loading = false,
  onApprove,
  onModify,
  onReject,
  readonly = false,
}: ResearchPlanCardProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [draftPlan, setDraftPlan] = useState<ResearchPlan | null>(null);
  const [isRejecting, setIsRejecting] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const t = useTranslation();

  if (loading) {
    return <ResearchPlanSkeleton />;
  }

  const displayedPlan = isEditing ? draftPlan : plan;

  if (!displayedPlan) {
    return null;
  }

  const handleSaveEdit = () => {
    if (!draftPlan || !onModify) return;

    onModify(draftPlan);
    setIsEditing(false);
    setDraftPlan(null);
  };

  const handleCancelEdit = () => {
    setDraftPlan(plan ? { ...plan, steps: [...plan.steps], estimated_tools: [...plan.estimated_tools] } : null);
    setIsEditing(false);
  };

  const handleStartEditing = () => {
    if (!plan) return;

    setDraftPlan({
      ...plan,
      steps: [...plan.steps],
      estimated_tools: [...plan.estimated_tools],
    });
    setIsEditing(true);
    setIsRejecting(false);
  };

  const handleRejectConfirm = () => {
    if (!onReject) {
      setIsRejecting(false);
      setRejectReason('');
      return;
    }

    onReject(rejectReason.trim());
    setIsRejecting(false);
    setRejectReason('');
  };

  const handleRejectCancel = () => {
    setRejectReason('');
    setIsRejecting(false);
  };

  const planForEditing = isEditing && draftPlan ? draftPlan : displayedPlan;

  const updateDraftPlan = (updater: (current: ResearchPlan) => ResearchPlan) => {
    setDraftPlan((current) => {
      const source = current ?? (plan ? { ...plan, steps: [...plan.steps], estimated_tools: [...plan.estimated_tools] } : null);
      if (!source) return current;
      return updater(source);
    });
  };

  const moveStep = (from: number, to: number) => {
    if (!draftPlan) return;
    if (to < 0 || to >= draftPlan.steps.length) return;

    const nextSteps = [...draftPlan.steps];
    const [moved] = nextSteps.splice(from, 1);
    nextSteps.splice(to, 0, moved);

    setDraftPlan({ ...draftPlan, steps: nextSteps });
  };

  const canSave = Boolean(
    isEditing &&
      draftPlan &&
      draftPlan.goal.trim().length > 0 &&
      draftPlan.steps.every((step) => step.trim().length > 0)
  );

  return (
    <Card className="manus-card border-l-4 border-primary animate-fadeIn overflow-hidden">

      <CardHeader className="pb-3 relative">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-primary rounded-md">
              <Lightbulb className="h-6 w-6 text-primary-foreground" />
            </div>
            <div>
              <h3 className="manus-heading text-lg">{t('plan.title')}</h3>
              <p className="manus-caption">
                {readonly ? t('plan.caption.readonly') : t('plan.caption.default')}
              </p>
            </div>
          </div>

          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="text-muted-foreground hover:text-foreground hover-subtle p-1 rounded-md"
            aria-label={isExpanded ? t('plan.collapse') : t('plan.expand')}
          >
            {isExpanded ? (
              <ChevronUp className="h-5 w-5" />
            ) : (
              <ChevronDown className="h-5 w-5" />
            )}
          </button>
        </div>
      </CardHeader>

      {isExpanded && (
        <CardContent className="space-y-4 animate-fadeIn relative">
          {/* Goal Section */}
          <div>
            <p className="manus-subheading text-sm mb-2 flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-primary rounded-full"></span>
              {t('plan.goal.label')}
            </p>
            {isEditing && planForEditing ? (
              <textarea
                value={planForEditing.goal}
                onChange={(e) =>
                  updateDraftPlan((current) => ({
                    ...current,
                    goal: e.target.value,
                  }))
                }
                className="manus-input text-sm leading-relaxed min-h-[80px]"
                aria-label={t('plan.edit.goal')}
              />
            ) : (
              <div className="manus-card p-4">
                <p className="manus-body text-sm">{displayedPlan.goal}</p>
              </div>
            )}
          </div>

          {/* Steps Section */}
          <div>
            <p className="manus-subheading text-sm mb-2 flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-primary rounded-full"></span>
              {t('plan.steps.label', { count: displayedPlan.steps.length })}
            </p>
            <div className="manus-card p-4 space-y-2">
              {displayedPlan.steps.map((step, idx) => (
                <div key={idx} className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-secondary text-secondary-foreground flex items-center justify-center text-xs font-semibold">
                    {idx + 1}
                  </div>
                  {isEditing && planForEditing ? (
                    <input
                      type="text"
                      value={planForEditing.steps[idx]}
                    onChange={(e) => {
                      const value = e.target.value;
                      updateDraftPlan((current) => {
                        const updatedSteps = [...current.steps];
                        updatedSteps[idx] = value;
                        return { ...current, steps: updatedSteps };
                      });
                    }}
                    className="flex-1 manus-input px-3 py-1.5 text-sm"
                    aria-label={t('plan.edit.stepLabel', { index: idx + 1 })}
                  />
                ) : (
                  <p className="flex-1 manus-body text-sm">{step}</p>
                )}

                {isEditing && displayedPlan.steps.length > 1 && (
                  <div className="flex flex-col gap-1">
                    <button
                      type="button"
                      onClick={() => moveStep(idx, idx - 1)}
                      className="rounded-md border border-border/60 bg-background/60 p-1 text-muted-foreground transition hover:bg-muted disabled:opacity-40"
                      aria-label={t('plan.move.up', { index: idx + 1 })}
                      disabled={idx === 0}
                    >
                      <ArrowUp className="h-3.5 w-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => moveStep(idx, idx + 1)}
                      className="rounded-md border border-border/60 bg-background/60 p-1 text-muted-foreground transition hover:bg-muted disabled:opacity-40"
                      aria-label={t('plan.move.down', { index: idx + 1 })}
                      disabled={idx === displayedPlan.steps.length - 1}
                    >
                      <ArrowDown className="h-3.5 w-3.5" />
                    </button>
                  </div>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Metadata Section */}
          <div className="flex items-center gap-3 flex-wrap">
            <Badge variant="info" className="flex items-center gap-1">
              <span className="text-xs">{t('plan.iterations')}</span>
              <span className="font-semibold">{displayedPlan.estimated_iterations}</span>
            </Badge>
            <Badge variant="default" className="flex items-center gap-1">
              <span className="text-xs">{t('plan.tools')}</span>
              <span className="font-semibold">{displayedPlan.estimated_tools.length}</span>
            </Badge>
            <div className="flex items-center gap-1 flex-wrap">
              {displayedPlan.estimated_tools.slice(0, 5).map((tool, idx) => (
                <span
                  key={idx}
                  className="text-xs px-2 py-1 bg-gray-100 text-gray-700 rounded border border-gray-200"
                >
                  {tool}
                </span>
              ))}
              {displayedPlan.estimated_tools.length > 5 && (
                <span className="text-xs px-2 py-1 bg-gray-100 text-gray-500 rounded border border-gray-200">
                  {t('plan.tools.more', { count: displayedPlan.estimated_tools.length - 5 })}
                </span>
              )}
            </div>
          </div>

          {/* Action Buttons */}
          {!readonly && (
            <div className="space-y-4 border-t border-border pt-4">
              {isEditing ? (
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    onClick={handleSaveEdit}
                    className="flex-1 manus-button-primary"
                    disabled={!canSave}
                  >
                    <Check className="h-4 w-4 mr-2" />
                    {t('plan.actions.saveChanges')}
                  </Button>
                  <Button
                    onClick={handleCancelEdit}
                    variant="outline"
                    className="flex-1 manus-button-secondary"
                  >
                    <X className="h-4 w-4 mr-2" />
                    {t('plan.actions.cancel')}
                  </Button>
                </div>
              ) : (
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    onClick={onApprove}
                    className="flex-1 manus-button-primary"
                    disabled={loading}
                  >
                    <Check className="h-4 w-4 mr-2" />
                    {t('plan.actions.approve')}
                  </Button>
                  <Button
                    onClick={handleStartEditing}
                    variant="outline"
                    className="flex-1 manus-button-secondary"
                  >
                    <Edit3 className="h-4 w-4 mr-2" />
                    {t('plan.actions.modify')}
                  </Button>
                  <Button
                    onClick={() => {
                      setRejectReason('');
                      setIsRejecting(true);
                    }}
                    variant="outline"
                    className="flex-1 border-destructive/40 text-destructive hover:border-destructive hover:text-destructive"
                  >
                    <Ban className="h-4 w-4 mr-2" />
                    {t('plan.actions.reject')}
                  </Button>
                </div>
              )}

              {isRejecting && (
                <div className="space-y-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4">
                  <div className="space-y-2">
                    <p className="text-xs font-semibold uppercase tracking-wide text-destructive">
                      {t('plan.reject.reasonLabel')}
                    </p>
                    <textarea
                      value={rejectReason}
                      onChange={(event) => setRejectReason(event.target.value)}
                      className="manus-input min-h-[80px] text-sm"
                      placeholder={t('plan.reject.placeholder')}
                    />
                  </div>
                  <div className="flex flex-wrap items-center gap-3">
                    <Button
                      onClick={handleRejectConfirm}
                      className="flex-1 manus-button-secondary"
                      disabled={!rejectReason.trim()}
                    >
                      <Ban className="h-4 w-4 mr-2" />
                      {t('plan.reject.confirm')}
                    </Button>
                    <Button
                      onClick={handleRejectCancel}
                      variant="outline"
                      className="flex-1"
                    >
                      <X className="h-4 w-4 mr-2" />
                      {t('plan.reject.cancel')}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}

function ResearchPlanSkeleton() {
  return (
    <Card className="manus-card border-l-4 border-primary animate-fadeIn overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <Skeleton className="h-12 w-12 rounded-md" />
          <div className="space-y-2 flex-1">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-4 w-64" />
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <Skeleton className="h-4 w-16 mb-2" />
          <Skeleton className="h-20 w-full" />
        </div>
        <div>
          <Skeleton className="h-4 w-32 mb-2" />
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        </div>
        <div className="flex gap-2">
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-6 w-24" />
          <Skeleton className="h-6 w-24" />
        </div>
      </CardContent>
    </Card>
  );
}
