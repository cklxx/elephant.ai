'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Check, X, Edit3, ChevronDown, ChevronUp, Lightbulb } from 'lucide-react';
import { cn } from '@/lib/utils';

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
  onCancel?: () => void;
  readonly?: boolean;
}

export function ResearchPlanCard({
  plan,
  loading = false,
  onApprove,
  onModify,
  onCancel,
  readonly = false,
}: ResearchPlanCardProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [editedPlan, setEditedPlan] = useState<ResearchPlan | null>(plan);

  if (loading) {
    return <ResearchPlanSkeleton />;
  }

  if (!plan && !editedPlan) {
    return null;
  }

  const currentPlan = editedPlan || plan!;

  const handleSaveEdit = () => {
    if (editedPlan && onModify) {
      onModify(editedPlan);
      setIsEditing(false);
    }
  };

  const handleCancelEdit = () => {
    setEditedPlan(plan);
    setIsEditing(false);
  };

  return (
    <Card className="border-l-4 border-blue-500 bg-gradient-to-br from-blue-50/50 via-white to-transparent backdrop-blur-sm shadow-medium animate-slideIn overflow-hidden">
      <div className="absolute top-0 right-0 w-32 h-32 bg-blue-100/20 rounded-full blur-3xl"></div>

      <CardHeader className="pb-3 relative">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-gradient-to-br from-blue-500 to-blue-600 rounded-xl shadow-lg">
              <Lightbulb className="h-6 w-6 text-white" />
            </div>
            <div>
              <h3 className="font-semibold text-lg text-blue-900">
                Research Plan
              </h3>
              <p className="text-sm text-blue-700 font-medium">
                {readonly ? 'Approved Plan' : 'Review and approve to start execution'}
              </p>
            </div>
          </div>

          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="text-gray-400 hover:text-gray-700 transition-all duration-300 hover:scale-110 p-1 rounded-lg hover:bg-gray-100"
            aria-label={isExpanded ? 'Collapse plan' : 'Expand plan'}
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
            <p className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-blue-500 rounded-full"></span>
              Goal:
            </p>
            {isEditing ? (
              <textarea
                value={currentPlan.goal}
                onChange={(e) =>
                  setEditedPlan({ ...currentPlan, goal: e.target.value })
                }
                className="w-full bg-white/60 backdrop-blur-sm p-3 rounded-xl border border-blue-200/50 shadow-soft text-sm text-gray-900 leading-relaxed min-h-[80px] focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all"
                aria-label="Edit goal"
              />
            ) : (
              <div className="bg-white/60 backdrop-blur-sm p-4 rounded-xl border border-blue-200/50 shadow-soft">
                <p className="text-sm text-gray-900 leading-relaxed">{currentPlan.goal}</p>
              </div>
            )}
          </div>

          {/* Steps Section */}
          <div>
            <p className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
              <span className="w-1.5 h-1.5 bg-purple-500 rounded-full"></span>
              Planned Steps ({currentPlan.steps.length}):
            </p>
            <div className="bg-white/60 backdrop-blur-sm p-4 rounded-xl border border-blue-200/50 shadow-soft space-y-2">
              {currentPlan.steps.map((step, idx) => (
                <div key={idx} className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-purple-100 text-purple-700 flex items-center justify-center text-xs font-semibold">
                    {idx + 1}
                  </div>
                  {isEditing ? (
                    <input
                      type="text"
                      value={step}
                      onChange={(e) => {
                        const newSteps = [...currentPlan.steps];
                        newSteps[idx] = e.target.value;
                        setEditedPlan({ ...currentPlan, steps: newSteps });
                      }}
                      className="flex-1 bg-white border border-gray-200 rounded-lg px-3 py-1.5 text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                      aria-label={`Edit step ${idx + 1}`}
                    />
                  ) : (
                    <p className="flex-1 text-sm text-gray-900 leading-relaxed">{step}</p>
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Metadata Section */}
          <div className="flex items-center gap-3 flex-wrap">
            <Badge variant="info" className="flex items-center gap-1">
              <span className="text-xs">Iterations:</span>
              <span className="font-semibold">{currentPlan.estimated_iterations}</span>
            </Badge>
            <Badge variant="default" className="flex items-center gap-1">
              <span className="text-xs">Tools:</span>
              <span className="font-semibold">{currentPlan.estimated_tools.length}</span>
            </Badge>
            <div className="flex items-center gap-1 flex-wrap">
              {currentPlan.estimated_tools.slice(0, 5).map((tool, idx) => (
                <span
                  key={idx}
                  className="text-xs px-2 py-1 bg-gray-100 text-gray-700 rounded border border-gray-200"
                >
                  {tool}
                </span>
              ))}
              {currentPlan.estimated_tools.length > 5 && (
                <span className="text-xs px-2 py-1 bg-gray-100 text-gray-500 rounded border border-gray-200">
                  +{currentPlan.estimated_tools.length - 5} more
                </span>
              )}
            </div>
          </div>

          {/* Action Buttons */}
          {!readonly && (
            <div className="pt-4 border-t border-blue-100">
              {isEditing ? (
                <div className="flex items-center gap-3">
                  <Button
                    onClick={handleSaveEdit}
                    className="flex-1 bg-blue-600 hover:bg-blue-700 text-white"
                  >
                    <Check className="h-4 w-4 mr-2" />
                    Save Changes
                  </Button>
                  <Button
                    onClick={handleCancelEdit}
                    variant="outline"
                    className="flex-1"
                  >
                    <X className="h-4 w-4 mr-2" />
                    Cancel
                  </Button>
                </div>
              ) : (
                <div className="flex items-center gap-3">
                  <Button
                    onClick={onApprove}
                    className="flex-1 bg-green-600 hover:bg-green-700 text-white"
                  >
                    <Check className="h-4 w-4 mr-2" />
                    Approve & Start
                  </Button>
                  <Button
                    onClick={() => setIsEditing(true)}
                    variant="outline"
                    className="flex-1"
                  >
                    <Edit3 className="h-4 w-4 mr-2" />
                    Modify Plan
                  </Button>
                  <Button
                    onClick={onCancel}
                    variant="outline"
                    className="bg-red-50 hover:bg-red-100 text-red-700 border-red-200"
                  >
                    <X className="h-4 w-4 mr-2" />
                    Cancel
                  </Button>
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
    <Card className="border-l-4 border-blue-500 bg-gradient-to-br from-blue-50/50 via-white to-transparent backdrop-blur-sm shadow-medium animate-slideIn overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <Skeleton className="h-12 w-12 rounded-xl" />
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
