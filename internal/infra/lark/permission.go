package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// PermissionService provides typed access to Lark Drive Permission APIs (v1).
type PermissionService struct {
	client *lark.Client
}

// GrantPermissionRequest defines parameters for granting a user permission on a document.
type GrantPermissionRequest struct {
	Token      string // Document/file/folder token
	Type       string // docx, sheet, bitable, folder, file, wiki, etc.
	MemberID   string // User's open_id (or other ID matching MemberType)
	MemberType string // openid, userid, unionid, email, openchat, etc.
	Perm       string // view, edit, full_access
}

// GrantPermission adds a collaborator with the specified permission to a document.
func (s *PermissionService) GrantPermission(ctx context.Context, req GrantPermissionRequest, opts ...CallOption) error {
	memberType := req.MemberType
	if memberType == "" {
		memberType = "openid"
	}
	perm := req.Perm
	if perm == "" {
		perm = "edit"
	}

	member := larkdrive.NewBaseMemberBuilder().
		MemberType(memberType).
		MemberId(req.MemberID).
		Perm(perm).
		Build()

	createReq := larkdrive.NewCreatePermissionMemberReqBuilder().
		Token(req.Token).
		Type(req.Type).
		NeedNotification(false).
		BaseMember(member).
		Build()

	resp, err := s.client.Drive.V1.PermissionMember.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("grant permission: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}
