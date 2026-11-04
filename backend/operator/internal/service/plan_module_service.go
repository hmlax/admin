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

type PlanModuleService struct {
	operatorV1.UnimplementedPlanModuleServiceServer
	log            *bLogger.Helper
	planModuleRepo *data.PlanModuleRepo
}

func NewPlanModuleService(
	ctx *bootstrap.Context,
	planModuleRepo *data.PlanModuleRepo,
) *PlanModuleService {
	return &PlanModuleService{
		log:            ctx.NewLoggerHelper("plan-module/service/admin-service"),
		planModuleRepo: planModuleRepo,
	}
}

func (s *PlanModuleService) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPlanModuleResponse, error) {
	return s.planModuleRepo.List(ctx, req)
}

func (s *PlanModuleService) Get(ctx context.Context, req *operatorV1.GetPlanModuleRequest) (*operatorV1.PlanModule, error) {
	return s.planModuleRepo.Get(ctx, req)
}

func (s *PlanModuleService) Create(ctx context.Context, req *operatorV1.CreatePlanModuleRequest) (*emptypb.Empty, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	// 获取操作人信息
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	req.Data.CreatedBy = trans.Ptr(operator.UserId)

	if err = s.planModuleRepo.Create(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PlanModuleService) Update(ctx context.Context, req *operatorV1.UpdatePlanModuleRequest) (*emptypb.Empty, error) {
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

	if err = s.planModuleRepo.Update(ctx, req); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *PlanModuleService) Delete(ctx context.Context, req *operatorV1.DeletePlanModuleRequest) (*emptypb.Empty, error) {
	if err := s.planModuleRepo.Delete(ctx, req.GetId()); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
