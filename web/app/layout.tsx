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

export const metadata: Metadata = {
  title: "elephant.ai",
  description:
    "Lark-native proactive personal agent — lives in your groups and DMs, remembers context, executes real work autonomously.",
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
          <SmartErrorBoundary level="page">
            <main className="flex min-h-screen flex-col">{children}</main>
          </SmartErrorBoundary>
        </Providers>
      </body>
    </html>
  );
}
