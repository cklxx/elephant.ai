'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { TaskAnalysisEvent } from '@/lib/types';
import { Target } from 'lucide-react';

interface TaskAnalysisCardProps {
  event: TaskAnalysisEvent;
}

export function TaskAnalysisCard({ event }: TaskAnalysisCardProps) {
  return (
    <Card className="manus-card border-l-4 border-primary animate-fadeIn overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-primary rounded-md">
            <Target className="h-6 w-6 text-primary-foreground" />
          </div>
          <div>
            <h3 className="manus-heading text-lg">
              Task Analysis
            </h3>
            <p className="manus-caption">
              {event.action_name}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="manus-card p-4">
          <p className="manus-subheading text-sm mb-2 flex items-center gap-2">
            <span className="w-1.5 h-1.5 bg-primary rounded-full"></span>
            Goal:
          </p>
          <p className="manus-body text-sm">{event.goal}</p>
        </div>
      </CardContent>
    </Card>
  );
}
