package bootstrap

import (
	infracal "alex/internal/infra/calendar"
)

// CalendarStage returns a BootstrapStage that initializes the calendar port
// and wires it into the DI container.
func (f *Foundation) CalendarStage() BootstrapStage {
	return BootstrapStage{
		Name: "calendar", Required: false,
		Init: func() error {
			larkCfg := f.Config.Channels.LarkConfig()
			if larkCfg.AppID == "" || larkCfg.AppSecret == "" {
				return nil
			}
			provider := infracal.NewLarkCalendarProvider(infracal.LarkCalendarConfig{
				AppID:      larkCfg.AppID,
				AppSecret:  larkCfg.AppSecret,
				BaseDomain: larkCfg.BaseDomain,
			}, f.Logger)
			f.Container.CalendarPort = provider
			f.Logger.Info("Calendar port created (lark, app_id=%s)", larkCfg.AppID)
			return nil
		},
	}
}
