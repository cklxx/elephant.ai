'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { TaskAnalysisEvent } from '@/lib/types';
import { Target } from 'lucide-react';

interface TaskAnalysisCardProps {
  event: TaskAnalysisEvent;
}

export function TaskAnalysisCard({ event }: TaskAnalysisCardProps) {
  return (
    <Card className="border-l-4 border-purple-500 bg-gradient-to-br from-purple-50/80 via-white to-transparent backdrop-blur-sm shadow-medium hover-lift animate-slideIn overflow-hidden">
      <div className="absolute top-0 right-0 w-32 h-32 bg-purple-100/30 rounded-full blur-3xl"></div>
      <CardHeader className="pb-3 relative">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-gradient-to-br from-purple-500 to-purple-600 rounded-xl shadow-lg hover-glow">
            <Target className="h-6 w-6 text-white" />
          </div>
          <div>
            <h3 className="font-semibold text-lg text-purple-900">
              Task Analysis
            </h3>
            <p className="text-sm text-purple-700 font-medium">
              {event.action_name}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="relative">
        <div className="bg-white/60 backdrop-blur-sm p-4 rounded-xl border border-purple-200/50 shadow-soft">
          <p className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
            <span className="w-1.5 h-1.5 bg-purple-500 rounded-full"></span>
            Goal:
          </p>
          <p className="text-sm text-gray-900 leading-relaxed">{event.goal}</p>
        </div>
      </CardContent>
    </Card>
  );
}
