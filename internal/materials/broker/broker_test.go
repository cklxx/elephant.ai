package broker

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/storage"
)

type fakeRegistry struct {
	lastRegister *materialapi.RegisterMaterialsRequest
	materials    []*materialapi.Material
}

func (f *fakeRegistry) RegisterMaterials(ctx context.Context, req *materialapi.RegisterMaterialsRequest) (*materialapi.RegisterMaterialsResponse, error) {
	f.lastRegister = req
	if f.materials == nil {
		for _, input := range req.Materials {
			f.materials = append(f.materials, &materialapi.Material{
				MaterialID: "mat-" + input.Name,
				Descriptor: &materialapi.MaterialDescriptor{
					Name:        input.Name,
					Placeholder: "[material:" + input.Name + "]",
					MimeType:    input.MimeType,
					Source:      input.Source,
					Description: input.Description,
				},
				Storage: &materialapi.MaterialStorage{
					StorageKey:  input.StorageKey,
					CDNURL:      input.CDNURL,
					ContentHash: input.ContentHash,
					SizeBytes:   input.SizeBytes,
				},
			})
		}
	}
	return &materialapi.RegisterMaterialsResponse{Materials: f.materials}, nil
}

func (f *fakeRegistry) ListMaterials(ctx context.Context, req *materialapi.ListMaterialsRequest) (*materialapi.ListMaterialsResponse, error) {
	return &materialapi.ListMaterialsResponse{Materials: f.materials}, nil
}

func TestAttachmentBrokerRegisterToolOutputs(t *testing.T) {
	mapper := storage.NewInMemoryMapper("https://cdn.example.com")
	registry := &fakeRegistry{}
	broker, err := NewAttachmentBroker(registry, mapper)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	png := []byte{0x89, 0x50, 0x4E, 0x47}
	encoded := base64.StdEncoding.EncodeToString(png)

	attachments := map[string]ports.Attachment{
		"[diagram.png]": {
			Name:        "diagram.png",
			MediaType:   "image/png",
			Data:        encoded,
			Description: "A sample diagram",
			Source:      "browser_tool",
		},
	}

	ctx := context.Background()
	result, err := broker.RegisterToolOutputs(ctx, RegisterToolOutputsRequest{
		Context:     &materialapi.RequestContext{RequestID: "req-1", ToolCallID: "tc-1"},
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("register outputs returned error: %v", err)
	}
if len(result) == 0 {
t.Fatalf("expected attachments to be returned")
}

	att, ok := result["[diagram.png]"]
	if !ok {
		t.Fatalf("expected legacy placeholder preservation, got %+v", result)
	}
	if att.URI == "" {
		t.Fatalf("expected CDN URI in normalized attachment: %+v", att)
	}
	if _, ok := result["[material:diagram.png]"]; !ok {
		t.Fatalf("expected material placeholder alongside original key")
	}

	if registry.lastRegister == nil || len(registry.lastRegister.Materials) != 1 {
		t.Fatalf("expected register request to contain material inputs")
	}
	if ttl := registry.lastRegister.Materials[0].RetentionTTLSeconds; ttl == 0 {
		t.Fatalf("expected retention ttl to be recorded")
	}
}

func TestAttachmentBrokerRejectsMissingPayload(t *testing.T) {
	mapper := storage.NewInMemoryMapper("https://cdn.example.com")
	registry := &fakeRegistry{}
	broker, _ := NewAttachmentBroker(registry, mapper)

	_, err := broker.RegisterToolOutputs(context.Background(), RegisterToolOutputsRequest{
		Context: &materialapi.RequestContext{RequestID: "req-1"},
		Attachments: map[string]ports.Attachment{
			"[missing.png]": {Name: "missing.png", MediaType: "image/png"},
		},
	})
	if err == nil {
		t.Fatalf("expected error for missing payload")
	}
}

type fakePublisher struct {
	materials []*materialapi.Material
}

func (f *fakePublisher) PublishMaterial(ctx context.Context, material *materialapi.Material) error {
	f.materials = append(f.materials, material)
	return nil
}

func TestAttachmentBrokerPublishesEvents(t *testing.T) {
	mapper := storage.NewInMemoryMapper("https://cdn.example.com")
	registry := &fakeRegistry{}
	publisher := &fakePublisher{}
	broker, err := NewAttachmentBroker(registry, mapper, WithEventPublisher(publisher))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attachments := map[string]ports.Attachment{
		"[diagram.png]": {
			Name:      "diagram.png",
			MediaType: "image/png",
			Data:      base64.StdEncoding.EncodeToString([]byte{0x89, 0x50, 0x4E, 0x47}),
		},
	}

	_, err = broker.RegisterToolOutputs(context.Background(), RegisterToolOutputsRequest{
		Context:             &materialapi.RequestContext{RequestID: "req-ev"},
		Attachments:         attachments,
		DefaultRetentionTTL: 2 * time.Hour,
	})
	if err != nil {
		t.Fatalf("register outputs returned error: %v", err)
	}

	if len(publisher.materials) != 1 {
		t.Fatalf("expected event publisher to receive material, got %d", len(publisher.materials))
	}
}
