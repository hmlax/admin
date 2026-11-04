package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"

	"go-admin/platform/pkg/middleware/auth"

	"go-admin/operator/internal/data"
	operatorV1 "go-admin/platform/pb/operator"
)

// roleStubRepo / userStubRepo：仅覆写 visibleMenuIDs 链路用到的两个方法，
// 其余方法嵌入接口零值（不会被调用）。
type roleStubRepo struct {
	data.RoleRepo
	menuIDsByRoles map[string][]uint32 // key: roleIDs 逗号串
}

func (r *roleStubRepo) GetRolesPermissionMenuIDs(_ context.Context, roleIDs []uint32) ([]uint32, error) {
	key := fmt.Sprintf("%v", roleIDs)
	if ids, ok := r.menuIDsByRoles[key]; ok {
		return ids, nil
	}
	return []uint32{}, nil
}

type userStubRepo struct {
	data.UserRepo
	usersByID map[uint32]*operatorV1.User
}

func (r *userStubRepo) Get(_ context.Context, req *operatorV1.GetUserRequest) (*operatorV1.User, error) {
	if u, ok := r.usersByID[req.GetId()]; ok {
		return u, nil
	}
	return nil, operatorV1.ErrorIdentityNotFound("user not found")
}

// TestVisibleMenuIDs 平台管理员不过滤；租户用户按角色解析；无角色返回空集。
func TestVisibleMenuIDs(t *testing.T) {
	roleRepo := &roleStubRepo{menuIDsByRoles: map[string][]uint32{
		"[1 2]": {10, 20, 30},
		"[1]":   {10, 20},
		"[2]":   {20, 30},
	}}
	userRepo := &userStubRepo{usersByID: map[uint32]*operatorV1.User{
		7: {Id: trans.Ptr(uint32(7)), RoleIds: []uint32{1, 2}},
		8: {Id: trans.Ptr(uint32(8)), RoleIds: nil},
	}}
	svc := &AiContentService{roleRepo: roleRepo, userRepo: userRepo}

	// 平台管理员：nil = 调用方不过滤
	ids, err := svc.visibleMenuIDs(context.Background(), &operatorV1.UserTokenPayload{
		UserId: 1, TenantId: trans.Ptr(uint32(0)),
	})
	require.NoError(t, err)
	require.Nil(t, ids, "平台管理员应返回 nil（不过滤）")

	// 租户用户：两角色并集去重
	ids, err = svc.visibleMenuIDs(context.Background(), &operatorV1.UserTokenPayload{
		UserId: 7, TenantId: trans.Ptr(uint32(5)),
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []uint32{10, 20, 30}, ids, "应取角色菜单并集")

	// 无角色用户：空集
	ids, err = svc.visibleMenuIDs(context.Background(), &operatorV1.UserTokenPayload{
		UserId: 8, TenantId: trans.Ptr(uint32(5)),
	})
	require.NoError(t, err)
	require.Empty(t, ids, "无角色用户应返回空集（AI 搜索不返回任何菜单）")
}

// TestSemanticSearchEmptyVisibleMenus 非平台用户且无可见菜单 → 直接空结果（不查库、不泄露）。
func TestSemanticSearchEmptyVisibleMenus(t *testing.T) {
	svc := &AiContentService{
		userRepo: &userStubRepo{usersByID: map[uint32]*operatorV1.User{
			9: {Id: trans.Ptr(uint32(9)), TenantId: trans.Ptr(uint32(5))},
		}},
	}
	ctx := auth.NewContext(context.Background(), &operatorV1.UserTokenPayload{
		UserId: 9, TenantId: trans.Ptr(uint32(5)),
	})
	resp, err := svc.SemanticSearch(ctx, &operatorV1.SemanticSearchRequest{
		Query: "用户管理", Limit: trans.Ptr(uint32(8)),
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetItems(), "无可见菜单应返回空结果")
}
