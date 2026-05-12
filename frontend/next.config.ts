import type { NextConfig } from "next";

const config: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  experimental: {
    typedRoutes: true,
  },
  images: {
    // R2 / MinIO origin for media. Override at deploy time.
    remotePatterns: [
      { protocol: "https", hostname: "minio.portal.localhost" },
      { protocol: "https", hostname: "media.portal.localhost" },
    ],
  },
};

export default config;
