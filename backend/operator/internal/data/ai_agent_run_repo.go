package data

import (
	"context"
	"time"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"
	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/trans"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/aiagentrun"
	operatorV1 "go-admin/platform/pb/operator"
)

type AiAgentRunRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper          *mapper.CopierMapper[operatorV1.AiAgentRun, store.AiAgentRun]
	statusConverter *mapper.EnumTypeConverter[operatorV1.AiAgentRun_Status, aiagentrun.Status]

	repository *store.Repository[operatorV1.AiAgentRun, store.AiAgentRun]
}

func NewAiAgentRunRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *AiAgentRunRepo {
	repo := &AiAgentRunRepo{
		log:      ctx.NewLoggerHelper("ai-agent-run/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.AiAgentRun, store.AiAgentRun](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.AiAgentRun_Status, aiagentrun.Status](
			operatorV1.AiAgentRun_Status_name, operatorV1.AiAgentRun_Status_value,
		),
	}
	repo.init()
	return repo
}

func (r *AiAgentRunRepo) init() {
	r.repository = store.NewRepository[operatorV1.AiAgentRun, store.AiAgentRun](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
	r.mapper.AppendConverters(r.statusConverter.NewConverterPair())
}

// List 分页查询运行台账（appId eq 过滤走通用机制；status 由读路径回填）。
func (r *AiAgentRunRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListAiAgentRunsResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiAgentRun.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		r.log.Errorf(ctx, "list ai agent runs failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("list ai agent runs failed")
	}
	if ret == nil {
		return &operatorV1.ListAiAgentRunsResponse{Total: 0}, nil
	}

	// status 是值型枚举列（Default 无 Nillable）→ DTO 可选指针，copier 失配（同渠道 Type 的教训）。
	entities, err := r.dbClient.Client().AiAgentRun.Query().
		Select(aiagentrun.FieldID, aiagentrun.FieldStatus).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "backfill status failed: %s", err.Error())
	} else {
		statusByID := make(map[uint32]aiagentrun.Status, len(entities))
		for _, e := range entities {
			statusByID[e.ID] = e.Status
		}
		for _, it := range ret.Items {
			if s, ok := statusByID[it.GetId()]; ok {
				sv := s
				if p := r.statusConverter.ToDTO(&sv); p != nil {
					it.Status = p
				}
			}
		}
	}

	return &operatorV1.ListAiAgentRunsResponse{Total: ret.Total, Items: ret.Items}, nil
}

// Get 按 ID 查询运行记录。
func (r *AiAgentRunRepo) Get(ctx context.Context, id uint32) (*operatorV1.AiAgentRun, error) {
	entity, err := r.dbClient.Client().AiAgentRun.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get ai agent run [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("ai agent run not found")
	}
	dto := r.mapper.ToDTO(entity)
	sv := entity.Status
	if p := r.statusConverter.ToDTO(&sv); p != nil {
		dto.Status = p
	}
	return dto, nil
}

// CreateRunning 建一条 RUNNING 运行行（Run 流程第一步）。
func (r *AiAgentRunRepo) CreateRunning(ctx context.Context, appID uint32, inputJSON string, operatorID uint32, tenantID uint32) (uint32, error) {
	created, err := r.dbClient.Client().AiAgentRun.Create().
		SetAppID(appID).
		SetStatus(aiagentrun.StatusRunning).
		SetNillableInput(trans.Ptr(inputJSON)).
		SetTrace("[]").
		SetNillableTenantID(trans.Ptr(tenantID)).
		SetCreatedBy(operatorID).
		SetCreatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert ai agent run failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("insert ai agent run failed")
	}
	return created.ID, nil
}

// AgentRunTokens 运行用量三件套（repo 落参用，避免一串同形参数）。
type AgentRunTokens struct {
	Prompt     uint32
	Completion uint32
	Total      uint32
}

// SettleResult 回写运行结论（终态 + 产出/错误/轨迹/用量/耗时）。
func (r *AiAgentRunRepo) SettleResult(
	ctx context.Context,
	id uint32,
	status aiagentrun.Status,
	output string,
	errMsg string,
	traceJSON string,
	usage AgentRunTokens,
	durationMs uint32,
) error {
	builder := r.dbClient.Client().AiAgentRun.UpdateOneID(id).
		SetStatus(status).
		SetTrace(traceJSON).
		SetPromptTokens(usage.Prompt).
		SetCompletionTokens(usage.Completion).
		SetTotalTokens(usage.Total).
		SetDurationMs(durationMs)
	if output != "" {
		builder.SetOutput(output)
	}
	if errMsg != "" {
		builder.SetError(errMsg)
	}
	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf(ctx, "settle ai agent run [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("settle ai agent run failed")
	}
	return nil
}
