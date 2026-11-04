// 本文件对 TenantAccessCheckerImpl 做决策矩阵测试（SQLite 内存库）：
//
//	租户不存在 → fail-closed 拒绝；
//	租户状态 OFF/EXPIRED/FREEZE → 拒绝；
//	API 表无 (path,method) 记录 → fail-closed 拒绝；
//	API 有记录但未归类业务模块（UNSPECIFIED）→ 拒绝；
//	租户未挂套餐 → 拒绝；套餐白名单不含该模块 → 拒绝；白名单命中 → 放行；
//	已到期 + READONLY 策略：GET/HEAD/OPTIONS 放行、写方法拒绝；
//	未到期或非 READONLY 策略：不做只读门控（按白名单判定）。
//
// 另附 mapProtoModuleToModel / mapApiBusinessModuleToProto 纯映射函数的全量枚举测试。
// 该闸门是租户隔离的 HTTP 层强制点，语义见 docs/tenant_isolation.md。
package data

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-kratos/kratos/v2/errors"

	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/plan"
	"go-admin/operator/internal/data/store/planmodule"
	"go-admin/operator/internal/data/store/tenant"
	storetest "go-admin/operator/internal/data/storetest"
	operatorV1 "go-admin/platform/pb/operator"
)

// newCheckerSqlite 白盒构造被测闸门（字段与生产构造器一致，仅 log 换 Nop）。
func newCheckerSqlite(t *testing.T) (*TenantAccessCheckerImpl, *store.Client, context.Context) {
	t.Helper()
	client := storetest.NewClientForTest(t).Client()
	sysCtx := storetest.NewSystemContext(context.Background())
	return &TenantAccessCheckerImpl{
		dbClient: client,
		log:      bLogger.NewHelper(bLogger.NopLogger()),
	}, client, sysCtx
}

// forbiddenMessage 取 kratos 错误的 message 文案（本闸门全部以 *errors.Error 返回）。
func forbiddenMessage(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	e := errors.FromError(err)
	require.NotNil(t, e)
	return e.GetMessage()
}

// TestTenantAccessCheckerDecisionMatrix 决策矩阵逐分支断言。
func TestTenantAccessCheckerDecisionMatrix(t *testing.T) {
	pastTime := time.Now().Add(-24 * time.Hour)

	t.Run("租户不存在_failclosed", func(t *testing.T) {
		checker, _, _ := newCheckerSqlite(t)
		err := checker.CheckTenantAccess(context.Background(), 999, "/x", "GET")
		require.Contains(t, forbiddenMessage(t, err), "access denied")
	})

	t.Run("租户状态OFF_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-off").SetStatus(tenant.StatusOff).SaveX(sysCtx).ID
		err := checker.CheckTenantAccess(context.Background(), tid, "/x", "GET")
		require.Contains(t, forbiddenMessage(t, err), "tenant is not active")
	})

	t.Run("租户状态FREEZE_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-freeze").SetStatus(tenant.StatusFreeze).SaveX(sysCtx).ID
		err := checker.CheckTenantAccess(context.Background(), tid, "/x", "GET")
		require.Contains(t, forbiddenMessage(t, err), "tenant is not active")
	})

	t.Run("租户状态EXPIRED_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-exp").SetStatus(tenant.StatusExpired).SaveX(sysCtx).ID
		err := checker.CheckTenantAccess(context.Background(), tid, "/x", "GET")
		require.Contains(t, forbiddenMessage(t, err), "tenant is not active")
	})

	t.Run("API表无记录_failclosed", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-noapi").SetStatus(tenant.StatusOn).SaveX(sysCtx).ID
		err := checker.CheckTenantAccess(context.Background(), tid, "/not/registered", "GET")
		require.Contains(t, forbiddenMessage(t, err), "access denied")
	})

	t.Run("API未归类模块_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-unclassified").SetStatus(tenant.StatusOn).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/unclassified").SetMethod("GET").SaveX(sysCtx) // business_module 留空 → UNSPECIFIED
		err := checker.CheckTenantAccess(context.Background(), tid, "/t/unclassified", "GET")
		require.Contains(t, forbiddenMessage(t, err), "module not allowed")
	})

	t.Run("租户未挂套餐_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		tid := client.Tenant.Create().SetName("t-noplan").SetStatus(tenant.StatusOn).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/noplan").SetMethod("GET").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		err := checker.CheckTenantAccess(context.Background(), tid, "/t/noplan", "GET")
		require.Contains(t, forbiddenMessage(t, err), "no subscription plan")
	})

	t.Run("套餐白名单不含模块_拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyReadonly).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-nowhitelist").SetStatus(tenant.StatusOn).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/nowhitelist").SetMethod("GET").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		// 套餐白名单只挂 FILE，请求 DICT 模块 → 拒绝
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleFile).SaveX(sysCtx)
		err := checker.CheckTenantAccess(context.Background(), tid, "/t/nowhitelist", "GET")
		require.Contains(t, forbiddenMessage(t, err), "module not allowed")
	})

	t.Run("白名单命中_放行", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyReadonly).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-allow").SetStatus(tenant.StatusOn).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/allow").SetMethod("GET").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleDict).SaveX(sysCtx)
		require.NoError(t, checker.CheckTenantAccess(context.Background(), tid, "/t/allow", "GET"))
	})

	t.Run("未到期READONLY_写方法放行", func(t *testing.T) {
		// 未到期：只读门控不生效（须同时满足已到期 + READONLY）
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyReadonly).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-notexpired").SetStatus(tenant.StatusOn).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/notexpired").SetMethod("POST").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleDict).SaveX(sysCtx)
		require.NoError(t, checker.CheckTenantAccess(context.Background(), tid, "/t/notexpired", "POST"))
	})

	t.Run("到期READONLY_GET放行", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyReadonly).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-readonly-get").SetStatus(tenant.StatusOn).
			SetNillableExpiredAt(&pastTime).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/readonlyget").SetMethod("GET").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleDict).SaveX(sysCtx)
		// 只读放行但仍过白名单：白名单未挂 FILE → FILE 模块即便 GET 也拒绝
		_ = client.Api.Create().SetPath("/t/readonlyget2").SetMethod("GET").SetBusinessModule(api.BusinessModuleFile).SaveX(sysCtx)
		require.NoError(t, checker.CheckTenantAccess(context.Background(), tid, "/t/readonlyget", "GET"))
		err := checker.CheckTenantAccess(context.Background(), tid, "/t/readonlyget2", "GET")
		require.Contains(t, forbiddenMessage(t, err), "module not allowed")
	})

	t.Run("到期READONLY_写方法拒绝", func(t *testing.T) {
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyReadonly).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-readonly-post").SetStatus(tenant.StatusOn).
			SetNillableExpiredAt(&pastTime).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/readonlypost").SetMethod("POST").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleDict).SaveX(sysCtx)
		err := checker.CheckTenantAccess(context.Background(), tid, "/t/readonlypost", "POST")
		require.Contains(t, forbiddenMessage(t, err), "read-only")
	})

	t.Run("到期BLOCKLOGIN_无只读门控", func(t *testing.T) {
		// 到期但策略为 BLOCK_LOGIN（≠READONLY）：只读门控不生效，按白名单判定
		checker, client, sysCtx := newCheckerSqlite(t)
		p := client.Plan.Create().SetExpiryPolicy(plan.ExpiryPolicyBlockLogin).SaveX(sysCtx)
		tid := client.Tenant.Create().SetName("t-blocklogin").SetStatus(tenant.StatusOn).
			SetNillableExpiredAt(&pastTime).SetPlanID(p.ID).SaveX(sysCtx).ID
		_ = client.Api.Create().SetPath("/t/blocklogin").SetMethod("POST").SetBusinessModule(api.BusinessModuleDict).SaveX(sysCtx)
		_ = client.PlanModule.Create().SetPlanID(p.ID).SetModule(planmodule.ModuleDict).SaveX(sysCtx)
		require.NoError(t, checker.CheckTenantAccess(context.Background(), tid, "/t/blocklogin", "POST"))
	})
}

