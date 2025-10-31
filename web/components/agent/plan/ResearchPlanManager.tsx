'use client';

import { useState } from 'react';
import {
  DndContext,
  DragEndEvent,
  PointerSensor,
  KeyboardSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import {
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
  arrayMove,
  sortableKeyboardCoordinates,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
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
  GripVertical,
  Ban,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTranslation } from '@/lib/i18n';
import type { LucideIcon } from 'lucide-react';
import { PlanProgressMetrics } from '@/hooks/usePlanProgress';
import { PlanProgressSummary } from './PlanProgressSummary';
import { PlanMetadata } from './PlanMetadata';
import { ResearchPlan } from './types';

type TranslateFn = ReturnType<typeof useTranslation>;

interface ResearchPlanManagerProps {
  plan: ResearchPlan | null;
  loading?: boolean;
  onApprove?: () => void;
  onModify?: (updatedPlan: ResearchPlan) => void;
  onReject?: (reason: string) => void;
  readonly?: boolean;
  progress?: PlanProgressMetrics | null;
}

export function ResearchPlanManager({
  plan,
  loading = false,
  onApprove,
  onModify,
  onReject,
  readonly = false,
  progress = null,
}: ResearchPlanManagerProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [draftPlan, setDraftPlan] = useState<ResearchPlan | null>(null);
  const [isRejecting, setIsRejecting] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const t = useTranslation();
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 6 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

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
    setDraftPlan((current) => {
      if (!current) return current;
      if (to < 0 || to >= current.steps.length) return current;

      const nextSteps = [...current.steps];
      const [moved] = nextSteps.splice(from, 1);
      nextSteps.splice(to, 0, moved);

      return { ...current, steps: nextSteps };
    });
  };

  const handleStepDragEnd = (event: DragEndEvent) => {
    if (!isEditing) return;

    const { active, over } = event;
    if (!over || active.id === over.id) {
      return;
    }

    const parseIndex = (identifier: typeof active.id) => {
      const value = Number(String(identifier).replace('step-', ''));
      return Number.isNaN(value) ? null : value;
    };

    const fromIndex = parseIndex(active.id);
    const toIndex = parseIndex(over.id);

    if (fromIndex === null || toIndex === null) {
      return;
    }

    setDraftPlan((current) => {
      if (!current) return current;
      if (fromIndex === toIndex) return current;

      const steps = arrayMove(current.steps, fromIndex, toIndex);
      return { ...current, steps };
    });
  };

  const canSave = Boolean(
    isEditing &&
      draftPlan &&
      draftPlan.goal.trim().length > 0 &&
      draftPlan.steps.every((step) => step.trim().length > 0)
  );

  return (
    <Card className="console-card border-l-4 border-primary animate-fadeIn overflow-hidden">

      <CardHeader className="pb-3 relative">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-primary rounded-md">
              <Lightbulb className="h-6 w-6 text-primary-foreground" />
            </div>
            <div>
              <h3 className="console-heading text-lg">{t('plan.title')}</h3>
              <p className="console-caption">
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
          {progress && <PlanProgressSummary progress={progress} />}
          {/* Goal Section */}
          <div>
            <p className="console-subheading text-sm mb-2 flex items-center gap-2">
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
                className="console-input text-sm leading-relaxed min-h-[80px]"
                aria-label={t('plan.edit.goal')}
              />
            ) : (
              <div className="console-card p-4">
                <p className="console-body text-sm">{displayedPlan.goal}</p>
              </div>
            )}
          </div>

          {/* Steps Section */}
          <div>
            <p className="console-subheading text-sm mb-2 flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-primary rounded-full"></span>
              {t('plan.steps.label', { count: displayedPlan.steps.length })}
            </p>
            <div className="console-card p-4 space-y-2">
              {isEditing && planForEditing ? (
                <DndContext
                  sensors={sensors}
                  collisionDetection={closestCenter}
                  onDragEnd={handleStepDragEnd}
                >
                  <SortableContext
                    items={planForEditing.steps.map((_, idx) => `step-${idx}`)}
                    strategy={verticalListSortingStrategy}
                  >
                    {planForEditing.steps.map((step, idx) => (
                      <SortablePlanStep
                        key={`step-${idx}`}
                        id={`step-${idx}`}
                        index={idx}
                        value={step}
                        onChange={(value) =>
                          updateDraftPlan((current) => {
                            const updatedSteps = [...current.steps];
                            updatedSteps[idx] = value;
                            return { ...current, steps: updatedSteps };
                          })
                        }
                        onMoveUp={() => moveStep(idx, idx - 1)}
                        onMoveDown={() => moveStep(idx, idx + 1)}
                        canMoveUp={idx > 0}
                        canMoveDown={idx < planForEditing.steps.length - 1}
                        t={t}
                      />
                    ))}
                  </SortableContext>
                </DndContext>
              ) : (
                displayedPlan.steps.map((step, idx) => (
                  <div key={idx} className="flex items-start gap-3">
                    <div className="flex-shrink-0 w-6 h-6 rounded-full bg-secondary text-secondary-foreground flex items-center justify-center text-xs font-semibold">
                      {idx + 1}
                    </div>
                    <p className="flex-1 console-body text-sm">{step}</p>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Metadata Section */}
          <PlanMetadata plan={displayedPlan} />

          {/* Action Buttons */}
          {!readonly && (
            <div className="space-y-4 border-t border-border pt-4">
              {isEditing ? (
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    onClick={handleSaveEdit}
                    className="flex-1 console-button console-button-primary"
                    disabled={!canSave}
                  >
                    <Check className="h-4 w-4 mr-2" />
                    {t('plan.actions.saveChanges')}
                  </Button>
                  <Button
                    onClick={handleCancelEdit}
                    variant="outline"
                    className="flex-1 console-button console-button-secondary"
                  >
                    <X className="h-4 w-4 mr-2" />
                    {t('plan.actions.cancel')}
                  </Button>
                </div>
              ) : (
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    onClick={onApprove}
                    className="flex-1 console-button console-button-primary"
                    disabled={loading}
                  >
                    <Check className="h-4 w-4 mr-2" />
                    {t('plan.actions.approve')}
                  </Button>
                  <Button
                    onClick={handleStartEditing}
                    variant="outline"
                    className="flex-1 console-button console-button-secondary"
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
                      className="console-input min-h-[80px] text-sm"
                      placeholder={t('plan.reject.placeholder')}
                    />
                  </div>
                  <div className="flex flex-wrap items-center gap-3">
                    <Button
                      onClick={handleRejectConfirm}
                      className="flex-1 console-button console-button-secondary"
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
    <Card className="console-card border-l-4 border-primary animate-fadeIn overflow-hidden">
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

function SortablePlanStep({
  id,
  index,
  value,
  onChange,
  onMoveUp,
  onMoveDown,
  canMoveUp,
  canMoveDown,
  t,
}: {
  id: string;
  index: number;
  value: string;
  onChange: (value: string) => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  canMoveUp: boolean;
  canMoveDown: boolean;
  t: TranslateFn;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        'flex items-start gap-3 rounded-md border border-transparent bg-background/80 p-2',
        isDragging && 'border-primary/40 bg-primary/5 shadow-sm'
      )}
    >
      <div className="flex-shrink-0 w-6 h-6 rounded-full bg-secondary text-secondary-foreground flex items-center justify-center text-xs font-semibold mt-1">
        {index + 1}
      </div>
      <div className="flex flex-1 items-start gap-2">
        <button
          type="button"
          className="mt-1 h-7 w-7 flex items-center justify-center rounded-md border border-border/60 bg-background text-muted-foreground transition hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-primary"
          aria-label={t('plan.move.dragHandle', { index: index + 1 })}
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-3.5 w-3.5" />
        </button>
        <input
          type="text"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="flex-1 console-input px-3 py-1.5 text-sm"
          aria-label={t('plan.edit.stepLabel', { index: index + 1 })}
        />
      </div>
      <div className="flex flex-col gap-1">
        <button
          type="button"
          onClick={onMoveUp}
          className="rounded-md border border-border/60 bg-background/60 p-1 text-muted-foreground transition hover:bg-muted disabled:opacity-40"
          aria-label={t('plan.move.up', { index: index + 1 })}
          disabled={!canMoveUp}
        >
          <ArrowUp className="h-3.5 w-3.5" />
        </button>
        <button
          type="button"
          onClick={onMoveDown}
          className="rounded-md border border-border/60 bg-background/60 p-1 text-muted-foreground transition hover:bg-muted disabled:opacity-40"
          aria-label={t('plan.move.down', { index: index + 1 })}
          disabled={!canMoveDown}
        >
          <ArrowDown className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}
