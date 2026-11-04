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
	"go-admin/operator/internal/data/store/notificationtemplate"
	operatorV1 "go-admin/platform/pb/operator"
)

// NotificationTemplateRepo 通知模板仓储：可复用的标题/正文占位模板，平台全局，
// code 全局唯一（SendDirect 与测试渲染的引用锚）。
type NotificationTemplateRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.NotificationTemplate, store.NotificationTemplate]

	repository *store.Repository[operatorV1.NotificationTemplate, store.NotificationTemplate]
}

func NewNotificationTemplateRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *NotificationTemplateRepo {
	return newNotificationTemplateRepo(ctx.NewLoggerHelper("notification-template/repo/admin-service"), dbClient)
}

func NewNotificationTemplateRepoForTest(dbClient *store.DBClient) *NotificationTemplateRepo {
	return newNotificationTemplateRepo(bLogger.NewHelper(bLogger.NopLogger()), dbClient)
}

func newNotificationTemplateRepo(log *bLogger.Helper, dbClient *store.DBClient) *NotificationTemplateRepo {
	repo := &NotificationTemplateRepo{
		log:      log,
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.NotificationTemplate, store.NotificationTemplate](),
	}

	repo.init()

	return repo
}

func (r *NotificationTemplateRepo) init() {
	r.repository = store.NewRepository[operatorV1.NotificationTemplate, store.NotificationTemplate](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())
}

func (r *NotificationTemplateRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListNotificationTemplateResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorAdminBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().NotificationTemplate.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListNotificationTemplateResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListNotificationTemplateResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *NotificationTemplateRepo) Get(ctx context.Context, id uint32) (*operatorV1.NotificationTemplate, error) {
	if id == 0 {
		return nil, operatorV1.ErrorAdminBadRequest("id is required")
	}

	entity, err := r.dbClient.Client().NotificationTemplate.Get(ctx, id)
	if err != nil {
		r.log.Errorf(ctx, "get notification template [%d] failed: %s", id, err.Error())
		return nil, operatorV1.ErrorAdminNotFound("notification template not found")
	}

	return r.mapper.ToDTO(entity), nil
}

// GetByCode 按编码取模板；没有该行时返回 (nil, nil)——
// 与 GetByEventType 同一区分："没配这个模板"是配置事实，"查库失败"是基础设施故障，
// 两者压成同一种 error 会把 SendDirect 的排障方向引反。
func (r *NotificationTemplateRepo) GetByCode(ctx context.Context, code string) (*operatorV1.NotificationTemplate, error) {
	if code == "" {
		return nil, nil
	}

	entity, err := r.dbClient.Client().NotificationTemplate.Query().
		Where(notificationtemplate.CodeEQ(code)).
		Only(ctx)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, nil
		}
		r.log.Errorf(ctx, "query notification template by code [%s] failed: %s", code, err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("query notification template failed")
	}

	return r.mapper.ToDTO(entity), nil
}

// Create 建一条模板；code 全局唯一，撞码在写入前显式拒掉（唯一索引兜底）。
// created_at 显式写入，理由同规则表（mixin 列没有 DB 默认值）。
func (r *NotificationTemplateRepo) Create(ctx context.Context, req *operatorV1.NotificationTemplate, operatorID uint32) (uint32, error) {
	if req == nil {
		return 0, operatorV1.ErrorAdminBadRequest("invalid request")
	}
	if req.GetCode() == "" {
		return 0, operatorV1.ErrorAdminBadRequest("code is required")
	}

	exist, err := r.dbClient.Client().NotificationTemplate.Query().
		Where(notificationtemplate.CodeEQ(req.GetCode())).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "check duplicated template code failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("check notification template failed")
	}
	if exist {
		return 0, operatorV1.ErrorAdminBadRequest("template code %s already exists", req.GetCode())
	}

	builder := r.dbClient.Client().NotificationTemplate.Create().
		SetNillableName(req.Name).
		SetNillableCode(req.Code).
		SetNillableTitleTemplate(req.TitleTemplate).
		SetNillableContentTemplate(req.ContentTemplate).
		SetNillableIsEnabled(req.IsEnabled).
		SetNillableRemark(req.Remark).
		SetCreatedAt(time.Now())

	if operatorID != 0 {
		builder.SetCreatedBy(operatorID)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		r.log.Errorf(ctx, "insert notification template failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("insert notification template failed")
	}

	return created.ID, nil
}

func (r *NotificationTemplateRepo) Update(ctx context.Context, req *operatorV1.UpdateNotificationTemplateRequest, operatorID uint32) error {
	if req == nil || req.Data == nil {
		return operatorV1.ErrorAdminBadRequest("invalid request")
	}
	if req.GetId() == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	// code 若在更新里出现且与当前行不同，查重（唯一索引兜底并发窗口）
	if req.Data.Code != nil {
		existing, err := r.dbClient.Client().NotificationTemplate.Get(ctx, req.GetId())
		if err != nil {
			r.log.Errorf(ctx, "get notification template [%d] for code check failed: %s", req.GetId(), err.Error())
			return operatorV1.ErrorAdminNotFound("notification template not found")
		}
		if existing.Code != nil && *existing.Code != req.Data.GetCode() {
			exist, err := r.dbClient.Client().NotificationTemplate.Query().
				Where(notificationtemplate.CodeEQ(req.Data.GetCode())).
				Exist(ctx)
			if err != nil {
				r.log.Errorf(ctx, "check duplicated template code failed: %s", err.Error())
				return operatorV1.ErrorAdminInternalServerError("check notification template failed")
			}
			if exist {
				return operatorV1.ErrorAdminBadRequest("template code %s already exists", req.Data.GetCode())
			}
		}
	}

	builder := r.dbClient.Client().NotificationTemplate.Update()
	return r.repository.UpdateX(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.NotificationTemplate) {
			builder.
				SetNillableName(req.Data.Name).
				SetNillableCode(req.Data.Code).
				SetNillableTitleTemplate(req.Data.TitleTemplate).
				SetNillableContentTemplate(req.Data.ContentTemplate).
				SetNillableIsEnabled(req.Data.IsEnabled).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(trans.Ptr(operatorID)).
				SetUpdatedAt(time.Now())
		},
		queryscope.Equal(notificationtemplate.FieldID, req.GetId()),
	)
}

func (r *NotificationTemplateRepo) Delete(ctx context.Context, id uint32) error {
	if id == 0 {
		return operatorV1.ErrorAdminBadRequest("id is required")
	}

	if err := r.dbClient.Client().NotificationTemplate.DeleteOneID(id).Exec(ctx); err != nil {
		r.log.Errorf(ctx, "delete notification template [%d] failed: %s", id, err.Error())
		return operatorV1.ErrorAdminInternalServerError("delete notification template failed")
	}

	return nil
}
