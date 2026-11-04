package data

import (
	"context"
	"time"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"github.com/tx7do/go-crud/viewer"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/aiusagelog"
	"go-admin/operator/internal/data/store/planquota"
	"go-admin/operator/internal/data/store/tenant"
	operatorV1 "go-admin/platform/pb/operator"
)

type AiUsageLogRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.AiUsageLog, store.AiUsageLog]

	repository *store.Repository[operatorV1.AiUsageLog, store.AiUsageLog]
}

func NewAiUsageLogRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *AiUsageLogRepo {
	repo := &AiUsageLogRepo{
		log:      ctx.NewLoggerHelper("ai_usage_log/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.AiUsageLog, store.AiUsageLog](),
	}

	repo.init()

	return repo
}

func (r *AiUsageLogRepo) init() {
	r.repository = store.NewRepository[operatorV1.AiUsageLog, store.AiUsageLog](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *AiUsageLogRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.CountAiUsageLogResponse, error) {
	builder := r.dbClient.Client().AiUsageLog.Query()

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query ai usage log count failed: %s", err.Error())
		return nil, operatorV1.ErrorAiInternalServerError("query ai usage log count failed")
	}

	return &operatorV1.CountAiUsageLogResponse{Count: uint64(count)}, nil
}

func (r *AiUsageLogRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListAiUsageLogResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiUsageLog.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListAiUsageLogResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListAiUsageLogResponse{Total: ret.Total, Items: ret.Items}, nil
}

// Create 记一笔成功调用的用量流水（只增不改）。
func (r *AiUsageLogRepo) Create(ctx context.Context, data *operatorV1.AiUsageLog) error {
	if data == nil {
		return operatorV1.ErrorAiBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiUsageLog.Create().
		SetNillableProviderID(data.ProviderId).
		SetNillableConversationID(data.ConversationId).
		SetNillableUserID(data.UserId).
		SetNillableTenantID(data.TenantId).
		SetNillableModelName(data.ModelName).
		SetNillablePromptTokens(data.PromptTokens).
		SetNillableCompletionTokens(data.CompletionTokens).
		SetNillableTotalTokens(data.TotalTokens).
		SetNillableDurationMs(data.DurationMs).
		SetCreatedAt(time.Now())

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf(ctx, "insert ai usage log failed: %s", err.Error())
		return operatorV1.ErrorAiInternalServerError("insert ai usage log failed")
	}

	return nil
}

// SumTokensByTenantSince 统计租户自某时刻起消耗的总 token 数（配额检查用）。
func (r *AiUsageLogRepo) SumTokensByTenantSince(ctx context.Context, tenantId uint32, since time.Time) (uint64, error) {
	sum := uint64(0)
	var rows []struct {
		Total uint64 `sql:"total"`
	}
	err := r.dbClient.Client().AiUsageLog.Query().
		Where(
			aiusagelog.TenantIDEQ(tenantId),
			aiusagelog.CreatedAtGTE(since),
		).
		Aggregate(store.As(store.Sum(aiusagelog.FieldTotalTokens), "total")).
		Scan(ctx, &rows)
	if err != nil {
		r.log.Errorf(ctx, "sum ai usage tokens failed: %s", err.Error())
		return 0, operatorV1.ErrorAiInternalServerError("sum ai usage tokens failed")
	}
	if len(rows) > 0 {
		sum = rows[0].Total
	}
	return sum, nil
}

// FetchTenantTokenQuotaLimit 读租户套餐上的 AI_TOKENS 配额上限。
// 返回 (上限, 是否配置, 错误)：未配置=false 时调用方视为不限量（配额缺省语义与
// 其他维度一致——没配的维度不设限）。
func (r *AiUsageLogRepo) FetchTenantTokenQuotaLimit(ctx context.Context, tenantId uint32) (uint64, bool, error) {
	if tenantId == 0 {
		return 0, false, nil
	}

	// 系统查看器绕租户谓词读套餐链（与 tenant_usage_repo.GetUsage 同型）。
	sysCtx := viewer.WithSystemContext(ctx)

	t, err := r.dbClient.Client().Tenant.Query().
		Where(tenant.IDEQ(tenantId)).
		WithPlan(func(q *store.PlanQuery) { q.WithQuotas() }).
		Only(sysCtx)
	if err != nil {
		if store.IsNotFound(err) {
			return 0, false, nil
		}
		r.log.Errorf(ctx, "fetch tenant plan for ai quota failed: %s", err.Error())
		return 0, false, operatorV1.ErrorAiInternalServerError("fetch tenant plan failed")
	}
	if t.Plan == nil {
		return 0, false, nil
	}
	for _, q := range t.Plan.Quotas {
		if q.QuotaType != nil && *q.QuotaType == planquota.QuotaTypeAiTokens && q.QuotaValue != nil {
			return *q.QuotaValue, true, nil
		}
	}
	return 0, false, nil
}

// MonthStats 聚合本月（自 monthStart 起）的用量：tokens 总和与调用次数。
// 口径与本表列表一致：租户管理员只算本租户，平台管理员（tenantId=0）算全量。
// 这里换 SystemContext 是为了绕开 privacy 做聚合，谓词就得自己按 viewer 补——原先无条件
// TenantIDEQ(tenantId) 让平台管理员的摘要只统计 tenant_id=0 的行，与它下方全量的流水列表对不上。
func (r *AiUsageLogRepo) MonthStats(ctx context.Context, tenantId uint32, monthStart time.Time) (uint64, uint64, error) {
	sysCtx := viewer.WithSystemContext(ctx)
	var rows []struct {
		Total uint64 `sql:"total"`
		Cnt   uint64 `sql:"cnt"`
	}
	q := r.dbClient.Client().AiUsageLog.Query().Where(aiusagelog.CreatedAtGTE(monthStart))
	if tenantId > 0 {
		q = q.Where(aiusagelog.TenantIDEQ(tenantId))
	}
	err := q.Aggregate(
		store.As(store.Sum(aiusagelog.FieldTotalTokens), "total"),
		store.As(store.Count(), "cnt"),
	).
		Scan(sysCtx, &rows)
	if err != nil {
		r.log.Errorf(ctx, "month stats failed: %s", err.Error())
		return 0, 0, operatorV1.ErrorAiInternalServerError("month stats failed")
	}
	if len(rows) > 0 {
		return rows[0].Total, rows[0].Cnt, nil
	}
	return 0, 0, nil
}
