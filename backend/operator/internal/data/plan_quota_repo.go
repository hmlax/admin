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
	"go-admin/operator/internal/data/store/planquota"
	operatorV1 "go-admin/platform/pb/operator"
)

type PlanQuotaRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper        *mapper.CopierMapper[operatorV1.PlanQuota, store.PlanQuota]
	quotaTypeConv *mapper.EnumTypeConverter[operatorV1.PlanQuota_QuotaType, planquota.QuotaType]

	repository *store.Repository[operatorV1.PlanQuota, store.PlanQuota]
}

func NewPlanQuotaRepo(
	ctx *bootstrap.Context,
	dbClient *store.DBClient,
) *PlanQuotaRepo {
	repo := &PlanQuotaRepo{
		log:      ctx.NewLoggerHelper("plan-quota/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PlanQuota, store.PlanQuota](),
		quotaTypeConv: mapper.NewEnumTypeConverter[operatorV1.PlanQuota_QuotaType, planquota.QuotaType](
			operatorV1.PlanQuota_QuotaType_name, operatorV1.PlanQuota_QuotaType_value,
		),
	}

	repo.init()

	return repo
}

func (r *PlanQuotaRepo) init() {
	r.repository = store.NewRepository[operatorV1.PlanQuota, store.PlanQuota](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.quotaTypeConv.NewConverterPair())
}

func (r *PlanQuotaRepo) Count(ctx context.Context, whereCond []queryscope.Scope) (int, error) {
	builder := r.dbClient.Client().PlanQuota.Query()
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

func (r *PlanQuotaRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPlanQuotaResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PlanQuota.Query().WithPlan()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if err != nil {
		r.log.Errorf(ctx, "parse list param error [%s]", err.Error())
		return nil, operatorV1.ErrorIdentityBadRequest("invalid query parameter")
	}

	entities, err := builder.All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query plan quota list failed: %s", err.Error())
		return nil, operatorV1.ErrorIdentityInternalServerError("query plan quota list failed")
	}

	dtos := make([]*operatorV1.PlanQuota, 0, len(entities))
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

	return &operatorV1.ListPlanQuotaResponse{
		Total: uint64(count),
		Items: dtos,
	}, nil
}

func (r *PlanQuotaRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().PlanQuota.Query().
		Where(planquota.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorIdentityInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *PlanQuotaRepo) Get(ctx context.Context, req *operatorV1.GetPlanQuotaRequest) (*operatorV1.PlanQuota, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PlanQuota.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetPlanQuotaRequest_Id:
		whereCond = append(whereCond, planquota.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *PlanQuotaRepo) Create(ctx context.Context, req *operatorV1.CreatePlanQuotaRequest) (err error) {
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

	builder := tx.PlanQuota.Create().
		SetNillableQuotaType(r.quotaTypeConv.ToEntity(req.Data.QuotaType)).
		SetNillableQuotaValue(req.Data.QuotaValue).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	// 原条件写反（== nil 时才 Set），导致请求带 planId 也落库为 NULL
	if req.Data.PlanId != nil {
		builder.SetPlanID(*req.Data.PlanId)
	}

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	if _, err = builder.Save(ctx); err != nil {
		r.log.Errorf(ctx, "insert plan quota failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("insert plan quota failed")
	}

	return nil
}

func (r *PlanQuotaRepo) Update(ctx context.Context, req *operatorV1.UpdatePlanQuotaRequest) (err error) {
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
			createReq := &operatorV1.CreatePlanQuotaRequest{Data: req.Data}
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

	builder := tx.PlanQuota.UpdateOneID(req.GetId())
	_, err = r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.PlanQuota) {
			builder.
				SetNillableQuotaType(r.quotaTypeConv.ToEntity(req.Data.QuotaType)).
				SetNillableQuotaValue(req.Data.QuotaValue).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(planquota.FieldID, req.GetId()),
	)
	if err != nil {
		r.log.Errorf(ctx, "update plan quota failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("update plan quota failed")
	}

	return err
}

func (r *PlanQuotaRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	if err := r.dbClient.Client().PlanQuota.DeleteOneID(id).Exec(ctx); err != nil {
		if store.IsNotFound(err) {
			return operatorV1.ErrorIdentityNotFound("plan quota not found")
		}

		r.log.Errorf(ctx, "delete one data failed: %s", err.Error())

		return operatorV1.ErrorIdentityInternalServerError("delete failed")
	}

	return nil
}
