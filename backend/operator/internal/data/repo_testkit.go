// 跨包（service 层）测试装配：导出免 bootstrap.Context 的 repo 构造器，
// 字段初始化必须与生产 NewXxxRepo 逐字段一致，改生产构造器须同步此处。
//
// 说明：
//   - 各构造器与对应 *_repo.go 的生产构造器逐字段对齐（含 mapper/converter 初始化与
//     init() 调用），唯一差异是 log 一律 bLogger.NewHelper(bLogger.NopLogger())；
//     dbClient 由调用方传入（测试场景为 storetest.NewClientForTest 的 SQLite 内存库）。
//   - 生产构造器中经由 *bootstrap.Context 注入的仅是日志助手，无其他隐藏依赖；
//     依赖 redis/minio 等外部件的构造器不在此导出。
//   - RoleRepo / OrgUnitRepo 的关联子 repo 由本文件内 testkitXxx 私有构造器组装，
//     组装关系对齐 cmd/server/wiring.go 的仓储层装配小节。
//   - NewTenantUsageRepoForTest 的 authenticator 参数按生产签名保留：
//     TenantUsageRepo 仅在 CleanupTenantData/EnforceExpiryPolicies 中解引用它且有
//     nil 守卫，GetUsage 不触碰；测试不覆盖吊销令牌链路时可传 nil。
package data

import (
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/internalmessage"
	"go-admin/operator/internal/data/store/internalmessagerecipient"
	"go-admin/operator/internal/data/store/orgunit"
	"go-admin/operator/internal/data/store/permission"
	"go-admin/operator/internal/data/store/role"
	"go-admin/operator/internal/data/store/rolemetadata"
	"go-admin/operator/internal/data/store/rolepermission"
	"go-admin/operator/internal/data/store/tenant"
	"go-admin/operator/internal/data/store/userorgunit"
	operatorV1 "go-admin/platform/pb/operator"
)

// NewTenantRepoForTest 逐字段复刻 tenant_repo.go 的 NewTenantRepo（log 换 NopLogger）。
func NewTenantRepoForTest(dbClient *store.DBClient) *TenantRepo {
	repo := &TenantRepo{
		log:                  bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:             dbClient,
		mapper:               mapper.NewCopierMapper[operatorV1.Tenant, store.Tenant](),
		statusConverter:      mapper.NewEnumTypeConverter[operatorV1.Tenant_Status, tenant.Status](operatorV1.Tenant_Status_name, operatorV1.Tenant_Status_value),
		typeConverter:        mapper.NewEnumTypeConverter[operatorV1.Tenant_Type, tenant.Type](operatorV1.Tenant_Type_name, operatorV1.Tenant_Type_value),
		auditStatusConverter: mapper.NewEnumTypeConverter[operatorV1.Tenant_AuditStatus, tenant.AuditStatus](operatorV1.Tenant_AuditStatus_name, operatorV1.Tenant_AuditStatus_value),
	}

	repo.init()

	return repo
}

// NewTenantUsageRepoForTest 逐字段复刻 tenant_usage_repo.go 的 NewTenantUsageRepo
// （log 换 NopLogger；authenticator 语义见文件头说明）。
func NewTenantUsageRepoForTest(dbClient *store.DBClient, authenticator *Authenticator) *TenantUsageRepo {
	return &TenantUsageRepo{
		dbClient:      dbClient,
		authenticator: authenticator,
		log:           bLogger.NewHelper(bLogger.NopLogger()),
	}
}

