import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Deployed on Vercel, which runs Next.js natively. Every page here is fully
  // static, so this also exports cleanly if you ever want to host it elsewhere.
  reactStrictMode: true,
};

export default nextConfig;
