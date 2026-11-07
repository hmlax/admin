package data

import (
	"context"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	assets "go-admin/platform/pkg/apidocs"
	"strings"
	"time"

	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/trans"
	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/menu"
	"go-admin/operator/internal/data/store/permission"
	"go-admin/operator/internal/data/store/permissionapi"
	"go-admin/operator/internal/data/store/permissiongroup"
	"go-admin/operator/internal/data/store/permissionmenu"
	"go-admin/operator/internal/data/store/rolepermission"
	pb "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/constants"
	"go-admin/platform/pkg/muskdeer"
)

// EnsureMuskdeerModule merges only this module's catalog and additive links.
// Existing role assignments, disabled resources and unrelated modules survive.
func (r *ApiRepo) EnsureMuskdeerModule(ctx context.Context) (err error) {
	ctx = viewer.WithSystemContext(ctx)
	document, err := openapi3.NewLoader().LoadFromData(assets.OpenApiData)
	if err != nil {
		return err
	}
	type endpointData struct {
		path   string
		method string
	}
	endpoints := make(map[string]endpointData)
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			if strings.HasPrefix(operation.OperationID, muskdeer.ServiceTag+"_") {
				endpoints[operation.OperationID] = endpointData{path: path, method: strings.ToUpper(method)}
			}
		}
	}
	tx, err := r.dbClient.Client().Tx(ctx)
	if err != nil {
		return err
	}
	defer store.MakeTxCleanup(tx, &err)()
	now := time.Now().UTC()
	root, err := tx.Menu.Query().Where(menu.NameEQ(muskdeer.RootName)).Only(ctx)
	if store.IsNotFound(err) {
		root, err = tx.Menu.Create().SetName(muskdeer.RootName).SetPath(muskdeer.RootPath).SetComponent("BasicLayout").SetModule(pb.Module_MUSKDEER.String()).SetType(pb.Menu_CATALOG.String()).SetStatus(pb.Menu_ON.String()).SetCreatedAt(now).SetUpdatedAt(now).SetMeta(&pb.MenuMeta{
			Title: trans.Ptr("routes:Muskdeer"),
			Icon:  trans.Ptr("lucide:chart-candlestick"),
			Order: trans.Ptr(int32(20)),
		}).Save(ctx)
	}
	if err != nil {
		return err
	}
	pages := make(map[string]*store.Menu)
	for index, page := range muskdeer.Pages() {
		row, findErr := tx.Menu.Query().Where(menu.NameEQ(page.Name)).Only(ctx)
		if store.IsNotFound(findErr) {
			authority := []string{"muskdeer:" + page.Resource + ":view", constants.SystemPlatformAdminPermissionCode}
			if !page.PlatformOnly {
				authority = append(authority, constants.SystemTenantManagerPermissionCode)
			}
			row, findErr = tx.Menu.Create().SetName(page.Name).SetParentID(root.ID).SetPath(page.Path).SetComponent("app/muskdeer/" + page.Path + "/index.tsx").SetModule(pb.Module_MUSKDEER.String()).SetType(pb.Menu_MENU.String()).SetStatus(pb.Menu_ON.String()).SetCreatedAt(now).SetUpdatedAt(now).SetMeta(&pb.MenuMeta{
				Title:     trans.Ptr("routes:" + page.Name),
				Icon:      trans.Ptr("lucide:activity"),
				Order:     trans.Ptr(int32(index + 1)),
				Authority: authority,
			}).Save(ctx)
		}
		if findErr != nil {
			return findErr
		}
		pages[page.Path] = row
	}
	group, err := tx.PermissionGroup.Query().Where(permissiongroup.ModuleEQ(muskdeer.Module)).First(ctx)
	if store.IsNotFound(err) {
		group, err = tx.PermissionGroup.Create().SetName("做市管理").SetModule(muskdeer.Module).SetStatus(pb.PermissionGroup_ON.String()).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
	}
	if err != nil {
		return err
	}
	for _, capability := range muskdeer.Capabilities() {
		perm, findErr := tx.Permission.Query().Where(permission.CodeEQ(capability.Code)).Only(ctx)
		if store.IsNotFound(findErr) {
			perm, findErr = tx.Permission.Create().SetCode(capability.Code).SetName(capability.Title).SetGroupID(group.ID).SetStatus(pb.Permission_ON.String()).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		}
		if findErr != nil {
			return findErr
		}
		if err := grantMuskdeerAdministrators(ctx, tx, perm.ID, capability.PlatformOnly); err != nil {
			return err
		}
		for _, id := range []uint32{root.ID, pages[capability.Page].ID} {
			if err := ensureMuskdeerMenu(ctx, tx, perm.ID, id); err != nil {
				return err
			}
		}
		for _, method := range capability.Methods {
			operation := muskdeer.ServiceTag + "_" + method
			endpoint, findErr := tx.Api.Query().Where(api.OperationEQ(operation)).Only(ctx)
			if store.IsNotFound(findErr) {
				route, ok := endpoints[operation]
				if !ok {
					return fmt.Errorf("missing embedded Muskdeer API %s", operation)
				}
				endpoint, findErr = tx.Api.Create().SetModule(muskdeer.ServiceTag).SetBusinessModule(pb.Module_MUSKDEER.String()).SetPath(route.path).SetMethod(route.method).SetScope("ADMIN").SetOperation(operation).SetStatus(pb.Api_ON.String()).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			}
			if findErr != nil {
				return findErr
			}
			if err := ensureMuskdeerAPI(ctx, tx, perm.ID, endpoint.ID); err != nil {
				return err
			}
		}
	}
	for _, code := range []string{constants.SystemPlatformAdminPermissionCode, constants.SystemTenantManagerPermissionCode} {
		perm, findErr := tx.Permission.Query().Where(permission.CodeEQ(code)).Only(ctx)
		if store.IsNotFound(findErr) {
			continue
		}
		if findErr != nil {
			return findErr
		}
		if err := ensureMuskdeerMenu(ctx, tx, perm.ID, root.ID); err != nil {
			return err
		}
		for _, page := range muskdeer.Pages() {
			if code == constants.SystemTenantManagerPermissionCode && page.PlatformOnly {
				continue
			}
			if err := ensureMuskdeerMenu(ctx, tx, perm.ID, pages[page.Path].ID); err != nil {
				return err
			}
		}
	}
	return nil
}
func ensureMuskdeerMenu(ctx context.Context, tx *store.Tx, permissionID, menuID uint32) error {
	return tx.PermissionMenu.Create().SetPermissionID(permissionID).SetMenuID(menuID).OnConflictColumns(permissionmenu.FieldPermissionID, permissionmenu.FieldMenuID).Ignore().Exec(ctx)
}
func ensureMuskdeerAPI(ctx context.Context, tx *store.Tx, permissionID, apiID uint32) error {
	return tx.PermissionApi.Create().SetPermissionID(permissionID).SetAPIID(apiID).OnConflictColumns(permissionapi.FieldPermissionID, permissionapi.FieldAPIID).Ignore().Exec(ctx)
}

