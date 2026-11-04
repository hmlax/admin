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
	"go-admin/operator/internal/data/store/aiconversation"
	"go-admin/operator/internal/data/store/predicate"
	operatorV1 "go-admin/platform/pb/operator"
)

type AiConversationRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.AiConversation, store.AiConversation]

	repository *store.Repository[operatorV1.AiConversation, store.AiConversation]
}

func NewAiConversationRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *AiConversationRepo {
	repo := &AiConversationRepo{
		log:      ctx.NewLoggerHelper("ai_conversation/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.AiConversation, store.AiConversation](),
	}

	repo.init()

	return repo
}

func (r *AiConversationRepo) init() {
	r.repository = store.NewRepository[operatorV1.AiConversation, store.AiConversation](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *AiConversationRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.CountAiConversationResponse, error) {
	builder := r.dbClient.Client().AiConversation.Query()

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query ai conversation count failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai conversation count failed")
	}

	return &operatorV1.CountAiConversationResponse{Count: uint64(count)}, nil
}

// List 按分页条件查询；ownerUserID > 0 时强制只返回该用户的会话（聊天场景不可信前端过滤）。
func (r *AiConversationRepo) List(ctx context.Context, req *paginationV1.PagingRequest, ownerUserID uint32) (*operatorV1.ListAiConversationResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiConversation.Query()
	if ownerUserID > 0 {
		builder.Where(aiconversation.UserIDEQ(ownerUserID))
	}

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListAiConversationResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListAiConversationResponse{Total: ret.Total, Items: ret.Items}, nil
}

func (r *AiConversationRepo) Get(ctx context.Context, req *operatorV1.GetAiConversationRequest) (*operatorV1.AiConversation, error) {
	if req == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiConversation.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetAiConversationRequest_Id:
		whereCond = append(whereCond, aiconversation.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, nil
}

// GetEntityByID 取实体（service 层做归属校验用）。
func (r *AiConversationRepo) GetEntityByID(ctx context.Context, id uint32) (*store.AiConversation, error) {
	entity, err := r.dbClient.Client().AiConversation.Query().
		Where(aiconversation.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, operatorV1.ErrorAiNotFound("ai conversation not found")
		}
		r.log.Errorf(ctx, "query ai conversation failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai conversation failed")
	}
	return entity, nil
}

func (r *AiConversationRepo) Create(ctx context.Context, data *operatorV1.AiConversation) (*store.AiConversation, error) {
	if data == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiConversation.Create().
		SetNillableTitle(data.Title).
		SetNillableProviderID(data.ProviderId).
		SetNillableUserID(data.UserId).
		SetNillableTenantID(data.TenantId).
		SetNillableCreatedBy(data.CreatedBy).
		SetCreatedAt(time.Now())

	if data.Id != nil {
		builder.SetID(data.GetId())
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert ai conversation failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("insert ai conversation failed")
	}

	return entity, nil
}

func (r *AiConversationRepo) Update(ctx context.Context, req *operatorV1.UpdateAiConversationRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAiBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorAiBadRequest("id is required")
	}

	builder := r.dbClient.Client().AiConversation.Update()

	if err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.AiConversation) {
			builder.
				SetNillableTitle(req.Data.Title).
				SetNillableProviderID(req.Data.ProviderId).
				SetNillableLastMessageAt(tsOf(req.Data.LastMessageAt)).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(aiconversation.FieldID, req.GetId()),
	); err != nil {
		r.log.Errorf(ctx, "update ai conversation failed: %s", err.Error())
		return err
	}

	return nil
}

// TouchLastMessage 更新最近消息时间（chat 主链路高频小写）。
func (r *AiConversationRepo) TouchLastMessage(ctx context.Context, id uint32) error {
	err := r.dbClient.Client().AiConversation.UpdateOneID(id).
		SetLastMessageAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		r.log.Errorf(ctx, "touch ai conversation last message failed: %s", err.Error())
		return operatorV1.ErrorAiInternalServerError("update ai conversation failed")
	}
	return nil
}

func (r *AiConversationRepo) Delete(ctx context.Context, req *operatorV1.DeleteAiConversationRequest) error {
	if req == nil {
		return operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiConversation.Delete()

	var predicates []predicate.AiConversation
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.DeleteAiConversationRequest_Id:
		predicates = append(predicates, aiconversation.IDEQ(req.GetId()))
	}

	// messages 边是 Cascade：会话删除时消息行随删（schema 级声明）。
	if _, err := r.repository.Delete(ctx, builder, predicates...); err != nil {
		r.log.Errorf(ctx, "delete ai conversation failed: %s", err.Error())
		return err
	}

	return nil
}

// ToDTO 实体转 DTO（chat 主链路在 service 层组装响应时使用）。
func (r *AiConversationRepo) ToDTO(entity *store.AiConversation) *operatorV1.AiConversation {
	if entity == nil {
		return nil
	}
	return r.mapper.ToDTO(entity)
}
