package data

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/permission"
	"go-admin/operator/internal/data/store/permissionapi"
	"go-admin/operator/internal/data/store/permissionmenu"
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
		require.Equal(t, 2, menuLinks)
		for _, method := range capability.Methods {
			covered[method]++
		}
	}
	methods := pb.File_operator_v1_muskdeer_proto.Services().ByName("MuskdeerService").Methods()
	require.Len(t, covered, methods.Len())
	for index := 0; index < methods.Len(); index++ {
		require.Equal(t, 1, covered[string(methods.Get(index).Name())], "each new method needs one capability")
	}
}
