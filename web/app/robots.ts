import type { MetadataRoute } from "next";

export const dynamic = "force-static";
export const revalidate = 3600;

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/api/", "/dev/", "/share/"],
    },
    sitemap: "https://github.com/cklxx/elephant.ai/sitemap.xml",
  };
}
