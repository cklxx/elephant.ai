package channels

import "testing"

func TestSanitizeErrorForUser(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "service temporarily unavailable with model tag and streaming suffix",
			input: "task execution failed: think step failed: LLM call failed: [anthropic/claude-sonnet-4-6] Upstream service temporarily unavailable. Please retry. Streaming request failed after 31s.",
			want:  "AI 服务暂时不可用（claude-sonnet-4-6），请稍后重试",
		},
		{
			name:  "rate limit",
			input: "task execution failed: think step failed: LLM call failed: [openai/gpt-4o] Rate limit reached. The system will retry automatically. Retried 3 times over 10s.",
			want:  "AI 服务请求频率超限（gpt-4o），请稍后重试",
		},
		{
			name:  "authentication failure",
			input: "LLM call failed: [anthropic/claude-sonnet-4-6] Authentication failed. Please verify your API key.",
			want:  "AI 服务认证失败（claude-sonnet-4-6），请检查 API 密钥配置",
		},
		{
			name:  "timeout",
			input: "task execution failed: think step failed: LLM call failed: [openai/gpt-4o] Upstream service timed out. Please retry. Streaming request failed after 5s.",
			want:  "AI 服务请求超时（gpt-4o），请稍后重试",
		},
		{
			name:  "context length exceeded",
			input: "LLM call failed: [anthropic/claude-sonnet-4-6] Input exceeds the model's context window. The system will trim and retry.",
			want:  "输入内容超出 AI 模型上下文长度限制（claude-sonnet-4-6）",
		},
		{
			name:  "connection refused",
			input: "think step failed: LLM call failed: Failed to reach LLM provider. Please retry shortly.",
			want:  "网络连接失败，请检查网络状态后重试",
		},
		{
			name:  "request rejected by upstream",
			input: "task execution failed: think step failed: LLM call failed: Request was rejected by the upstream service. Streaming request failed after 1s.",
			want:  "AI 服务暂时不可用，请稍后重试",
		},
		{
			name:  "HTTP 400 request rejected with model",
			input: "LLM call failed: [openai/kimi-for-coding] Request rejected (HTTP 400): some error detail",
			want:  "AI 服务请求被拒绝（kimi-for-coding），请检查请求参数",
		},
		{
			name:  "model not found",
			input: "LLM call failed: [openai/nonexistent] Model not found (HTTP 404): no such model",
			want:  "AI 模型配置错误（nonexistent），请检查模型名称设置",
		},
		{
			name:  "nil response",
			input: "think step failed: LLM call failed: nil response",
			want:  "AI 服务返回空结果，请重试",
		},
		{
			name:  "responses input empty",
			input: "task execution failed: think step failed: LLM call failed: responses input is empty after converting 2 message(s) — nothing to send Streaming request failed after 0s.",
			want:  "消息内容为空，无法发送给 AI",
		},
		{
			name:  "rate limit exceeded for user",
			input: "task execution failed: think step failed: LLM call failed: llm rate limit exceeded for user",
			want:  "AI 服务请求频率超限，请稍后重试",
		},
		{
			name:  "task execution with IDs stripped",
			input: "task execution failed (task_id=abc, session_id=xyz): think step failed: LLM call failed: deadline exceeded",
			want:  "AI 服务请求超时，请稍后重试",
		},
		{
			name:  "no model tag - plain unavailable",
			input: "Upstream service temporarily unavailable. Please retry.",
			want:  "AI 服务暂时不可用，请稍后重试",
		},
		{
			name:  "unknown error truncated",
			input: "some completely unknown and very long error message that goes on and on and on and on and would be confusing to a user if shown verbatim so it should be truncated at a reasonable length to avoid overwhelming them",
			want:  "some completely unknown and very long error message that goes on and on and on and on and would be confusing to a user if shown verbatim so it should …",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeErrorForUser(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeErrorForUser(%q)\n  got  = %q\n  want = %q", tt.input, got, tt.want)
			}
		})
	}
}
