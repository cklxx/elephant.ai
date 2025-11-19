const repositoryName = process.env.NEXT_PUBLIC_BASE_PATH || '';
const assetPrefix = process.env.NEXT_PUBLIC_ASSET_PREFIX || repositoryName || undefined;

/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  basePath: repositoryName || undefined,
  assetPrefix,
  images: {
    unoptimized: true,
  },
  env: {
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL ?? 'auto',
  },
  experimental: {
    /**
     * Opt-in to the Rust-based Turbopack compiler for both dev and build
     * pipelines. This significantly reduces incremental build latency while
     * keeping configuration flexible for future loader overrides.
     */
    turbo: {},
  },
};

export default nextConfig;
