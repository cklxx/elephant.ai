package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/shared/config"
)

func TestAppsConfigHandlerGet(t *testing.T) {
	handler := NewAppsConfigHandler(
		func(...config.Option) (config.AppsConfig, string, error) {
			return config.AppsConfig{
				Plugins: []config.AppPluginConfig{{ID: "lark", Name: "Lark"}},
			}, "/tmp/config.yaml", nil
		},
		func(config.AppsConfig, ...config.Option) (string, error) {
			return "/tmp/config.yaml", nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/internal/config/apps", nil)
	rr := httptest.NewRecorder()
	handler.HandleAppsConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload appsConfigResponse
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Apps.Plugins) != 1 || payload.Apps.Plugins[0].ID != "lark" {
		t.Fatalf("unexpected payload: %#v", payload.Apps.Plugins)
	}
}

func TestAppsConfigHandlerUpdateRejectsEmptyID(t *testing.T) {
	handler := NewAppsConfigHandler(
		func(...config.Option) (config.AppsConfig, string, error) {
			return config.AppsConfig{}, "", nil
		},
		func(config.AppsConfig, ...config.Option) (string, error) {
			return "", nil
		},
	)

	body := appsConfigResponse{
		Apps: config.AppsConfig{
			Plugins: []config.AppPluginConfig{{ID: " "}},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/internal/config/apps", bytes.NewReader(data))
	rr := httptest.NewRecorder()
	handler.HandleAppsConfig(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

func TestAppsConfigHandlerUpdateSavesNormalizedPlugins(t *testing.T) {
	var saved config.AppsConfig
	handler := NewAppsConfigHandler(
		func(...config.Option) (config.AppsConfig, string, error) {
			return config.AppsConfig{}, "", nil
		},
		func(apps config.AppsConfig, _ ...config.Option) (string, error) {
			saved = apps
			return "", nil
		},
	)

	body := appsConfigResponse{
		Apps: config.AppsConfig{
			Plugins: []config.AppPluginConfig{
				{
					ID:              " Lark ",
					Name:            " Lark ",
					Description:     " Chat ",
					IntegrationNote: " Note ",
					Capabilities:    []string{" receive ", " ", "reply"},
					Sources:         []string{" https://open.larksuite.com/ ", ""},
				},
			},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/internal/config/apps", bytes.NewReader(data))
	rr := httptest.NewRecorder()
	handler.HandleAppsConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if len(saved.Plugins) != 1 {
		t.Fatalf("expected 1 plugin saved, got %#v", saved.Plugins)
	}
	plugin := saved.Plugins[0]
	if plugin.ID != "lark" {
		t.Fatalf("expected normalized id, got %q", plugin.ID)
	}
	if plugin.Name != "Lark" || plugin.Description != "Chat" || plugin.IntegrationNote != "Note" {
		t.Fatalf("expected trimmed fields, got %#v", plugin)
	}
	if len(plugin.Capabilities) != 2 || plugin.Capabilities[0] != "receive" || plugin.Capabilities[1] != "reply" {
		t.Fatalf("unexpected capabilities: %#v", plugin.Capabilities)
	}
	if len(plugin.Sources) != 1 || plugin.Sources[0] != "https://open.larksuite.com/" {
		t.Fatalf("unexpected sources: %#v", plugin.Sources)
	}
}
