/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  output: 'export',
  images: {
    unoptimized: true,
  },
  env: {
    NEXT_PUBLIC_EVAL_API_URL: process.env.NEXT_PUBLIC_EVAL_API_URL ?? 'http://localhost:8081',
  },
};

export default nextConfig;
