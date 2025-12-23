import type { Metadata } from "next";
import { JetBrains_Mono, Plus_Jakarta_Sans } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { cn } from "@/lib/utils";

const sans = Plus_Jakarta_Sans({
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700"],
  variable: "--font-sans",
  display: "swap",
});

const mono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: "elephant.ai",
  description:
    "Build controllable, auditable agents with Plan + Clearify + ReAct.",
  icons: {
    icon: "/elephant.jpg",
  },
};

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
        <Providers>
          <ErrorBoundary>
            <main className="flex min-h-screen flex-col">{children}</main>
          </ErrorBoundary>
        </Providers>
      </body>
    </html>
  );
}
