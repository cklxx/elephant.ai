'use client';

import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { ReactNode } from 'react';
import clsx from 'clsx';

export interface WorkTypeCardProps {
  href: string;
  title: string;
  description: string;
  icon: ReactNode;
  highlights?: string[];
}

export function WorkTypeCard({ href, title, description, icon, highlights }: WorkTypeCardProps) {
  return (
    <Link
      href={href}
      className={clsx(
        'group relative flex h-full flex-col justify-between rounded-2xl border border-slate-800/80 bg-slate-900/60',
        'p-6 transition-all duration-200 hover:-translate-y-1 hover:border-cyan-400/80 hover:bg-slate-900'
      )}
    >
      <div>
        <div className="flex items-center gap-4 text-cyan-300">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl border border-cyan-500/30 bg-cyan-500/10">
            {icon}
          </div>
          <div>
            <h3 className="text-xl font-semibold text-slate-100">{title}</h3>
            <p className="text-sm text-slate-400">{description}</p>
          </div>
        </div>
        {highlights && highlights.length > 0 && (
          <ul className="mt-4 space-y-2 text-sm text-slate-300">
            {highlights.map((item) => (
              <li key={item} className="flex items-start gap-2">
                <span className="mt-1 h-1.5 w-1.5 rounded-full bg-cyan-400" aria-hidden />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="mt-6 flex items-center justify-between text-sm text-cyan-200">
        <span>立即进入</span>
        <ArrowRight className="h-5 w-5 transition-transform group-hover:translate-x-1" />
      </div>
    </Link>
  );
}
