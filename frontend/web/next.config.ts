import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    const mediaUrl = process.env.NEXT_PUBLIC_MEDIA_URL?.trim().replace(/\/+$/, "");
    if (!mediaUrl) return [];
    const dest = mediaUrl.endsWith("/media") ? mediaUrl : `${mediaUrl}/media`;
    return [
      {
        source: "/media/:path*",
        destination: `${dest}/:path*`,
      },
    ];
  },
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
