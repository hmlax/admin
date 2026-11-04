package data

import (
	"context"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/plan"
	"go-admin/operator/internal/data/store/planmodule"
	operatorV1 "go-admin/platform/pb/operator"
)

type PlanModuleRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper     *mapper.CopierMapper[operatorV1.PlanModule, store.PlanModule]
	moduleConv *mapper.EnumTypeConverter[operatorV1.Module_Module, planmodule.Module]

	repository *store.Repository[operatorV1.PlanModule, store.PlanModule]
}

func NewPlanModuleRepo(
	ctx *bootstrap.Context,
	dbClient *store.DBClient,
) *PlanModuleRepo {
	repo := &PlanModuleRepo{
		log:      ctx.NewLoggerHelper("plan-module/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PlanModule, store.PlanModule](),
		moduleConv: mapper.NewEnumTypeConverter[operatorV1.Module_Module, planmodule.Module](
			operatorV1.Module_Module_name, operatorV1.Module_Module_value,
		),
	}

	repo.init()

	return repo
}

func (r *PlanModuleRepo) init() {
	r.repository = store.NewRepository[operatorV1.PlanModule, store.PlanModule](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.moduleConv.NewConverterPair())
}

func (r *PlanModuleRepo) Count(ctx context.Context, whereCond []queryscope.Scope) (int, error) {
	builder := r.dbClient.Client().PlanModule.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query count failed: %s", err.Error())
		return 0, operatorV1.ErrorIdentityInternalServerError("query count failed")
	}

	return count, nil
}

func (r *PlanModuleRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPlanModuleResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PlanModule.Query().WithPlan()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if err != nil {
		r.log.Errorf(ctx, "parse list param error [%s]", err.Error())
		return nil, operatorV1.ErrorIdentityBadRequest("invalid query parameter")
	}

	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query plan module list failed: %s", err.Error())
		return nil, operatorV1.ErrorIdentityInternalServerError("query plan module list failed")
	}

	dtos := make([]*operatorV1.PlanModule, 0, len(entities))
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		if entity.Plan != nil {
			dto.PlanId = &entity.Plan.ID
		}
		dtos = append(dtos, dto)
	}

	count, err := r.Count(ctx, whereSelectors)
	if err != nil {
		return nil, err
	}

	return &operatorV1.ListPlanModuleResponse{
		Total: uint64(count),
		Items: dtos,
	}, nil
}

func (r *PlanModuleRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().PlanModule.Query().
		Where(planmodule.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorIdentityInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *PlanModuleRepo) Get(ctx context.Context, req *operatorV1.GetPlanModuleRequest) (*operatorV1.PlanModule, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PlanModule.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetPlanModuleRequest_Id:
		whereCond = append(whereCond, planmodule.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *PlanModuleRepo) Create(ctx context.Context, req *operatorV1.CreatePlanModuleRequest) (err error) {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	var tx *store.Tx
	tx, err = r.dbClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf(ctx, "start transaction failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf(ctx, "transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf(ctx, "transaction commit failed: %s", commitErr.Error())
			err = operatorV1.ErrorIdentityInternalServerError("transaction commit failed")
		}
	}()

	builder := tx.PlanModule.Create().
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	// module 为 proto 零值（MODULE_UNSPECIFIED）时跳过：GORM schema 未声明该值，
	// SetNillableModule 会触发 ModuleValidator 失败。未指定即留空。
	if req.Data.Module != nil && *req.Data.Module != operatorV1.Module_MODULE_UNSPECIFIED {
		builder.SetNillableModule(r.moduleConv.ToEntity(req.Data.Module))
	}

	// 此前守卫倒置（PlanId == nil 才设置），导致 plan_id 永远写空、
	// 套餐模块关联自始无法通过 API 建立。有值即设置。
	if req.Data.PlanId != nil {
		builder.SetPlanID(req.Data.GetPlanId())
	}

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	if _, err = builder.Save(ctx); err != nil {
		r.log.Errorf(ctx, "insert plan module failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("insert plan module failed")
	}

	return nil
}

func (r *PlanModuleRepo) Update(ctx context.Context, req *operatorV1.UpdatePlanModuleRequest) (err error) {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorIdentityBadRequest("id is required")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		var exist bool
		exist, err = r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &operatorV1.CreatePlanModuleRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	var tx *store.Tx
	tx, err = r.dbClient.Client().Tx(ctx)
	if err != nil {
		r.log.Errorf(ctx, "start transaction failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("start transaction failed")
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				r.log.Errorf(ctx, "transaction rollback failed: %s", rollbackErr.Error())
			}
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			r.log.Errorf(ctx, "transaction commit failed: %s", commitErr.Error())
			err = operatorV1.ErrorIdentityInternalServerError("transaction commit failed")
		}
	}()

	builder := tx.PlanModule.UpdateOneID(req.GetId())
	_, err = r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.PlanModule) {
			builder.
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			// module 为 proto 零值（MODULE_UNSPECIFIED）时跳过：GORM schema 未声明该值，
			// SetNillableModule 会触发 ModuleValidator 失败。未指定即不更新该字段。
			if req.Data.Module != nil && *req.Data.Module != operatorV1.Module_MODULE_UNSPECIFIED {
				builder.SetNillableModule(r.moduleConv.ToEntity(req.Data.Module))
			}
		},
		queryscope.Equal(planmodule.FieldID, req.GetId()),
	)
	if err != nil {
		r.log.Errorf(ctx, "update plan module failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("update plan module failed")
	}

	return err
}

