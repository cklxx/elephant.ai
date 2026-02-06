package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

type stubCatalogService struct {
	catalog subscription.Catalog
}

func (s *stubCatalogService) Catalog(context.Context) subscription.Catalog {
	return s.catalog
}

func TestHandleGetSubscriptionCatalogUsesCatalogService(t *testing.T) {
	manager := configadmin.NewManager(&memoryStore{}, runtimeconfig.Overrides{})
	resolver := func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, nil
	}

	handler := NewConfigHandler(manager, resolver, nil, nil)
	handler.catalogService = &stubCatalogService{catalog: subscription.Catalog{Providers: []subscription.CatalogProvider{{Provider: "codex"}}}}

	req := httptest.NewRequest(http.MethodGet, "/api/internal/subscription/catalog", nil)
	rr := httptest.NewRecorder()

	handler.HandleGetSubscriptionCatalog(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
