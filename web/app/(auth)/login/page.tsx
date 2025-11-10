'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { clearAuthToken, getAuthToken, setAuthToken } from '@/lib/auth';

export default function LoginPage() {
  const [tokenInput, setTokenInput] = useState('');
  const [currentToken, setCurrentToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  useEffect(() => {
    setCurrentToken(getAuthToken());
  }, []);

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = tokenInput.trim();
    if (!trimmed) {
      setError('请输入有效的访问令牌。');
      return;
    }
    setAuthToken(trimmed);
    setCurrentToken(trimmed);
    setTokenInput('');
    setError(null);
    router.push('/sessions');
  };

  const handleLogout = () => {
    clearAuthToken();
    setCurrentToken('');
    setTokenInput('');
    setError(null);
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 p-6">
      <Card className="w-full max-w-md space-y-6 border-none bg-white p-8 shadow-xl">
        <div className="space-y-2 text-center">
          <h1 className="text-2xl font-semibold text-slate-900">登录 ALEX</h1>
          <p className="text-sm text-slate-500">
            输入平台发放的访问令牌以解锁多租户会话与 Crafts 管理能力。
          </p>
        </div>

        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2 text-left">
            <label htmlFor="token" className="text-sm font-medium text-slate-700">
              访问令牌
            </label>
            <input
              id="token"
              name="token"
              type="text"
              value={tokenInput}
              onChange={(event) => setTokenInput(event.target.value)}
              className="w-full rounded-xl border border-slate-200 px-4 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200"
              placeholder="Bearer Token"
              autoComplete="off"
            />
            {error && <p className="text-xs text-red-500">{error}</p>}
          </div>

          <Button type="submit" className="w-full rounded-xl bg-sky-500 text-sm font-semibold text-white hover:bg-sky-600">
            保存并继续
          </Button>
        </form>

        <div className="space-y-2 rounded-xl bg-slate-50 p-4 text-sm text-slate-600">
          <p className="font-medium text-slate-700">当前令牌</p>
          {currentToken ? (
            <code className="block truncate rounded-lg bg-slate-900/90 px-3 py-2 text-xs text-sky-100">
              {currentToken}
            </code>
          ) : (
            <p>尚未设置。</p>
          )}
          {currentToken && (
            <Button variant="outline" className="mt-3 w-full" onClick={handleLogout}>
              清除令牌
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
}
