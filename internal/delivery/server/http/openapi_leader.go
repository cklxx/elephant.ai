package http

import "net/http"

// LeaderOpenAPISpec is the OpenAPI 3.0 specification for the leader agent
// API endpoints, encoded as a JSON string constant.
const LeaderOpenAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Leader Agent API",
    "description": "API for inspecting and managing the leader agent's task orchestration, blocker detection, and scheduling subsystems.",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "/api/leader",
      "description": "Leader agent API base path"
    }
  ],
  "paths": {
    "/dashboard": {
      "get": {
        "operationId": "getDashboard",
        "summary": "Get leader agent dashboard",
        "description": "Returns an aggregated dashboard view including task counts by status, recent blocker alerts, a daily summary, and scheduled job metadata. Components that are unavailable degrade gracefully — the response always returns 200 with partial data.",
        "tags": ["dashboard"],
        "responses": {
          "200": {
            "description": "Dashboard data",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/DashboardResponse" }
              }
            }
          }
        }
      }
    },
    "/tasks": {
      "get": {
        "operationId": "listLeaderTasks",
        "summary": "List tasks visible to the leader agent",
        "description": "Returns paginated tasks from the unified task store. Supports optional status filtering and pagination via query parameters.",
        "tags": ["tasks"],
        "parameters": [
          {
            "name": "status",
            "in": "query",
            "description": "Filter by task status.",
            "required": false,
            "schema": {
              "type": "string",
              "enum": ["pending", "running", "waiting_input", "completed", "failed", "cancelled"]
            }
          },
          {
            "name": "limit",
            "in": "query",
            "description": "Maximum number of tasks to return (default 50, max 200).",
            "required": false,
            "schema": { "type": "integer", "default": 50, "minimum": 1, "maximum": 200 }
          },
          {
            "name": "offset",
            "in": "query",
            "description": "Number of tasks to skip for pagination.",
            "required": false,
            "schema": { "type": "integer", "default": 0, "minimum": 0 }
          }
        ],
        "responses": {
          "200": {
            "description": "Paginated task list",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/TaskListResponse" }
              }
            }
          }
        }
      }
    },
    "/tasks/{id}/unblock": {
      "post": {
        "operationId": "unblockTask",
        "summary": "Request unblock for a stuck task",
        "description": "Submits an unblock request for a task that the blocker radar has identified as stuck. The leader agent will attempt to intervene by injecting guidance or escalating. Returns the action taken.",
        "tags": ["tasks"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "description": "Task ID to unblock.",
            "schema": { "type": "string" }
          }
        ],
        "requestBody": {
          "required": false,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/UnblockRequest" }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Unblock action accepted",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/UnblockResponse" }
              }
            }
          },
          "400": {
            "description": "Invalid request (missing or empty task ID)",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ErrorResponse" }
              }
            }
          },
          "404": {
            "description": "Task not found",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ErrorResponse" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "DashboardResponse": {
        "type": "object",
        "properties": {
          "tasks_by_status": { "$ref": "#/components/schemas/TaskStatusCounts" },
          "recent_blockers": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/BlockerAlert" }
          },
          "daily_summary": {
            "nullable": true,
            "$ref": "#/components/schemas/DailySummary"
          },
          "scheduled_jobs": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/ScheduledJob" }
          }
        },
        "required": ["tasks_by_status", "recent_blockers"]
      },
      "TaskStatusCounts": {
        "type": "object",
        "properties": {
          "pending":     { "type": "integer" },
          "in_progress": { "type": "integer" },
          "blocked":     { "type": "integer" },
          "completed":   { "type": "integer" }
        },
        "required": ["pending", "in_progress", "blocked", "completed"]
      },
      "BlockerAlert": {
        "type": "object",
        "properties": {
          "task_id":     { "type": "string" },
          "description": { "type": "string" },
          "reason":      { "type": "string", "enum": ["stale_progress", "has_error", "waiting_input", "dependency_blocked"] },
          "detail":      { "type": "string" },
          "status":      { "type": "string" }
        },
        "required": ["task_id", "description", "reason", "detail", "status"]
      },
      "DailySummary": {
        "type": "object",
        "properties": {
          "new_tasks":       { "type": "integer" },
          "completed":       { "type": "integer" },
          "in_progress":     { "type": "integer" },
          "blocked":         { "type": "integer" },
          "completion_rate": { "type": "number", "format": "float" }
        },
        "required": ["new_tasks", "completed", "in_progress", "blocked", "completion_rate"]
      },
      "ScheduledJob": {
        "type": "object",
        "properties": {
          "name":      { "type": "string" },
          "cron_expr": { "type": "string" },
          "status":    { "type": "string" },
          "next_run":  { "type": "string", "format": "date-time" },
          "last_run":  { "type": "string", "format": "date-time" }
        },
        "required": ["name", "cron_expr", "status"]
      },
      "TaskListResponse": {
        "type": "object",
        "properties": {
          "tasks": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/TaskSummary" }
          },
          "total":  { "type": "integer" },
          "limit":  { "type": "integer" },
          "offset": { "type": "integer" }
        },
        "required": ["tasks", "total", "limit", "offset"]
      },
      "TaskSummary": {
        "type": "object",
        "properties": {
          "task_id":           { "type": "string" },
          "description":       { "type": "string" },
          "status":            { "type": "string", "enum": ["pending", "running", "waiting_input", "completed", "failed", "cancelled"] },
          "user_id":           { "type": "string" },
          "current_iteration": { "type": "integer" },
          "tokens_used":       { "type": "integer" },
          "error":             { "type": "string" },
          "created_at":        { "type": "string", "format": "date-time" },
          "updated_at":        { "type": "string", "format": "date-time" }
        },
        "required": ["task_id", "status", "created_at", "updated_at"]
      },
      "UnblockRequest": {
        "type": "object",
        "properties": {
          "reason": {
            "type": "string",
            "description": "Optional human-provided context for the unblock attempt."
          }
        }
      },
      "UnblockResponse": {
        "type": "object",
        "properties": {
          "task_id": { "type": "string" },
          "action":  { "type": "string", "enum": ["injected", "escalated", "no_action"], "description": "The intervention action taken." },
          "detail":  { "type": "string", "description": "Human-readable description of what was done." }
        },
        "required": ["task_id", "action", "detail"]
      },
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error":   { "type": "string" },
          "details": { "type": "string" }
        },
        "required": ["error"]
      }
    }
  }
}`

// HandleLeaderOpenAPISpec serves the OpenAPI specification for the leader
// agent endpoints as JSON.
func HandleLeaderOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(LeaderOpenAPISpec))
}
