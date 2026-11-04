package data

import (
	"context"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/internalmessagecategory"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
)

type InternalMessageCategoryRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.InternalMessageCategory, store.InternalMessageCategory]

	repository *store.Repository[operatorV1.InternalMessageCategory, store.InternalMessageCategory]
}

func NewInternalMessageCategoryRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *InternalMessageCategoryRepo {
	repo := &InternalMessageCategoryRepo{
		log:      ctx.NewLoggerHelper("internal-message-category/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.InternalMessageCategory, store.InternalMessageCategory](),
	}

	repo.init()

	return repo
}

func (r *InternalMessageCategoryRepo) init() {
	r.repository = store.NewRepository[operatorV1.InternalMessageCategory, store.InternalMessageCategory](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *InternalMessageCategoryRepo) Count(ctx context.Context, whereCond []queryscope.Scope) (int, error) {
	builder := r.dbClient.Client().InternalMessageCategory.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query count failed: %s", err.Error())
		return 0, operatorV1.ErrorInternalMessageInternalServerError("query count failed")
	}

	return count, nil
}

func (r *InternalMessageCategoryRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListInternalMessageCategoryResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessageCategory.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListInternalMessageCategoryResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListInternalMessageCategoryResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *InternalMessageCategoryRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().InternalMessageCategory.Query().
		Where(internalmessagecategory.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorInternalMessageInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *InternalMessageCategoryRepo) Get(ctx context.Context, req *operatorV1.GetInternalMessageCategoryRequest) (*operatorV1.InternalMessageCategory, error) {
	if req == nil {
		return nil, operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessageCategory.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetInternalMessageCategoryRequest_Id:
		whereCond = append(whereCond, internalmessagecategory.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

// ListCategoriesByIds 根据ID列表获取分类列表
func (r *InternalMessageCategoryRepo) ListCategoriesByIds(ctx context.Context, ids []uint32) ([]*operatorV1.InternalMessageCategory, error) {
	if len(ids) == 0 {
		return []*operatorV1.InternalMessageCategory{}, nil
	}

	entities, err := r.dbClient.Client().InternalMessageCategory.Query().
		Where(internalmessagecategory.IDIn(ids...)).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query internal message category by ids failed: %s", err.Error())
		return nil, operatorV1.ErrorInternalMessageInternalServerError("query internal message category by ids failed")
	}

	dtos := make([]*operatorV1.InternalMessageCategory, 0, len(entities))
	for _, entity := range entities {
		dto := r.mapper.ToDTO(entity)
		dtos = append(dtos, dto)
	}

	return dtos, nil
}

func (r *InternalMessageCategoryRepo) Create(ctx context.Context, req *operatorV1.CreateInternalMessageCategoryRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessageCategory.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableName(req.Data.Name).
		SetNillableCode(req.Data.Code).
		SetNillableIconURL(req.Data.IconUrl).
		SetNillableSortOrder(req.Data.SortOrder).
		SetNillableIsEnabled(req.Data.IsEnabled).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf(ctx, "insert internal message category failed: %s", err.Error())
		return operatorV1.ErrorInternalMessageInternalServerError("insert internal message category failed")
	}

	return nil
}

func (r *InternalMessageCategoryRepo) Update(ctx context.Context, req *operatorV1.UpdateInternalMessageCategoryRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorInternalMessageBadRequest("id is required")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &operatorV1.CreateInternalMessageCategoryRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.dbClient.Client().InternalMessageCategory.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.InternalMessageCategory) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableIconURL(req.Data.IconUrl).
				SetNillableSortOrder(req.Data.SortOrder).
				SetNillableIsEnabled(req.Data.IsEnabled).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(internalmessagecategory.FieldID, req.GetId()),
	)

	return err
}

func (r *InternalMessageCategoryRepo) Delete(ctx context.Context, req *operatorV1.DeleteInternalMessageCategoryRequest) error {
	if req == nil {
		return operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	// 消息分类为平铺结构（sys_internal_message_categories 无 parent_id 列），
	// 不做树形级联，仅删除分类自身。
	builder := r.dbClient.Client().InternalMessageCategory.Delete()

	_, err := r.repository.Delete(ctx, builder, queryscope.Equal(internalmessagecategory.FieldID, req.GetId()))
	if err != nil {
		r.log.Errorf(ctx, "delete internal message categories failed: %s", err.Error())
		return operatorV1.ErrorInternalMessageInternalServerError("delete internal message categories failed")
	}

	return nil
}
