package coding

import "strings"

// TranslateTask converts a raw prompt into a TaskRequest.
func TranslateTask(prompt string) TaskRequest {
	return TaskRequest{Prompt: strings.TrimSpace(prompt)}
}
