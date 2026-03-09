package oauth

import (
	"context"
	"fmt"
	"time"
)

// DeviceFlowResult holds the result of initiating a device flow.
type DeviceFlowResult struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int
	Interval                int
}

// StartDeviceFlow initiates the device authorization flow.
// Returns the device code and verification URL for the user to visit.
func (s *Service) StartDeviceFlow(ctx context.Context, scopes []string) (*DeviceFlowResult, error) {
	cfg := DeviceFlowConfig{
		AppID:     s.cfg.AppID,
		AppSecret: s.cfg.AppSecret,
		Brand:     s.resolveBrand(),
		Scopes:    scopes,
	}

	resp, err := RequestDeviceAuthorization(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("start device flow: %w", err)
	}

	return &DeviceFlowResult{
		DeviceCode:              resp.DeviceCode,
		UserCode:                resp.UserCode,
		VerificationURI:         resp.VerificationURI,
		VerificationURIComplete: resp.VerificationURIComplete,
		ExpiresIn:               resp.ExpiresIn,
		Interval:                resp.Interval,
	}, nil
}

// PollAndStoreDeviceToken polls for the device flow token and stores it upon success.
// This should be called as a background goroutine after StartDeviceFlow.
// The openID parameter associates the resulting token with a user.
// It calls onSuccess when the token is obtained and stored successfully.
func (s *Service) PollAndStoreDeviceToken(
	ctx context.Context,
	deviceCode string,
	interval, expiresIn int,
	openID string,
	onSuccess func(token Token),
) error {
	cfg := DeviceFlowConfig{
		AppID:     s.cfg.AppID,
		AppSecret: s.cfg.AppSecret,
		Brand:     s.resolveBrand(),
	}

	tokenResp, err := PollDeviceToken(ctx, cfg, deviceCode, interval, expiresIn)
	if err != nil {
		return err
	}

	now := s.now()
	token := Token{
		OpenID:       openID,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scope:        tokenResp.Scope,
		TokenType:    tokenResp.TokenType,
		UpdatedAt:    now,
	}

	if err := s.tokens.Upsert(ctx, token); err != nil {
		return fmt.Errorf("store device flow token: %w", err)
	}

	if onSuccess != nil {
		onSuccess(token)
	}

	return nil
}

// resolveBrand maps the base domain to a brand for endpoint resolution.
func (s *Service) resolveBrand() string {
	bd := s.cfg.BaseDomain
	switch {
	case bd == "" || bd == "https://open.feishu.cn":
		return "feishu"
	case bd == "https://open.larksuite.com":
		return "lark"
	default:
		return bd
	}
}
