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
	"go-admin/operator/internal/data/store/aiagentapp"
	operatorV1 "go-admin/platform/pb/operator"
)

type AiAgentAppRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.AiAgentApp, store.AiAgentApp]

	repository *store.Repository[operatorV1.AiAgentApp, store.AiAgentApp]
}

func NewAiAgentAppRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *AiAgentAppRepo {
	repo := &AiAgentAppRepo{
		log:      ctx.NewLoggerHelper("ai-agent-app/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.AiAgentApp, store.AiAgentApp](),
	}
	repo.init()
	return repo
}

func (r *AiAgentAppRepo) init() {
	r.repository = store.NewRepository[operatorV1.AiAgentApp, store.AiAgentApp](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

// List 分页查询应用（通用过滤机制走 modelCrud；enabled 由读路径显式回填）。
func (r *AiAgentAppRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListAiAgentAppsResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiAgentApp.Query()
	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		r.log.Errorf(ctx, "list ai agent apps failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("list ai agent apps failed")
	}
	if ret == nil {
		return &operatorV1.ListAiAgentAppsResponse{Total: 0}, nil
	}

	// 实体 enabled 是值型 bool、DTO 是可选指针：copier 对这一形态失配（同渠道 Type 的教训），
	// 读路径批量回填。
	entities, err := r.dbClient.Client().AiAgentApp.Query().
		Select(aiagentapp.FieldID, aiagentapp.FieldEnabled).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "backfill enabled failed: %s", err.Error())
	} else {
		enabledByID := make(map[uint32]bool, len(entities))
		for _, e := range entities {
			enabledByID[e.ID] = e.GetEnabled()
		}
		for _, it := range ret.Items {
			it.Enabled = trans.Ptr(enabledByID[it.GetId()])
		}
	}

	return &operatorV1.ListAiAgentAppsResponse{Total: ret.Total, Items: ret.Items}, nil
}

// Get 按 ID 查询应用。
func (r *AiAgentAppRepo) Get(ctx context.Context, id uint32) (*operatorV1.AiAgentApp, error) {
	entity, err := r.dbClient.Client().AiAgentApp.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get ai agent app [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("ai agent app not found")
	}
	return r.toDTO(entity), nil
}

// GetEntityByID 取应用实体（Run 流程要 graph 原文与 enabled，DTO 不够用）。
func (r *AiAgentAppRepo) GetEntityByID(ctx context.Context, id uint32) (*store.AiAgentApp, error) {
	entity, err := r.dbClient.Client().AiAgentApp.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get ai agent app entity [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("ai agent app not found")
	}
	return entity, nil
}

func (r *AiAgentAppRepo) toDTO(entity *store.AiAgentApp) *operatorV1.AiAgentApp {
	dto := r.mapper.ToDTO(entity)
	dto.Enabled = entity.Enabled
	return dto
}

// Create 创建应用；租户内 name 唯一约束冲突转成可读的 400。
func (r *AiAgentAppRepo) Create(ctx context.Context, data *operatorV1.AiAgentApp, operatorID uint32) (uint32, error) {
	if data == nil {
		return 0, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}
	if data.GetName() == "" {
		return 0, operatorV1.ErrorAdminBadRequest("name is required")
	}

	builder := r.dbClient.Client().AiAgentApp.Create().
		SetName(data.GetName()).
		SetNillableDescription(data.Description).
		SetNillableGraph(data.Graph).
		SetEnabled(data.GetEnabled()).
		SetNillableTenantID(data.TenantId).
		SetCreatedBy(operatorID).
		SetCreatedAt(time.Now())

	created, err := builder.Save(ctx)
	if err != nil {
		if store.IsConstraintError(err) {
			return 0, operatorV1.ErrorAdminBadRequest("agent app name already exists")
		}
		r.log.Errorf(ctx, "insert ai agent app failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("insert ai agent app failed")
	}
	return created.ID, nil
}

// Update 更新应用；updateMask 点名要改的字段，留空 = 全量按入参。
func (r *AiAgentAppRepo) Update(ctx context.Context, id uint32, data *operatorV1.AiAgentApp, updateMask []string, operatorID uint32) error {
	if data == nil || id == 0 {
		return operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().AiAgentApp.Update().
		Where(aiagentapp.IDEQ(id))

	apply := func() *store.AiAgentAppUpdate {
		return builder.
			SetNillableDescription(data.Description).
			SetNillableGraph(data.Graph).
			SetEnabled(data.GetEnabled()).
			SetNillableName(data.Name).
			SetNillableUpdatedBy(trans.Ptr(operatorID)).
			SetUpdatedAt(time.Now())
	}

	mask := map[string]bool{}
	for _, m := range updateMask {
		mask[m] = true
	}
	if len(mask) > 0 {
		b := builder
		if mask["name"] {
			b = b.SetNillableName(data.Name)
		}
		if mask["description"] {
			b = b.SetNillableDescription(data.Description)
		}
		if mask["graph"] {
			b = b.SetNillableGraph(data.Graph)
		}
		if mask["enabled"] {
			b = b.SetEnabled(data.GetEnabled())
		}
		b = b.SetNillableUpdatedBy(trans.Ptr(operatorID)).SetUpdatedAt(time.Now())
		if _, err := b.Save(ctx); err != nil {
			r.log.Errorf(ctx, "update ai agent app [%d] failed: %s", id, err.Error())
			return operatorV1.ErrorAdminInternalServerError("update ai agent app failed")
		}
		return nil
	}

	if _, err := apply().Save(ctx); err != nil {
		r.log.Errorf(ctx, "update ai agent app [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("update ai agent app failed")
	}
	return nil
}

func (r *AiAgentAppRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}
	if err := r.dbClient.Client().AiAgentApp.DeleteOneID(id).Exec(ctx); err != nil {
		r.log.Errorf(ctx, "delete ai agent app [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("delete ai agent app failed")
	}
	return nil
}
