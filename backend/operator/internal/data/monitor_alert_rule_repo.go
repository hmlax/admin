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
	"go-admin/operator/internal/data/store/monitoralertrule"
	operatorV1 "go-admin/platform/pb/operator"
)

// MonitorAlertRuleRepo 监控告警规则仓储：指标阈值 → 触发通知，平台全局。
type MonitorAlertRuleRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper           *mapper.CopierMapper[operatorV1.MonitorAlertRule, store.MonitorAlertRule]
	metricConverter  *mapper.EnumTypeConverter[operatorV1.MonitorMetric_MonitorMetric, monitoralertrule.Metric]
	opConverter      *mapper.EnumTypeConverter[operatorV1.AlertOp_AlertOp, monitoralertrule.Op]
	channelConverter *mapper.EnumTypeConverter[operatorV1.Channel_Channel, monitoralertrule.Channel]

	repository *store.Repository[operatorV1.MonitorAlertRule, store.MonitorAlertRule]
}

func NewMonitorAlertRuleRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *MonitorAlertRuleRepo {
	return newMonitorAlertRuleRepo(ctx.NewLoggerHelper("monitor-alert/repo/admin-service"), dbClient)
}

func NewMonitorAlertRuleRepoForTest(dbClient *store.DBClient) *MonitorAlertRuleRepo {
	return newMonitorAlertRuleRepo(bLogger.NewHelper(bLogger.NopLogger()), dbClient)
}

func newMonitorAlertRuleRepo(log *bLogger.Helper, dbClient *store.DBClient) *MonitorAlertRuleRepo {
	repo := &MonitorAlertRuleRepo{
		log:              log,
		dbClient:         dbClient,
		mapper:           mapper.NewCopierMapper[operatorV1.MonitorAlertRule, store.MonitorAlertRule](),
		metricConverter:  mapper.NewEnumTypeConverter[operatorV1.MonitorMetric_MonitorMetric, monitoralertrule.Metric](operatorV1.MonitorMetric_MonitorMetric_name, operatorV1.MonitorMetric_MonitorMetric_value),
		opConverter:      mapper.NewEnumTypeConverter[operatorV1.AlertOp_AlertOp, monitoralertrule.Op](operatorV1.AlertOp_AlertOp_name, operatorV1.AlertOp_AlertOp_value),
		channelConverter: mapper.NewEnumTypeConverter[operatorV1.Channel_Channel, monitoralertrule.Channel](operatorV1.Channel_Channel_name, operatorV1.Channel_Channel_value),
	}

	repo.init()

	return repo
}

func (r *MonitorAlertRuleRepo) init() {
	r.repository = store.NewRepository[operatorV1.MonitorAlertRule, store.MonitorAlertRule](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.metricConverter.NewConverterPair())
	r.mapper.AppendConverters(r.opConverter.NewConverterPair())
	r.mapper.AppendConverters(r.channelConverter.NewConverterPair())
}

func (r *MonitorAlertRuleRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListMonitorAlertRuleResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().MonitorAlertRule.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListMonitorAlertRuleResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListMonitorAlertRuleResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// ListEnabled 取全部启用规则（评估器每轮用，量小不分页）。
func (r *MonitorAlertRuleRepo) ListEnabled(ctx context.Context) ([]*operatorV1.MonitorAlertRule, error) {
	entities, err := r.dbClient.Client().MonitorAlertRule.Query().
		Where(monitoralertrule.IsEnabledEQ(true)).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "list enabled monitor alert rules failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("list monitor alert rules failed")
	}

	dtos := make([]*operatorV1.MonitorAlertRule, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, r.mapper.ToDTO(entity))
	}
	return dtos, nil
}

func (r *MonitorAlertRuleRepo) Get(ctx context.Context, id uint32) (*operatorV1.MonitorAlertRule, error) {
	if id == 0 {
		return nil, operatorV1.ErrorAdminBadRequest("id is required")
	}

	entity, err := r.dbClient.Client().MonitorAlertRule.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get monitor alert rule [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("monitor alert rule not found")
	}

	return r.mapper.ToDTO(entity), nil
}

// Create 建一条规则；created_at 显式写入（mixin 列没有 DB 默认值，理由同规则表）。
func (r *MonitorAlertRuleRepo) Create(ctx context.Context, req *operatorV1.MonitorAlertRule, operatorID uint32) (uint32, error) {
	if req == nil {
		return 0, operatorV1.ErrorAdminBadRequest("invalid request")
	}

	builder := r.dbClient.Client().MonitorAlertRule.Create().
		SetNillableName(req.Name).
		SetNillableMetric(r.metricConverter.ToEntity(req.Metric)).
		SetNillableOp(r.opConverter.ToEntity(req.Op)).
		SetNillableThreshold(req.Threshold).
		SetNillableCooldownMinutes(req.CooldownMinutes).
		SetNillableChannel(r.channelConverter.ToEntity(req.Channel)).
		SetNillableTarget(req.Target).
		SetNillableIsEnabled(req.IsEnabled).
		SetNillableRemark(req.Remark).
		SetCreatedAt(time.Now())

	if operatorID != 0 {
		builder.SetCreatedBy(operatorID)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert monitor alert rule failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("insert monitor alert rule failed")
	}

	return created.ID, nil
}

func (r *MonitorAlertRuleRepo) Update(ctx context.Context, req *operatorV1.UpdateMonitorAlertRuleRequest, operatorID uint32) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAdminBadRequest("invalid request")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	builder := r.dbClient.Client().MonitorAlertRule.Update()
	return r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.MonitorAlertRule) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableMetric(r.metricConverter.ToEntity(req.Data.Metric)).
				SetNillableOp(r.opConverter.ToEntity(req.Data.Op)).
				SetNillableThreshold(req.Data.Threshold).
				SetNillableCooldownMinutes(req.Data.CooldownMinutes).
				SetNillableChannel(r.channelConverter.ToEntity(req.Data.Channel)).
				SetNillableTarget(req.Data.Target).
				SetNillableIsEnabled(req.Data.IsEnabled).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(trans.Ptr(operatorID)).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(monitoralertrule.FieldID, req.GetId()),
	)
}

// MarkFiring 评估器回写规则状态：firing 状态、本次指标值、告警时间。
// 独立成一个写路径（不走 Update 的掩码语义）：评估回写是系统行为，
// 与管理员的编辑互不覆盖——掩码更新永不触碰这三列，本方法只写这三列。
func (r *MonitorAlertRuleRepo) MarkFiring(ctx context.Context, id uint32, firing bool, value *float64, alertedAt *time.Time) error {
	builder := r.dbClient.Client().MonitorAlertRule.UpdateOneID(id).
		SetLastFiring(firing).
		SetNillableLastValue(value)
	builder.SetNillableLastAlertedAt(alertedAt)
	if _, err := builder.Save(ctx); err != nil {
		r.log.Errorf(ctx, "mark monitor alert rule [%d] firing=%v failed: %s", id, firing, err.Error())
		return operatorV1.ErrorAdminInternalServerError("update monitor alert rule failed")
	}
	return nil
}

func (r *MonitorAlertRuleRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	if err := r.dbClient.Client().MonitorAlertRule.DeleteOneID(id).Exec(ctx); err != nil {
		r.log.Errorf(ctx, "delete monitor alert rule [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("delete monitor alert rule failed")
	}

	return nil
}
