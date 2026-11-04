package data

import (
	"context"
	"time"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/aimessage"
	"go-admin/operator/internal/data/store/predicate"
	operatorV1 "go-admin/platform/pb/operator"
)

type AiMessageRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper        *mapper.CopierMapper[operatorV1.AiMessage, store.AiMessage]
	roleConverter *mapper.EnumTypeConverter[operatorV1.AiRole_AiRole, aimessage.Role]

	repository *store.Repository[operatorV1.AiMessage, store.AiMessage]
}

func NewAiMessageRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *AiMessageRepo {
	repo := &AiMessageRepo{
		log:           ctx.NewLoggerHelper("ai_message/repo/admin-service"),
		dbClient:      dbClient,
		mapper:        mapper.NewCopierMapper[operatorV1.AiMessage, store.AiMessage](),
		roleConverter: mapper.NewEnumTypeConverter[operatorV1.AiRole_AiRole, aimessage.Role](operatorV1.AiRole_AiRole_name, operatorV1.AiRole_AiRole_value),
	}

	repo.init()

	return repo
}

func (r *AiMessageRepo) init() {
	r.repository = store.NewRepository[operatorV1.AiMessage, store.AiMessage](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.roleConverter.NewConverterPair())
}

func (r *AiMessageRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.CountAiMessageResponse, error) {
	builder := r.dbClient.Client().AiMessage.Query()

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query ai message count failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai message count failed")
	}

	return &operatorV1.CountAiMessageResponse{Count: uint64(count)}, nil
}

// List 按分页条件查询；ownerUserID > 0 时强制只返回该用户的消息（service 层已校验会话归属，
// 这里再兜一层，防 filter 注入越权读他人在其他会话里的消息）。
func (r *AiMessageRepo) List(ctx context.Context, req *paginationV1.PagingRequest, ownerUserID uint32) (*operatorV1.ListAiMessageResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiMessage.Query()
	if ownerUserID > 0 {
		builder.Where(aimessage.UserIDEQ(ownerUserID))
	}

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListAiMessageResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListAiMessageResponse{Total: ret.Total, Items: ret.Items}, nil
}

// ListRecentByConversation 取会话内最近 N 条消息（按创建时间升序返回，供 LLM 上下文裁剪）。
func (r *AiMessageRepo) ListRecentByConversation(ctx context.Context, conversationId uint32, limit int) ([]*store.AiMessage, error) {
	if limit <= 0 {
		limit = 20
	}

	entities, err := r.dbClient.Client().AiMessage.Query().
		Where(aimessage.ConversationIDEQ(conversationId)).
		Order(store.Desc(aimessage.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query recent ai messages failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai messages failed")
	}

	// 反转成时间升序（旧→新），LLM 上下文要求按序。
	for i, j := 0, len(entities)-1; i < j; i, j = i+1, j-1 {
		entities[i], entities[j] = entities[j], entities[i]
	}

	return entities, nil
}

func (r *AiMessageRepo) Create(ctx context.Context, data *operatorV1.AiMessage) (*store.AiMessage, error) {
	if data == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiMessage.Create().
		SetNillableConversationID(data.ConversationId).
		SetNillableRole(r.roleConverter.ToEntity(data.Role)).
		SetNillableContent(data.Content).
		SetNillableModelName(data.ModelName).
		SetNillablePromptTokens(data.PromptTokens).
		SetNillableCompletionTokens(data.CompletionTokens).
		SetNillableDurationMs(data.DurationMs).
		SetNillableErrorMessage(data.ErrorMessage).
		SetNillableUserID(data.UserId).
		SetNillableTenantID(data.TenantId).
		SetNillableCreatedBy(data.CreatedBy).
		SetCreatedAt(time.Now())

	if data.Id != nil {
		builder.SetID(data.GetId())
	}

	entity, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert ai message failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("insert ai message failed")
	}

	return entity, nil
}

func (r *AiMessageRepo) Delete(ctx context.Context, req *operatorV1.DeleteAiMessageRequest) error {
	if req == nil {
		return operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiMessage.Delete()

	var predicates []predicate.AiMessage
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.DeleteAiMessageRequest_Id:
		predicates = append(predicates, aimessage.IDEQ(req.GetId()))
	}

	if _, err := r.repository.Delete(ctx, builder, predicates...); err != nil {
		r.log.Errorf(ctx, "delete ai message failed: %s", err.Error())
		return err
	}

	return nil
}

// ToDTO 实体转 DTO（chat 主链路在 service 层组装响应时使用）。
func (r *AiMessageRepo) ToDTO(entity *store.AiMessage) *operatorV1.AiMessage {
	if entity == nil {
		return nil
	}
	return r.mapper.ToDTO(entity)
}

// GetEntityByID 取消息实体（service 层做归属校验用）。
func (r *AiMessageRepo) GetEntityByID(ctx context.Context, id uint32) (*store.AiMessage, error) {
	entity, err := r.dbClient.Client().AiMessage.Query().
		Where(aimessage.IDEQ(id)).
		Only(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, operatorV1.ErrorAiNotFound("ai message not found")
		}
		r.log.Errorf(ctx, "query ai message failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai message failed")
	}
	return entity, nil
}
