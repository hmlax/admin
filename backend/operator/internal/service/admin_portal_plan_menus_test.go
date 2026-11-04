package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-admin/operator/internal/data"
	"go-admin/operator/internal/data/store/menu"
	"go-admin/operator/internal/data/store/permissionmenu"
	modelPlan "go-admin/operator/internal/data/store/plan"
	"go-admin/operator/internal/data/store/planmodule"
	storetest "go-admin/operator/internal/data/storetest"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/constants"
	"go-admin/platform/pkg/middleware/auth"
)

func TestTenantNavigationFiltersNestedPlanModules(t *testing.T) {
	client := storetest.NewClientForTest(t)
	ctx := storetest.NewSystemContext(t.Context())
	plan, err := client.Client().Plan.Create().SetName("menu whitelist").Save(ctx)
	require.NoError(t, err)
	_, err = client.Client().PlanModule.Create().SetPlanID(plan.ID).SetModule(planmodule.ModuleOpm).Save(ctx)
	require.NoError(t, err)
	tenant, err := client.Client().Tenant.Create().SetName("menu tenant").SetCode("menu-tenant").SetPlanID(plan.ID).Save(ctx)
	require.NoError(t, err)
	role, err := client.Client().Role.Create().SetTenantID(tenant.ID).SetCode(constants.TenantAdminRoleCode).SetName("manager").Save(ctx)
	require.NoError(t, err)
	perm, err := client.Client().Permission.Create().SetCode(constants.SystemTenantManagerPermissionCode).SetName("manager").Save(ctx)
	require.NoError(t, err)
	_, err = client.Client().RolePermission.Create().SetRoleID(role.ID).SetPermissionID(perm.ID).SetTenantID(tenant.ID).Save(ctx)
	require.NoError(t, err)
	root, err := client.Client().Menu.Create().SetName("MixedRoot").SetPath("/mixed").SetComponent("BasicLayout").SetType(menu.TypeCatalog).Save(ctx)
	require.NoError(t, err)
	allowedUser, err := client.Client().Menu.Create().SetName("AllowedUserPage").SetPath("users").SetParentID(root.ID).
		SetComponent("app/opm/user/list/index.vue").SetType(menu.TypeMenu).SetModule(menu.ModuleOpm).Save(ctx)
	require.NoError(t, err)
	catalog, err := client.Client().Menu.Create().SetName("TenantRoot").SetPath("/tenant").SetComponent("BasicLayout").SetType(menu.TypeCatalog).Save(ctx)
	require.NoError(t, err)
	nested, err := client.Client().Menu.Create().SetName("NestedTenantCatalog").SetPath("manage").SetParentID(catalog.ID).
		SetComponent("BasicLayout").SetType(menu.TypeCatalog).Save(ctx)
	require.NoError(t, err)
	_, err = client.Client().Menu.Create().SetName("TenantPage").SetPath("members").SetParentID(nested.ID).
		SetComponent("app/tenant/tenant/index.vue").SetType(menu.TypeMenu).SetModule(menu.ModuleTenant).Save(ctx)
	require.NoError(t, err)
	_, err = client.Client().Menu.Create().SetName("HiddenTenantPage").SetPath("tenant-plans").SetParentID(root.ID).
		SetComponent("app/tenant/plan/index.vue").SetType(menu.TypeMenu).SetModule(menu.ModuleTenant).Save(ctx)
	require.NoError(t, err)
	_, err = client.Client().Menu.Create().SetName("UnclassifiedBusinessPage").SetPath("unknown").SetParentID(root.ID).
		SetComponent("custom/unclassified-page").SetType(menu.TypeMenu).Save(ctx)
	require.NoError(t, err)
	menuIDs, err := client.Client().Menu.Query().IDs(ctx)
	require.NoError(t, err)
	for _, id := range menuIDs {
		require.NoError(t, client.Client().PermissionMenu.Create().SetPermissionID(perm.ID).SetMenuID(id).Exec(ctx))
	}
	svc := &AdminPortalService{
		log:            bLogger.NewHelper(bLogger.NopLogger()),
		menuRepo:       data.NewMenuRepoForTest(client),
		roleRepo:       data.NewRoleRepoForTest(client),
		apiRepo:        data.NewApiRepoForTest(client),
		permissionRepo: data.NewPermissionRepoForTest(client),
		planModuleRepo: data.NewPlanModuleRepoForTest(client),
		tenantRepo:     data.NewTenantRepoForTest(client),
		userRepo: &authSvcUserRepoStub{
			usersByID: map[uint32]*operatorV1.User{
				1: {
					Id:      trans.Ptr(uint32(1)),
					RoleIds: []uint32{role.ID},
				},
			},
		},
	}
	actor := auth.NewContext(ctx, &operatorV1.UserTokenPayload{
		UserId:   1,
		TenantId: trans.Ptr(tenant.ID),
	})
	navigation, err := svc.GetNavigation(actor, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, navigation.Items, 1, "empty tenant catalogs must disappear")
	require.Equal(t, "MixedRoot", navigation.Items[0].GetName())
	require.Len(t, navigation.Items[0].Children, 1, "nested denied and unclassified pages must disappear")
	require.Equal(t, "AllowedUserPage", navigation.Items[0].Children[0].GetName())
	initial, err := svc.GetInitialContext(actor, &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, navigation.Items[0].GetName(), initial.Menus[0].GetName())
	require.Len(t, initial.Menus, 1)

	_, err = client.Client().PlanModule.Create().SetPlanID(plan.ID).SetModule(planmodule.ModuleTenant).Save(ctx)
	require.NoError(t, err)
	navigation, err = svc.GetNavigation(actor, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, navigation.Items, 2, "restoring TENANT must restore permitted tenant menus")
	_, err = client.Client().PlanModule.Delete().Where(planmodule.HasPlanWith(modelPlan.IDEQ(plan.ID)), planmodule.ModuleEQ(planmodule.ModuleOpm)).Exec(ctx)
	require.NoError(t, err)
	navigation, err = svc.GetNavigation(actor, &emptypb.Empty{})
	require.NoError(t, err)
	for _, item := range navigation.Items {
		for _, child := range item.Children {
			require.NotEqual(t, "AllowedUserPage", child.GetName())
		}
	}

	platform := auth.NewContext(ctx, &operatorV1.UserTokenPayload{UserId: 1})
	navigation, err = svc.GetNavigation(platform, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, navigation.Items, 2, "platform navigation must retain role-authorized menus")
	var mixed *operatorV1.MenuRouteItem
	for _, item := range navigation.Items {
		if item.GetName() == "MixedRoot" {
			mixed = item
		}
	}
	require.NotNil(t, mixed)
	require.Len(t, mixed.Children, 3, "platform menus do not use tenant plan filtering")

	// Permission forms bind leaves. Their ungranted containers are necessary for
	// navigation, while sibling pages must remain inaccessible.
	_, err = client.Client().PermissionMenu.Delete().Where(permissionmenu.PermissionIDEQ(perm.ID)).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Client().PermissionMenu.Create().SetPermissionID(perm.ID).SetMenuID(allowedUser.ID).Exec(ctx))
	_, err = client.Client().PlanModule.Create().SetPlanID(plan.ID).SetModule(planmodule.ModuleOpm).Save(ctx)
	require.NoError(t, err)
	for _, principal := range []context.Context{actor, platform} {
		navigation, err = svc.GetNavigation(principal, &emptypb.Empty{})
		require.NoError(t, err)
		require.Len(t, navigation.Items, 1)
		require.Equal(t, "MixedRoot", navigation.Items[0].GetName())
		require.Len(t, navigation.Items[0].Children, 1)
		require.Equal(t, "AllowedUserPage", navigation.Items[0].Children[0].GetName())
	}
	_, err = client.Client().PermissionMenu.Delete().Where(permissionmenu.PermissionIDEQ(perm.ID)).Exec(ctx)
	require.NoError(t, err)
	navigation, err = svc.GetNavigation(actor, &emptypb.Empty{})
	require.NoError(t, err)
	require.Empty(t, navigation.Items, "empty permission bindings must never load all menus")
}
