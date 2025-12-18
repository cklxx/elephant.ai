import { render, screen } from "@testing-library/react";
import { describe, it, beforeAll, afterAll, expect, vi } from "vitest";
import { ConversationPageContent } from "../conversation/ConversationPageContent";
import { AnyAgentEvent } from "@/lib/types";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactElement, ReactNode } from "react";
import { LanguageProvider } from "@/lib/i18n";

const mockEventsRef: { current: AnyAgentEvent[] } = { current: [] };

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    prefetch: vi.fn(),
  }),
  usePathname: () => "/conversation",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/hooks/useTaskExecution", () => ({
  useTaskExecution: () => ({ mutate: vi.fn(), isPending: false }),
  useCancelTask: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("@/hooks/useAgentEventStream", () => ({
  useAgentEventStream: () => ({
    events: mockEventsRef.current,
    isConnected: true,
    isReconnecting: false,
    error: null,
    reconnectAttempts: 0,
    clearEvents: vi.fn(),
    reconnect: vi.fn(),
    addEvent: vi.fn(),
  }),
}));

vi.mock("@/hooks/useSessionStore", () => ({
  useSessionStore: () => ({
    currentSessionId: null,
    setCurrentSession: vi.fn(),
    addToHistory: vi.fn(),
    clearCurrentSession: vi.fn(),
    removeSession: vi.fn(),
    renameSession: vi.fn(),
    sessionHistory: [],
    sessionLabels: {},
  }),
  useDeleteSession: () => ({
    mutateAsync: vi.fn(),
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock("@/lib/auth/context", () => ({
  AuthProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  useAuth: () => ({
    status: "authenticated",
    session: null,
    user: {
      id: "test-user",
      email: "tester@example.com",
      displayName: "Tester",
      pointsBalance: 0,
      subscription: {
        tier: "free",
        monthlyPriceCents: 0,
        expiresAt: null,
        isPaid: false,
      },
    },
    accessToken: "token",
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    refresh: vi.fn(),
    loginWithProvider: vi.fn(),
    startOAuth: vi.fn(),
    awaitOAuthSession: vi.fn(),
    adjustPoints: vi.fn(),
    updateSubscription: vi.fn(),
    listPlans: vi.fn().mockResolvedValue([]),
  }),
}));

describe("Conversation page plan progress UI", () => {
  const renderWithProviders = (ui: ReactElement) => {
    const queryClient = new QueryClient();
    return render(
      <LanguageProvider>
        <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
      </LanguageProvider>,
    );
  };

  beforeAll(() => {
    vi.stubGlobal("CSS", {
      escape: (value: string) => value,
    });
    Object.defineProperty(window.HTMLElement.prototype, "scrollIntoView", {
      value: vi.fn(),
      configurable: true,
    });
    Object.defineProperty(window.HTMLElement.prototype, "focus", {
      value: vi.fn(),
      configurable: true,
    });
  });

  afterAll(() => {
    vi.unstubAllGlobals();
  });

  it("renders step-focused progress without tool details", async () => {
    const baseTimestamp = new Date().toISOString();
    mockEventsRef.current = [
      {
        event_type: "workflow.plan.created",
        steps: ["Research existing implementations", "Write report", "总结"],
        timestamp: baseTimestamp,
        agent_level: "core",
        session_id: "test-session",
        task_id: "test-task",
      } as AnyAgentEvent,
      {
        event_type: "workflow.node.started",
        step_index: 0,
        step_description: "Research existing implementations",
        timestamp: baseTimestamp,
        agent_level: "core",
        session_id: "test-session",
        task_id: "test-task",
      } as AnyAgentEvent,
    ];

    renderWithProviders(<ConversationPageContent />);

    expect(
      await screen.findByText("Research existing implementations"),
    ).toBeInTheDocument();
    expect(screen.getByText("Write report")).toBeInTheDocument();
    expect(screen.getByText("总结")).toBeInTheDocument();

    // The legacy mobile timeline dialog trigger should not appear in the new minimal UI.
    expect(screen.queryByTestId("mobile-timeline-trigger")).not.toBeInTheDocument();
  });
});

