'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { TaskAnalysisEvent } from '@/lib/types';
import { Target } from 'lucide-react';

interface TaskAnalysisCardProps {
  event: TaskAnalysisEvent;
}

export function TaskAnalysisCard({ event }: TaskAnalysisCardProps) {
  return (
    <Card className="border-l-4 border-purple-400 bg-gradient-to-r from-purple-50 to-transparent">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-purple-100 rounded-lg">
            <Target className="h-6 w-6 text-purple-600" />
          </div>
          <div>
            <h3 className="font-semibold text-lg text-purple-900">
              Task Analysis
            </h3>
            <p className="text-sm text-purple-700">
              {event.action_name}
            </p>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="bg-white p-4 rounded-md border border-purple-100">
          <p className="text-sm font-medium text-gray-700 mb-2">Goal:</p>
          <p className="text-sm text-gray-900">{event.goal}</p>
        </div>
      </CardContent>
    </Card>
  );
}
