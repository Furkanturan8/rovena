import type { NextConfig } from "next";

const mediaBase = process.env.NEXT_PUBLIC_MEDIA_URL?.trim().replace(/\/+$/, "");
const apiBase = (process.env.API_BASE_URL ?? "http://localhost:8080").replace(/\/+$/, "");

const nextConfig: NextConfig = {
  images: {
    // Product photos are stored as absolute URLs in the catalog database (public media bucket).
    remotePatterns: mediaBase
      ? [{ protocol: "https", hostname: new URL(mediaBase).hostname, pathname: "/storage/v1/object/public/**" }]
      : [],
  },
  async rewrites() {
    const rules = [
      // Public, read-only storefront API: lets client components read the catalog without CORS.
      { source: "/api/v1/:resource(products|categories|stock)", destination: `${apiBase}/api/v1/:resource` },
      { source: "/api/v1/:resource(products|categories|stock)/:path*", destination: `${apiBase}/api/v1/:resource/:path*` },
    ];
    if (mediaBase) {
      const dest = mediaBase.endsWith("/media") ? mediaBase : `${mediaBase}/media`;
      // Static site assets that live in the media bucket (hero video, posters, ...).
      rules.push({ source: "/media/:path*", destination: `${dest}/:path*` });
    }
    return rules;
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
