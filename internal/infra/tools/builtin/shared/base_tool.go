package shared

import "alex/internal/agent/ports"

// BaseTool provides default Definition() and Metadata() implementations.
// Tool structs embed BaseTool to avoid repeating these two getter methods.
//
//	type myTool struct {
//		shared.BaseTool
//	}
//
//	func NewMyTool() tools.ToolExecutor {
//		return &myTool{
//			BaseTool: shared.NewBaseTool(
//				ports.ToolDefinition{Name: "my_tool", ...},
//				ports.ToolMetadata{Name: "my_tool", ...},
//			),
//		}
//	}
type BaseTool struct {
	def  ports.ToolDefinition
	meta ports.ToolMetadata
}

// NewBaseTool constructs a BaseTool with the given definition and metadata.
func NewBaseTool(def ports.ToolDefinition, meta ports.ToolMetadata) BaseTool {
	return BaseTool{def: def, meta: meta}
}

// Definition returns the tool's schema for the LLM.
func (b *BaseTool) Definition() ports.ToolDefinition { return b.def }

// Metadata returns the tool's runtime metadata.
func (b *BaseTool) Metadata() ports.ToolMetadata { return b.meta }
