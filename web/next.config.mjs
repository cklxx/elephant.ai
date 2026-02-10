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
   * Generate a fully static output so GitHub Pages has an `index.html` in `web/out`.
   * This keeps `npm run build` aligned with the CI expectation that checks the
   * static export directory.
   *
   * NOTE: Commented out for visualizer development (requires API routes)
   */
  // output: 'export',
  images: {
    unoptimized: true,
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL ?? 'auto',
  },
};

export default withBundleAnalyzer(nextConfig);
