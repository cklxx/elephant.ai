package lark

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// testServer creates an httptest server whose handler dispatches based on URL path.
// It automatically handles the tenant_access_token endpoint so that the SDK can
// authenticate. The provided handler handles all other API calls.
// Caller must close the server when done.
func testServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle token requests automatically.
		if strings.Contains(r.URL.Path, "tenant_access_token") ||
			strings.Contains(r.URL.Path, "app_access_token") {
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"tenant_access_token": "test-token",
				"app_access_token":    "test-token",
				"expire":              7200,
			}))
			return
		}
		handler(w, r)
	})
	srv := httptest.NewServer(wrapped)
	sdkClient := lark.NewClient("test-app-id", "test-app-secret",
		lark.WithOpenBaseUrl(srv.URL),
	)
	return srv, Wrap(sdkClient)
}

// jsonResponse is a convenience for building Lark API response bodies.
func jsonResponse(code int, msg string, data interface{}) []byte {
	resp := map[string]interface{}{
		"code": code,
		"msg":  msg,
	}
	if data != nil {
		resp["data"] = data
	}
	b, _ := json.Marshal(resp)
	return b
}

// routeMux returns an http.HandlerFunc that routes requests by matching
// path substrings to specific handlers. If no match is found, 404 is returned.
func routeMux(routes map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for pattern, handler := range routes {
			if strings.Contains(r.URL.Path, pattern) {
				handler(w, r)
				return
			}
		}
		http.NotFound(w, r)
	}
}

// readBody reads and closes the request body.
func readBody(r *http.Request) []byte {
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	return b
}

// counter is a simple atomic counter for sequencing mock responses.
type counter struct {
	n atomic.Int64
}

func (c *counter) next() int64 {
	return c.n.Add(1)
}

// calendarEventJSON returns a Lark calendar event JSON fragment for test use.
func calendarEventJSON(eventID, summary string, startUnix, endUnix int64) map[string]interface{} {
	return map[string]interface{}{
		"event_id":    eventID,
		"summary":     summary,
		"description": "",
		"start_time":  map[string]interface{}{"timestamp": fmt.Sprintf("%d", startUnix)},
		"end_time":    map[string]interface{}{"timestamp": fmt.Sprintf("%d", endUnix)},
		"status":      "confirmed",
	}
}

// taskJSON returns a Lark task JSON fragment for test use.
func taskJSON(guid, summary, completedAt string) map[string]interface{} {
	t := map[string]interface{}{
		"guid":    guid,
		"summary": summary,
	}
	if completedAt != "" {
		t["completed_at"] = completedAt
	}
	return t
}
