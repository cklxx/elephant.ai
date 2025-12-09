"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Toaster } from "@/components/ui/toast";
import { LanguageProvider } from "@/lib/i18n";
import { AuthProvider } from "@/lib/auth/context";
import { initAnalytics } from "@/lib/analytics/posthog";

export function Providers({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 60 * 1000, // 1 minute
            retry: 1,
          },
        },
      }),
  );

  useEffect(() => {
    initAnalytics();
  }, []);

  return (
    <LanguageProvider>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          {children}
          <Toaster />
        </AuthProvider>
      </QueryClientProvider>
    </LanguageProvider>
  );
}
