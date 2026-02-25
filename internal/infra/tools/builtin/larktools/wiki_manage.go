package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// larkWikiManage handles wiki space/node operations via the unified channel tool.
type larkWikiManage struct{}

func (t *larkWikiManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "wiki operations require a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	sdkClient, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}
	client := larkapi.Wrap(sdkClient)

	action := strings.ToLower(strings.TrimSpace(shared.StringArg(call.Arguments, "action")))
	switch action {
	case "list_spaces":
		return t.listSpaces(ctx, client, call)
	case "list_nodes":
		return t.listNodes(ctx, client, call)
	case "create_node":
		return t.createNode(ctx, client, call)
	case "get_node":
		return t.getNode(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported wiki action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkWikiManage) listSpaces(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Wiki().ListSpaces(ctx, larkapi.ListSpacesRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list wiki spaces: %v", err),
			Error:   err,
		}, nil
	}

	if len(resp.Spaces) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No wiki spaces found.",
		}, nil
	}

	payload, _ := json.MarshalIndent(resp.Spaces, "", "  ")
	metadata := map[string]any{"space_count": len(resp.Spaces)}
	if resp.HasMore {
		metadata["has_more"] = true
		metadata["page_token"] = resp.PageToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d wiki spaces:\n%s", len(resp.Spaces), string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkWikiManage) listNodes(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	spaceID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "space_id")
	if errResult != nil {
		return errResult, nil
	}

	parentToken := shared.StringArg(call.Arguments, "parent_node_token")
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Wiki().ListNodes(ctx, larkapi.ListNodesRequest{
		SpaceID:     spaceID,
		ParentToken: parentToken,
		PageSize:    pageSize,
		PageToken:   pageToken,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list wiki nodes: %v", err),
			Error:   err,
		}, nil
	}

	if len(resp.Nodes) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No wiki nodes found.",
		}, nil
	}

	payload, _ := json.MarshalIndent(resp.Nodes, "", "  ")
	metadata := map[string]any{"node_count": len(resp.Nodes)}
	if resp.HasMore {
		metadata["has_more"] = true
		metadata["page_token"] = resp.PageToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d wiki nodes:\n%s", len(resp.Nodes), string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkWikiManage) createNode(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	spaceID, errResult := shared.RequireStringArg(call.Arguments, call.ID, "space_id")
	if errResult != nil {
		return errResult, nil
	}

	objType := shared.StringArg(call.Arguments, "obj_type")
	if objType == "" {
		objType = "docx"
	}

	parentToken := shared.StringArg(call.Arguments, "parent_node_token")
	title := shared.StringArg(call.Arguments, "title")

	node, err := client.Wiki().CreateNode(ctx, larkapi.CreateNodeRequest{
		SpaceID:     spaceID,
		ObjType:     objType,
		ParentToken: parentToken,
		Title:       title,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to create wiki node: %v", err),
			Error:   err,
		}, nil
	}

	payload, _ := json.MarshalIndent(node, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wiki node created successfully.\n%s", string(payload)),
		Metadata: map[string]any{
			"node_token": node.NodeToken,
			"obj_token":  node.ObjToken,
			"obj_type":   node.ObjType,
		},
	}, nil
}

func (t *larkWikiManage) getNode(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	nodeToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "node_token")
	if errResult != nil {
		return errResult, nil
	}

	node, err := client.Wiki().GetNode(ctx, nodeToken)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to get wiki node: %v", err),
			Error:   err,
		}, nil
	}

	payload, _ := json.MarshalIndent(node, "", "  ")
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wiki node details:\n%s", string(payload)),
		Metadata: map[string]any{
			"node_token": node.NodeToken,
			"title":      node.Title,
			"obj_type":   node.ObjType,
		},
	}, nil
}
