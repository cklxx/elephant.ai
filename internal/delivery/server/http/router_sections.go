package http

import "net/http"

func registerRoute(mux *http.ServeMux, pattern, route string, handler http.Handler) {
	if mux == nil || handler == nil {
		return
	}
	mux.Handle(pattern, routeHandler(route, handler))
}

func registerHandler(mux *http.ServeMux, pattern, route string, handler http.HandlerFunc) {
	registerRoute(mux, pattern, route, handler)
}

func registerContextConfigRoutes(mux *http.ServeMux, handler *ContextConfigHandler) {
	if handler == nil {
		return
	}
	registerHandler(mux, "GET /api/dev/context-config", "/api/dev/context-config", handler.HandleGetContextConfig)
	registerHandler(mux, "PUT /api/dev/context-config", "/api/dev/context-config", handler.HandleUpdateContextConfig)
	registerHandler(mux, "GET /api/dev/context-config/preview", "/api/dev/context-config/preview", handler.HandleContextPreview)
}

func registerRuntimeConfigRoutes(mux *http.ServeMux, handler *ConfigHandler) {
	if handler == nil {
		return
	}
	registerHandler(mux, "GET /api/internal/config/runtime", "/api/internal/config/runtime", handler.HandleGetRuntimeConfig)
	registerHandler(mux, "PUT /api/internal/config/runtime", "/api/internal/config/runtime", handler.HandleUpdateRuntimeConfig)
	registerHandler(mux, "GET /api/internal/config/runtime/stream", "/api/internal/config/runtime/stream", handler.HandleRuntimeStream)
	registerHandler(mux, "GET /api/internal/config/runtime/models", "/api/internal/config/runtime/models", handler.HandleGetRuntimeModels)
	registerHandler(mux, "GET /api/internal/subscription/catalog", "/api/internal/subscription/catalog", handler.HandleGetSubscriptionCatalog)
}

func registerOnboardingStateRoutes(mux *http.ServeMux, handler *OnboardingStateHandler) {
	if handler == nil {
		return
	}
	registerHandler(mux, "GET /api/internal/onboarding/state", "/api/internal/onboarding/state", handler.HandleGetOnboardingState)
	registerHandler(mux, "PUT /api/internal/onboarding/state", "/api/internal/onboarding/state", handler.HandleUpdateOnboardingState)
}

func registerLarkOAuthRoutes(mux *http.ServeMux, handler *LarkOAuthHandler) {
	if handler == nil {
		return
	}
	registerHandler(mux, "GET /api/lark/oauth/start", "/api/lark/oauth/start", handler.HandleStart)
	registerHandler(mux, "GET /api/lark/oauth/callback", "/api/lark/oauth/callback", handler.HandleCallback)
}

func registerHookRoutes(mux *http.ServeMux, hooksBridge, runtimeHooksBridge http.Handler) {
	registerRoute(mux, "POST /api/hooks/claude-code", "/api/hooks/claude-code", hooksBridge)
	registerRoute(mux, "POST /api/hooks/runtime", "/api/hooks/runtime", runtimeHooksBridge)
}

func registerTaskRoutes(mux *http.ServeMux, apiHandler *APIHandler, sseHandler *SSEHandler) {
	registerHandler(mux, "POST /api/tasks", "/api/tasks", apiHandler.HandleCreateTask)
	registerHandler(mux, "GET /api/tasks", "/api/tasks", apiHandler.HandleListTasks)
	registerHandler(mux, "GET /api/tasks/active", "/api/tasks/active", apiHandler.HandleListActiveTasks)
	registerHandler(mux, "GET /api/tasks/stats", "/api/tasks/stats", apiHandler.HandleGetTaskStats)
	registerHandler(mux, "GET /api/tasks/{task_id}", "/api/tasks/:task_id", apiHandler.HandleGetTask)
	registerHandler(mux, "GET /api/tasks/{task_id}/events", "/api/tasks/:task_id/events", sseHandler.HandleTaskSSEStream)
	registerHandler(mux, "POST /api/tasks/{task_id}/cancel", "/api/tasks/:task_id/cancel", apiHandler.HandleCancelTask)
}

