import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { cn } from "@/lib/utils";

const inter = Inter({ subsets: ["latin"], weight: ["300", "400", "500", "600", "700"] });

export const metadata: Metadata = {
  title: "Alex Code",
  description: "Build controllable, auditable agents with Plan + Clearify + ReAct.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh" className="h-full" suppressHydrationWarning>
      <body
        className={cn(
          "h-full bg-app-canvas text-foreground antialiased",
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
