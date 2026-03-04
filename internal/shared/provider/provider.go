package provider

import "alex/internal/shared/utils"

// Family constants.
const (
	FamilyAnthropic = "anthropic"
	FamilyCodex     = "codex"
	FamilyOpenAI    = "openai"
	FamilyLlamaCpp  = "llamacpp"
	FamilyMock      = "mock"
)

// familyMap maps every known provider alias to its canonical family.
var familyMap = map[string]string{
	"anthropic":        FamilyAnthropic,
	"claude":           FamilyAnthropic,
	"openai-responses": FamilyCodex,
	"responses":        FamilyCodex,
	"codex":            FamilyCodex,
	"openai":           FamilyOpenAI,
	"openrouter":       FamilyOpenAI,
	"deepseek":         FamilyOpenAI,
	"kimi":             FamilyOpenAI,
	"glm":              FamilyOpenAI,
	"minimax":          FamilyOpenAI,
	"llama.cpp":        FamilyLlamaCpp,
	"llama-cpp":        FamilyLlamaCpp,
	"llamacpp":         FamilyLlamaCpp,
	"mock":             FamilyMock,
}

// Family returns the canonical family for the given provider name.
// Returns the lowercased name as-is if the provider is not in the known set.
func Family(provider string) string {
	key := utils.TrimLower(provider)
	if f, ok := familyMap[key]; ok {
		return f
	}
	return key
}

// UsesCLIAuth reports whether the provider supports CLI-based credential loading
// (OAuth tokens, CLI login flows).
func UsesCLIAuth(provider string) bool {
	switch Family(provider) {
	case FamilyAnthropic, FamilyCodex:
		return true
	}
	// Virtual sentinels that trigger CLI loading.
	key := utils.TrimLower(provider)
	return key == "auto" || key == "cli"
}

// RequiresAPIKey reports whether the provider requires API key authentication.
func RequiresAPIKey(provider string) bool {
	key := utils.TrimLower(provider)
	switch key {
	case "", "mock", "llama.cpp", "llamacpp", "llama-cpp", "ollama":
		return false
	default:
		return true
	}
}
