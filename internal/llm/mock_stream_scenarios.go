package llm

import (
	"strings"

	"alex/internal/agent/ports"
)

type mockStreamScenario struct {
	name   string
	match  func(ports.CompletionRequest) bool
	chunks []string
}

var defaultMockScenario = mockStreamScenario{
	name:  "default",
	match: func(ports.CompletionRequest) bool { return true },
	chunks: []string{
		"Mock ",
		"LLM ",
		"response",
	},
}

var mockStreamScenarios = []mockStreamScenario{
	{
		name:  "math.2_plus_2",
		match: messageContains("2 + 2", "2+2"),
		chunks: []string{
			"Let's compute 2 + 2.\n\n",
			"2 + 2 = 4.\n",
			"Therefore, the answer is 4.",
		},
	},
	{
		name:  "workflow.echo",
		match: messageContains("echo hello world"),
		chunks: []string{
			"Echoing your request:\n",
			"hello world\n",
			"Done.",
		},
	},
	{
		name:  "filesystem.list_files",
		match: messageContains("list files"),
		chunks: []string{
			"Here is a sample directory listing:\n",
			"- README.md\n",
			"- main.go\n",
			"- web/\n",
		},
	},
	{
		name:  "filesystem.create_test_file",
		match: messageContains("create a file called test.txt"),
		chunks: []string{
			"Creating test.txt with the requested contents...\n",
			"Wrote \"Hello World\" to the file.\n",
			"Operation complete.",
		},
	},
	{
		name:  "filesystem.read_test_file",
		match: messageContains("read the content of test.txt"),
		chunks: []string{
			"Reading test.txt...\n",
			"The file contains: \"Hello World\".",
		},
	},
	{
		name:  "workflow.count_to",
		match: messageContains("count to 5"),
		chunks: []string{
			"Counting to five:\n",
			"1, 2, 3, 4, 5.\n",
			"Sequence complete.",
		},
	},
}

func selectMockScenario(req ports.CompletionRequest) mockStreamScenario {
	for _, scenario := range mockStreamScenarios {
		if scenario.match(req) {
			return scenario
		}
	}
	return defaultMockScenario
}

func messageContains(substrings ...string) func(ports.CompletionRequest) bool {
	lowered := make([]string, 0, len(substrings))
	for _, candidate := range substrings {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			lowered = append(lowered, strings.ToLower(trimmed))
		}
	}

	return func(req ports.CompletionRequest) bool {
		if len(lowered) == 0 {
			return false
		}

		message := strings.ToLower(strings.TrimSpace(lastUserMessage(req)))
		if message == "" {
			return false
		}

		for _, candidate := range lowered {
			if strings.Contains(message, candidate) {
				return true
			}
		}

		return false
	}
}

func lastUserMessage(req ports.CompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if strings.EqualFold(msg.Role, "user") {
			return msg.Content
		}
	}

	if len(req.Messages) == 0 {
		return ""
	}

	return req.Messages[len(req.Messages)-1].Content
}
