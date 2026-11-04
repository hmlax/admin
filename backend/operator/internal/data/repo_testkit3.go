// 跨包（service 层）测试装配：导出免 bootstrap.Context 的 repo 构造器，
// 字段初始化必须与生产 NewXxxRepo 逐字段一致，改生产构造器须同步此处。
//
// 说明：
//   - 本文件覆盖 repo_testkit.go / repo_testkit2.go 之外批次的 repo
//     （script / script_log / task）。各构造器与对应 *_repo.go 的生产构造器
//     逐字段对齐（含 mapper/converter 初始化与 init() 调用，ScriptLogRepo 的
//     生产构造器无独立 init()，其 repository 组装与时间转换器追加语句在
//     生产构造器内联，此处按同样顺序原样复刻），唯一差异是 log 一律
//     bLogger.NewHelper(bLogger.NopLogger())；dbClient 由调用方传入
//     （测试场景为 storetest.NewClientForTest 的 SQLite 内存库）。
//   - 生产构造器中经由 *bootstrap.Context 注入的仅是日志助手，无其他隐藏依赖；
//     依赖 redis/minio 等外部件的构造器不在此导出。
package data

import (
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"github.com/tx7do/go-utils/copierutil"
	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	modelScript "go-admin/operator/internal/data/store/script"
	modelTask "go-admin/operator/internal/data/store/task"
	operatorV1 "go-admin/platform/pb/operator"
)

// NewScriptRepoForTest 与生产 NewScriptRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewScriptRepoForTest(dbClient *store.DBClient) *ScriptRepo {
	repo := &ScriptRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.Script, store.Script](),
		languageConverter: mapper.NewEnumTypeConverter[operatorV1.ScriptLanguage_ScriptLanguage, modelScript.Language](
			operatorV1.ScriptLanguage_ScriptLanguage_name, operatorV1.ScriptLanguage_ScriptLanguage_value,
		),
	}

	repo.init()

	return repo
}

// NewScriptLogRepoForTest 与生产 NewScriptLogRepo 逐字段一致（log 换 NopLogger；
// 生产构造器内联的 repository 组装与时间转换器追加语句按原顺序复刻，无独立 init()）。
func NewScriptLogRepoForTest(dbClient *store.DBClient) *ScriptLogRepo {
	repo := &ScriptLogRepo{
		dbClient: dbClient,
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		mapper:   mapper.NewCopierMapper[operatorV1.ScriptLog, store.ScriptLog](),
	}
	repo.repository = store.NewRepository[operatorV1.ScriptLog, store.ScriptLog](repo.mapper)

	repo.mapper.AppendConverters(copierutil.NewTimeStringConverterPair())
	repo.mapper.AppendConverters(copierutil.NewTimeTimestamppbConverterPair())

	return repo
}

// NewTaskRepoForTest 与生产 NewTaskRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewTaskRepoForTest(dbClient *store.DBClient) *TaskRepo {
	repo := &TaskRepo{
		log:           bLogger.NewHelper(bLogger.NopLogger()),
		dbClient:      dbClient,
		mapper:        mapper.NewCopierMapper[operatorV1.Task, store.Task](),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.Task_Type, modelTask.Type](operatorV1.Task_Type_name, operatorV1.Task_Type_value),
	}

	repo.init()

	return repo
}
