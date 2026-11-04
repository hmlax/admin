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
	"go-admin/operator/internal/data/store/scriptlog"
	operatorV1 "go-admin/platform/pb/operator"
)

// ScriptLogRepo 脚本执行日志仓储：仅写入与简单查询，不走 proto/管理面。
// 高频写入路径，方法保持最小面。
type ScriptLogRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper

	mapper *mapper.CopierMapper[operatorV1.ScriptLog, store.ScriptLog]

	repository *store.Repository[operatorV1.ScriptLog, store.ScriptLog]
}

func NewScriptLogRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *ScriptLogRepo {
	repo := &ScriptLogRepo{
		dbClient: dbClient,
		log:      ctx.NewLoggerHelper("script-log/repo/admin-service"),
		mapper:   mapper.NewCopierMapper[operatorV1.ScriptLog, store.ScriptLog](),
	}
	repo.repository = store.NewRepository[operatorV1.ScriptLog, store.ScriptLog](repo.mapper)

	repo.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	repo.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	return repo
}

// ScriptLogRecord 一次脚本执行的记录。
type ScriptLogRecord struct {
	ScriptID   uint32
	ScriptName string
	Language   string
	Trigger    string // hook / task / test_run / manual
	HookPoint  string
	Version    uint32
	Success    bool
	DurationMS int64
	Error      string
}

// Record 落一条执行日志。失败只记运维日志，不影响业务调用方。
func (r *ScriptLogRepo) Record(ctx context.Context, rec ScriptLogRecord) {
	err := r.dbClient.Client().ScriptLog.Create().
		SetNillableScriptID(nonZero(rec.ScriptID)).
		SetNillableScriptName(nonEmptyStr(rec.ScriptName)).
		SetNillableLanguage(nonEmptyStr(rec.Language)).
		SetNillableTriggerType(nonEmptyStr(rec.Trigger)).
		SetNillableHookPoint(nonEmptyStr(rec.HookPoint)).
		SetNillableVersion(nonZero(rec.Version)).
		SetSuccess(rec.Success).
		SetDurationMs(rec.DurationMS).
		SetNillableError(nonEmptyStr(rec.Error)).
		SetCreatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		r.log.Errorf(ctx, "record script log failed: %s", err.Error())
	}
}

// PurgeBefore 删除指定时间之前的日志（滚动清理），返回删除行数。
func (r *ScriptLogRepo) PurgeBefore(ctx context.Context, before time.Time) (int, error) {
	deleted, err := r.dbClient.Client().ScriptLog.Delete().
		Where(scriptlog.CreatedAtLT(before)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func nonZero(v uint32) *uint32 {
	if v == 0 {
		return nil
	}
	return &v
}

func nonEmptyStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// List 分页查询执行日志（repository 通用路径：query JSON 过滤 + created_at 倒序）。
func (r *ScriptLogRepo) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListScriptLogsResponse, error) {
	if req == nil {
		return nil, operatorV1.ErrorScriptBadRequest("invalid parameter")
	}

	builder := r.dbClient.Client().ScriptLog.Query()

	ret, err := r.repository.ListWithPaging(ctx, builder, builder.Clone(), req)
	if err != nil {
		return nil, err
	}
	if ret == nil {
		return &operatorV1.ListScriptLogsResponse{Total: 0, Items: nil}, nil
	}

	return &operatorV1.ListScriptLogsResponse{
		Total: ret.Total,
		Items: ret.Items,
	}, nil
}

// Count 统计日志数量。
func (r *ScriptLogRepo) CountLog(ctx context.Context) (*operatorV1.CountScriptLogsResponse, error) {
	count, err := r.dbClient.Client().ScriptLog.Query().Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count script logs failed: %s", err.Error())
		return nil, operatorV1.ErrorScriptInternalServerError("count script logs failed")
	}
	return &operatorV1.CountScriptLogsResponse{Count: uint64(count)}, nil
}

// Purge 清理指定时间之前的日志。
func (r *ScriptLogRepo) Purge(ctx context.Context, before time.Time) (uint64, error) {
	deleted, err := r.PurgeBefore(ctx, before)
	if err != nil {
		r.log.Errorf(ctx, "purge script logs failed: %s", err.Error())
		return 0, operatorV1.ErrorScriptInternalServerError("purge script logs failed")
	}
	return uint64(deleted), nil
}
