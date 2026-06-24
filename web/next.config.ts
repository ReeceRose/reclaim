import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Static export — the Go binary serves the built /out via embed.FS (no Node
  // server in production). Every data-touching page is a client component that
  // talks to the Go API directly.
  output: "export",
  // next/image optimization needs a server; disable it for the static export.
  images: { unoptimized: true },
  // Trailing slashes keep relative asset paths resolving cleanly when served
  // from the Go static handler.
  trailingSlash: true,

  // Dev-only: proxy API + WS to the Go backend so the browser talks to a single
  // origin (cookies "just work", no CORS). Rewrites are ignored by `output:
  // 'export'`, so production is unaffected — there the Go binary serves both.
  async rewrites() {
    const backend = process.env.RECLAIM_BACKEND ?? "http://localhost:8080";
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
      { source: "/healthz", destination: `${backend}/healthz` },
    ];
  },
};

export default nextConfig;