func registerEvaluationRoutes(mux *http.ServeMux, apiHandler *APIHandler) {
	registerHandler(mux, "GET /api/evaluations", "/api/evaluations", apiHandler.HandleListEvaluations)
	registerHandler(mux, "POST /api/evaluations", "/api/evaluations", apiHandler.HandleStartEvaluation)
	registerHandler(mux, "GET /api/evaluations/{evaluation_id}", "/api/evaluations/:evaluation_id", apiHandler.HandleGetEvaluation)
	registerHandler(mux, "DELETE /api/evaluations/{evaluation_id}", "/api/evaluations/:evaluation_id", apiHandler.HandleDeleteEvaluation)
	registerHandler(mux, "GET /api/agents", "/api/agents", apiHandler.HandleListAgents)
	registerHandler(mux, "GET /api/agents/{agent_id}", "/api/agents/:agent_id", apiHandler.HandleGetAgent)
	registerHandler(mux, "GET /api/agents/{agent_id}/evaluations", "/api/agents/:agent_id/evaluations", apiHandler.HandleListAgentEvaluations)
}

func registerSessionRoutes(mux *http.ServeMux, apiHandler *APIHandler) {
	registerHandler(mux, "GET /api/sessions", "/api/sessions", apiHandler.HandleListSessions)
	registerHandler(mux, "POST /api/sessions", "/api/sessions", apiHandler.HandleCreateSession)
	registerHandler(mux, "GET /api/sessions/{session_id}", "/api/sessions/:session_id", apiHandler.HandleGetSession)
	registerHandler(mux, "DELETE /api/sessions/{session_id}", "/api/sessions/:session_id", apiHandler.HandleDeleteSession)
	registerHandler(mux, "GET /api/sessions/{session_id}/persona", "/api/sessions/:session_id/persona", apiHandler.HandleGetSessionPersona)
	registerHandler(mux, "PUT /api/sessions/{session_id}/persona", "/api/sessions/:session_id/persona", apiHandler.HandleUpdateSessionPersona)
	registerHandler(mux, "GET /api/sessions/{session_id}/snapshots", "/api/sessions/:session_id/snapshots", apiHandler.HandleListSnapshots)
	registerHandler(mux, "GET /api/sessions/{session_id}/turns/{turn_id}", "/api/sessions/:session_id/turns/:turn_id", apiHandler.HandleGetTurnSnapshot)
	registerHandler(mux, "POST /api/sessions/{session_id}/share", "/api/sessions/:session_id/share", apiHandler.HandleCreateSessionShare)
	registerHandler(mux, "POST /api/sessions/{session_id}/fork", "/api/sessions/:session_id/fork", apiHandler.HandleForkSession)
}

func registerLeaderRoutes(mux *http.ServeMux, handler *LeaderDashboardHandler, leaderAPIToken string) {
	leaderAuth := BearerAuthMiddleware(leaderAPIToken)
	if handler != nil {
		registerRoute(mux, "GET /api/leader/dashboard", "/api/leader/dashboard", leaderAuth(http.HandlerFunc(handler.HandleGetDashboard)))
		registerRoute(mux, "GET /api/leader/tasks", "/api/leader/tasks", leaderAuth(http.HandlerFunc(handler.HandleListTasks)))
		registerRoute(mux, "POST /api/leader/tasks/{id}/unblock", "/api/leader/tasks/{id}/unblock", leaderAuth(http.HandlerFunc(handler.HandleUnblockTask)))
	}
	registerRoute(mux, "GET /api/leader/openapi.json", "/api/leader/openapi.json", leaderAuth(http.HandlerFunc(HandleLeaderOpenAPISpec)))
}
