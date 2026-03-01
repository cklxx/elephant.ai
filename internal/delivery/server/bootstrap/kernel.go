package bootstrap

import (
	"context"
	"fmt"
	"strings"

	kernel "alex/internal/app/agent/kernel"
	"alex/internal/app/lifecycle"
	"alex/internal/app/scheduler"
	larkgw "alex/internal/delivery/channels/lark"
	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"

	larksdk "github.com/larksuite/oapi-sdk-go/v3"
)

type kernelNoticeLoader func() (string, bool, error)
type kernelNoticeSender func(ctx context.Context, chatID, text string) error
type kernelNarrator func(ctx context.Context, rawText string) (string, error)

func resolveKernelNoticePipeline(f *Foundation, logger logging.Logger) (kernelNoticeLoader, kernelNoticeSender, kernelNarrator) {
	if f == nil || f.Container == nil {
		return nil, nil, nil
	}

	// Prefer the live Lark gateway when available.
	if gw := f.Container.LarkGateway; gw != nil {
		if loader := gw.NoticeLoader(); loader != nil {
			return loader, gw.SendNotification, gw.NarrateCycleNotification
		}
	}

	// kernel-daemon mode has no Lark gateway; fall back to local notice-state
	// file loading plus direct Lark message sending.
	sender := newKernelDirectLarkSender(f.Config.Channels.Lark, logger)
	if sender == nil {
		return nil, nil, nil
	}
	return larkgw.NewNoticeStateLoader(logger), sender, nil
}

func newKernelDirectLarkSender(cfg LarkGatewayConfig, logger logging.Logger) kernelNoticeSender {
	if !cfg.Enabled {
		return nil
	}
	appID := strings.TrimSpace(cfg.AppID)
	appSecret := strings.TrimSpace(cfg.AppSecret)
	if appID == "" || appSecret == "" {
		return nil
	}

	var clientOpts []larksdk.ClientOptionFunc
	if domain := strings.TrimSpace(cfg.BaseDomain); domain != "" {
		clientOpts = append(clientOpts, larksdk.WithOpenBaseUrl(domain))
	}
	client := larksdk.NewClient(appID, appSecret, clientOpts...)
	notifier := scheduler.NewLarkNotifierWithClient(client, logger)
	return notifier.SendLark
}

// KernelStage returns a BootstrapStage that starts the kernel agent loop engine
// if enabled.
func (f *Foundation) KernelStage(sm *SubsystemManager) BootstrapStage {
	logger := logging.NewKernelLogger("KernelStage")
	return BootstrapStage{
		Name: "kernel", Required: false,
		Init: func() error {
			if f.Container.KernelEngine == nil {
				return fmt.Errorf("kernel engine unavailable")
			}

			// Wire cycle notifier via /notice binding.
			if loader, sender, narrator := resolveKernelNoticePipeline(f, logger); loader != nil && sender != nil {
				kernelID := kernel.DefaultRuntimeSettings().KernelID
				if kernelID == "" {
					kernelID = kernel.DefaultKernelID
				}
				f.Container.KernelEngine.SetNotifier(func(ctx context.Context, result *kerneldomain.CycleResult, err error) {
					chatID, ok, loadErr := loader()
					if loadErr != nil {
						logger.Warn("Kernel notifier: load notice target: %v", loadErr)
						return
					}
					if !ok {
						return // notice not bound, skip
					}

					// Try LLM narration of the structured data, fall back to template.
					raw := kernel.FormatCycleNotification(kernelID, result, err)
					var narrated string
					if narrator != nil {
						narrated, _ = narrator(ctx, raw)
					}
					if narrated == "" {
						narrated = kernel.NarrateCycleFallback(result, err)
					}

					// Append metrics line for quantitative context.
					text := narrated
					if metrics := kernel.FormatCycleMetrics(result); metrics != "" {
						text = narrated + "\n" + metrics
					}

					if sendErr := sender(ctx, chatID, text); sendErr != nil {
						logger.Warn("Kernel notifier: send failed: %v", sendErr)
					}
				})
			}

			logger.Info("Starting kernel engine subsystem")
			return sm.Start(context.Background(), &gatewaySubsystem{
				name: "kernel",
				startFn: func(ctx context.Context) (func(), error) {
					engine := f.Container.KernelEngine
					async.Go(f.Logger, "kernel-engine", func() {
						engine.Run(ctx)
					})
					// Register as drainable so graceful shutdown waits for in-flight cycles.
					if drainable, ok := engine.(lifecycle.Drainable); ok {
						f.Container.Drainables = append(f.Container.Drainables, drainable)
					}
					logger.Info("Kernel engine subsystem started")
					return engine.Stop, nil
				},
			})
		},
	}
}