// NewRoleRepoForTest 逐字段复刻 role_repo.go 的 NewRoleRepo（log 换 NopLogger；
// 关联子 repo 由 testkit 私有构造器组装，对齐 wiring.go 的装配关系）。
func NewRoleRepoForTest(dbClient *store.DBClient) *RoleRepo {
	repo := &RoleRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Role, store.Role](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.Role_Status, role.Status](
			operatorV1.Role_Status_name,
			operatorV1.Role_Status_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.Role_Type, role.Type](
			operatorV1.Role_Type_name,
			operatorV1.Role_Type_value,
		),
		dataScopeConverter: mapper.NewEnumTypeConverter[operatorV1.DataScope_DataScope, role.DataScope](
			operatorV1.DataScope_DataScope_name,
			operatorV1.DataScope_DataScope_value,
		),
		permissionRepo:          testkitPermissionRepo(dbClient),
		rolePermissionRepo:      testkitRolePermissionRepo(dbClient),
		roleOrgUnitRepo:         testkitRoleOrgUnitRepo(dbClient),
		roleMetadataRepo:        testkitRoleMetadataRepo(dbClient),
		roleFieldPermissionRepo: testkitRoleFieldPermissionRepo(dbClient),
	}

	repo.init()

	return repo
}

// NewOrgUnitRepoForTest 逐字段复刻 org_unit_repo.go 的 NewOrgUnitRepo
// （log 换 NopLogger；userOrgUnitRepo 由 testkit 私有构造器组装）。
func NewOrgUnitRepoForTest(dbClient *store.DBClient) *OrgUnitRepo {
	repo := &OrgUnitRepo{
		log:             bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:        dbClient,
		userOrgUnitRepo: testkitUserOrgUnitRepo(dbClient),
		mapper:          mapper.NewCopierMapper[operatorV1.OrgUnit, store.OrgUnit](),
		typeConverter:   mapper.NewEnumTypeConverter[operatorV1.OrgUnit_Type, orgunit.Type](operatorV1.OrgUnit_Type_name, operatorV1.OrgUnit_Type_value),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.OrgUnit_Status, orgunit.Status](operatorV1.OrgUnit_Status_name, operatorV1.OrgUnit_Status_value),
	}

	repo.init()

	return repo
}

// NewNotificationPreferenceRepoForTest 逐字段复刻 notification_preference_repo.go 的
// NewNotificationPreferenceRepo（log 换 NopLogger）。
func NewNotificationPreferenceRepoForTest(dbClient *store.DBClient) *NotificationPreferenceRepo {
	return &NotificationPreferenceRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
	}
}

// NewInternalMessageRepoForTest 逐字段复刻 internal_message_repo.go 的
// NewInternalMessageRepo（log 换 NopLogger）。
func NewInternalMessageRepoForTest(dbClient *store.DBClient) *InternalMessageRepo {
	repo := &InternalMessageRepo{
		log:             bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:        dbClient,
		mapper:          mapper.NewCopierMapper[operatorV1.InternalMessage, store.InternalMessage](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Status, internalmessage.Status](operatorV1.InternalMessage_Status_name, operatorV1.InternalMessage_Status_value),
		typeConverter:   mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Type, internalmessage.Type](operatorV1.InternalMessage_Type_name, operatorV1.InternalMessage_Type_value),
	}

	repo.init()

	return repo
}

// NewInternalMessageCategoryRepoForTest 逐字段复刻 internal_message_category_repo.go 的
// NewInternalMessageCategoryRepo（log 换 NopLogger）。
func NewInternalMessageCategoryRepoForTest(dbClient *store.DBClient) *InternalMessageCategoryRepo {
	repo := &InternalMessageCategoryRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.InternalMessageCategory, store.InternalMessageCategory](),
	}

	repo.init()

	return repo
}

// NewInternalMessageRecipientRepoForTest 逐字段复刻 internal_message_recipient_repo.go 的
// NewInternalMessageRecipientRepo（log 换 NopLogger）。
func NewInternalMessageRecipientRepoForTest(dbClient *store.DBClient) *InternalMessageRecipientRepo {
	repo := &InternalMessageRecipientRepo{
		log:             bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:        dbClient,
		mapper:          mapper.NewCopierMapper[operatorV1.InternalMessageRecipient, store.InternalMessageRecipient](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.InternalMessageRecipient_Status, internalmessagerecipient.Status](operatorV1.InternalMessageRecipient_Status_name, operatorV1.InternalMessageRecipient_Status_value),
	}

	repo.init()

	return repo
}

