package builtin

import (
	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/utils"
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
	}

	// Add grep and ripgrep tools only if ripgrep is available
	if utils.CheckDependenciesQuiet() {
		tools = append(tools, CreateRipgrepTool())
	}

	return tools
}


// GetToolByName creates a tool instance by name
func GetToolByName(name string) Tool {
	return GetToolByNameWithConfig(name, nil)
}

// GetToolByNameWithConfig creates a tool instance by name with configuration
func GetToolByNameWithConfig(name string, configManager *config.Manager) Tool {

	switch name {
	case "think":
		return NewThinkTool()
	case "todo_read":
		return CreateTodoReadTool()
	case "todo_update":
		return CreateNewTodoUpdateTool()
	case "file_read":
		return CreateFileReadTool()
	case "file_update":
		return CreateFileUpdateTool()
	case "file_replace":
		return CreateFileReplaceTool()
	case "file_list":
		return CreateFileListTool()
	case "grep":
		return CreateGrepTool()
	case "ripgrep":
		if utils.CheckDependenciesQuiet() {
			return CreateRipgrepTool()
		}
		return nil
	case "find":
		return CreateFindTool()
	case "web_search":
		webSearchTool := CreateWebSearchTool()
		if configManager != nil {
			if apiKey, err := configManager.Get("tavilyApiKey"); err == nil {
				if apiKeyStr, ok := apiKey.(string); ok && apiKeyStr != "" {
					webSearchTool.SetAPIKey(apiKeyStr)
				}
			}
		}
		return webSearchTool
	case "web_fetch":
		// Try to get LLM client for web_fetch tool
		if llmClient, err := llm.GetLLMInstance(llm.BasicModel); err == nil {
			return CreateWebFetchToolWithLLM(llmClient)
		}
		// Fallback without LLM client
		return CreateWebFetchTool()
	case "bash":
		return CreateBashTool()
	case "code_execute":
		return CreateCodeExecutorTool()
	default:
		return nil
	}
}

// GetToolsByCategory returns tools grouped by category
func GetToolsByCategory() map[string][]Tool {
	return GetToolsByCategoryWithConfig(nil)
}

// GetToolsByCategoryWithConfig returns tools grouped by category with configuration
func GetToolsByCategoryWithConfig(configManager *config.Manager) map[string][]Tool {

	// Create web search tools and configure them if config is available
	webSearchTool := CreateWebSearchTool()

	if configManager != nil {
		if apiKey, err := configManager.Get("tavilyApiKey"); err == nil {
			if apiKeyStr, ok := apiKey.(string); ok && apiKeyStr != "" {
				webSearchTool.SetAPIKey(apiKeyStr)
			}
		}
	}

	searchTools := []Tool{CreateFindTool(), CreateGrepTool()}
	if utils.CheckDependenciesQuiet() {
		searchTools = append(searchTools, CreateRipgrepTool())
	}

	return map[string][]Tool{
		"reasoning": {
			NewThinkTool(),
		},
		"task_management": {
			CreateTodoReadTool(),
			CreateNewTodoUpdateTool(),
		},
		"file": {
			CreateFileReadTool(),
			CreateFileUpdateTool(),
			CreateFileReplaceTool(),
			CreateFileListTool(),
		},
		"search": searchTools,
		"web": {
			webSearchTool,
		},
		"execution": {
			CreateBashTool(),
			CreateCodeExecutorTool(),
		},
	}
}
