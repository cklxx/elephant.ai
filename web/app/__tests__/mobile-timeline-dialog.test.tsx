import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, beforeAll, afterAll, expect, vi } from "vitest";
import ConversationPage from "../conversation/page";
import { AnyAgentEvent } from "@/lib/types";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ReactElement, ReactNode } from "react";
import { LanguageProvider } from "@/lib/i18n";

const mockEventsRef: { current: AnyAgentEvent[] } = { current: [] };

const replaceMock = vi.fn();
const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
    replace: replaceMock,
    prefetch: vi.fn(),
  }),
  usePathname: () => "/conversation",
  useSearchParams: () => new URLSearchParams(),
}));

const mockMutate = vi.fn();
const mockCancel = vi.fn();

vi.mock("@/hooks/useTaskExecution", () => ({
  useTaskExecution: () => ({ mutate: mockMutate, isPending: false }),
  useCancelTask: () => ({ mutate: mockCancel, isPending: false }),
}));

const mockAddEvent = vi.fn();
const mockReconnect = vi.fn();
const mockClearEvents = vi.fn();

vi.mock("@/hooks/useAgentEventStream", () => ({
  useAgentEventStream: () => ({
    events: mockEventsRef.current,
    isConnected: true,
    isReconnecting: false,
    error: null,
    reconnectAttempts: 0,
    clearEvents: mockClearEvents,
    reconnect: mockReconnect,
    addEvent: mockAddEvent,
  }),
}));

const mockSetCurrentSession = vi.fn();
const mockAddToHistory = vi.fn();
const mockClearCurrentSession = vi.fn();
const mockRemoveSession = vi.fn();
const mockRenameSession = vi.fn();
const mockDeleteSession = vi.fn();

vi.mock("@/hooks/useSessionStore", () => ({
  useSessionStore: () => ({
    currentSessionId: null,
    setCurrentSession: mockSetCurrentSession,
    addToHistory: mockAddToHistory,
    clearCurrentSession: mockClearCurrentSession,
    removeSession: mockRemoveSession,
    renameSession: mockRenameSession,
    sessionHistory: [],
    sessionLabels: {},
  }),
  useDeleteSession: () => ({
    mutateAsync: mockDeleteSession,
    mutate: mockDeleteSession,
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

describe("Conversation page mobile timeline dialog", () => {
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

  it("opens the timeline dialog on mobile and closes after selecting a step", async () => {
    const baseTimestamp = new Date().toISOString();
    mockEventsRef.current = [
      {
        event_type: "step_started",
        step_index: 0,
        step_description: "Research existing implementations",
        timestamp: baseTimestamp,
        agent_level: "core",
        session_id: "test-session",
        task_id: "test-task",
      } as AnyAgentEvent,
    ];

    renderWithProviders(<ConversationPage />);

    const openButton = await screen.findByTestId("mobile-timeline-trigger");
    fireEvent.click(openButton);

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toBeInTheDocument();
    expect(screen.getByText("Execution timeline")).toBeInTheDocument();

    const stepHeading = screen.getAllByText("Step 1")[0];
    const stepTrigger = stepHeading.closest('[role="button"]');
    expect(stepTrigger).toBeTruthy();
    fireEvent.click(stepTrigger!);

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });
});
