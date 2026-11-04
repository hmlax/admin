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
	operatorV1 "go-admin/platform/pb/operator"
)

type PlanRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper           *mapper.CopierMapper[operatorV1.Plan, store.Plan]
	versionConverter *mapper.EnumTypeConverter[operatorV1.Plan_Version, plan.Version]
	expiryPolicyConv *mapper.EnumTypeConverter[operatorV1.Plan_ExpiryPolicy, plan.ExpiryPolicy]

	repository *store.Repository[operatorV1.Plan, store.Plan]
}

func NewPlanRepo(
	ctx *bootstrap.Context,
	dbClient *store.DBClient,
) *PlanRepo {
	repo := &PlanRepo{
		log:      ctx.NewLoggerHelper("plan/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Plan, store.Plan](),
		versionConverter: mapper.NewEnumTypeConverter[operatorV1.Plan_Version, plan.Version](
			operatorV1.Plan_Version_name, operatorV1.Plan_Version_value,
		),
		expiryPolicyConv: mapper.NewEnumTypeConverter[operatorV1.Plan_ExpiryPolicy, plan.ExpiryPolicy](
			operatorV1.Plan_ExpiryPolicy_name, operatorV1.Plan_ExpiryPolicy_value,
		),
	}

	repo.init()

	return repo
}

func (r *PlanRepo) init() {
	r.repository = store.NewRepository[operatorV1.Plan, store.Plan](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.versionConverter.NewConverterPair())
	r.mapper.AppendConverters(r.expiryPolicyConv.NewConverterPair())
}

func (r *PlanRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (int, error) {
	builder := r.dbClient.Client().Plan.Query()

	whereSelectors, _, err := r.repository.BuildListSelectorWithPaging(builder, req)
	if err != nil {
		r.log.Errorf(ctx, "parse count param error [%s]", err.Error())
		return 0, operatorV1.ErrorIdentityInternalServerError("invalid query parameter")
	}
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query count failed: %s", err.Error())
		return 0, operatorV1.ErrorIdentityInternalServerError("query count failed")
	}

	return count, nil
}

func (r *PlanRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPlanResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Plan.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListPlanResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListPlanResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *PlanRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().Plan.Query().
		Where(plan.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorIdentityInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *PlanRepo) Get(ctx context.Context, req *operatorV1.GetPlanRequest) (*operatorV1.Plan, error) {
	if req == nil {
		return nil, operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Plan.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetPlanRequest_Id:
		whereCond = append(whereCond, plan.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *PlanRepo) Create(ctx context.Context, req *operatorV1.CreatePlanRequest) (err error) {
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

	builder := tx.Plan.Create().
		SetNillableName(req.Data.Name).
		SetNillableVersion(r.versionConverter.ToEntity(req.Data.Version)).
		SetNillableExpiryPolicy(r.expiryPolicyConv.ToEntity(req.Data.ExpiryPolicy)).
		SetNillableDataRetentionDays(req.Data.DataRetentionDays).
		SetNillableDescription(req.Data.Description).
		SetNillableRemark(req.Data.Remark).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	if _, err = builder.Save(ctx); err != nil {
		// sys_plans.name 有唯一索引：重名是用户可自行纠正的输入错误，返回 400 而非 500
		if store.IsConstraintError(err) {
			r.log.Warnf(ctx, "insert plan duplicated: %s", err.Error())
			return operatorV1.ErrorIdentityBadRequest("plan name [%s] already exists", req.Data.GetName())
		}
		r.log.Errorf(ctx, "insert plan failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("insert plan failed")
	}

	return nil
}

func (r *PlanRepo) Update(ctx context.Context, req *operatorV1.UpdatePlanRequest) (err error) {
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
			createReq := &operatorV1.CreatePlanRequest{Data: req.Data}
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

	builder := tx.Plan.UpdateOneID(req.GetId())
	_, err = r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.Plan) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableVersion(r.versionConverter.ToEntity(req.Data.Version)).
				SetNillableExpiryPolicy(r.expiryPolicyConv.ToEntity(req.Data.ExpiryPolicy)).
				SetNillableDataRetentionDays(req.Data.DataRetentionDays).
				SetNillableDescription(req.Data.Description).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(plan.FieldID, req.GetId()),
	)
	if err != nil {
		// 名称改重时命中 uix_sys_plans_name 唯一索引，返回 400 而非 500
		if store.IsConstraintError(err) {
			r.log.Warnf(ctx, "update plan duplicated: %s", err.Error())
			return operatorV1.ErrorIdentityBadRequest("plan name [%s] already exists", req.Data.GetName())
		}
		r.log.Errorf(ctx, "update plan failed: %s", err.Error())
		return operatorV1.ErrorIdentityInternalServerError("update plan failed")
	}

	return err
}

func (r *PlanRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorIdentityBadRequest("invalid parameter")
	}

	if err := r.dbClient.Client().Plan.DeleteOneID(id).Exec(ctx); err != nil {
		if store.IsNotFound(err) {
			return operatorV1.ErrorIdentityNotFound("plan not found")
		}

		r.log.Errorf(ctx, "delete one data failed: %s", err.Error())

		return operatorV1.ErrorIdentityInternalServerError("delete failed")
	}

	return nil
}
