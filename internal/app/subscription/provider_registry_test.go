package subscription

import "testing"

func TestLookupProviderPresetReturnsCopiedRecommendations(t *testing.T) {
	first, ok := LookupProviderPreset(" OpenAI ")
	if !ok {
		t.Fatal("expected openai preset")
	}
	if len(first.RecommendedModels) == 0 {
		t.Fatal("expected recommended models")
	}

	first.RecommendedModels[0].ID = "mutated-model"

	second, ok := LookupProviderPreset("openai")
	if !ok {
		t.Fatal("expected openai preset")
	}
	if len(second.RecommendedModels) == 0 {
		t.Fatal("expected recommended models")
	}
	if second.RecommendedModels[0].ID == "mutated-model" {
		t.Fatal("expected lookup to return a defensive copy")
	}
}

func TestListProviderPresetsReturnsSortedCopies(t *testing.T) {
	first := ListProviderPresets()
	if len(first) == 0 {
		t.Fatal("expected non-empty presets")
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].Provider > first[i].Provider {
			t.Fatalf("expected sorted provider IDs, got %q before %q", first[i-1].Provider, first[i].Provider)
		}
	}

	openAIFirst := findProviderPreset(first, "openai")
	if openAIFirst == nil || len(openAIFirst.RecommendedModels) == 0 {
		t.Fatal("expected openai recommendation list")
	}
	openAIFirst.RecommendedModels[0].ID = "mutated-model"

	second := ListProviderPresets()
	openAISecond := findProviderPreset(second, "openai")
	if openAISecond == nil || len(openAISecond.RecommendedModels) == 0 {
		t.Fatal("expected openai recommendation list")
	}
	if openAISecond.RecommendedModels[0].ID == "mutated-model" {
		t.Fatal("expected list call to return defensive copies")
	}
}

func findProviderPreset(items []ProviderPreset, provider string) *ProviderPreset {
	for i := range items {
		if items[i].Provider == provider {
			return &items[i]
		}
	}
	return nil
}
