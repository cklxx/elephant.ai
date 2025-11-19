import type { Metadata } from "next";
import type { NextWebVitalsMetric } from "next/app";
import { Inter } from "next/font/google";
import "./globals.css";
import "@uiw/react-markdown-preview/markdown.css";
import { Providers } from "./providers";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { cn } from "@/lib/utils";
import { buildApiUrl } from "@/lib/api-base";

const inter = Inter({ subsets: ["latin"], weight: ["300", "400", "500", "600", "700"] });

export const metadata: Metadata = {
  title: "ALEX Research Console",
  description: "Streamlined operator console for the ALEX agent.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="h-full">
      <body
        className={cn(
          "h-full bg-app-canvas text-slate-900 antialiased",
          inter.className
        )}
      >
        <Providers>
          <ErrorBoundary>
            <main className="flex min-h-screen flex-col">
              {children}
            </main>
          </ErrorBoundary>
        </Providers>
      </body>
    </html>
  );
}

export function reportWebVitals(metric: NextWebVitalsMetric) {
  if (typeof window === "undefined") {
    return;
  }

  const payload = {
    name: metric.name,
    value: metric.value,
    delta: "delta" in metric ? metric.delta : undefined,
    id: metric.id,
    label: metric.label,
    page: window.location?.pathname ?? ("path" in metric ? metric.path : undefined) ?? "/",
    navigation_type: "navigationType" in metric ? metric.navigationType : undefined,
    ts: Date.now(),
  };

  const body = JSON.stringify(payload);
  const endpoint = buildApiUrl("/api/metrics/web-vitals");
  const blob = new Blob([body], { type: "application/json" });

  if (navigator.sendBeacon) {
    navigator.sendBeacon(endpoint, blob);
    return;
  }

  fetch(endpoint, {
    method: "POST",
    body,
    headers: { "Content-Type": "application/json" },
    keepalive: true,
    credentials: "omit",
  }).catch(() => {
    // Silently ignore failures to avoid impacting UX
  });
}
