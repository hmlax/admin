package data

import (
	"context"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/notificationrule"
	operatorV1 "go-admin/platform/pb/operator"
)

// NotificationRuleRepo 通知路由规则仓储：事件类型 → 渠道 + 是否异步。
//
// 这张表是 SendDirect 每次派发都要读一次的运行时真相（不再是 Go 常量），
// 所以 GetByEventType 刻意不带缓存：路由改了立刻生效，比省一次索引查询重要
// （单表单行、走 event_type 唯一索引，量级见 docs/notification_domain_design.md §4 P2-C）。
type NotificationRuleRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper             *mapper.CopierMapper[operatorV1.NotificationRule, store.NotificationRule]
	channelConverter   *mapper.EnumTypeConverter[operatorV1.Channel_Channel, notificationrule.Channel]
	eventTypeConverter *mapper.EnumTypeConverter[operatorV1.EventType_EventType, notificationrule.EventType]

	repository *store.Repository[operatorV1.NotificationRule, store.NotificationRule]
}

func NewNotificationRuleRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *NotificationRuleRepo {
	return newNotificationRuleRepo(ctx.NewLoggerHelper("notification-rule/repo/admin-service"), dbClient)
}

// NewNotificationRuleRepoForTest 供服务层 sqlite 测试构造真实仓储（同 delivery repo 的做法）。
func NewNotificationRuleRepoForTest(dbClient *store.DBClient) *NotificationRuleRepo {
	return newNotificationRuleRepo(bLogger.NewHelper(bLogger.NopLogger()), dbClient)
}

func newNotificationRuleRepo(log *bLogger.Helper, dbClient *store.DBClient) *NotificationRuleRepo {
	repo := &NotificationRuleRepo{
		log:      log,
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.NotificationRule, store.NotificationRule](),
		channelConverter: mapper.NewEnumTypeConverter[operatorV1.Channel_Channel, notificationrule.Channel](
			operatorV1.Channel_Channel_name, operatorV1.Channel_Channel_value,
		),
		eventTypeConverter: mapper.NewEnumTypeConverter[operatorV1.EventType_EventType, notificationrule.EventType](
			operatorV1.EventType_EventType_name, operatorV1.EventType_EventType_value,
		),
	}

	repo.init()

	return repo
}

func (r *NotificationRuleRepo) init() {
	r.repository = store.NewRepository[operatorV1.NotificationRule, store.NotificationRule](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.channelConverter.NewConverterPair())
	r.mapper.AppendConverters(r.eventTypeConverter.NewConverterPair())
}

func (r *NotificationRuleRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListNotificationRuleResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().NotificationRule.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListNotificationRuleResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListNotificationRuleResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *NotificationRuleRepo) Get(ctx context.Context, id uint32) (*operatorV1.NotificationRule, error) {
	if id == 0 {
		return nil, operatorV1.ErrorAdminBadRequest("id is required")
	}

	entity, err := r.dbClient.Client().NotificationRule.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get notification rule [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("notification rule not found")
	}

	return r.mapper.ToDTO(entity), nil
}

// GetByEventType 按事件类型取路由规则；没有该行时返回 (nil, nil)。
//
// 为什么不返回 NotFound 错误：调用方要区分"没有这条路由"（配置事实，回给业务侧的
// 是一句人话）与"查库失败"（基础设施故障，必须原样上抛）。把两者都压成 error 会让
// SendDirect 在 DB 抖动时报出"没有路由规则"，把人往错的方向引。
func (r *NotificationRuleRepo) GetByEventType(ctx context.Context, eventType operatorV1.EventType_EventType) (*operatorV1.NotificationRule, error) {
	entity, err := r.dbClient.Client().NotificationRule.Query().
		Where(notificationrule.EventTypeEQ(r.eventTypeString(eventType))).
		Only(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf(ctx, "query notification rule of event type [%s] failed: %s", eventType.String(), err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("query notification rule failed")
	}

	return r.mapper.ToDTO(entity), nil
}

// eventTypeString 把 proto 枚举换成实体枚举字面量（两者同名，见 GORM schema 的 NamedValues）。
// 不在受支持取值内（含 UNSPECIFIED）返回空串，让唯一索引那一侧的查询自然落空。
func (r *NotificationRuleRepo) eventTypeString(eventType operatorV1.EventType_EventType) notificationrule.EventType {
	if name, ok := operatorV1.EventType_EventType_name[int32(eventType)]; ok && eventType != operatorV1.EventType_EVENT_TYPE_UNSPECIFIED {
		return notificationrule.EventType(name)
	}
	return ""
}

// Count 规则总行数：播种守卫用（只在空表播，理由见 service 侧 init 的注释）。
func (r *NotificationRuleRepo) Count(ctx context.Context) (int, error) {
	count, err := r.dbClient.Client().NotificationRule.Query().Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count notification rules failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("count notification rules failed")
	}
	return count, nil
}

// Create 建一条规则；operatorID 为 0 表示系统播种（没有操作人）。
//
// 与渠道/台账两张表一样，created_at 显式写入：mixin 的列没有 DB 默认值，
// 留给 NULL 会让"按创建时间排序"的列表页出现一排空行。
func (r *NotificationRuleRepo) Create(ctx context.Context, req *operatorV1.NotificationRule, operatorID uint32) (uint32, error) {
	if req == nil {
		return 0, operatorV1.ErrorAdminBadRequest("invalid request")
	}
	if req.GetEventType() == operatorV1.EventType_EVENT_TYPE_UNSPECIFIED {
		return 0, operatorV1.ErrorAdminBadRequest("event_type is required")
	}
	if req.GetChannel() == operatorV1.Channel_CHANNEL_UNSPECIFIED {
		return 0, operatorV1.ErrorAdminBadRequest("channel is required")
	}

	exist, err := r.dbClient.Client().NotificationRule.Query().
		Where(notificationrule.EventTypeEQ(r.eventTypeString(req.GetEventType()))).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "check duplicated event type failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("check notification rule failed")
	}
	if exist {
		return 0, operatorV1.ErrorAdminBadRequest("event type %s already has a routing rule", req.GetEventType().String())
	}

	builder := r.dbClient.Client().NotificationRule.Create().
		SetNillableEventType(r.eventTypeConverter.ToEntity(req.EventType)).
		SetNillableChannel(r.channelConverter.ToEntity(req.Channel)).
		SetNillableIsAsync(req.IsAsync).
		SetNillableIsEnabled(req.IsEnabled).
		SetNillableRemark(req.Remark).
		SetCreatedAt(time.Now())

	if operatorID != 0 {
		builder.SetCreatedBy(operatorID)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert notification rule failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("insert notification rule failed")
	}

	return created.ID, nil
}

func (r *NotificationRuleRepo) Update(ctx context.Context, req *operatorV1.UpdateNotificationRuleRequest, operatorID uint32) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAdminBadRequest("invalid request")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	builder := r.dbClient.Client().NotificationRule.Update()
	return r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.NotificationRule) {
			builder.
				SetNillableEventType(r.eventTypeConverter.ToEntity(req.Data.EventType)).
				SetNillableChannel(r.channelConverter.ToEntity(req.Data.Channel)).
				SetNillableIsAsync(req.Data.IsAsync).
				SetNillableIsEnabled(req.Data.IsEnabled).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(trans.Ptr(operatorID)).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(notificationrule.FieldID, req.GetId()),
	)
}

func (r *NotificationRuleRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	if err := r.dbClient.Client().NotificationRule.DeleteOneID(id).Exec(ctx); err != nil {
		r.log.Errorf(ctx, "delete notification rule [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("delete notification rule failed")
	}

	return nil
}
