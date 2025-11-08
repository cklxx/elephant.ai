'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';
import { LayoutDashboard, MessagesSquare } from 'lucide-react';

const NAV_ITEMS = [
  {
    href: '/workbench',
    label: '工作台',
    description: '集中管理文章、图片、网页与代码任务',
    icon: LayoutDashboard,
  },
  {
    href: '/conversation',
    label: '对话',
    description: '保持与 Agent 的实时沟通',
    icon: MessagesSquare,
  },
] as const;

function isActive(pathname: string | null, href: string) {
  if (!pathname) return false;
  if (href === '/') {
    return pathname === '/';
  }
  return pathname === href || pathname.startsWith(`${href}/`);
}

export function AppNav() {
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-50 border-b border-slate-800/70 bg-slate-950/85 backdrop-blur">
      <div className="mx-auto flex h-14 w-full max-w-6xl items-center justify-between px-6">
        <Link
          href="/workbench"
          className="text-sm font-semibold text-cyan-300"
        >
          Alex
        </Link>
        <nav className="flex items-center gap-1">
          {NAV_ITEMS.map((item) => {
            const Icon = item.icon;
            const active = isActive(pathname, item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  'group flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors',
                  active
                    ? 'bg-cyan-500/15 text-cyan-200'
                    : 'text-slate-400 hover:bg-slate-900/80 hover:text-slate-100'
                )}
                aria-current={active ? 'page' : undefined}
              >
                <Icon className="h-4 w-4" aria-hidden />
                <span>{item.label}</span>
              </Link>
            );
          })}
        </nav>
      </div>
    </header>
  );
}
