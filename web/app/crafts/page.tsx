'use client';

import { useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { formatDistanceToNow } from 'date-fns';
import { zhCN } from 'date-fns/locale';

export default function CraftsPage() {
  const queryClient = useQueryClient();
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['crafts'],
    queryFn: apiClient.listCrafts,
  });

  const deleteMutation = useMutation({
    mutationFn: (craftId: string) => apiClient.deleteCraft(craftId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['crafts'] });
    },
  });

  const handleDownload = useCallback(async (craftId: string) => {
    try {
      const response = await apiClient.getCraftDownloadUrl(craftId);
      if (response.url) {
        window.open(response.url, '_blank', 'noopener');
      }
    } catch (err) {
      console.error('Failed to fetch download url', err);
    }
  }, []);

  const crafts = data?.crafts ?? [];

  return (
    <div className="console-shell">
      <div className="space-y-6">
        <section className="console-panel p-8">
          <div className="flex flex-col gap-6">
            <header className="flex flex-col gap-2">
              <p className="console-pane-title">Crafts</p>
              <h1 className="text-3xl font-semibold text-slate-900">产物仓库</h1>
              <p className="text-sm text-slate-500">
                查看并下载会话生成的文件资源，支持批量治理与安全下载。
              </p>
            </header>

            {isLoading ? (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {Array.from({ length: 6 }).map((_, index) => (
                  <Skeleton key={index} className="h-32 rounded-2xl" />
                ))}
              </div>
            ) : isError ? (
              <Card className="rounded-2xl border border-red-100 bg-red-50/70 p-6 text-sm text-red-600">
                {(error as Error)?.message ?? '无法加载 Crafts 列表'}
              </Card>
            ) : crafts.length === 0 ? (
              <Card className="rounded-2xl border border-slate-100 bg-slate-50 p-8 text-center text-sm text-slate-500">
                暂无产物。提交任务后生成的文件会显示在此处。
              </Card>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {crafts.map((craft) => (
                  <Card key={craft.id} className="flex h-full flex-col justify-between rounded-2xl border border-slate-100 bg-white p-6 shadow-sm">
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <h2 className="text-lg font-semibold text-slate-900">{craft.name}</h2>
                        <Badge>{craft.media_type}</Badge>
                      </div>
                      <p className="text-sm text-slate-500">
                        会话 <span className="font-medium text-slate-700">{craft.session_id}</span>
                      </p>
                      <p className="text-xs text-slate-400">
                        创建于 {formatDistanceToNow(new Date(craft.created_at), { addSuffix: true, locale: zhCN })}
                      </p>
                      {craft.description && (
                        <p className="text-sm text-slate-600 line-clamp-3">{craft.description}</p>
                      )}
                    </div>
                    <div className="mt-4 flex flex-col gap-2 sm:flex-row">
                      <Button
                        className="flex-1 rounded-xl bg-sky-500 text-sm font-semibold text-white hover:bg-sky-600"
                        onClick={() => handleDownload(craft.id)}
                      >
                        下载
                      </Button>
                      <Button
                        variant="outline"
                        className="flex-1 rounded-xl text-sm"
                        onClick={() => deleteMutation.mutate(craft.id)}
                        disabled={deleteMutation.isLoading}
                      >
                        删除
                      </Button>
                    </div>
                  </Card>
                ))}
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
