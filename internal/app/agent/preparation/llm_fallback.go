package preparation

import (
	"context"
	"strings"
	"sync/atomic"

	"alex/internal/app/agent/llmclient"
	"alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

type pinnedRateLimitFallbackClient struct {
	primary       llm.LLMClient
	fallback      llm.LLMClient
	logger        agentports.Logger
	primaryLabel  string
	fallbackLabel string
	useFallback   atomic.Bool
}

var _ llm.StreamingLLMClient = (*pinnedRateLimitFallbackClient)(nil)

func newPinnedRateLimitFallbackClient(
	primary llm.LLMClient,
	fallback llm.LLMClient,
	logger agentports.Logger,
	primaryProfile runtimeconfig.LLMProfile,
	fallbackProfile runtimeconfig.LLMProfile,
) *pinnedRateLimitFallbackClient {
	if logger == nil {
		logger = agentports.NoopLogger{}
	}
	return &pinnedRateLimitFallbackClient{
		primary:       primary,
		fallback:      fallback,
		logger:        logger,
		primaryLabel:  profileLabel(primaryProfile),
		fallbackLabel: profileLabel(fallbackProfile),
	}
}

func (c *pinnedRateLimitFallbackClient) Model() string {
	if c.useFallback.Load() {
		return c.fallback.Model()
	}
	return c.primary.Model()
}

func (c *pinnedRateLimitFallbackClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if c.useFallback.Load() {
		return c.fallback.Complete(ctx, req)
	}
	resp, err := c.primary.Complete(ctx, req)
	if err == nil || !llmclient.IsRateLimitError(err) {
		return resp, err
	}
	c.activateFallback(err)
	return c.fallback.Complete(ctx, req)
}

func (c *pinnedRateLimitFallbackClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if c.useFallback.Load() {
		return llm.EnsureStreamingClient(c.fallback).(llm.StreamingLLMClient).StreamComplete(ctx, req, callbacks)
	}
	streamingPrimary := llm.EnsureStreamingClient(c.primary).(llm.StreamingLLMClient)
	resp, err := streamingPrimary.StreamComplete(ctx, req, callbacks)
	if err == nil || !llmclient.IsRateLimitError(err) {
		return resp, err
	}
	c.activateFallback(err)
	return llm.EnsureStreamingClient(c.fallback).(llm.StreamingLLMClient).StreamComplete(ctx, req, callbacks)
}

func (c *pinnedRateLimitFallbackClient) activateFallback(err error) {
	if c.useFallback.CompareAndSwap(false, true) {
		c.logger.Warn("Pinned LLM rate-limited on %s; switched to fallback %s for this run: %v", c.primaryLabel, c.fallbackLabel, err)
	}
}

func (s *ExecutionPreparationService) wrapPinnedRateLimitFallback(
	ctx context.Context,
	selectionPinned bool,
	task string,
	preloadedAttachments map[string]ports.Attachment,
	userAttachments []ports.Attachment,
	primaryProfile runtimeconfig.LLMProfile,
	primaryClient llm.LLMClient,
) llm.LLMClient {
	if !selectionPinned || primaryClient == nil {
		return primaryClient
	}
	fallbackProfile, ok := s.resolvePinnedFallbackProfile(task, preloadedAttachments, userAttachments)
	if !ok {
		return primaryClient
	}
	if sameLLMEndpoint(primaryProfile, fallbackProfile) {
		return primaryClient
	}

	fallbackClient, _, err := llmclient.GetIsolatedClientFromProfile(
		s.llmFactory,
		fallbackProfile,
		llmclient.CredentialRefresher(s.credentialRefresher),
		true,
	)
	if err != nil {
		s.logger.Warn("Pinned fallback client init failed; continue with pinned client: %v", err)
		return primaryClient
	}
	s.logger.Info("Pinned LLM fallback armed: primary=%s fallback=%s", profileLabel(primaryProfile), profileLabel(fallbackProfile))
	return newPinnedRateLimitFallbackClient(primaryClient, fallbackClient, s.logger, primaryProfile, fallbackProfile)
}

func (s *ExecutionPreparationService) resolvePinnedFallbackProfile(
	task string,
	preloadedAttachments map[string]ports.Attachment,
	userAttachments []ports.Attachment,
) (runtimeconfig.LLMProfile, bool) {
	profile := s.config.DefaultLLMProfile()
	if utils.IsBlank(profile.Provider) || utils.IsBlank(profile.Model) {
		return runtimeconfig.LLMProfile{}, false
	}
	if taskNeedsVision(task, preloadedAttachments, userAttachments) {
		if vision, ok := s.config.VisionLLMProfile(); ok {
			profile = vision
		}
	}
	return profile, true
}

func sameLLMEndpoint(a, b runtimeconfig.LLMProfile) bool {
	return strings.EqualFold(strings.TrimSpace(a.Provider), strings.TrimSpace(b.Provider)) &&
		strings.EqualFold(strings.TrimSpace(a.Model), strings.TrimSpace(b.Model)) &&
		strings.EqualFold(strings.TrimSpace(a.BaseURL), strings.TrimSpace(b.BaseURL))
}

func profileLabel(profile runtimeconfig.LLMProfile) string {
	return strings.TrimSpace(profile.Provider) + "/" + strings.TrimSpace(profile.Model)
}
