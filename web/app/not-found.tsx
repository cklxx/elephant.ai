'use client';

import { useEffect } from 'react';

function computeRedirectUrl(): string {
  if (typeof window === 'undefined') {
    return '/';
  }

  const currentUrl = new URL(window.location.href);
  const segments = currentUrl.pathname.split('/').filter(Boolean);
  const sessionsIndex = segments.indexOf('sessions');
  const baseSegments = sessionsIndex === -1 ? segments.slice(0, -1) : segments.slice(0, sessionsIndex);
  const basePath = baseSegments.length ? `/${baseSegments.join('/')}` : '';

  let targetPath = `${basePath}/`;
  if (sessionsIndex !== -1 && segments[sessionsIndex + 1]) {
    targetPath = `${basePath}/sessions`;
  }

  const targetUrl = new URL(targetPath || '/', currentUrl.origin);

  if (sessionsIndex !== -1 && segments[sessionsIndex + 1]) {
    targetUrl.searchParams.set('sessionId', segments[sessionsIndex + 1]);
  }

  currentUrl.searchParams.forEach((value, key) => {
    if (key !== 'sessionId') {
      targetUrl.searchParams.append(key, value);
    }
  });

  if (currentUrl.hash) {
    targetUrl.hash = currentUrl.hash;
  }

  return targetUrl.toString();
}

export default function NotFound() {
  useEffect(() => {
    const redirectUrl = computeRedirectUrl();
    window.location.replace(redirectUrl);
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50 text-gray-600">
      <div className="text-center space-y-4">
        <p className="text-lg font-semibold">Redirecting you to the correct page…</p>
        <p className="text-sm text-gray-500">If you are not redirected automatically, please use the navigation menu.</p>
      </div>
    </div>
  );
}
