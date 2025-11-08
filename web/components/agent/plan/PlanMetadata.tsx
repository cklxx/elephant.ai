'use client';

import { useTranslation } from '@/lib/i18n';
import { ResearchPlan, CloudExportTarget } from './types';
import { LucideIcon } from 'lucide-react';
import { Gauge, Clock, Wrench, Cloud } from 'lucide-react';

type TranslateFn = ReturnType<typeof useTranslation>;

export function PlanMetadata({ plan }: { plan: ResearchPlan }) {
  const t = useTranslation();
  const estimatedDuration = formatEstimatedDurationLabel(t, plan.estimated_duration_minutes);
  const hasEstimatedTools = plan.estimated_tools.length > 0;
  const cloudExports = plan.cloud_exports ?? [];
  const hasCloudExports = cloudExports.length > 0;

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <PlanEstimatePill
          icon={Gauge}
          label={t('plan.estimates.iterations', { count: plan.estimated_iterations })}
        />
        {estimatedDuration && <PlanEstimatePill icon={Clock} label={estimatedDuration} />}
        {hasEstimatedTools && (
          <PlanEstimatePill
            icon={Wrench}
            label={t('plan.estimates.tools', { count: plan.estimated_tools.length })}
          />
        )}
        {hasCloudExports && (
          <PlanEstimatePill
            icon={Cloud}
            label={t('plan.storage.count', { count: cloudExports.length })}
          />
        )}
      </div>

      {hasEstimatedTools && (
        <div className="flex flex-wrap items-center gap-1.5">
          {plan.estimated_tools.slice(0, 5).map((tool, idx) => (
            <span
              key={`${tool}-${idx}`}
              className="text-xs px-2 py-1 rounded border border-border/60 bg-background/80 text-muted-foreground"
            >
              {tool}
            </span>
          ))}
          {plan.estimated_tools.length > 5 && (
            <span className="text-xs px-2 py-1 rounded border border-border/60 bg-background/60 text-muted-foreground/80">
              {t('plan.tools.more', { count: plan.estimated_tools.length - 5 })}
            </span>
          )}
        </div>
      )}

      {hasCloudExports && <CloudExportSummary exports={cloudExports} t={t} />}
    </div>
  );
}

function PlanEstimatePill({ icon: Icon, label }: { icon: LucideIcon; label: string }) {
  return (
    <span className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-background/80 px-3 py-1 text-[11px] font-medium text-muted-foreground">
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{label}</span>
    </span>
  );
}

function formatEstimatedDurationLabel(
  t: TranslateFn,
  minutes?: number,
) {
  if (minutes === undefined || minutes === null || Number.isNaN(minutes)) {
    return null;
  }

  if (minutes <= 0) {
    return null;
  }

  if (minutes < 60) {
    return t('plan.estimates.durationMinutes', { minutes: Math.round(minutes) });
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = Math.round(minutes % 60);

  if (remainingMinutes === 0) {
    return t('plan.estimates.durationHours', { hours });
  }

  if (hours > 0) {
    return t('plan.estimates.durationHoursMinutes', {
      hours,
      minutes: remainingMinutes,
    });
  }

  return t('plan.estimates.durationMinutes', { minutes: remainingMinutes });
}

function CloudExportSummary({ exports, t }: { exports: CloudExportTarget[]; t: TranslateFn }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
        <Cloud className="h-3.5 w-3.5" aria-hidden="true" />
        <span>{t('plan.storage.heading')}</span>
      </div>
      <div className="space-y-2">
        {exports.map((target, index) => (
          <CloudExportCard key={`${target.provider}-${target.bucket}-${target.path}-${index}`} target={target} t={t} />
        ))}
      </div>
    </div>
  );
}

function CloudExportCard({ target, t }: { target: CloudExportTarget; t: TranslateFn }) {
  const accessLabel = formatAccessLabel(t, target.access);
  const retentionLabel = formatRetentionLabel(t, target.retention_days);
  const regionLabel = target.region ? t('plan.storage.region', { region: target.region }) : null;
  const descriptionLabel = target.description ?? null;
  const bucketPath = formatBucketPath(target.bucket, target.path);

  return (
    <div className="rounded-xl border border-dashed border-border/60 bg-background/80 px-3 py-2 shadow-sm">
      <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span className="text-sm font-semibold text-foreground">
          {formatProviderLabel(target.provider)}
        </span>
        <span className="font-mono text-[11px] text-muted-foreground/80 break-all">{bucketPath}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
        <StorageBadge label={accessLabel} />
        {regionLabel && <StorageBadge label={regionLabel} />}
        {retentionLabel && <StorageBadge label={retentionLabel} />}
        {descriptionLabel && <StorageBadge label={descriptionLabel} />}
      </div>
    </div>
  );
}

function StorageBadge({ label }: { label: string }) {
  return (
    <span className="inline-flex items-center rounded-full border border-border/50 bg-background px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
      {label}
    </span>
  );
}

function formatBucketPath(bucket: string, path?: string) {
  const trimmedBucket = bucket.trim();
  const trimmedPath = (path ?? '').trim();
  if (!trimmedPath) {
    return trimmedBucket;
  }
  const normalizedBucket = trimmedBucket.replace(/\/+$/, '');
  const normalizedPath = trimmedPath.replace(/^\/+/, '');
  return `${normalizedBucket}/${normalizedPath}`;
}

function formatProviderLabel(provider: string) {
  const normalized = provider.toLowerCase();
  const providerMap: Record<string, string> = {
    s3: 'Amazon S3',
    'aws_s3': 'Amazon S3',
    'amazon s3': 'Amazon S3',
    gcs: 'Google Cloud Storage',
    'google_cloud_storage': 'Google Cloud Storage',
    'google cloud storage': 'Google Cloud Storage',
    azure: 'Azure Blob Storage',
    'azure_blob': 'Azure Blob Storage',
    'azure blob storage': 'Azure Blob Storage',
    r2: 'Cloudflare R2',
    'cloudflare_r2': 'Cloudflare R2',
  };

  if (providerMap[normalized]) {
    return providerMap[normalized];
  }

  return provider
    .replace(/[_-]+/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatAccessLabel(t: TranslateFn, access: CloudExportTarget['access']) {
  switch (access) {
    case 'read':
      return t('plan.storage.access.read');
    case 'write':
      return t('plan.storage.access.write');
    case 'read_write':
      return t('plan.storage.access.read_write');
    default:
      return t('plan.storage.access.unknown', { access });
  }
}

function formatRetentionLabel(t: TranslateFn, days?: number | null) {
  if (days === undefined || days === null) {
    return null;
  }

  if (days <= 0) {
    return t('plan.storage.noRetention');
  }

  if (days === 1) {
    return t('plan.storage.retentionDay', { days: '1' });
  }

  return t('plan.storage.retentionDays', { days: String(days) });
}
