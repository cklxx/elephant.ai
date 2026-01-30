package app

import "errors"

const (
	shareTokenMetadataKey   = "share_token"
	shareEnabledMetadataKey = "share_enabled"
)

// ErrShareTokenInvalid signals an invalid or missing share token.
var ErrShareTokenInvalid = errors.New("share token invalid")
