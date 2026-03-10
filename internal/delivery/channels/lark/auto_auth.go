package lark

import (
	"context"
	"errors"
	"sync"
	"time"

	oauth "alex/internal/infra/lark/oauth"
	"alex/internal/shared/logging"
)

// AutoAuth manages automatic in-message OAuth authorization when tool calls
// fail due to missing or expired user tokens.
//
// When a tool call fails with NeedUserAuthError, AutoAuth:
//  1. Initiates a device flow (RFC 8628) to get a device code + verification URL
//  2. Sends an interactive card with a "Go to authorize" button into the chat
//  3. Polls in the background for user authorization
//  4. On success: updates the card to "success" and injects a synthetic message
//     to trigger the AI to retry the failed operation
type AutoAuth struct {
	oauthService *oauth.Service
	messenger    LarkMessenger
	logger       logging.Logger

	mu           sync.Mutex
	pendingFlows map[string]pendingFlow // key: "appId:openId"
	cooldowns    map[string]time.Time   // key: "appId:openId" -> earliest next auth card
}

type pendingFlow struct {
	deviceCode string
	cardMsgID  string
	cancel     context.CancelFunc
}

// NewAutoAuth creates an AutoAuth instance.
func NewAutoAuth(oauthService *oauth.Service, messenger LarkMessenger, logger logging.Logger) *AutoAuth {
	return &AutoAuth{
		oauthService: oauthService,
		messenger:    messenger,
		logger:       logger,
		pendingFlows: make(map[string]pendingFlow),
		cooldowns:    make(map[string]time.Time),
	}
}

// HandleAuthError checks if an error is an auth error and triggers the
// device flow authorization card. Returns true if an auth flow was initiated.
func (a *AutoAuth) HandleAuthError(ctx context.Context, err error, chatID, openID string, scopes []string) bool {
	if a == nil || a.oauthService == nil {
		return false
	}

	var needAuth *oauth.NeedUserAuthError
	if !errors.As(err, &needAuth) {
		return false
	}

	flowKey := openID

	a.mu.Lock()
	// Check cooldown (30 seconds between auth cards for the same user).
	if cooldown, ok := a.cooldowns[flowKey]; ok && time.Now().Before(cooldown) {
		a.mu.Unlock()
		a.logger.Info("auto_auth: cooldown active for %s, skipping", flowKey)
		return true
	}

	// Cancel any existing flow for this user.
	if existing, ok := a.pendingFlows[flowKey]; ok {
		existing.cancel()
		delete(a.pendingFlows, flowKey)
	}
	a.mu.Unlock()

	// Start device flow.
	result, err := a.oauthService.StartDeviceFlow(ctx, scopes)
	if err != nil {
		a.logger.Error("auto_auth: start device flow failed: %v", err)
		return false
	}

	// Send auth card.
	cardJSON := buildAuthCard(result.VerificationURIComplete, result.UserCode, scopes, result.ExpiresIn)
	msgID, err := a.messenger.SendMessage(ctx, chatID, "interactive", cardJSON)
	if err != nil {
		a.logger.Error("auto_auth: send auth card failed: %v", err)
		return false
	}

	a.logger.Info("auto_auth: sent auth card to chat=%s msg=%s, user_code=%s", chatID, msgID, result.UserCode)

	// Set cooldown.
	a.mu.Lock()
	a.cooldowns[flowKey] = time.Now().Add(30 * time.Second)

	// Start background polling.
	pollCtx, pollCancel := context.WithTimeout(context.Background(), time.Duration(result.ExpiresIn)*time.Second)
	a.pendingFlows[flowKey] = pendingFlow{
		deviceCode: result.DeviceCode,
		cardMsgID:  msgID,
		cancel:     pollCancel,
	}
	a.mu.Unlock()

	go a.pollAndFinalize(pollCtx, flowKey, result, chatID, openID, msgID)

	return true
}

func (a *AutoAuth) pollAndFinalize(
	ctx context.Context,
	flowKey string,
	result *oauth.DeviceFlowResult,
	chatID, openID, cardMsgID string,
) {
	defer func() {
		a.mu.Lock()
		if f, ok := a.pendingFlows[flowKey]; ok && f.deviceCode == result.DeviceCode {
			f.cancel()
			delete(a.pendingFlows, flowKey)
		}
		a.mu.Unlock()
	}()

	err := a.oauthService.PollAndStoreDeviceToken(
		ctx,
		result.DeviceCode,
		result.Interval,
		result.ExpiresIn,
		openID,
		func(token oauth.Token) {
			a.logger.Info("auto_auth: device flow succeeded for %s", openID)

			// Update card to success state.
			successCard := buildAuthSuccessCard()
			if updateErr := a.messenger.UpdateMessage(context.Background(), cardMsgID, "interactive", successCard); updateErr != nil {
				a.logger.Warn("auto_auth: update card to success failed: %v", updateErr)
			}

			// Send a synthetic message to trigger the AI to retry.
			syntheticContent := `{"text":"I have completed Feishu authorization, please continue the previous operation."}`
			if _, sendErr := a.messenger.SendMessage(context.Background(), chatID, "text", syntheticContent); sendErr != nil {
				a.logger.Warn("auto_auth: send synthetic message failed: %v", sendErr)
			}
		},
	)

	if err != nil {
		a.logger.Warn("auto_auth: device flow polling ended: %v", err)

		reason := "Authorization timed out. Please try the operation again to get a new authorization link."
		if errors.Is(err, oauth.ErrDeviceFlowDenied) {
			reason = "Authorization was denied. Please try again if this was unintentional."
		}

		failedCard := buildAuthFailedCard(reason)
		if updateErr := a.messenger.UpdateMessage(context.Background(), cardMsgID, "interactive", failedCard); updateErr != nil {
			a.logger.Warn("auto_auth: update card to failed state: %v", updateErr)
		}
	}
}
