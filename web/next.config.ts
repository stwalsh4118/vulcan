import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return {
      // Keep filesystem/dynamic API routes (like SSE streaming handlers) ahead
      // of the catch-all backend proxy.
      fallback: [
        {
          source: "/api/v1/:path*",
          destination: "http://localhost:8080/v1/:path*",
        },
      ],
    };
  },
};

export default nextConfig;
