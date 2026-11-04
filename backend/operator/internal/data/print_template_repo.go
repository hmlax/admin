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
	"go-admin/operator/internal/data/store/printtemplate"
	operatorV1 "go-admin/platform/pb/operator"
)

type PrintTemplateRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.PrintTemplate, store.PrintTemplate]

	repository *store.Repository[operatorV1.PrintTemplate, store.PrintTemplate]
}

func NewPrintTemplateRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *PrintTemplateRepo {
	repo := &PrintTemplateRepo{
		log:      ctx.NewLoggerHelper("print-template/repo/admin-service"),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.PrintTemplate, store.PrintTemplate](),
	}

	repo.init()

	return repo
}

func (r *PrintTemplateRepo) init() {
	r.repository = store.NewRepository[operatorV1.PrintTemplate, store.PrintTemplate](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *PrintTemplateRepo) Count(ctx context.Context, req *paginationV1.PagingRequest) (uint64, error) {
	builder := r.dbClient.Client().PrintTemplate.Query()

	whereSelectors, _, _ := r.repository.BuildListSelectorWithPaging(builder, req)
	if len(whereSelectors) != 0 {
		builder.Modify(whereSelectors...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query print template count failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("query print template count failed")
	}

	return uint64(count), nil
}

func (r *PrintTemplateRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListPrintTemplateResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PrintTemplate.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListPrintTemplateResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListPrintTemplateResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *PrintTemplateRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().PrintTemplate.Query().
		Where(printtemplate.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorAdminInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *PrintTemplateRepo) Get(ctx context.Context, req *operatorV1.GetPrintTemplateRequest) (*operatorV1.PrintTemplate, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().PrintTemplate.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetPrintTemplateRequest_Id:
		whereCond = append(whereCond, printtemplate.IDEQ(req.GetId()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *PrintTemplateRepo) Create(ctx context.Context, req *operatorV1.CreatePrintTemplateRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.newPrintTemplateCreate(req.Data)

	if err := builder.Exec(ctx); err != nil {
		r.log.Errorf(ctx, "insert print template failed: %s", err.Error())
		if store.IsConstraintError(err) {
			return operatorV1.ErrorAdminBadRequest("print template code already exists")
		}
		return operatorV1.ErrorAdminInternalServerError("insert print template failed")
	}

	return nil
}

func (r *PrintTemplateRepo) newPrintTemplateCreate(dto *operatorV1.PrintTemplate) *store.PrintTemplateCreate {
	builder := r.dbClient.Client().PrintTemplate.Create().
		SetNillableName(dto.Name).
		SetNillableCode(dto.Code).
		SetNillableResourceKey(dto.ResourceKey).
		SetNillablePayload(dto.Payload).
		SetNillableIsBuiltIn(dto.IsBuiltIn).
		SetNillablePaperSize(dto.PaperSize).
		SetNillableOrientation(dto.Orientation).
		SetNillableIsEnabled(dto.IsEnabled).
		SetNillableSortOrder(dto.SortOrder).
		SetNillableCreatedBy(dto.CreatedBy).
		SetCreatedAt(time.Now())

	if dto.Id != nil {
		builder.SetID(dto.GetId())
	}

	return builder
}

func (r *PrintTemplateRepo) Update(ctx context.Context, req *operatorV1.UpdatePrintTemplateRequest) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAdminBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return err
		}
		if !exist {
			createReq := &operatorV1.CreatePrintTemplateRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.dbClient.Client().PrintTemplate.Update()
	err := r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.PrintTemplate) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableResourceKey(req.Data.ResourceKey).
				SetNillablePayload(req.Data.Payload).
				SetNillableIsBuiltIn(req.Data.IsBuiltIn).
				SetNillablePaperSize(req.Data.PaperSize).
				SetNillableOrientation(req.Data.Orientation).
				SetNillableIsEnabled(req.Data.IsEnabled).
				SetNillableSortOrder(req.Data.SortOrder).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(printtemplate.FieldID, req.GetId()),
	)
	if err != nil {
		r.log.Errorf(ctx, "update print template failed: %s", err.Error())
		if store.IsConstraintError(err) {
			return operatorV1.ErrorAdminBadRequest("print template code already exists")
		}
		return operatorV1.ErrorAdminInternalServerError("update print template failed")
	}

	return nil
}

func (r *PrintTemplateRepo) Delete(ctx context.Context, req *operatorV1.DeletePrintTemplateRequest) error {
	if req == nil {
		return operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	// 删除前先取行：内置模板禁删守卫
	entity, err := r.dbClient.Client().PrintTemplate.Query().
		Where(printtemplate.IDEQ(req.GetId())).
		Only(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			// 幂等删除：目标不存在视为已删除
			return nil
		}
		r.log.Errorf(ctx, "query print template before delete failed: %s", err.Error())
		return operatorV1.ErrorAdminInternalServerError("query print template failed")
	}
	if entity.IsBuiltIn != nil && *entity.IsBuiltIn {
		return operatorV1.ErrorAdminBadRequest("built-in print template cannot be deleted")
	}

	builder := r.dbClient.Client().PrintTemplate.Delete()

	_, err = r.repository.Delete(ctx, builder, queryscope.Equal(printtemplate.FieldID, req.GetId()))
	if err != nil {
		r.log.Errorf(ctx, "delete print template failed: %s", err.Error())
		return operatorV1.ErrorAdminInternalServerError("delete print template failed")
	}

	return nil
}

// SeedDefaults 按编码缺一补一地播种内置打印模板（启动期）：
// 编码不存在的按给定默认值创建，已存在的跳过不覆盖。
// 与 sys_configs 的 SeedDefaults 同款语义：管理员自建行不应阻断内置模板补种。
// 内置行落平台上下文（tid=0）；租户侧使用走"复制内置模板派生本租户行"的链路（M2 管理页）。
func (r *PrintTemplateRepo) SeedDefaults(ctx context.Context, defaults []*operatorV1.PrintTemplate) error {
	for _, item := range defaults {
		if item == nil || item.GetCode() == "" {
			continue
		}
		exists, err := r.dbClient.Client().PrintTemplate.Query().
			Where(printtemplate.CodeEQ(item.GetCode())).
			Exist(ctx)
		if err != nil {
			r.log.Errorf(ctx, "seed print template %q: exists query failed: %s", item.GetCode(), err.Error())
			return err
		}
		if exists {
			continue
		}
		if err := r.newPrintTemplateCreate(item).Exec(ctx); err != nil {
			r.log.Errorf(ctx, "seed print template %q: insert failed: %s", item.GetCode(), err.Error())
			return err
		}
	}
	return nil
}