// ---------------------------------------------------------------------------
// testkit 私有子 repo 构造器（不导出）：逐字段复刻各自生产构造器，
// 仅供本文件的 ForTest 导出构造器组装使用。
// ---------------------------------------------------------------------------

// testkitRolePermissionRepo 复刻 role_permission_repo.go 的 NewRolePermissionRepo。
func testkitRolePermissionRepo(dbClient *store.DBClient) *RolePermissionRepo {
	repo := &RolePermissionRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.RolePermission, store.RolePermission](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.RolePermission_Status, rolepermission.Status](
			operatorV1.RolePermission_Status_name,
			operatorV1.RolePermission_Status_value,
		),
		effectConverter: mapper.NewEnumTypeConverter[operatorV1.RolePermission_EffectiveStatus, rolepermission.Effect](
			operatorV1.RolePermission_EffectiveStatus_name,
			operatorV1.RolePermission_EffectiveStatus_value,
		),
	}

	repo.init()

	return repo
}

// testkitRoleOrgUnitRepo 复刻 role_org_unit_repo.go 的 NewRoleOrgUnitRepo。
func testkitRoleOrgUnitRepo(dbClient *store.DBClient) *RoleOrgUnitRepo {
	repo := &RoleOrgUnitRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
	}

	return repo
}

// testkitPermissionApiRepo 复刻 permission_api_repo.go 的 NewPermissionApiRepo。
func testkitPermissionApiRepo(dbClient *store.DBClient) *PermissionApiRepo {
	return &PermissionApiRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
	}
}

// testkitPermissionMenuRepo 复刻 permission_menu_repo.go 的 NewPermissionMenuRepo。
func testkitPermissionMenuRepo(dbClient *store.DBClient) *PermissionMenuRepo {
	return &PermissionMenuRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
	}
}

// testkitPermissionRepo 复刻 permission_repo.go 的 NewPermissionRepo。
func testkitPermissionRepo(dbClient *store.DBClient) *PermissionRepo {
	repo := &PermissionRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Permission, store.Permission](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.Permission_Status, permission.Status](
			operatorV1.Permission_Status_name,
			operatorV1.Permission_Status_value,
		),
		permissionApiRepo:  testkitPermissionApiRepo(dbClient),
		permissionMenuRepo: testkitPermissionMenuRepo(dbClient),
	}

	repo.init()

	return repo
}

// testkitRoleMetadataRepo 复刻 role_metadata_repo.go 的 NewRoleMetadataRepo。
func testkitRoleMetadataRepo(dbClient *store.DBClient) *RoleMetadataRepo {
	repo := &RoleMetadataRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.RoleMetadata, store.RoleMetadata](),
		syncPolicyConverter: mapper.NewEnumTypeConverter[operatorV1.RoleMetadata_SyncPolicy, rolemetadata.SyncPolicy](
			operatorV1.RoleMetadata_SyncPolicy_name,
			operatorV1.RoleMetadata_SyncPolicy_value,
		),
		scopeConverter: mapper.NewEnumTypeConverter[operatorV1.RoleMetadata_Scope, rolemetadata.Scope](
			operatorV1.RoleMetadata_Scope_name,
			operatorV1.RoleMetadata_Scope_value,
		),
	}

	repo.init()

	return repo
}

// testkitRoleFieldPermissionRepo 复刻 role_field_permission_repo.go 的 NewRoleFieldPermissionRepo。
func testkitRoleFieldPermissionRepo(dbClient *store.DBClient) *RoleFieldPermissionRepo {
	repo := &RoleFieldPermissionRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
	}

	return repo
}

// testkitUserOrgUnitRepo 复刻 user_org_unit_repo.go 的 NewUserOrgUnitRepo。
func testkitUserOrgUnitRepo(dbClient *store.DBClient) *UserOrgUnitRepo {
	return &UserOrgUnitRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.UserOrgUnit_Status, userorgunit.Status](
			operatorV1.UserOrgUnit_Status_name,
			operatorV1.UserOrgUnit_Status_value,
		),
	}
}
