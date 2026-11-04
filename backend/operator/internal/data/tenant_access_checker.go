package data

import (
	"context"
	"time"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/apiauditlog"
	"go-admin/operator/internal/data/store/plan"
	"go-admin/operator/internal/data/store/planmodule"
	"go-admin/operator/internal/data/store/tenant"

	"go-admin/platform/pkg/middleware/auth"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/tx7do/go-crud/viewer"
)

// TenantAccessCheckerImpl 是 TenantAccessChecker 接口的 data 层实现。
// 仅对租户用户（tenantId>0）调用（中间件已保证），平台管理员（tid=0）不会进入此检查。
type TenantAccessCheckerImpl struct {
	dbClient *store.Client
	log      *bLogger.Helper
}

func NewTenantAccessCheckerImpl(
	ctx *bootstrap.Context,
	dbClient *store.DBClient,
) auth.TenantAccessChecker {
	return &TenantAccessCheckerImpl{
		dbClient: dbClient.Client(),
		log:      ctx.NewLoggerHelper("tenant-access-checker/admin-service"),
	}
}

// CheckTenantAccess 实现 auth.TenantAccessChecker。
// tenantId 由中间件保证 > 0。
func (c *TenantAccessCheckerImpl) CheckTenantAccess(ctx context.Context, tenantId uint32, path string, method string) error {
	sysCtx := viewer.WithSystemContext(ctx)

	// 1. 查租户状态与到期时间（WithPlan(WithQuotas) 预载套餐及其配额边：
	// expiry_policy 供第 2 步，API_CALL 上限供第 4 步——与 GetUsage 同一预载形态）
	t, err := c.dbClient.Tenant.Query().
		Where(tenant.IDEQ(tenantId)).
		WithPlan(func(q *store.PlanQuery) { q.WithQuotas() }).
		Only(sysCtx)
	if err != nil || t == nil {
		c.log.Errorf(ctx, "tenant access check: tenant %d not found: %v", tenantId, err)
		return operatorV1.ErrorAdminForbidden("access denied")
	}

	if t.Status != nil && *t.Status != tenant.StatusOn {
		return operatorV1.ErrorAdminForbidden("tenant is not active")
	}

	// 2. 到期只读判定（READONLY 策略 + 已到期）
	planId := uint32(0)
	var planExpiryPolicy plan.ExpiryPolicy
	if t.Plan != nil {
		planId = t.Plan.ID
		if t.Plan.ExpiryPolicy != nil {
			planExpiryPolicy = *t.Plan.ExpiryPolicy
		}
	}
	planExpired := t.ExpiredAt != nil && !t.ExpiredAt.IsZero() && t.ExpiredAt.Before(time.Now())

	if planExpired && planExpiryPolicy == plan.ExpiryPolicyReadonly {
		switch method {
		case "GET", "HEAD", "OPTIONS":
			// 只读放行，但仍需过白名单（见第 3 步）
		default:
			return operatorV1.ErrorAdminForbidden("tenant is read-only due to expiry")
		}
	}

	// 3. 套餐模块白名单
	bizModule := operatorV1.Module_MODULE_UNSPECIFIED
	a, aerr := c.dbClient.Api.Query().
		Where(api.PathEQ(path), api.MethodEQ(method)).
		Only(sysCtx)
	if aerr == nil && a != nil {
		if a.BusinessModule != nil {
			bizModule = mapApiBusinessModuleToProto(*a.BusinessModule)
		}
	} else {
		// 找不到对应 API 记录，无法判定模块。fail-closed 拒绝。
		c.log.Errorf(ctx, "tenant access check: api not found for %s %s", method, path)
		return operatorV1.ErrorAdminForbidden("access denied")
	}

	if bizModule == operatorV1.Module_MODULE_UNSPECIFIED {
		// 未归类模块：不在任何白名单内，拒绝。
		return operatorV1.ErrorAdminForbidden("module not allowed")
	}

	if planId == 0 {
		// 租户未关联套餐，无白名单，拒绝所有业务模块。
		return operatorV1.ErrorAdminForbidden("no subscription plan")
	}

	allowed, lerr := c.isModuleAllowed(sysCtx, planId, bizModule)
	if lerr != nil {
		c.log.Errorf(ctx, "tenant access check: whitelist query failed: %v", lerr)
		return operatorV1.ErrorAdminForbidden("access denied")
	}
	if !allowed {
		return operatorV1.ErrorAdminForbidden("module not allowed")
	}

	// 4. 套餐配额：API_CALL 调用量（plan_billing.md §7.2）。上限取自第 1 步预载的
	// 套餐配额边，当前计数与 §7.1 ApiCallCount 同源（sys_api_audit_logs 按租户
	// COUNT，活表现存行数即口径）。判定函数与 fail-open 语义见 quota_gate.go。
	if t.Plan != nil {
		limit, hasLimit := apiCallLimitFromPlanQuotas(t.Plan.Quotas)
		if hasLimit {
			cnt, cerr := c.dbClient.ApiAuditLog.Query().
				Where(apiauditlog.TenantIDEQ(tenantId)).
				Count(sysCtx)
			if cerr != nil {
				// 计量查询失败 fail-open：配额基础设施抖动不阻断租户业务。
				c.log.Errorf(ctx, "tenant access check: api call metering for tenant %d failed: %v", tenantId, cerr)
				return nil
			}
			if qerr := checkApiCallQuota(limit, true, cnt); qerr != nil {
				c.log.Warnf(ctx, "tenant access check: tenant %d request %s %s blocked by API_CALL quota (limit %d, current %d)", tenantId, method, path, limit, cnt)
				return qerr
			}
		}
	}

	return nil
}

