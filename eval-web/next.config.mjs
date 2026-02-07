/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // No static export — eval-web has dynamic routes (e.g., /evaluations/[id])
  images: {
    unoptimized: true,
  },
  env: {
    NEXT_PUBLIC_EVAL_API_URL: process.env.NEXT_PUBLIC_EVAL_API_URL ?? 'http://localhost:8081',
  },
};

export default nextConfig;
