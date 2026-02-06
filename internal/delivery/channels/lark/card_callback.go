package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"alex/internal/shared/logging"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

const maxCardCallbackBodyBytes = 1 << 20

// NewCardCallbackHandler returns an HTTP handler for Lark card callbacks.
// It returns nil only when cards are disabled.
func NewCardCallbackHandler(gateway *Gateway, logger logging.Logger) http.Handler {
	if gateway == nil {
		return nil
	}
	cfg := gateway.cfg
	if !cfg.CardsEnabled {
		return nil
	}
	verificationToken := strings.TrimSpace(cfg.CardCallbackVerificationToken)
	if verificationToken == "" {
		logging.OrNop(logger).Warn("Lark card callback verification token missing: url verification challenge may fail")
	}
	encryptKey := strings.TrimSpace(cfg.CardCallbackEncryptKey)
	plainDispatcher := dispatcher.NewEventDispatcher(verificationToken, "")
	plainDispatcher.OnP2CardActionTrigger(gateway.handleCardAction)

	var encryptedDispatcher *dispatcher.EventDispatcher
	var encryptedNoSignDispatcher *dispatcher.EventDispatcher
	if encryptKey != "" {
		encryptedDispatcher = dispatcher.NewEventDispatcher(verificationToken, encryptKey)
		encryptedDispatcher.OnP2CardActionTrigger(gateway.handleCardAction)

		encryptedNoSignDispatcher = dispatcher.NewEventDispatcher(verificationToken, encryptKey)
		encryptedNoSignDispatcher.InitConfig(larkevent.WithSkipSignVerify(true))
		encryptedNoSignDispatcher.OnP2CardActionTrigger(gateway.handleCardAction)
	}

	return &cardCallbackHandler{
		plaintextDispatcher:       plainDispatcher,
		encryptedDispatcher:       encryptedDispatcher,
		encryptedNoSignDispatcher: encryptedNoSignDispatcher,
		logger:                    logging.OrNop(logger),
	}
}

type cardCallbackHandler struct {
	plaintextDispatcher       *dispatcher.EventDispatcher
	encryptedDispatcher       *dispatcher.EventDispatcher
	encryptedNoSignDispatcher *dispatcher.EventDispatcher
	logger                    logging.Logger
}

func (h *cardCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.plaintextDispatcher == nil {
		http.Error(w, "handler not ready", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCardCallbackBodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}

	disp := h.plaintextDispatcher
	if isEncryptedCallbackPayload(body) && h.encryptedDispatcher != nil {
		if hasLarkCallbackSignatureHeaders(r.Header) {
			disp = h.encryptedDispatcher
		} else if h.encryptedNoSignDispatcher != nil {
			h.logger.Warn("Lark card callback missing signature headers; fallback to skip-sign verification")
			disp = h.encryptedNoSignDispatcher
		}
	}

	req := &larkevent.EventReq{
		Header:     r.Header,
		Body:       body,
		RequestURI: r.RequestURI,
	}
	resp := disp.Handle(r.Context(), req)
	if resp == nil {
		http.Error(w, "empty response", http.StatusInternalServerError)
		return
	}
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(resp.Body)
}

func isEncryptedCallbackPayload(body []byte) bool {
	var envelope struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	return strings.TrimSpace(envelope.Encrypt) != ""
}

func hasLarkCallbackSignatureHeaders(header http.Header) bool {
	if header == nil {
		return false
	}
	signature := strings.TrimSpace(header.Get(larkevent.EventSignature))
	timestamp := strings.TrimSpace(header.Get(larkevent.EventRequestTimestamp))
	nonce := strings.TrimSpace(header.Get(larkevent.EventRequestNonce))
	return signature != "" && timestamp != "" && nonce != ""
}

func (g *Gateway) handleCardAction(_ context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	if g == nil || event == nil || event.Event == nil {
		return cardToast("无效操作"), nil
	}
	action := event.Event.Action
	if action == nil {
		return cardToast("无效操作"), nil
	}

	chatID := ""
	messageID := ""
	if event.Event.Context != nil {
		chatID = strings.TrimSpace(event.Event.Context.OpenChatID)
		messageID = strings.TrimSpace(event.Event.Context.OpenMessageID)
	}
	tag := strings.ToLower(strings.TrimSpace(action.Tag))
	if tag == "attachment_send" {
		if chatID == "" {
			return cardToast("缺少会话信息"), nil
		}
		imageKey := extractActionValue(action, "image_key")
		fileKey := extractActionValue(action, "file_key")
		if imageKey == "" && fileKey == "" {
			return cardToast("附件信息缺失"), nil
		}
		go func() {
			ctx := context.Background()
			target := replyTarget(messageID, true)
			if imageKey != "" {
				g.dispatch(ctx, chatID, target, "image", imageContent(imageKey))
				return
			}
			g.dispatch(ctx, chatID, target, "file", fileContent(fileKey))
		}()
		return cardToast("附件已发送"), nil
	}

	input := cardActionToUserInput(action)
	if strings.TrimSpace(input) == "" {
		return cardToast("暂不支持的操作"), nil
	}

	senderID := ""
	if event.Event.Operator != nil {
		senderID = strings.TrimSpace(event.Event.Operator.OpenID)
		if senderID == "" && event.Event.Operator.UserID != nil {
			senderID = strings.TrimSpace(*event.Event.Operator.UserID)
		}
	}
	if chatID == "" || senderID == "" {
		return cardToast("缺少会话信息"), nil
	}

	go func() {
		ctx := context.Background()
		if err := g.InjectMessage(ctx, chatID, "", senderID, "", input); err != nil {
			g.logger.Warn("Lark card action inject failed: %v", err)
		}
		if messageID != "" {
			g.logger.Info("Lark card action handled (chat=%s message=%s)", chatID, messageID)
		}
	}()

	return cardToast("已收到，处理中"), nil
}

func cardToast(content string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "success",
			Content: content,
		},
	}
}

func cardActionToUserInput(action *callback.CallBackAction) string {
	if action == nil {
		return ""
	}
	tag := strings.ToLower(strings.TrimSpace(action.Tag))
	switch tag {
	case "plan_review_approve":
		return "OK"
	case "plan_review_request_changes":
		if feedback := extractActionValue(action, "plan_feedback"); feedback != "" {
			return feedback
		}
		if feedback := strings.TrimSpace(action.InputValue); feedback != "" {
			return feedback
		}
		return "需要修改"
	case "confirm_yes":
		return "OK"
	case "confirm_no":
		return "取消"
	default:
		if input := strings.TrimSpace(action.InputValue); input != "" {
			return input
		}
		if text := extractActionValue(action, "text"); text != "" {
			return text
		}
	}
	return ""
}

func extractActionValue(action *callback.CallBackAction, key string) string {
	if action == nil {
		return ""
	}
	if action.FormValue != nil {
		if val, ok := action.FormValue[key]; ok {
			return strings.TrimSpace(fmt.Sprint(val))
		}
	}
	if action.Value != nil {
		if val, ok := action.Value[key]; ok {
			return strings.TrimSpace(fmt.Sprint(val))
		}
	}
	return ""
}