// isModuleAllowed 查询套餐白名单是否包含指定模块。
func (c *TenantAccessCheckerImpl) isModuleAllowed(ctx context.Context, planId uint32, module operatorV1.Module_Module) (bool, error) {
	modelModule := mapProtoModuleToModel(module)
	if modelModule == "" {
		return false, nil
	}
	cnt, err := c.dbClient.PlanModule.Query().
		Where(
			planmodule.HasPlanWith(plan.IDEQ(planId)),
			planmodule.ModuleEQ(modelModule),
		).
		Count(ctx)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// mapProtoModuleToModel 将 proto Module 值映射到 GORM planmodule.Module 字符串。
// 返回空串表示 UNSPECIFIED。
func mapProtoModuleToModel(m operatorV1.Module_Module) planmodule.Module {
	switch m {
	case operatorV1.Module_DASHBOARD:
		return planmodule.ModuleDashboard
	case operatorV1.Module_OPM:
		return planmodule.ModuleOpm
	case operatorV1.Module_SYSTEM:
		return planmodule.ModuleSystem
	case operatorV1.Module_DICT:
		return planmodule.ModuleDict
	case operatorV1.Module_TENANT:
		return planmodule.ModuleTenant
	case operatorV1.Module_PERMISSION:
		return planmodule.ModulePermission
	case operatorV1.Module_LOG:
		return planmodule.ModuleLog
	case operatorV1.Module_INTERNAL_MESSAGE:
		return planmodule.ModuleInternalMessage
	case operatorV1.Module_FILE:
		return planmodule.ModuleFile
	case operatorV1.Module_TASK:
		return planmodule.ModuleTask
	case operatorV1.Module_AI:
		return planmodule.ModuleAi
	default:
		return ""
	}
}

// mapApiBusinessModuleToProto 将 GORM api.BusinessModule 字符串枚举映射到 proto identityV1.Module。
// 返回 MODULE_UNSPECIFIED 表示未归类。
func mapApiBusinessModuleToProto(m api.BusinessModule) operatorV1.Module_Module {
	switch m {
	case api.BusinessModuleDashboard:
		return operatorV1.Module_DASHBOARD
	case api.BusinessModuleOpm:
		return operatorV1.Module_OPM
	case api.BusinessModuleSystem:
		return operatorV1.Module_SYSTEM
	case api.BusinessModuleDict:
		return operatorV1.Module_DICT
	case api.BusinessModuleTenant:
		return operatorV1.Module_TENANT
	case api.BusinessModulePermission:
		return operatorV1.Module_PERMISSION
	case api.BusinessModuleLog:
		return operatorV1.Module_LOG
	case api.BusinessModuleInternalMessage:
		return operatorV1.Module_INTERNAL_MESSAGE
	case api.BusinessModuleFile:
		return operatorV1.Module_FILE
	case api.BusinessModuleTask:
		return operatorV1.Module_TASK
	case api.BusinessModuleAi:
		return operatorV1.Module_AI
	default:
		return operatorV1.Module_MODULE_UNSPECIFIED
	}
}
