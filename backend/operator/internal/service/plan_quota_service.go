package service

import (
	"context"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-admin/operator/internal/data"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"
)

type PlanQuotaService struct {
	operatorV1.UnimplementedPlanQuotaServiceServer
	log           *bLogger.Helper
	planQuotaRepo *data.PlanQuotaRepo
}

func NewPlanQuotaService(
	ctx *bootstrap.Context,
	planQuotaRepo *data.PlanQuotaRepo,
) *PlanQuotaService {
	return &PlanQuotaService{
		log:           ctx.NewLoggerHelper("plan-quota/service/admin-service"),
		planQuotaRepo: planQuotaRepo,
	}
}

func (s *PlanQuotaService) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPlanQuotaResponse, error) {
	return s.planQuotaRepo.List(ctx, req)
}

func (s *PlanQuotaService) Get(ctx context.Context, req *operatorV1.GetPlanQuotaRequest) (*operatorV1.PlanQuota, error) {
	return s.planQuotaRepo.Get(ctx, req)
}

func (s *PlanQuotaService) Create(ctx context.Context, req *operatorV1.CreatePlanQuotaRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.Data.CreatedBy = trans.Ptr(operator.UserId)

	if err = s.planQuotaRepo.Create(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PlanQuotaService) Update(ctx context.Context, req *operatorV1.UpdatePlanQuotaRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.Data.Id = trans.Ptr(req.GetId())

	req.Data.UpdatedBy = trans.Ptr(operator.UserId)
	if req.UpdateMask != nil {
		req.UpdateMask.Paths = append(req.UpdateMask.Paths, "updated_by")
	}

	if err = s.planQuotaRepo.Update(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PlanQuotaService) Delete(ctx context.Context, req *operatorV1.DeletePlanQuotaRequest) (*emptypb.Empty, error) {
	if err := s.planQuotaRepo.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
