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
          <div className="min-h-screen bg-gradient-to-br from-gray-50 via-blue-50/30 to-purple-50/30">
            {/* Header */}
            <header className="glass-card border-b border-gray-200/50 sticky top-0 z-50 shadow-soft">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <div className="flex items-center justify-between h-16">
                  <Link href="/" className="flex items-center gap-3 group">
                    <div className="p-2 bg-gradient-to-br from-blue-600 to-purple-600 rounded-xl shadow-md group-hover:shadow-lg transition-all duration-300 group-hover:scale-105">
                      <Code2 className="h-6 w-6 text-white" />
                    </div>
                    <div>
                      <h1 className="text-xl font-bold gradient-text">ALEX</h1>
                      <p className="text-xs text-gray-500 font-medium">AI Programming Agent</p>
                    </div>
                  </Link>
                  <nav className="flex items-center gap-6">
                    <Link
                      href="/"
                      className="text-sm font-semibold text-gray-700 hover:text-blue-600 transition-all duration-200 hover:scale-105"
                    >
                      Home
                    </Link>
                    <Link
                      href="/sessions"
                      className="flex items-center gap-2 px-4 py-2 text-sm font-semibold text-gray-700 hover:text-blue-600 transition-all duration-200 rounded-lg hover:bg-gray-100/50"
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
            <footer className="glass-card border-t border-gray-200/50 mt-auto shadow-soft">
              <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
                <p className="text-center text-sm text-gray-600 font-medium">
                  ALEX - Agile Light Easy Xpert Code Agent
                  <span className="mx-2">•</span>
                  <span className="gradient-text">Powered by AI</span>
                </p>
              </div>
            </footer>
          </div>
        </Providers>
      </body>
    </html>
  );
}
