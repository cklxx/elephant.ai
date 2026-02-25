import bundleAnalyzer from '@next/bundle-analyzer';

const repositoryName = process.env.NEXT_PUBLIC_BASE_PATH || '';
const assetPrefix = process.env.NEXT_PUBLIC_ASSET_PREFIX || repositoryName || undefined;
const withBundleAnalyzer = bundleAnalyzer({ enabled: process.env.ANALYZE === 'true' });

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  basePath: repositoryName || undefined,
  assetPrefix,
  experimental: {
    turbopackUseSystemTlsCerts: true,
  },
  /**
   * Static export for GitHub Pages. Enabled via STATIC_EXPORT=1 in CI.
   * Disabled locally because the visualizer dev pages use API routes.
   */
  ...(process.env.STATIC_EXPORT === '1' ? { output: 'export' } : {}),
  images: {
    unoptimized: true,
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL ?? 'auto',
  },
};

export default withBundleAnalyzer(nextConfig);
