import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
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
          <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <header className="bg-white border-b border-gray-200 sticky top-0 z-50">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex items-center justify-between h-16">
                  <Link href="/" className="flex items-center gap-2">
                    <Code2 className="h-8 w-8 text-blue-600" />
                    <div>
                      <h1 className="text-xl font-bold text-gray-900">ALEX</h1>
                      <p className="text-xs text-gray-500">AI Programming Agent</p>
                    </div>
                  </Link>
                  <nav className="flex items-center gap-6">
                    <Link
                      href="/"
                      className="text-sm font-medium text-gray-700 hover:text-blue-600 transition-colors"
                    >
                      Home
                    </Link>
                    <Link
                      href="/sessions"
                      className="flex items-center gap-1 text-sm font-medium text-gray-700 hover:text-blue-600 transition-colors"
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
              {children}
            </main>

            {/* Footer */}
            <footer className="bg-white border-t border-gray-200 mt-auto">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
                <p className="text-center text-sm text-gray-500">
                  ALEX - Agile Light Easy Xpert Code Agent
                </p>
              </div>
            </footer>
          </div>
        </Providers>
      </body>
    </html>
  );
}
