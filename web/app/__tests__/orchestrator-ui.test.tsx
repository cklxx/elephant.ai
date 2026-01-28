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

describe("Conversation page orchestrator UI tools", () => {
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

  it("renders plan/clarify UI tools with correct levels", async () => {
    const baseTimestamp = new Date().toISOString();
    mockEventsRef.current = [
      {
        event_type: "workflow.tool.completed",
        timestamp: baseTimestamp,
        agent_level: "core",
        session_id: "test-session",
        task_id: "test-task",
        call_id: "call-plan",
        tool_name: "plan",
        result: "重构 Planner UI，并引入 plan/clarify 两个 UI 工具。",
        duration: 5,
        metadata: {
          internal_plan: {
            branches: [{ branch_goal: "SHOULD_NOT_RENDER", tasks: [] }],
          },
        },
      } as AnyAgentEvent,
      {
        event_type: "workflow.tool.completed",
        timestamp: baseTimestamp,
        agent_level: "core",
        session_id: "test-session",
        task_id: "test-task",
        call_id: "call-clarify",
        tool_name: "clarify",
        result: "更新 EventLine 的分级渲染",
        duration: 3,
        metadata: {
          task_goal_ui: "更新 EventLine 的分级渲染",
          success_criteria: ["plan 显示为 Level 1", "clarify 显示为 Level 2"],
        },
      } as AnyAgentEvent,
    ];

    renderWithProviders(<ConversationPageContent />);

    expect(
      await screen.findByText(
        "重构 Planner UI，并引入 plan/clarify 两个 UI 工具。",
        undefined,
        { timeout: 5000 },
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("更新 EventLine 的分级渲染")).toBeInTheDocument();
    expect(screen.getByText("plan 显示为 Level 1")).toBeInTheDocument();
    expect(screen.getByText("clarify 显示为 Level 2")).toBeInTheDocument();

    // internal_plan must never be rendered.
    expect(screen.queryByText("SHOULD_NOT_RENDER")).not.toBeInTheDocument();

    // The legacy mobile timeline dialog trigger should not appear in the new minimal UI.
    expect(screen.queryByTestId("mobile-timeline-trigger")).not.toBeInTheDocument();
  });
});
