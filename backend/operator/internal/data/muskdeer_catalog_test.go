package data

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/permission"
	"go-admin/operator/internal/data/store/permissionapi"
	"go-admin/operator/internal/data/store/permissionmenu"
	"go-admin/operator/internal/data/store/rolepermission"
	"go-admin/operator/internal/data/storetest"
	pb "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/constants"
	"go-admin/platform/pkg/muskdeer"
)

func TestMuskdeerModuleMergePreservesIDsCustomPermissionsAndTenantBoundary(t *testing.T) {
	db := storetest.NewClientForTest(t)
	ctx := storetest.NewSystemContext(t.Context())
	for _, code := range []string{constants.SystemPlatformAdminPermissionCode, constants.SystemTenantManagerPermissionCode, "custom:existing:view"} {
		_, err := db.Client().Permission.Create().SetCode(code).SetName(code).Save(ctx)
		require.NoError(t, err)
	}
	custom, err := db.Client().Api.Create().SetOperation("Custom_Existing").SetMethod("GET").SetPath("/custom/existing").Save(ctx)
	require.NoError(t, err)
	repo := newApiRepoSqlite(t, db)
	require.NoError(t, repo.EnsureMuskdeerModule(ctx))
	before, err := db.Client().Permission.Query().All(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.EnsureMuskdeerModule(ctx))
	after, err := db.Client().Permission.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, after, len(before))
	for index, row := range before {
		require.Equal(t, row.ID, after[index].ID, "repeated merge must preserve permission identities")
	}
	require.NoError(t, repo.EnsureBuiltInAdminAPIs(ctx))
	tenantPermission, err := db.Client().Permission.Query().Where(permission.CodeEQ(constants.SystemTenantManagerPermissionCode)).Only(ctx)
	require.NoError(t, err)
	links, err := db.Client().PermissionApi.Query().Where(permissionapi.PermissionIDEQ(tenantPermission.ID)).All(ctx)
	require.NoError(t, err)
	ids := make([]uint32, 0, len(links))
	for _, link := range links {
		ids = append(ids, *link.APIID)
	}
	tenantAPIs, err := db.Client().Api.Query().Where(api.IDIn(ids...)).All(ctx)
	require.NoError(t, err)
	var operations []string
	for _, row := range tenantAPIs {
		operations = append(operations, row.GetOperation())
	}
	require.Contains(t, operations, "MuskdeerService_StopInstance")
	require.NotContains(t, operations, "MuskdeerService_ListAccountBindings")
	require.NotContains(t, operations, "MuskdeerService_CreateAccountBinding")
	require.NotContains(t, operations, "MuskdeerService_SetAccountBindingEnabled")
	_, err = db.Client().Api.Query().Where(api.IDEQ(custom.ID)).Only(ctx)
	require.NoError(t, err, "merge must preserve unrelated APIs")
	covered := make(map[string]int)
	for _, capability := range muskdeer.Capabilities() {
		perm, err := db.Client().Permission.Query().Where(permission.CodeEQ(capability.Code)).Only(ctx)
		require.NoError(t, err)
		menuLinks, err := db.Client().PermissionMenu.Query().Where(permissionmenu.PermissionIDEQ(perm.ID)).Count(ctx)
		require.NoError(t, err)
		if capability.Page == "" {
			require.Equal(t, 1, menuLinks)
		} else {
			require.Equal(t, 2, menuLinks)
		}
		for _, method := range capability.Methods {
			covered[method]++
		}
	}
	methods := pb.File_operator_v1_muskdeer_proto.Services().ByName("MuskdeerService").Methods()
	require.Len(t, covered, methods.Len())
	for index := 0; index < methods.Len(); index++ {
		require.GreaterOrEqual(t, covered[string(methods.Get(index).Name())], 1, "each new method needs one capability")
	}
}

func TestMuskdeerRealmUpgradeRevokesOldBuiltInBusinessGrantsWithoutChangingIDs(t *testing.T) {
	db := storetest.NewClientForTest(t)
	ctx := storetest.NewSystemContext(t.Context())
	roles := make(map[bool]uint32)
	for platform, code := range map[bool]string{
		true:  constants.SystemPlatformAdminPermissionCode,
		false: constants.SystemTenantManagerPermissionCode,
	} {
		perm, err := db.Client().Permission.Create().SetCode(code).SetName(code).Save(ctx)
		require.NoError(t, err)
		tenantID := uint32(7)
		if platform {
			tenantID = 0
		}
		role, err := db.Client().Role.Create().SetTenantID(tenantID).SetCode(code).SetName(code).Save(ctx)
		require.NoError(t, err)
		roles[platform] = role.ID
		_, err = db.Client().RolePermission.Create().SetTenantID(tenantID).SetRoleID(role.ID).SetPermissionID(perm.ID).SetEffect(pb.RolePermission_ALLOW.String()).SetStatus(pb.RolePermission_ON.String()).Save(ctx)
		require.NoError(t, err)
	}
	repo := newApiRepoSqlite(t, db)
	require.NoError(t, repo.EnsureMuskdeerModule(ctx))
	ids := make(map[string]uint32)
	// Simulate the previous release: both built-in roles held every module grant.
	for _, capability := range muskdeer.Capabilities() {
		id, err := db.Client().Permission.Query().Where(permission.CodeEQ(capability.Code)).OnlyID(ctx)
		require.NoError(t, err)
		ids[capability.Code] = id
		for platform, roleID := range roles {
			tenantID := uint32(7)
			if platform {
				tenantID = 0
			}
			err := db.Client().RolePermission.Create().SetTenantID(tenantID).SetRoleID(roleID).SetPermissionID(id).SetEffect(pb.RolePermission_ALLOW.String()).SetStatus(pb.RolePermission_ON.String()).OnConflictColumns(rolepermission.FieldTenantID, rolepermission.FieldPermissionID, rolepermission.FieldRoleID).Ignore().Exec(ctx)
			require.NoError(t, err)
		}
	}
	require.NoError(t, repo.EnsureMuskdeerModule(ctx))
	require.NoError(t, repo.EnsureMuskdeerModule(ctx))
	for _, capability := range muskdeer.Capabilities() {
		id, err := db.Client().Permission.Query().Where(permission.CodeEQ(capability.Code)).OnlyID(ctx)
		require.NoError(t, err)
		require.Equal(t, ids[capability.Code], id)
		for platform, roleID := range roles {
			count, err := db.Client().RolePermission.Query().Where(rolepermission.RoleIDEQ(roleID), rolepermission.PermissionIDEQ(id)).Count(ctx)
			require.NoError(t, err)
			require.Equal(t, capability.Allows(platform), count == 1, capability.Code)
		}
	}
	require.NoError(t, repo.EnsureBuiltInAdminAPIs(ctx))
	for platform, code := range map[bool]string{true: constants.SystemPlatformAdminPermissionCode, false: constants.SystemTenantManagerPermissionCode} {
		id, err := db.Client().Permission.Query().Where(permission.CodeEQ(code)).OnlyID(ctx)
		require.NoError(t, err)
		links, err := db.Client().PermissionApi.Query().Where(permissionapi.PermissionIDEQ(id)).All(ctx)
		require.NoError(t, err)
		for _, link := range links {
			endpoint, err := db.Client().Api.Get(ctx, *link.APIID)
			require.NoError(t, err)
			method := strings.TrimPrefix(endpoint.GetOperation(), muskdeer.ServiceTag+"_")
			require.True(t, muskdeer.MethodAllowed(method, platform), endpoint.GetOperation())
		}
	}
}