// IsMuskdeerMenu protects module-managed IDs from destructive global rebuilding.
func IsMuskdeerMenu(name, path string) bool {
	return strings.HasPrefix(name, muskdeer.RootName) || strings.HasPrefix(strings.Trim(path, "/"), muskdeer.Module+"/") || path == muskdeer.RootPath
}

// Only roles already holding an active built-in administrator capability gain
// module permissions. Custom roles and explicit module denies remain unchanged.
func grantMuskdeerAdministrators(ctx context.Context, tx *store.Tx, permissionID uint32, platformOnly bool) error {
	codes := []string{constants.SystemPlatformAdminPermissionCode}
	if !platformOnly {
		codes = append(codes, constants.SystemTenantManagerPermissionCode)
	}
	ids, err := tx.Permission.Query().Where(permission.CodeIn(codes...)).IDs(ctx)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	roles, err := tx.RolePermission.Query().Where(rolepermission.PermissionIDIn(ids...), rolepermission.StatusEQ(pb.RolePermission_ON.String()), rolepermission.EffectEQ(pb.RolePermission_ALLOW.String())).All(ctx)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if platformOnly && role.GetTenantId() != 0 {
			continue
		}
		if err := tx.RolePermission.Create().SetTenantID(role.GetTenantId()).SetRoleID(role.GetRoleId()).SetPermissionID(permissionID).SetStatus(pb.RolePermission_ON.String()).SetEffect(pb.RolePermission_ALLOW.String()).SetCreatedAt(time.Now().UTC()).SetUpdatedAt(time.Now().UTC()).OnConflictColumns(rolepermission.FieldTenantID, rolepermission.FieldPermissionID, rolepermission.FieldRoleID).Ignore().Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}
