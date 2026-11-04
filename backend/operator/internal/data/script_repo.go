package data

import (
	"context"
	"fmt"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/script"
	operatorV1 "go-admin/platform/pb/operator"
)

type ScriptRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper            *mapper.CopierMapper[operatorV1.Script, store.Script]
	languageConverter *mapper.EnumTypeConverter[operatorV1.ScriptLanguage_ScriptLanguage, script.Language]

	repository *store.Repository[operatorV1.Script, store.Script]
}

func NewScriptRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *ScriptRepo {
	repo := &ScriptRepo{
		log:      ctx.NewLoggerHelper("script/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Script, store.Script](),
		languageConverter: mapper.NewEnumTypeConverter[operatorV1.ScriptLanguage_ScriptLanguage, script.Language](
			operatorV1.ScriptLanguage_ScriptLanguage_name, operatorV1.ScriptLanguage_ScriptLanguage_value,
		),
	}

	repo.init()

	return repo
}

func (r *ScriptRepo) init() {
	r.repository = store.NewRepository[operatorV1.Script, store.Script](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.languageConverter.NewConverterPair())
}

func (r *ScriptRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.CountScriptsResponse, error) {
	builder := r.dbClient.Client().Script.Query()

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query script count failed: %s", err.Error())
		return nil, operatorV1.ErrorScriptInternalServerError("query script count failed")
	}

	return &operatorV1.CountScriptsResponse{
		Count: uint64(count),
	}, nil
}

func (r *ScriptRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListScriptsResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorScriptBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Script.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListScriptsResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListScriptsResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *ScriptRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().Script.Query().
		Where(script.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query script exist failed: %s", err.Error())
		return false, operatorV1.ErrorScriptInternalServerError("query script exist failed")
	}
	return exist, nil
}

func (r *ScriptRepo) IsNameExist(ctx context.Context, name string, excludeID uint32) (bool, error) {
	query := r.dbClient.Client().Script.Query().Where(script.NameEQ(name))
	if excludeID > 0 {
		query = query.Where(script.IDNEQ(excludeID))
	}
	exist, err := query.Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query script name exist failed: %s", err.Error())
		return false, operatorV1.ErrorScriptInternalServerError("query script name exist failed")
	}
	return exist, nil
}

func (r *ScriptRepo) Get(ctx context.Context, req *operatorV1.GetScriptRequest) (*operatorV1.Script, error) {
	if req == nil {
		return nil, operatorV1.ErrorScriptBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Script.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	case *operatorV1.GetScriptRequest_Name:
		whereCond = append(whereCond, script.NameEQ(req.GetName()))
	default:
		whereCond = append(whereCond, script.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, nil
}

// GetByNameByName 按唯一名取脚本（内部用，返回 DTO）。
func (r *ScriptRepo) GetByName(ctx context.Context, name string) (*operatorV1.Script, error) {
	entity, err := r.dbClient.Client().Script.Query().
		Where(script.NameEQ(name)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDTO(entity), nil
}

func (r *ScriptRepo) Create(ctx context.Context, req *operatorV1.CreateScriptRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorScriptBadRequest("invalid parameter")
	}

	builder := r.newScriptCreate(req.Data)

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf(ctx, "insert script failed: %s", err.Error())
		return operatorV1.ErrorScriptInternalServerError("insert script failed")
	}

	return nil
}

func (r *ScriptRepo) newScriptCreate(dto *operatorV1.Script) *store.ScriptCreate {
	builder := r.dbClient.Client().Script.Create().
		SetNillableName(dto.Name).
		SetNillableHookPoint(dto.HookPoint).
		SetNillablePriority(dto.Priority).
		SetNillableDescription(dto.Description).
		SetNillableCritical(dto.Critical).
		SetNillableVersion(dto.Version).
		SetNillableIsEnabled(dto.IsEnabled).
		SetNillableCreatedBy(dto.CreatedBy).
		SetCreatedAt(time.Now())

	// language：nil 留空走 GORM 默认 LUA
	if dto.Language != nil {
		builder.SetNillableLanguage(r.languageConverter.ToEntity(dto.Language))
	}

	if dto.Source != nil {
		builder.SetSource(dto.GetSource())
	}

	if dto.Id != nil {
		builder.SetID(dto.GetId())
	}

	return builder
}

func (r *ScriptRepo) Update(ctx context.Context, req *operatorV1.UpdateScriptRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorScriptBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorScriptBadRequest("id is required")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &operatorV1.CreateScriptRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.dbClient.Client().Script.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.Script) {
			builder.
				SetNillableName(dto.Name).
				SetNillableHookPoint(dto.HookPoint).
				SetNillablePriority(dto.Priority).
				SetNillableDescription(dto.Description).
				SetNillableCritical(dto.Critical).
				SetNillableIsEnabled(dto.IsEnabled).
				SetNillableUpdatedBy(dto.UpdatedBy).
				SetUpdatedAt(time.Now())

			// version 每次更新自增（热更新指纹）
			builder.AddVersion(1)

			if dto.Language != nil {
				builder.SetNillableLanguage(r.languageConverter.ToEntity(dto.Language))
			}

			if dto.Source != nil {
				builder.SetSource(dto.GetSource())
			}
		},
		queryscope.Equal(script.FieldID, req.GetId()),
	)
	if err != nil {
		r.log.Errorf(ctx, "update script failed: %s", err.Error())
		return operatorV1.ErrorScriptInternalServerError("update script failed")
	}

	return nil
}

