import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PlanMetadata } from '../PlanMetadata';
import { LanguageProvider } from '@/lib/i18n';
import type { ResearchPlan } from '../types';

describe('PlanMetadata', () => {
  const basePlan: ResearchPlan = {
    goal: 'Validate cloud export planning flow',
    steps: ['Collect baseline metrics'],
    estimated_tools: ['analysis'],
    estimated_iterations: 2,
    estimated_duration_minutes: 45,
  };

  it('renders cloud export details when provided', () => {
    const planWithExports: ResearchPlan = {
      ...basePlan,
      cloud_exports: [
        {
          provider: 's3',
          bucket: 'alex-data',
          path: 'research/demo',
          access: 'read_write',
          retention_days: 10,
          region: 'us-west-1',
          description: 'Primary export bucket',
        },
        {
          provider: 'gcs',
          bucket: 'alex-backup',
          path: 'research/demo-backup',
          access: 'read',
          retention_days: 0,
        },
      ],
    };

    render(
      <LanguageProvider>
        <PlanMetadata plan={planWithExports} />
      </LanguageProvider>
    );

    expect(screen.getByText('Cloud export targets')).toBeInTheDocument();
    expect(screen.getByText('2 cloud export(s)')).toBeInTheDocument();
    expect(screen.getByText('alex-data/research/demo')).toBeInTheDocument();
    expect(screen.getByText('Read & write')).toBeInTheDocument();
    expect(screen.getByText('Retention: 10 days')).toBeInTheDocument();
    expect(screen.getByText('No automatic retention')).toBeInTheDocument();
  });

  it('omits cloud export section when none are configured', () => {
    render(
      <LanguageProvider>
        <PlanMetadata plan={{ ...basePlan, cloud_exports: [] }} />
      </LanguageProvider>
    );

    expect(screen.queryByText('Cloud export targets')).not.toBeInTheDocument();
    expect(screen.queryByText(/cloud export\(s\)/i)).not.toBeInTheDocument();
  });
});
