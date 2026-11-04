package data

import (
	"context"
	"time"

	"go-admin/operator/internal/data/store/queryscope"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/task"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"
)

type TaskRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper        *mapper.CopierMapper[operatorV1.Task, store.Task]
	typeConverter *mapper.EnumTypeConverter[operatorV1.Task_Type, task.Type]

	repository *store.Repository[operatorV1.Task, store.Task]
}

func NewTaskRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *TaskRepo {
	repo := &TaskRepo{
		log:           ctx.NewLoggerHelper("task/repo/admin-service"),
		dbClient:      dbClient,
		mapper:        mapper.NewCopierMapper[operatorV1.Task, store.Task](),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.Task_Type, task.Type](operatorV1.Task_Type_name, operatorV1.Task_Type_value),
	}

	repo.init()

	return repo
}

func (r *TaskRepo) init() {
	r.repository = store.NewRepository[operatorV1.Task, store.Task](r.mapper)

	r.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	r.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	r.mapper.AppendConverters(r.typeConverter.NewConverterPair())
}

func (r *TaskRepo) Count(ctx context.Context, whereCond []queryscope.Scope) (int, error) {
	builder := r.dbClient.Client().Task.Query()
	if len(whereCond) != 0 {
		builder.Modify(whereCond...)
	}

	count, err := builder.Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query count failed: %s", err.Error())
		return 0, operatorV1.ErrorTaskInternalServerError("query count failed")
	}

	return count, nil
}

func (r *TaskRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListTaskResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorTaskBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Task.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListTaskResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListTaskResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

func (r *TaskRepo) IsExist(ctx context.Context, id uint32) (bool, error) {
	exist, err := r.dbClient.Client().Task.Query().
		Where(task.IDEQ(id)).
		Exist(ctx)
	if err != nil {
		r.log.Errorf(ctx, "query exist failed: %s", err.Error())
		return false, operatorV1.ErrorTaskInternalServerError("query exist failed")
	}
	return exist, nil
}

func (r *TaskRepo) Get(ctx context.Context, req *operatorV1.GetTaskRequest) (*operatorV1.Task, error) {
	if req == nil {
		return nil, operatorV1.ErrorTaskBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Task.Query()

	var whereCond []queryscope.Scope
	switch req.QueryBy.(type) {
	default:
	case *operatorV1.GetTaskRequest_Id:
		whereCond = append(whereCond, task.IDEQ(req.GetId()))

	case *operatorV1.GetTaskRequest_TypeName:
		// type_name 仅在 (tenant_id, type_name) 维度唯一，平台上下文(tid=0)下按 type_name 查询
		// 会跨租户匹配多行导致 .Only() 报 not singular，且会跨租户操控其他租户定时任务。
		// 仅允许在具名租户上下文(tid>0)下按 type_name 查询。
		if _, hasTenant := maybeTenantFromViewer(ctx); !hasTenant {
			return nil, operatorV1.ErrorTaskBadRequest("tenant scope required to query task by type name")
		}
		whereCond = append(whereCond, task.TypeNameEQ(req.GetTypeName()))
	}

	dto, err := r.repository.Get(ctx, builder, req.GetViewMask(), whereCond...)
	if err != nil {
		return nil, err
	}

	return dto, err
}

func (r *TaskRepo) Create(ctx context.Context, req *operatorV1.CreateTaskRequest) (*operatorV1.Task, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorTaskBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().Task.Create().
		SetNillableTenantID(req.Data.TenantId).
		SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
		SetNillableTypeName(req.Data.TypeName).
		SetNillableTaskPayload(req.Data.TaskPayload).
		SetNillableCronSpec(req.Data.CronSpec).
		SetNillableEnable(req.Data.Enable).
		SetNillableRemark(req.Data.Remark).
		SetNillableCreatedBy(req.Data.CreatedBy).
		SetCreatedAt(time.Now())

	if req.Data.TaskOptions != nil {
		builder.SetTaskOptions(req.Data.TaskOptions)
	}

	if req.Data.Id != nil {
		builder.SetID(req.GetData().GetId())
	}

	t, err := builder.Save(ctx)
	if err != nil {
		// (tenant_id, type_name) 有唯一约束：同租户下重复创建同名任务属于用户可自行纠正的输入错误，
		// 返回 400 而非 500（读库预检查存在并发窗口，以约束冲突为准）。
		if store.IsConstraintError(err) {
			r.log.Warnf(ctx, "insert task duplicated: %s", err.Error())
			return nil, operatorV1.ErrorTaskBadRequest("task type [%s] already exists, please use a different type or update the existing task", req.Data.GetTypeName())
		}
		r.log.Errorf(ctx, "insert task failed: %s", err.Error())
		return nil, operatorV1.ErrorTaskInternalServerError("insert task failed")
	}

	return r.mapper.ToDTO(t), nil
}

func (r *TaskRepo) Update(ctx context.Context, req *operatorV1.UpdateTaskRequest) (*operatorV1.Task, error) {
	if req == nil || req.Data == nil {
		return nil, operatorV1.ErrorTaskBadRequest("invalid parameter")
	}
	if req.GetId() == 0 {
		return nil, operatorV1.ErrorTaskBadRequest("id is required")
	}

	// 如果不存在则创建
	if req.GetAllowMissing() {
		exist, err := r.IsExist(ctx, req.GetId())
		if err != nil {
			return nil, err
		}
		if !exist {
			createReq := &operatorV1.CreateTaskRequest{Data: req.Data}
			createReq.Data.CreatedBy = createReq.Data.UpdatedBy
			createReq.Data.UpdatedBy = nil
			return r.Create(ctx, createReq)
		}
	}

	builder := r.dbClient.Client().Task.UpdateOneID(req.GetId())
	result, err := r.repository.UpdateOne(ctx, builder, req.Data, req.GetUpdateMask(),
		func(dto *operatorV1.Task) {
			builder.
				SetNillableType(r.typeConverter.ToEntity(req.Data.Type)).
				SetNillableTypeName(req.Data.TypeName).
				SetNillableTaskPayload(req.Data.TaskPayload).
				SetNillableCronSpec(req.Data.CronSpec).
				SetNillableEnable(req.Data.Enable).
				SetNillableRemark(req.Data.Remark).
				SetNillableUpdatedBy(req.Data.UpdatedBy).
				SetUpdatedAt(time.Now())

			if req.Data.TaskOptions != nil {
				builder.SetTaskOptions(req.Data.TaskOptions)
			}
		},
		queryscope.Equal(task.FieldID, req.GetId()),
	)

	return result, err
}

func (r *TaskRepo) Delete(ctx context.Context, req *operatorV1.DeleteTaskRequest) error {
	if req == nil {
		return operatorV1.ErrorTaskBadRequest("invalid parameter")
	}

	if err := r.dbClient.Client().Task.DeleteOneID(req.GetId()).Exec(ctx); err != nil {
		if store.IsNotFound(err) {
			return operatorV1.ErrorTaskNotFound("task not found")
		}

		r.log.Errorf(ctx, "delete one data failed: %s", err.Error())

		return operatorV1.ErrorTaskInternalServerError("delete failed")
	}

	return nil
}
