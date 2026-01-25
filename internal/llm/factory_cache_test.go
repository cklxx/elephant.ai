package llm

import (
	"testing"
	"time"

	portsllm "alex/internal/agent/ports/llm"
)

func TestFactoryCacheEvictsLRU(t *testing.T) {
	factory := NewFactory()
	factory.SetCacheOptions(2, time.Hour)

	cfg := portsllm.LLMConfig{}

	clientA1, err := factory.GetClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("expected client A1, got error: %v", err)
	}
	clientB1, err := factory.GetClient("mock", "model-b", cfg)
	if err != nil {
		t.Fatalf("expected client B1, got error: %v", err)
	}
	clientA2, err := factory.GetClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("expected client A2, got error: %v", err)
	}
	if clientA1 != clientA2 {
		t.Fatalf("expected cached client A to be reused")
	}

	if _, err := factory.GetClient("mock", "model-c", cfg); err != nil {
		t.Fatalf("expected client C, got error: %v", err)
	}

	clientB2, err := factory.GetClient("mock", "model-b", cfg)
	if err != nil {
		t.Fatalf("expected client B2, got error: %v", err)
	}
	if clientB1 == clientB2 {
		t.Fatalf("expected client B to be evicted and recreated")
	}
}

func TestFactoryCacheExpiresTTL(t *testing.T) {
	factory := NewFactory()
	factory.SetCacheOptions(2, 10*time.Millisecond)

	cfg := portsllm.LLMConfig{}

	client1, err := factory.GetClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("expected client, got error: %v", err)
	}

	time.Sleep(25 * time.Millisecond)

	client2, err := factory.GetClient("mock", "model-a", cfg)
	if err != nil {
		t.Fatalf("expected client after TTL, got error: %v", err)
	}
	if client1 == client2 {
		t.Fatalf("expected TTL to expire cached client")
	}
}
