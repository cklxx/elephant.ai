package builtin

import (
	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/utils"
	"os/exec"
)

// GetAllBuiltinTools returns a list of all builtin tools
func GetAllBuiltinTools() []Tool {
	return GetAllBuiltinToolsWithConfig(nil)
}

// GetAllBuiltinToolsWithConfig returns a list of all builtin tools with configuration
func GetAllBuiltinToolsWithConfig(configManager *config.Manager) []Tool {
	return GetAllBuiltinToolsWithAgent(configManager, nil)
}

// GetAllBuiltinToolsWithAgent returns a list of all builtin tools with configuration and agent access
func GetAllBuiltinToolsWithAgent(configManager *config.Manager, sessionManager *session.Manager) []Tool {

	// Create web search tool and configure it if config is available
	webSearchTool := CreateWebSearchTool()
	if configManager != nil {
		if apiKey, err := configManager.Get("tavilyApiKey"); err == nil {
			if apiKeyStr, ok := apiKey.(string); ok && apiKeyStr != "" {
				webSearchTool.SetAPIKey(apiKeyStr)
			}
		}
	}

	// Create web fetch tool with LLM client (following successful pattern)
	var webFetchTool *WebFetchTool
	if llmClient, err := llm.GetLLMInstance(llm.BasicModel); err == nil {
		webFetchTool = CreateWebFetchToolWithLLM(llmClient)
	} else {
		// Fallback without LLM client
		webFetchTool = CreateWebFetchTool()
	}

	tools := []Tool{
		// Thinking and reasoning tools
		NewThinkTool(),

		// Task management tools - now with direct session manager access
		CreateTodoReadToolWithSessionManager(sessionManager),
		CreateTodoUpdateToolWithSessionManager(sessionManager),

		// Search tools
		CreateGrepTool(),

		// File tools
		CreateFileReadTool(),
		CreateFileUpdateTool(),
		CreateFileReplaceTool(),
		CreateFileListTool(),

		// Search tools (conditionally include grep tools if ripgrep is available)
		CreateFindTool(),

		// Web tools
		webSearchTool,
		webFetchTool,

		// Shell tools
		CreateBashTool(),
		CreateCodeExecutorTool(),

		// Background command management tools
		CreateBashStatusTool(),
		CreateBashControlTool(),
	}

	// Add grep and ripgrep tools only if ripgrep is available
	if utils.CheckDependenciesQuiet() {
		tools = append(tools, CreateRipgrepTool())
	}

	// Add ast-grep tool if available (optional dependency)
	if isAstGrepAvailable() {
		tools = append(tools, CreateAstGrepTool())
	}

	return tools
}

// isAstGrepAvailable checks if ast-grep command is available
func isAstGrepAvailable() bool {
	// Use the same pattern as the ast-grep tool
	cmd := exec.Command("ast-grep", "--version")
	err := cmd.Run()
	return err == nil
}
