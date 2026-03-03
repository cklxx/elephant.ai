import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";
import { Providers } from "./providers";
import { SmartErrorBoundary } from "@/components/SmartErrorBoundary";
import { cn } from "@/lib/utils";

const sans = localFont({
  src: "../public/fonts/PlusJakartaSans-Variable.woff2",
  variable: "--font-sans",
  display: "swap",
  weight: "300 700",
});

const mono = localFont({
  src: "../public/fonts/JetBrainsMono-Variable.woff2",
  variable: "--font-mono",
  display: "swap",
  weight: "100 800",
});

const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? "";

export const metadata: Metadata = {
  metadataBase: basePath
    ? new URL(`https://cklxx.github.io${basePath}`)
    : undefined,
  title: "elephant.ai — Proactive AI Assistant",
  description:
    "Your AI teammate, always on. Lives in Lark, remembers everything, executes real work autonomously. Open source, self-hosted, 8 LLM providers.",
  icons: {
    icon: `${basePath}/elephant.jpg`,
  },
  openGraph: {
    title: "elephant.ai — Proactive AI Assistant",
    description:
      "Your AI teammate, always on. Lives in Lark, remembers everything, executes real work autonomously.",
    url: "https://github.com/cklxx/elephant.ai",
    siteName: "elephant.ai",
    images: [{ url: `${basePath}/og-image.png`, width: 1280, height: 720, alt: "elephant.ai" }],
    locale: "en_US",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "elephant.ai — Proactive AI Assistant",
    description:
      "Your AI teammate, always on. Lives in Lark, remembers everything, executes real work autonomously.",
    images: [`${basePath}/og-image.png`],
  },
  alternates: {
    languages: { "zh-CN": "/zh" },
  },
  robots: { index: true, follow: true },
};

const jsonLdString = JSON.stringify({
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "elephant.ai",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Cross-platform",
  description:
    "Proactive AI assistant that lives in Lark, remembers everything, and executes real work autonomously.",
  url: "https://github.com/cklxx/elephant.ai",
  license: "https://opensource.org/licenses/MIT",
  offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
  author: { "@type": "Person", name: "cklxx" },
});

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={cn("h-full", sans.variable, mono.variable)}
      suppressHydrationWarning
    >
      <body
        className={cn(
          "h-full bg-app-canvas font-sans text-foreground antialiased",
        )}
      >
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: jsonLdString }}
        />
        <Providers>
          <SmartErrorBoundary level="page">
            <main className="flex min-h-screen flex-col">{children}</main>
          </SmartErrorBoundary>
        </Providers>
      </body>
    </html>
  );
}
