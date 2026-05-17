import type { NextConfig } from "next";
import { config } from "dotenv";
import { resolve } from "path";

config({ path: resolve(__dirname, "../../.env") });

const remoteApiUrl = process.env.REMOTE_API_URL || "http://localhost:8088";

const nextConfig: NextConfig = {
  ...(process.env.STANDALONE === "true" ? { output: "standalone" as const } : {}),
  transpilePackages: ["@medigt/core", "@medigt/ui", "@medigt/views"],
  images: {
    formats: ["image/avif", "image/webp"],
  },
  async rewrites() {
    return {
      afterFiles: [
        { source: "/api/:path*", destination: `${remoteApiUrl}/api/:path*` },
        { source: "/ws", destination: `${remoteApiUrl}/ws` },
        { source: "/uploads/:path*", destination: `${remoteApiUrl}/uploads/:path*` },
      ],
      beforeFiles: [],
      fallback: [],
    };
  },
};

export default nextConfig;
