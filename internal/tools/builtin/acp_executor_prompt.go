package builtin

import (
	"context"
	"sort"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tools "alex/internal/agent/ports/tools"

	"gopkg.in/yaml.v3"
)

func buildPromptBlocks(prompt string, attachmentNames []string, ctx context.Context) []any {
	blocks := []any{map[string]any{"type": "text", "text": prompt}}
	if len(attachmentNames) == 0 {
		return blocks
	}
	attachments, _ := tools.GetAttachmentContext(ctx)
	for _, name := range attachmentNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		att, ok := attachments[name]
		if !ok {
			continue
		}
		if block := attachmentToContentBlock(att); block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

type executorTaskPackage struct {
	SessionID    string          `yaml:"session_id"`
	TaskID       string          `yaml:"task_id,omitempty"`
	ParentTaskID string          `yaml:"parent_task_id,omitempty"`
	Instruction  string          `yaml:"instruction"`
	Context      executorContext `yaml:"context,omitempty"`
	Runtime      executorRuntime `yaml:"runtime"`
}

type executorContext struct {
	SystemPrompt string                     `yaml:"system_prompt,omitempty"`
	Messages     []executorMessage          `yaml:"messages,omitempty"`
	Attachments  []executorAttachment       `yaml:"attachments,omitempty"`
	Important    []ports.ImportantNote      `yaml:"important,omitempty"`
	Plans        []agent.PlanNode           `yaml:"plans,omitempty"`
	Beliefs      []agent.Belief             `yaml:"beliefs,omitempty"`
	Knowledge    []agent.KnowledgeReference `yaml:"knowledge_refs,omitempty"`
	WorldState   map[string]any             `yaml:"world_state,omitempty"`
	WorldDiff    map[string]any             `yaml:"world_diff,omitempty"`
	Feedback     []agent.FeedbackSignal     `yaml:"feedback,omitempty"`
	Meta         executorContextMeta        `yaml:"meta,omitempty"`
}

type executorContextMeta struct {
	Iterations   int `yaml:"iterations,omitempty"`
	TokenCount   int `yaml:"token_count,omitempty"`
	MessageCount int `yaml:"message_count,omitempty"`
}

type executorRuntime struct {
	CWD             string         `yaml:"cwd"`
	ToolMode        string         `yaml:"tool_mode,omitempty"`
	Limits          executorLimits `yaml:"limits,omitempty"`
	RequireManifest bool           `yaml:"require_manifest"`
}

type executorLimits struct {
	MaxCLICalls        int `yaml:"max_cli_calls,omitempty"`
	MaxDurationSeconds int `yaml:"max_duration_seconds,omitempty"`
}

type executorMessage struct {
	Role        string               `yaml:"role"`
	Content     string               `yaml:"content"`
	ToolCalls   []ports.ToolCall     `yaml:"tool_calls,omitempty"`
	ToolResults []executorToolResult `yaml:"tool_results,omitempty"`
	Metadata    map[string]any       `yaml:"metadata,omitempty"`
	Attachments []executorAttachment `yaml:"attachments,omitempty"`
	Source      ports.MessageSource  `yaml:"source,omitempty"`
}

type executorToolResult struct {
	CallID      string               `yaml:"call_id"`
	Content     string               `yaml:"content"`
	Error       string               `yaml:"error,omitempty"`
	Metadata    map[string]any       `yaml:"metadata,omitempty"`
	Attachments []executorAttachment `yaml:"attachments,omitempty"`
}

type executorAttachment struct {
	Name        string `yaml:"name"`
	MediaType   string `yaml:"media_type,omitempty"`
	URI         string `yaml:"uri,omitempty"`
	Source      string `yaml:"source,omitempty"`
	Description string `yaml:"description,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	Format      string `yaml:"format,omitempty"`
}

func buildExecutorPromptBlocks(ctx context.Context, instruction string, call ports.ToolCall, cfg ACPExecutorConfig, attachmentNames []string) ([]any, error) {
	pkg := executorTaskPackage{
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
		Instruction:  instruction,
		Runtime: executorRuntime{
			CWD:      cfg.CWD,
			ToolMode: strings.TrimSpace(cfg.Mode),
			Limits: executorLimits{
				MaxCLICalls:        cfg.MaxCLICalls,
				MaxDurationSeconds: cfg.MaxDurationSeconds,
			},
			RequireManifest: cfg.RequireArtifactManifest,
		},
	}

	if snapshot := agent.GetTaskStateSnapshot(ctx); snapshot != nil {
		pkg.Context = executorContext{
			SystemPrompt: snapshot.SystemPrompt,
			Messages:     buildExecutorMessages(snapshot.Messages),
			Attachments:  buildExecutorAttachments(snapshot.Attachments),
			Important:    cloneImportantNotes(snapshot.Important),
			Plans:        agent.ClonePlanNodes(snapshot.Plans),
			Beliefs:      agent.CloneBeliefs(snapshot.Beliefs),
			Knowledge:    agent.CloneKnowledgeReferences(snapshot.KnowledgeRefs),
			WorldState:   cloneMapAny(snapshot.WorldState),
			WorldDiff:    cloneMapAny(snapshot.WorldDiff),
			Feedback:     agent.CloneFeedbackSignals(snapshot.FeedbackSignals),
			Meta: executorContextMeta{
				Iterations:   snapshot.Iterations,
				TokenCount:   snapshot.TokenCount,
				MessageCount: len(snapshot.Messages),
			},
		}
	}

	payload, err := yaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	prompt := "Task Package (YAML):\n" + string(payload)
	return buildPromptBlocks(prompt, attachmentNames, ctx), nil
}

func buildExecutorMessages(messages []ports.Message) []executorMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]executorMessage, 0, len(messages))
	for _, msg := range messages {
		outMsg := executorMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Source:  msg.Source,
		}
		if len(msg.ToolCalls) > 0 {
			outMsg.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
		}
		if len(msg.ToolResults) > 0 {
			outMsg.ToolResults = buildExecutorToolResults(msg.ToolResults)
		}
		if len(msg.Metadata) > 0 {
			meta := make(map[string]any, len(msg.Metadata))
			for k, v := range msg.Metadata {
				meta[k] = v
			}
			outMsg.Metadata = meta
		}
		if len(msg.Attachments) > 0 {
			outMsg.Attachments = buildExecutorAttachments(msg.Attachments)
		}
		out = append(out, outMsg)
	}
	return out
}

func buildExecutorToolResults(results []ports.ToolResult) []executorToolResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]executorToolResult, 0, len(results))
	for _, result := range results {
		outRes := executorToolResult{
			CallID:  result.CallID,
			Content: result.Content,
		}
		if result.Error != nil {
			outRes.Error = result.Error.Error()
		}
		if len(result.Metadata) > 0 {
			meta := make(map[string]any, len(result.Metadata))
			for k, v := range result.Metadata {
				meta[k] = v
			}
			outRes.Metadata = meta
		}
		if len(result.Attachments) > 0 {
			outRes.Attachments = buildExecutorAttachments(result.Attachments)
		}
		out = append(out, outRes)
	}
	return out
}

func buildExecutorAttachments(attachments map[string]ports.Attachment) []executorAttachment {
	if len(attachments) == 0 {
		return nil
	}
	names := make([]string, 0, len(attachments))
	seen := make(map[string]bool, len(attachments))
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]executorAttachment, 0, len(names))
	for _, name := range names {
		att := attachments[name]
		attName := strings.TrimSpace(att.Name)
		if attName == "" {
			attName = name
		}
		out = append(out, executorAttachment{
			Name:        attName,
			MediaType:   strings.TrimSpace(att.MediaType),
			URI:         strings.TrimSpace(att.URI),
			Source:      strings.TrimSpace(att.Source),
			Description: strings.TrimSpace(att.Description),
			Kind:        strings.TrimSpace(att.Kind),
			Format:      strings.TrimSpace(att.Format),
		})
	}
	return out
}

func cloneImportantNotes(notes map[string]ports.ImportantNote) []ports.ImportantNote {
	if len(notes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(notes))
	for key := range notes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ports.ImportantNote, 0, len(keys))
	for _, key := range keys {
		out = append(out, notes[key])
	}
	return out
}

func cloneMapAny(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func attachmentToContentBlock(att ports.Attachment) map[string]any {
	if att.MediaType != "" {
		if strings.HasPrefix(att.MediaType, "image/") && att.Data != "" {
			return map[string]any{
				"type":     "image",
				"data":     att.Data,
				"mimeType": att.MediaType,
			}
		}
		if strings.HasPrefix(att.MediaType, "audio/") && att.Data != "" {
			return map[string]any{
				"type":     "audio",
				"data":     att.Data,
				"mimeType": att.MediaType,
			}
		}
	}

	if att.URI != "" && att.Data == "" {
		block := map[string]any{
			"type": "resource_link",
			"uri":  att.URI,
			"name": att.Name,
		}
		if att.MediaType != "" {
			block["mimeType"] = att.MediaType
		}
		if att.Description != "" {
			block["description"] = att.Description
		}
		return block
	}

	resource := map[string]any{
		"uri": att.URI,
	}
	if att.URI == "" {
		resource["uri"] = att.Name
	}
	if att.MediaType != "" {
		resource["mimeType"] = att.MediaType
	}
	if att.Data != "" {
		resource["blob"] = att.Data
	}

	return map[string]any{
		"type":     "resource",
		"resource": resource,
	}
}
