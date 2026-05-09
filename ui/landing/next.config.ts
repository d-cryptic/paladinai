import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",      // static HTML/CSS/JS — works on Cloudflare Pages
  trailingSlash: true,   // avoids 404s on static hosts
  images: {
    unoptimized: true,   // required when output: "export"
  },
};

export default nextConfig;