func (r *PlanModuleRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	if err := r.dbClient.Client().PlanModule.DeleteOneID(id).Exec(ctx); err != nil {
		if store.IsNotFound(err) {
			return operatorV1.ErrorIdentityNotFound("plan module not found")
		}

		r.log.Errorf(ctx, "delete one data failed: %s", err.Error())

		return operatorV1.ErrorIdentityInternalServerError("delete failed")
	}

	return nil
}

// ListModulesByPlanId 返回指定套餐允许的模块集合。
// 用于菜单树过滤和请求拦截的白名单判定。
func (r *PlanModuleRepo) ListModulesByPlanId(ctx context.Context, planId uint32) (map[operatorV1.Module_Module]bool, error) {
	if planId == 0 {
		return nil, nil
	}
	entities, err := r.dbClient.Client().PlanModule.Query().
		Where(planmodule.HasPlanWith(plan.IDEQ(planId))).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query plan modules by plan id failed: %s", err.Error())
		return nil, operatorV1.ErrorIdentityInternalServerError("query plan modules failed")
	}
	result := make(map[operatorV1.Module_Module]bool, len(entities))
	for _, e := range entities {
		if e.Module != nil {
			result[mapModelModuleToProto(*e.Module)] = true
		}
	}
	return result, nil
}

// mapModelModuleToProto 将 GORM planmodule.Module 字符串枚举映射到 proto identityV1.Module。
func mapModelModuleToProto(m planmodule.Module) operatorV1.Module_Module {
	switch m {
	case planmodule.ModuleDashboard:
		return operatorV1.Module_DASHBOARD
	case planmodule.ModuleOpm:
		return operatorV1.Module_OPM
	case planmodule.ModuleSystem:
		return operatorV1.Module_SYSTEM
	case planmodule.ModuleDict:
		return operatorV1.Module_DICT
	case planmodule.ModuleTenant:
		return operatorV1.Module_TENANT
	case planmodule.ModulePermission:
		return operatorV1.Module_PERMISSION
	case planmodule.ModuleLog:
		return operatorV1.Module_LOG
	case planmodule.ModuleInternalMessage:
		return operatorV1.Module_INTERNAL_MESSAGE
	case planmodule.ModuleFile:
		return operatorV1.Module_FILE
	case planmodule.ModuleTask:
		return operatorV1.Module_TASK
	default:
		return operatorV1.Module_MODULE_UNSPECIFIED
	}
}
