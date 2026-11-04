package data

import (
	"context"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/internalmessage"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
)

type InternalMessageRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper          *mapper.CopierMapper[operatorV1.InternalMessage, store.InternalMessage]
	statusConverter *mapper.EnumTypeConverter[operatorV1.InternalMessage_Status, internalmessage.Status]
	typeConverter   *mapper.EnumTypeConverter[operatorV1.InternalMessage_Type, internalmessage.Type]

	repository *store.Repository[operatorV1.InternalMessage, store.InternalMessage]
}

func NewInternalMessageRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *InternalMessageRepo {
	repo := &InternalMessageRepo{
		log:             ctx.NewLoggerHelper("internal-message/repo/admin-service"),
		dbClient:        dbClient,
		mapper:          mapper.NewCopierMapper[operatorV1.InternalMessage, store.InternalMessage](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Status, internalmessage.Status](operatorV1.InternalMessage_Status_name, operatorV1.InternalMessage_Status_value),
		typeConverter:   mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Type, internalmessage.Type](operatorV1.InternalMessage_Type_name, operatorV1.InternalMessage_Type_value),
	}

	repo.init()

	return repo
}

func (r *InternalMessageRepo) init() {
	r.repository = store.NewRepository[operatorV1.InternalMessage, store.InternalMessage](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *InternalMessageRepo) Count(ctx context.Context, whereCond []queryscope.Scope) (int, error) {
	builder := r.dbClient.Client().InternalMessage.Query()
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

func (r *InternalMessageRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListInternalMessageResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessage.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListInternalMessageResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListInternalMessageResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *InternalMessageRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().InternalMessage.Query().
		Where(internalmessage.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorInternalMessageInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *InternalMessageRepo) Get(ctx context.Context, req *operatorV1.GetInternalMessageRequest) (*operatorV1.InternalMessage, error) {
	if req == nil {
		return nil, operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessage.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetInternalMessageRequest_Id:
		whereCond = append(whereCond, internalmessage.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

// ListByIds 按主键批量取消息本体，用于收件箱列表回填 title/content，
// 替代逐条 Get 的 N+1 查询。缺失的 id（如消息已被撤销删除）不会出现在返回的 map 里。
func (r *InternalMessageRepo) ListByIds(ctx context.Context, ids []uint32) (map[uint32]*operatorV1.InternalMessage, error) {
	if len(ids) == 0 {
		return map[uint32]*operatorV1.InternalMessage{}, nil
	}

	entities, err := r.dbClient.Client().InternalMessage.Query().
		Where(internalmessage.IDIn(ids...)).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "list internal messages by ids failed: %s", err.Error())
		return nil, operatorV1.ErrorInternalMessageInternalServerError("query internal messages failed")
	}

	ret := make(map[uint32]*operatorV1.InternalMessage, len(entities))
	for _, entity := range entities {
		ret[entity.ID] = r.mapper.ToDTO(entity)
	}
	return ret, nil
}

func (r *InternalMessageRepo) Create(ctx context.Context, req *operatorV1.CreateInternalMessageRequest) (*operatorV1.InternalMessage, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().InternalMessage.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableTitle(req.Data.Title).
		SetNillableContent(req.Data.Content).
		SetSenderID(req.Data.GetSenderId()).
		SetNillableCategoryID(req.Data.CategoryId).
		SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
		SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	var err error
	var entity *store.InternalMessage
	if entity, err = builder.Save(ctx); err != nil {
		r.log.Errorf(ctx, "insert internal message failed: %s", err.Error())
		return nil, operatorV1.ErrorInternalMessageInternalServerError("insert internal message failed")
	}

	return r.mapper.ToDTO(entity), nil
}

func (r *InternalMessageRepo) Update(ctx context.Context, req *operatorV1.UpdateInternalMessageRequest) error {
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
			createReq := &operatorV1.CreateInternalMessageRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			_, err = r.Create(ctx, createReq)
			return err
		}
	}

	builder := r.dbClient.Client().InternalMessage.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.InternalMessage) {
			builder.
				SetNillableTitle(req.Data.Title).
				SetNillableContent(req.Data.Content).
				SetNillableSenderID(req.Data.SenderId).
				SetNillableCategoryID(req.Data.CategoryId).
				SetNillableStatus(r.statusConverter.ToEntity(req.Data.Status)).
				SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(internalmessage.FieldID, req.GetId()),
	)

	return err
}

func (r *InternalMessageRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorInternalMessageBadRequest("invalid parameter")
	}

	if err := r.dbClient.Client().InternalMessage.DeleteOneID(id).Exec(ctx); err != nil {
		if store.IsNotFound(err) {
			return operatorV1.ErrorInternalMessageNotFound("internal message not found")
		}

		r.log.Errorf(ctx, "delete one data failed: %s", err.Error())

		return operatorV1.ErrorInternalMessageInternalServerError("delete failed")
	}

	return nil
}
