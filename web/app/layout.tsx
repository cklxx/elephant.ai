import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import Link from "next/link";
import { Code2, ListTodo } from "lucide-react";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "ALEX - AI Programming Agent",
  description: "Agile Light Easy Xpert Code Agent - Terminal-native AI programming agent",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Providers>
          <div className="min-h-screen bg-background">
            {/* Header */}
            <header className="border-b border-border bg-background sticky top-0 z-50">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex items-center justify-between h-16">
                  <Link href="/" className="flex items-center gap-3 hover-subtle rounded-md px-2 py-1">
                    <div className="p-2 bg-primary rounded-md">
                      <Code2 className="h-6 w-6 text-primary-foreground" />
                    </div>
                    <div>
                      <h1 className="text-xl manus-heading">ALEX</h1>
                      <p className="text-xs manus-caption">AI Programming Agent</p>
                    </div>
                  </Link>
                  <nav className="flex items-center gap-6">
                    <Link
                      href="/"
                      className="text-sm font-semibold text-foreground hover:text-primary hover-subtle rounded-md px-3 py-2"
                    >
                      Home
                    </Link>
                    <Link
                      href="/sessions"
                      className="flex items-center gap-2 px-4 py-2 text-sm font-semibold text-foreground hover:text-primary hover-subtle rounded-md"
                    >
                      <ListTodo className="h-4 w-4" />
                      Sessions
                    </Link>
                  </nav>
                </div>
              </div>
            </header>

            {/* Main content */}
            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
              <ErrorBoundary>
                {children}
              </ErrorBoundary>
            </main>

            {/* Footer */}
            <footer className="border-t border-border mt-auto bg-background">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
                <p className="text-center text-sm manus-caption">
                  ALEX - Agile Light Easy Xpert Code Agent
                  <span className="mx-2">•</span>
                  <span className="text-foreground">Powered by AI</span>
                </p>
              </div>
            </footer>
          </div>
        </Providers>
      </body>
    </html>
  );
}
