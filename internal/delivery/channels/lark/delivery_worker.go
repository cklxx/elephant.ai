package lark

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"
)

const deliveryWorkerConcurrency = 8

func (g *Gateway) startDeliveryWorker(ctx context.Context) {
	if g == nil {
		return
	}
	if normalizeDeliveryMode(g.cfg.DeliveryMode) != DeliveryModeOutbox {
		return
	}
	if !g.cfg.DeliveryWorker.Enabled {
		g.logger.Warn("Lark delivery worker disabled while delivery_mode=outbox; falling back to direct dispatch")
		return
	}
	if g.deliveryOutboxStore == nil {
		g.logger.Warn("Lark delivery outbox store not configured while delivery_mode=outbox; falling back to direct dispatch")
		return
	}
	poll := g.cfg.DeliveryWorker.PollInterval
	if poll <= 0 {
		poll = defaultDeliveryWorkerPollInterval
	}

	g.cleanupWG.Add(1)
	go func() {
		defer g.cleanupWG.Done()
		ticker := time.NewTicker(poll)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.processDeliveryOutbox(ctx)
			}
		}
	}()
}

func (g *Gateway) processDeliveryOutbox(ctx context.Context) int {
	if g == nil || g.deliveryOutboxStore == nil {
		return 0
	}
	batchSize := g.cfg.DeliveryWorker.BatchSize
	if batchSize <= 0 {
		batchSize = defaultDeliveryWorkerBatchSize
	}
	now := g.currentTime()
	storeCtx := context.Background()
	if ctx != nil {
		storeCtx = context.WithoutCancel(ctx)
	}
	intents, err := g.deliveryOutboxStore.ClaimPending(storeCtx, batchSize, now)
	if err != nil {
		g.logger.Warn("Lark delivery outbox claim failed: %v", err)
		return 0
	}
	if len(intents) == 0 {
		return 0
	}

	maxAttempts := g.cfg.DeliveryWorker.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultDeliveryWorkerMaxAttempts
	}

	var processed atomic.Int32
	sem := make(chan struct{}, deliveryWorkerConcurrency)
	var wg sync.WaitGroup
	for _, intent := range intents {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(intent DeliveryIntent) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := g.deliverIntent(ctx, intent); err != nil {
				processed.Add(1)
				if !isRetryableDeliveryError(err) || intent.AttemptCount >= maxAttempts {
					if markErr := g.deliveryOutboxStore.MarkDead(storeCtx, intent.IntentID, err.Error()); markErr != nil {
						g.logger.Warn("Lark delivery mark dead failed: intent=%s err=%v", intent.IntentID, markErr)
					}
					g.logger.Warn("Lark terminal delivery dead-lettered: intent=%s attempts=%d err=%v", intent.IntentID, intent.AttemptCount, err)
					return
				}
				nextAttempt := now.Add(g.nextDeliveryBackoff(intent.AttemptCount))
				if markErr := g.deliveryOutboxStore.MarkRetry(storeCtx, intent.IntentID, nextAttempt, err.Error()); markErr != nil {
					g.logger.Warn("Lark delivery mark retry failed: intent=%s err=%v", intent.IntentID, markErr)
				}
				g.logger.Warn("Lark terminal delivery retry scheduled: intent=%s attempts=%d next=%s err=%v", intent.IntentID, intent.AttemptCount, nextAttempt.Format(time.RFC3339), err)
				return
			}
			processed.Add(1)
			if markErr := g.deliveryOutboxStore.MarkSent(storeCtx, intent.IntentID, g.currentTime()); markErr != nil {
				g.logger.Warn("Lark delivery mark sent failed: intent=%s err=%v", intent.IntentID, markErr)
			}
		}(intent)
	}
	wg.Wait()

	return int(processed.Load())
}

func (g *Gateway) deliverIntent(parentCtx context.Context, intent DeliveryIntent) error {
	dispatchCtx, cancel := detachedContext(parentCtx, 15*time.Second)
	defer cancel()

	edited := false
	if intent.ProgressMessageID != "" {
		if err := g.updateMessage(dispatchCtx, intent.ProgressMessageID, intent.MsgType, intent.Content); err != nil {
			g.logger.Warn("Lark outbox progress→reply edit failed, fallback to new message: intent=%s err=%v", intent.IntentID, err)
		} else {
			edited = true
		}
	}
	if !edited {
		replyTo := replyTarget(intent.ReplyToMessageID, true)
		if _, err := g.dispatchMessage(dispatchCtx, intent.ChatID, replyTo, intent.MsgType, intent.Content); err != nil {
			return err
		}
	}
	if len(intent.Attachments) > 0 {
		g.sendAttachments(dispatchCtx, intent.ChatID, intent.ReplyToMessageID, &agent.TaskResult{Attachments: intent.Attachments})
	}
	return nil
}

func (g *Gateway) nextDeliveryBackoff(attempt int) time.Duration {
	base := g.cfg.DeliveryWorker.BaseBackoff
	if base <= 0 {
		base = defaultDeliveryWorkerBaseBackoff
	}
	maxBackoff := g.cfg.DeliveryWorker.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = defaultDeliveryWorkerMaxBackoff
	}
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(base) * multiplier)
	if delay > maxBackoff {
		delay = maxBackoff
	}
	jitterRatio := g.cfg.DeliveryWorker.JitterRatio
	if jitterRatio > 0 {
		factor := 1 + ((rand.Float64()*2 - 1) * jitterRatio)
		delay = time.Duration(float64(delay) * factor)
	}
	if delay < base {
		delay = base
	}
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func isRetryableDeliveryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	lower := utils.TrimLower(err.Error())
	if lower == "" {
		return true
	}
	if strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") {
		return true
	}
	if strings.Contains(lower, "500") || strings.Contains(lower, "502") || strings.Contains(lower, "503") || strings.Contains(lower, "504") {
		return true
	}
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "temporary") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "eof") {
		return true
	}
	if strings.Contains(lower, "400") || strings.Contains(lower, "401") || strings.Contains(lower, "403") || strings.Contains(lower, "404") {
		return false
	}
	return true
}
