package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-admin/operator/internal/data"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"
)

// MyTenantUsageService 租户自助用量：租户管理员查看本租户的套餐用量与配额。
//
// 与 TenantService.GetUsage（平台管理员按 ID 查任意租户）互补——本服务把
// 租户 ID 钉死为 operator 的租户，不接受客户端传参，租户管理员看不到别的租户。
//
// 存在的意义：配额硬限制（USER_LIMIT/STORAGE）落地后，租户用户撞 403
// plan quota exceeded 时需要有地方看到"用了多少/上限多少"，否则报错无解释。
type MyTenantUsageService struct {
	operatorV1.UnimplementedMyTenantUsageServiceServer
	usageRepo *data.TenantUsageRepo
	log       *bLogger.Helper
}

func NewMyTenantUsageService(ctx *bootstrap.Context, usageRepo *data.TenantUsageRepo) *MyTenantUsageService {
	return &MyTenantUsageService{
		usageRepo: usageRepo,
		log:       ctx.NewLoggerHelper("my-tenant-usage/service/admin-service"),
	}
}

// GetMyTenantUsage 返回当前操作者所属租户的用量与配额。
// 平台用户（tenantId==0）无套餐语义，返回空 Usage（前端不展示该卡片）。
func (s *MyTenantUsageService) GetMyTenantUsage(ctx context.Context, _ *emptypb.Empty) (*operatorV1.TenantUsage, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := operator.GetTenantId()
	if tenantID == 0 {
		return &operatorV1.TenantUsage{TenantId: 0}, nil
	}

	usage, err := s.usageRepo.GetUsage(ctx, tenantID)
	if err != nil {
		s.log.Errorf(ctx, "get my tenant usage failed for tenant [%d]: %v", tenantID, err)
		return nil, operatorV1.ErrorIdentityInternalServerError("get tenant usage failed")
	}
	return usage, nil
}
