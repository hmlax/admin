package data

import (
	"context"

	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/api"
	"go-admin/operator/internal/data/store/apiauditlog"
	"go-admin/operator/internal/data/store/tenant"
	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/portalscope"
)

// GatewayAccessRepo reads entry-gate facts; admin makes every access decision.
type GatewayAccessRepo struct{ checker *TenantAccessCheckerImpl }

func NewGatewayAccessRepo(ctx *bootstrap.Context, client *store.DBClient) *GatewayAccessRepo {
	return &GatewayAccessRepo{checker: &TenantAccessCheckerImpl{
		dbClient: client.Client(),
		log:      ctx.NewLoggerHelper("gateway/access-data"),
	}}
}

func (r *GatewayAccessRepo) ResolveTenantCode(ctx context.Context, code string) (*operatorV1.ResolveTenantCodeResponse, error) {
	ctx = viewer.WithSystemContext(ctx)
	row, err := r.checker.dbClient.Tenant.Query().Where(tenant.CodeEQ(code)).Only(ctx)
	if store.IsNotFound(err) {
		return nil, operatorV1.ErrorAdminForbidden(portalscope.ErrorUnavailable)
	}
	if err != nil {
		return nil, err
	}
	return &operatorV1.ResolveTenantCodeResponse{TenantId: row.ID}, nil
}
func (r *GatewayAccessRepo) Get(ctx context.Context, req *operatorV1.TenantAccessRequest) (*operatorV1.TenantAccessData, error) {
	ctx = viewer.WithSystemContext(ctx)
	out := new(operatorV1.TenantAccessData)
	client := r.checker.dbClient
	t, err := client.Tenant.Query().Where(tenant.IDEQ(req.GetTenantId())).WithPlan(func(q *store.PlanQuery) { q.WithQuotas() }).Only(ctx)
	if store.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out.TenantExists = true
	if t.Status != nil {
		v := string(*t.Status)
		out.Status = &v
	}
	if t.ExpiredAt != nil && !t.ExpiredAt.IsZero() {
		v := t.ExpiredAt.Unix()
		out.ExpiresAt = &v
	}
	if p := t.Plan; p != nil {
		out.PlanId = p.ID
		if p.ExpiryPolicy != nil {
			out.ExpiryPolicy = string(*p.ExpiryPolicy)
		}
		if limit, ok := apiCallLimitFromPlanQuotas(p.Quotas); ok {
			out.ApiCallLimit = &limit
			count, countErr := client.ApiAuditLog.Query().Where(apiauditlog.TenantIDEQ(req.GetTenantId())).Count(ctx)
			if countErr == nil {
				n := uint64(count)
				out.ApiCallCount = &n
			} else {
				r.checker.log.Errorf(ctx, "API metering failed: %v", countErr)
			}
		}
	}
	a, err := client.Api.Query().Where(api.PathEQ(req.GetPath()), api.MethodEQ(req.GetMethod())).Only(ctx)
	if store.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	out.ApiExists = true
	if a.BusinessModule == nil {
		return out, nil
	}
	module := mapApiBusinessModuleToProto(*a.BusinessModule)
	out.ModuleClassified = module != operatorV1.Module_MODULE_UNSPECIFIED
	if out.PlanId != 0 && out.ModuleClassified {
		out.ModuleInPlan, err = r.checker.isModuleAllowed(ctx, out.PlanId, module)
	}
	return out, err
}
