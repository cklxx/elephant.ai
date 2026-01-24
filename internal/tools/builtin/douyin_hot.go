package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/httpclient"
)

const douyinHotURL = "https://www.iesdouyin.com/web/api/v2/hotsearch/billboard/word/"

type douyinHot struct {
	client *http.Client
}

func NewDouyinHot() ports.ToolExecutor {
	return &douyinHot{client: httpclient.NewWithCircuitBreaker(10*time.Second, nil, "douyin_hot")}
}

func (t *douyinHot) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "douyin_hot",
		Version:  "1.0.0",
		Category: "web",
		Tags:     []string{"douyin", "trending", "hotlist"},
	}
}

func (t *douyinHot) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "douyin_hot",
		Description: "Fetch current Douyin trending keywords for ideation and search seeding.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"limit": {
					Type:        "integer",
					Description: "Maximum number of hot keywords to return (default 20).",
				},
			},
		},
	}
}

type douyinHotResponse struct {
	StatusCode int `json:"status_code"`
	WordList   []struct {
		Word     string `json:"word"`
		HotValue int64  `json:"hot_value"`
		Label    int    `json:"label"`
	} `json:"word_list"`
}

func (t *douyinHot) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	limit := 20
	switch raw := call.Arguments["limit"].(type) {
	case float64:
		if raw > 0 {
			limit = int(raw)
		}
	case int:
		if raw > 0 {
			limit = raw
		}
	}
	if limit > 50 {
		limit = 50
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, douyinHotURL, nil)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("build request: %v", err), Error: err}, nil
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("request failed: %v", err), Error: err}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	var payload douyinHotResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("decode response: %v", err), Error: err}, nil
	}

	if payload.StatusCode != 0 || len(payload.WordList) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "douyin hot list unavailable"}, nil
	}

	sort.Slice(payload.WordList, func(i, j int) bool {
		return payload.WordList[i].HotValue > payload.WordList[j].HotValue
	})

	if limit > len(payload.WordList) {
		limit = len(payload.WordList)
	}
	top := payload.WordList[:limit]

	var b strings.Builder
	b.WriteString("Douyin hot keywords:\n\n")
	keywords := make([]string, 0, len(top))
	for idx, item := range top {
		keyword := strings.TrimSpace(item.Word)
		if keyword == "" {
			continue
		}
		keywords = append(keywords, keyword)
		fmt.Fprintf(&b, "%d. %s (heat: %d)\n", idx+1, keyword, item.HotValue)
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: b.String(),
		Metadata: map[string]any{
			"results_count": len(keywords),
			"keywords":      keywords,
		},
	}, nil
}
