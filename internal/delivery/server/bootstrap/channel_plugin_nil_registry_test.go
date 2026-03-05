package bootstrap

import "testing"

func TestRegisterLarkChannelNilRegistryNoPanic(t *testing.T) {
	registry := NewChannelRegistry()
	channelsCfg := ChannelsConfig{Registry: registry}
	channelsCfg.SetLarkConfig(LarkGatewayConfig{Enabled: true})
	cfg := Config{Channels: channelsCfg}

	registerLarkChannel(cfg, nil, nil, nil, nil)
}

func TestRegisterTelegramChannelNilRegistryNoPanic(t *testing.T) {
	registry := NewChannelRegistry()
	channelsCfg := ChannelsConfig{Registry: registry}
	channelsCfg.SetTelegramConfig(TelegramGatewayConfig{Enabled: true})
	cfg := Config{Channels: channelsCfg}

	registerTelegramChannel(cfg, nil, nil, nil, nil)
}