// TestModuleMapping 全量枚举 proto↔GORM 模块映射：九个业务模块双向一致，
// UNSPECIFIED / 未知值 / 空串各自落到默认分支。
func TestModuleMapping(t *testing.T) {
	pairs := map[operatorV1.Module_Module]struct {
		modelModule planmodule.Module
		apiModule   api.BusinessModule
	}{
		operatorV1.Module_DASHBOARD:        {planmodule.ModuleDashboard, api.BusinessModuleDashboard},
		operatorV1.Module_OPM:              {planmodule.ModuleOpm, api.BusinessModuleOpm},
		operatorV1.Module_SYSTEM:           {planmodule.ModuleSystem, api.BusinessModuleSystem},
		operatorV1.Module_DICT:             {planmodule.ModuleDict, api.BusinessModuleDict},
		operatorV1.Module_TENANT:           {planmodule.ModuleTenant, api.BusinessModuleTenant},
		operatorV1.Module_PERMISSION:       {planmodule.ModulePermission, api.BusinessModulePermission},
		operatorV1.Module_LOG:              {planmodule.ModuleLog, api.BusinessModuleLog},
		operatorV1.Module_INTERNAL_MESSAGE: {planmodule.ModuleInternalMessage, api.BusinessModuleInternalMessage},
		operatorV1.Module_FILE:             {planmodule.ModuleFile, api.BusinessModuleFile},
		operatorV1.Module_TASK:             {planmodule.ModuleTask, api.BusinessModuleTask},
	}

	for protoMod, modelMods := range pairs {
		require.Equal(t, modelMods.modelModule, mapProtoModuleToModel(protoMod), "proto→GORM(planmodule) 映射漂移: %v", protoMod)
		require.Equal(t, protoMod, mapApiBusinessModuleToProto(modelMods.apiModule), "GORM(api)→proto 映射漂移: %v", modelMods.apiModule)
	}

	// 默认分支：UNSPECIFIED/未知 proto 值 → 空串；空 GORM 串 → UNSPECIFIED。
	require.Empty(t, mapProtoModuleToModel(operatorV1.Module_MODULE_UNSPECIFIED))
	require.Empty(t, mapProtoModuleToModel(operatorV1.Module_Module(9999)))
	require.Equal(t, operatorV1.Module_MODULE_UNSPECIFIED, mapApiBusinessModuleToProto(""))
}
