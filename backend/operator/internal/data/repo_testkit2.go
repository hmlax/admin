// 跨包（service 层）测试装配：导出免 bootstrap.Context 的 repo 构造器，
// 字段初始化必须与生产 NewXxxRepo 逐字段一致，改生产构造器须同步此处。
//
// 说明：
//   - 本文件覆盖 repo_testkit.go 之外批次的 repo（plan / plan_module / plan_quota /
//     language / dict_type / dict_entry / position / api / permission_group / menu）。
//     各构造器与对应 *_repo.go 的生产构造器逐字段对齐（含 mapper/converter 初始化
//     与 init() 调用），唯一差异是 log 一律 bLogger.NewHelper(bLogger.NopLogger())；
//     dbClient 由调用方传入（测试场景为 storetest.NewClientForTest 的 SQLite 内存库）。
//   - 生产构造器中经由 *bootstrap.Context 注入的仅是日志助手，无其他隐藏依赖；
//     依赖 redis/minio 等外部件的构造器不在此导出。
//   - DictEntryRepo 的关联子 repo（DictEntryI18nRepo）由本文件内私有构造器组装，
//     装配关系对齐生产 NewDictEntryRepo。
package data

import (
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	modelApi "go-admin/operator/internal/data/store/api"
	modelMenu "go-admin/operator/internal/data/store/menu"
	modelPermissionGroup "go-admin/operator/internal/data/store/permissiongroup"
	modelPlan "go-admin/operator/internal/data/store/plan"
	modelPlanModule "go-admin/operator/internal/data/store/planmodule"
	modelPlanQuota "go-admin/operator/internal/data/store/planquota"
	modelPosition "go-admin/operator/internal/data/store/position"
	operatorV1 "go-admin/platform/pb/operator"
)

// NewPlanRepoForTest 与生产 NewPlanRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewPlanRepoForTest(dbClient *store.DBClient) *PlanRepo {
	repo := &PlanRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Plan, store.Plan](),
		versionConverter: mapper.NewEnumTypeConverter[operatorV1.Plan_Version, modelPlan.Version](
			operatorV1.Plan_Version_name, operatorV1.Plan_Version_value,
		),
		expiryPolicyConv: mapper.NewEnumTypeConverter[operatorV1.Plan_ExpiryPolicy, modelPlan.ExpiryPolicy](
			operatorV1.Plan_ExpiryPolicy_name, operatorV1.Plan_ExpiryPolicy_value,
		),
	}

	repo.init()

	return repo
}

// NewPlanModuleRepoForTest 与生产 NewPlanModuleRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewPlanModuleRepoForTest(dbClient *store.DBClient) *PlanModuleRepo {
	repo := &PlanModuleRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PlanModule, store.PlanModule](),
		moduleConv: mapper.NewEnumTypeConverter[operatorV1.Module_Module, modelPlanModule.Module](
			operatorV1.Module_Module_name, operatorV1.Module_Module_value,
		),
	}

	repo.init()

	return repo
}

// NewPlanQuotaRepoForTest 与生产 NewPlanQuotaRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewPlanQuotaRepoForTest(dbClient *store.DBClient) *PlanQuotaRepo {
	repo := &PlanQuotaRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PlanQuota, store.PlanQuota](),
		quotaTypeConv: mapper.NewEnumTypeConverter[operatorV1.PlanQuota_QuotaType, modelPlanQuota.QuotaType](
			operatorV1.PlanQuota_QuotaType_name, operatorV1.PlanQuota_QuotaType_value,
		),
	}

	repo.init()

	return repo
}

// NewPositionRepoForTest 与生产 NewPositionRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewPositionRepoForTest(dbClient *store.DBClient) *PositionRepo {
	repo := &PositionRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Position, store.Position](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.Position_Status, modelPosition.Status](
			operatorV1.Position_Status_name, operatorV1.Position_Status_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.Position_Type, modelPosition.Type](
			operatorV1.Position_Type_name, operatorV1.Position_Type_value,
		),
	}

	repo.init()

	return repo
}

// NewApiRepoForTest 与生产 NewApiRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewApiRepoForTest(dbClient *store.DBClient) *ApiRepo {
	repo := &ApiRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Api, store.Api](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.Api_Status, modelApi.Status](
			operatorV1.Api_Status_name, operatorV1.Api_Status_value,
		),
		scopeConverter: mapper.NewEnumTypeConverter[operatorV1.Api_Scope, modelApi.Scope](
			operatorV1.Api_Scope_name, operatorV1.Api_Scope_value,
		),
		businessModuleConverter: mapper.NewEnumTypeConverter[operatorV1.Module_Module, modelApi.BusinessModule](
			operatorV1.Module_Module_name, operatorV1.Module_Module_value,
		),
	}

	repo.init()

	return repo
}

// NewPermissionGroupRepoForTest 与生产 NewPermissionGroupRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewPermissionGroupRepoForTest(dbClient *store.DBClient) *PermissionGroupRepo {
	repo := &PermissionGroupRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PermissionGroup, store.PermissionGroup](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.PermissionGroup_Status, modelPermissionGroup.Status](
			operatorV1.PermissionGroup_Status_name, operatorV1.PermissionGroup_Status_value,
		),
	}

	repo.init()

	return repo
}

// NewMenuRepoForTest 与生产 NewMenuRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewMenuRepoForTest(dbClient *store.DBClient) *MenuRepo {
	repo := &MenuRepo{
		log:             bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:        dbClient,
		mapper:          mapper.NewCopierMapper[operatorV1.Menu, store.Menu](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.Menu_Status, modelMenu.Status](operatorV1.Menu_Status_name, operatorV1.Menu_Status_value),
		typeConverter:   mapper.NewEnumTypeConverter[operatorV1.Menu_Type, modelMenu.Type](operatorV1.Menu_Type_name, operatorV1.Menu_Type_value),
		moduleConverter: mapper.NewEnumTypeConverter[operatorV1.Module_Module, modelMenu.Module](operatorV1.Module_Module_name, operatorV1.Module_Module_value),
	}

	repo.init()

	return repo
}

// NewLanguageRepoForTest 与生产 NewLanguageRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewLanguageRepoForTest(dbClient *store.DBClient) *LanguageRepo {
	repo := &LanguageRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Language, store.Language](),
	}

	repo.init()

	return repo
}

// NewDictTypeRepoForTest 与生产 NewDictTypeRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewDictTypeRepoForTest(dbClient *store.DBClient) *DictTypeRepo {
	repo := &DictTypeRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.DictType, store.DictType](),
	}

	repo.init()

	return repo
}

// testkitDictEntryI18nRepo 私有构造器：与生产 NewDictEntryI18nRepo 逐字段一致
// （log 换 NopLogger），仅供本文件 NewDictEntryRepoForTest 组装关联子 repo 使用。
func testkitDictEntryI18nRepo(dbClient *store.DBClient) *DictEntryI18nRepo {
	repo := &DictEntryI18nRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.DictEntryI18N, store.DictEntryI18n](),
	}

	repo.init()

	return repo
}

// NewDictEntryRepoForTest 与生产 NewDictEntryRepo 逐字段一致（log 换 NopLogger，
// 关联 i18n 子 repo 由 testkitDictEntryI18nRepo 组装），并调用 init()。
func NewDictEntryRepoForTest(dbClient *store.DBClient) *DictEntryRepo {
	repo := &DictEntryRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.DictEntry, store.DictEntry](),
		i18n:     testkitDictEntryI18nRepo(dbClient),
	}

	repo.init()

	return repo
}