func (r *ScriptRepo) Delete(ctx context.Context, req *operatorV1.DeleteScriptRequest) error {
	if req == nil {
		return operatorV1.ErrorScriptBadRequest("invalid parameter")
	}
	if len(req.GetIds()) == 0 {
		return operatorV1.ErrorScriptBadRequest("ids is required")
	}

	if _, err := r.dbClient.Client().Script.Delete().
		Where(script.IDIn(req.GetIds()...)).
		Exec(ctx); err != nil {
		r.log.Errorf(ctx, "delete script failed: %s", err.Error())
		return operatorV1.ErrorScriptInternalServerError("delete script failed")
	}

	return nil
}

// ListEnabledScripts 返回全部已启用脚本（脚本运行时启动加载/全量重同步用）。
func (r *ScriptRepo) ListEnabledScripts(ctx context.Context) ([]*operatorV1.Script, error) {
	entities, err := r.dbClient.Client().Script.Query().
		Where(script.IsEnabledEQ(true)).
		All(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query enabled scripts failed: %s", err.Error())
		return nil, operatorV1.ErrorScriptInternalServerError("query enabled scripts failed")
	}

	dtos := make([]*operatorV1.Script, 0, len(entities))
	for _, entity := range entities {
		dtos = append(dtos, r.mapper.ToDTO(entity))
	}
	return dtos, nil
}

// GetSourceByName 按唯一名取脚本源码（DBSource 加载回调）。
func (r *ScriptRepo) GetSourceByName(ctx context.Context, name string) (string, error) {
	entity, err := r.dbClient.Client().Script.Query().
		Where(script.NameEQ(name)).
		Only(ctx)
	if err != nil {
		return "", err
	}
	if entity.Source == nil {
		return "", nil
	}
	return *entity.Source, nil
}

// GetVersionByName 按唯一名返回脚本版本指纹（DBSource 热更新变更检测回调）。
func (r *ScriptRepo) GetVersionByName(ctx context.Context, name string) (string, error) {
	entity, err := r.dbClient.Client().Script.Query().
		Where(script.NameEQ(name)).
		Only(ctx)
	if err != nil {
		return "", err
	}
	// Version 为 nillable 列（*uint32）：此前 fmt.Sprintf("%d", entity.Version)
	// 格式化的是指针地址而非版本号，DBSource 热更新的变更检测指纹因此失效。
	if entity.Version == nil {
		return "", nil
	}
	return fmt.Sprintf("%d", *entity.Version), nil
}
