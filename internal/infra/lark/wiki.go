package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkwiki "github.com/larksuite/oapi-sdk-go/v3/service/wiki/v2"
)

// WikiService provides typed access to Lark Wiki APIs (v2).
type WikiService struct {
	client *lark.Client
}

// WikiSpace is a simplified view of a Lark wiki space.
type WikiSpace struct {
	SpaceID     string
	Name        string
	Description string
	Visibility  string // public / private
}

// WikiNode is a simplified view of a Lark wiki node.
type WikiNode struct {
	NodeToken   string
	SpaceID     string
	Title       string
	ObjToken    string // underlying document/sheet/bitable token
	ObjType     string // doc / sheet / bitable / file / docx / slides
	ParentToken string
	HasChild    bool
	NodeType    string // origin / shortcut
}

// ListSpacesRequest defines parameters for listing wiki spaces.
type ListSpacesRequest struct {
	PageSize  int
	PageToken string
}

// ListSpacesResponse contains paginated wiki spaces.
type ListSpacesResponse struct {
	Spaces    []WikiSpace
	PageToken string
	HasMore   bool
}

// ListSpaces returns wiki spaces the user can access.
func (s *WikiService) ListSpaces(ctx context.Context, req ListSpacesRequest, opts ...CallOption) (*ListSpacesResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkwiki.NewListSpaceReqBuilder().
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Wiki.V2.Space.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list wiki spaces: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	spaces := make([]WikiSpace, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		spaces = append(spaces, parseWikiSpace(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListSpacesResponse{
		Spaces:    spaces,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// GetSpace retrieves a wiki space by ID.
func (s *WikiService) GetSpace(ctx context.Context, spaceID string, opts ...CallOption) (*WikiSpace, error) {
	getReq := larkwiki.NewGetSpaceReqBuilder().
		SpaceId(spaceID).
		Build()

	resp, err := s.client.Wiki.V2.Space.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get wiki space: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	space := parseWikiSpace(resp.Data.Space)
	return &space, nil
}

// ListNodesRequest defines parameters for listing wiki nodes.
type ListNodesRequest struct {
	SpaceID     string
	ParentToken string // filter by parent node (optional)
	PageSize    int
	PageToken   string
}

// ListNodesResponse contains paginated wiki nodes.
type ListNodesResponse struct {
	Nodes     []WikiNode
	PageToken string
	HasMore   bool
}

// ListNodes lists nodes in a wiki space.
func (s *WikiService) ListNodes(ctx context.Context, req ListNodesRequest, opts ...CallOption) (*ListNodesResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkwiki.NewListSpaceNodeReqBuilder().
		SpaceId(req.SpaceID).
		PageSize(pageSize)

	if req.ParentToken != "" {
		builder.ParentNodeToken(req.ParentToken)
	}
	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Wiki.V2.SpaceNode.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list wiki nodes: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	nodes := make([]WikiNode, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		nodes = append(nodes, parseWikiNode(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListNodesResponse{
		Nodes:     nodes,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// CreateNodeRequest defines parameters for creating a wiki node.
type CreateNodeRequest struct {
	SpaceID     string
	ObjType     string // docx | sheet | bitable | file
	ParentToken string // parent node token
	Title       string
}

// CreateNode creates a new wiki node in the given space.
func (s *WikiService) CreateNode(ctx context.Context, req CreateNodeRequest, opts ...CallOption) (*WikiNode, error) {
	nodeBuilder := larkwiki.NewNodeBuilder().
		ObjType(req.ObjType).
		ParentNodeToken(req.ParentToken)

	if req.Title != "" {
		nodeBuilder.Title(req.Title)
	}

	createReq := larkwiki.NewCreateSpaceNodeReqBuilder().
		SpaceId(req.SpaceID).
		Node(nodeBuilder.Build()).
		Build()

	resp, err := s.client.Wiki.V2.SpaceNode.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create wiki node: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	node := parseWikiNode(resp.Data.Node)
	return &node, nil
}

// GetNode retrieves a single wiki node by token.
func (s *WikiService) GetNode(ctx context.Context, nodeToken string, opts ...CallOption) (*WikiNode, error) {
	getReq := larkwiki.NewGetNodeSpaceReqBuilder().
		Token(nodeToken).
		Build()

	resp, err := s.client.Wiki.V2.Space.GetNode(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get wiki node: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	node := parseWikiNode(resp.Data.Node)
	return &node, nil
}

// --- helpers ---

func parseWikiSpace(space *larkwiki.Space) WikiSpace {
	if space == nil {
		return WikiSpace{}
	}
	ws := WikiSpace{}
	if space.SpaceId != nil {
		ws.SpaceID = *space.SpaceId
	}
	if space.Name != nil {
		ws.Name = *space.Name
	}
	if space.Description != nil {
		ws.Description = *space.Description
	}
	if space.Visibility != nil {
		ws.Visibility = *space.Visibility
	}
	return ws
}

func parseWikiNode(node *larkwiki.Node) WikiNode {
	if node == nil {
		return WikiNode{}
	}
	wn := WikiNode{}
	if node.NodeToken != nil {
		wn.NodeToken = *node.NodeToken
	}
	if node.SpaceId != nil {
		wn.SpaceID = *node.SpaceId
	}
	if node.Title != nil {
		wn.Title = *node.Title
	}
	if node.ObjToken != nil {
		wn.ObjToken = *node.ObjToken
	}
	if node.ObjType != nil {
		wn.ObjType = *node.ObjType
	}
	if node.ParentNodeToken != nil {
		wn.ParentToken = *node.ParentNodeToken
	}
	if node.HasChild != nil {
		wn.HasChild = *node.HasChild
	}
	if node.NodeType != nil {
		wn.NodeType = *node.NodeType
	}
	return wn
}
