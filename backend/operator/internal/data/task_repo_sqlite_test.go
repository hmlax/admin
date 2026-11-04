package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/trans"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/task"
	storetest "go-admin/operator/internal/data/storetest"
	operatorV1 "go-admin/platform/pb/operator"
)

// newTaskRepoSqlite 在给定 storetest client 上白盒构造 TaskRepo，
// 逐字段复刻 NewTaskRepo 的 mapper/converter 初始化，再调用 init()。
func newTaskRepoSqlite(t *testing.T, dbClient *store.DBClient) *TaskRepo {
	t.Helper()
	repo := &TaskRepo{
		dbClient:      dbClient,
		log:           bLogger.NewHelper(bLogger.NopLogger()),
		mapper:        mapper.NewCopierMapper[operatorV1.Task, store.Task](),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.Task_Type, task.Type](operatorV1.Task_Type_name, operatorV1.Task_Type_value),
	}
	repo.init()
	return repo
}

// TestTaskRepoSqlite_Create 通过 repo.Create 写入后直查 SQLite 断言落库
// （含 type 枚举经 converter 的落库值）。
func TestTaskRepoSqlite_Create(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	_, err := repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{
			Type:     operatorV1.Task_DELAY.Enum(),
			TypeName: trans.Ptr("sqlite_task_create_name"),
			CronSpec: trans.Ptr("0 */5 * * * *"),
			Enable:   trans.Ptr(true),
			Remark:   trans.Ptr("创建备注"),
		},
	})
	require.NoError(t, err, "repo.Create 应写入 SQLite 成成功")

	rows, err := repo.dbClient.Client().Task.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "SQLite 中应有 1 条 task 记录")
	require.Equal(t, "sqlite_task_create_name", *rows[0].TypeName, "type_name 应按请求落库")
	require.Equal(t, "0 */5 * * * *", *rows[0].CronSpec, "cron_spec 应按请求落库")
	require.NotNil(t, rows[0].Enable)
	require.True(t, *rows[0].Enable, "enable 应按请求落库")
	require.Equal(t, "创建备注", *rows[0].Remark, "remark 应按请求落库")
	require.NotNil(t, rows[0].Type, "type 枚举应经 converter 落库")
	require.Equal(t, task.TypeDelay, *rows[0].Type, "type 枚举应落为 DELAY")
}

// TestTaskRepoSqlite_ListContainsFilter 验证 List 的 contains 模糊搜索语义。
func TestTaskRepoSqlite_ListContainsFilter(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	_, createErr := repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{
			TypeName: trans.Ptr("任务-markeruvw-甲"),
		},
	})
	require.NoError(t, createErr)
	_, createErr = repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{
			TypeName: trans.Ptr("任务-无关行-乙"),
		},
	})
	require.NoError(t, createErr)

	filtered, err := repo.List(ctx, &paginationV1.PagingRequest{
		FilteringType: &paginationV1.PagingRequest_FilterExpr{
			FilterExpr: &paginationV1.FilterExpr{
				Type: paginationV1.ExprType_AND,
				Conditions: []*paginationV1.FilterCondition{
					{
						Field:      "type_name",
						Op:         paginationV1.Operator_CONTAINS,
						ValueOneof: &paginationV1.FilterCondition_Value{Value: "markeruvw"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(1), filtered.Total, "contains 过滤后 Total 应为 1")
	require.Len(t, filtered.Items, 1, "contains 过滤后只应返回 1 行")
	require.Contains(t, *filtered.Items[0].TypeName, "markeruvw", "命中行应是携带标记的行")

	none, err := repo.List(ctx, &paginationV1.PagingRequest{
		FilteringType: &paginationV1.PagingRequest_FilterExpr{
			FilterExpr: &paginationV1.FilterExpr{
				Type: paginationV1.ExprType_AND,
				Conditions: []*paginationV1.FilterCondition{
					{
						Field:      "type_name",
						Op:         paginationV1.Operator_CONTAINS,
						ValueOneof: &paginationV1.FilterCondition_Value{Value: "no-such-marker-zzz"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint64(0), none.Total, "无命中 contains 应返回 Total=0")
	require.Empty(t, none.Items)

	all, err := repo.List(ctx, &paginationV1.PagingRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(2), all.Total, "无过滤时应返回全部 2 行")
	require.Len(t, all.Items, 2)
}

// TestTaskRepoSqlite_Get 验证按 ID 命中/未命中，
// 以及平台上下文按 type_name 查询被拒绝的分支。
func TestTaskRepoSqlite_Get(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	_, createErr := repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{TypeName: trans.Ptr("sqlite_task_get_name")},
	})
	require.NoError(t, createErr)
	rows, err := repo.dbClient.Client().Task.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	createdID := rows[0].ID

	// 命中：按 ID
	hit, err := repo.Get(ctx, &operatorV1.GetTaskRequest{
		QueryBy: &operatorV1.GetTaskRequest_Id{Id: createdID},
	})
	require.NoError(t, err, "按存在的 ID 查询应命中")
	require.Equal(t, createdID, hit.GetId())

	// 未命中：不存在的 ID
	_, err = repo.Get(ctx, &operatorV1.GetTaskRequest{
		QueryBy: &operatorV1.GetTaskRequest_Id{Id: 9999999},
	})
	require.Error(t, err, "不存在的 ID 查询应返回错误")

	// 平台上下文（无租户）按 type_name 查询应被拒绝
	_, err = repo.Get(ctx, &operatorV1.GetTaskRequest{
		QueryBy: &operatorV1.GetTaskRequest_TypeName{TypeName: "sqlite_task_get_name"},
	})
	require.Error(t, err, "系统级上下文按 type_name 查询应返回 BadRequest")
}

// TestTaskRepoSqlite_Update 验证 Update 只更新掩码内字段（remark），
// 掩码外字段（type_name/cron_spec）保持原值。
func TestTaskRepoSqlite_Update(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	_, createErr := repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{
			TypeName: trans.Ptr("sqlite_task_update_name"),
			CronSpec: trans.Ptr("0 0 0 * * *"),
			Remark:   trans.Ptr("更新前备注"),
		},
	})
	require.NoError(t, createErr)
	rows, err := repo.dbClient.Client().Task.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	createdID := rows[0].ID

	_, err = repo.Update(ctx, &operatorV1.UpdateTaskRequest{
		Id:         createdID,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"remark"}},
		Data: &operatorV1.Task{
			Remark: trans.Ptr("更新后备注-sqlite"),
		},
	})
	require.NoError(t, err, "更新 remark 应成功")

	after, err := repo.dbClient.Client().Task.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, "更新后备注-sqlite", *after[0].Remark, "掩码内字段应被更新")
	require.Equal(t, "sqlite_task_update_name", *after[0].TypeName, "掩码外字段 type_name 应保持原值")
	require.Equal(t, "0 0 0 * * *", *after[0].CronSpec, "掩码外字段 cron_spec 应保持原值")
}

// TestTaskRepoSqlite_TypeReadView 验证全部 3 个任务类型枚举值经 converter
// 落库后，在一切返回 Task DTO 的读路径（Create/Update 的回显、Get 按主键、
// List）的 DTO 视图如实呈现。
//
// 枚举字段读视图机制注记：实体侧 type 为可空指针枚举列（*task.Type），
// DTO 侧为可选指针字段。mapper 的枚举转换对（经 &srcType/&dstType 取址
// 注册）恰为指针↔指针形态的键，指针对字段能被 copier 直接转换赋值——与
// 值型实体枚举列（如 position.type、notification_channel.type）读侧被
// 丢弃的情形不同。type_name 为普通字符串列、无枚举转换参与，不在本测试
// 范围。本测试将该读视图行为钉死。
func TestTaskRepoSqlite_TypeReadView(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	cases := []struct {
		protoType operatorV1.Task_Type
		wantModel task.Type
		marker    string
	}{
		{operatorV1.Task_PERIODIC, task.TypePeriodic, "sqlite_task_readview_periodic"},
		{operatorV1.Task_DELAY, task.TypeDelay, "sqlite_task_readview_delay"},
		{operatorV1.Task_WAIT_RESULT, task.TypeWaitResult, "sqlite_task_readview_wait_result"},
	}

	for _, c := range cases {
		// 读路径一：Create 回显的 DTO 应如实呈现类型枚举。
		created, err := repo.Create(ctx, &operatorV1.CreateTaskRequest{
			Data: &operatorV1.Task{
				Type:     c.protoType.Enum(),
				TypeName: trans.Ptr(c.marker),
			},
		})
		require.NoError(t, err, "类型 %v 创建应成功", c.protoType)
		require.Equal(t, c.protoType, created.GetType(), "Create 回显 DTO 应如实呈现类型 %v", c.protoType)

		rows, err := dbClient.Client().Task.Query().
			Where(task.TypeNameEQ(c.marker)).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, rows, 1, "按 type_name 应反查到刚写入的行")
		require.Equal(t, c.wantModel, *rows[0].Type, "类型 %v 应经转换器如实落库", c.protoType)

		// 读路径二：Get 按主键命中后，DTO 视图应如实呈现类型枚举。
		got, err := repo.Get(ctx, &operatorV1.GetTaskRequest{
			QueryBy: &operatorV1.GetTaskRequest_Id{Id: rows[0].ID},
		})
		require.NoError(t, err, "按主键读取应命中")
		require.Equal(t, c.protoType, got.GetType(), "读视图应如实呈现类型 %v", c.protoType)

		// 读路径三：List 应含该行（按 type_name 标记匹配），DTO 视图如实呈现类型枚举。
		listed, err := repo.List(ctx, &paginationV1.PagingRequest{})
		require.NoError(t, err)
		var hit *operatorV1.Task
		for _, item := range listed.Items {
			if item.GetTypeName() == c.marker {
				hit = item
				break
			}
		}
		require.NotNil(t, hit, "List 应包含标记行 %s", c.marker)
		require.Equal(t, c.protoType, hit.GetType(), "List 读视图应如实呈现类型 %v", c.protoType)
	}

	// 掩码内更新 type 后：实体行、Update 回显 DTO、Get 读视图均应呈现新类型。
	delayRows, err := dbClient.Client().Task.Query().
		Where(task.TypeNameEQ("sqlite_task_readview_delay")).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, delayRows, 1)
	updated, err := repo.Update(ctx, &operatorV1.UpdateTaskRequest{
		Id:         delayRows[0].ID,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"type"}},
		Data: &operatorV1.Task{
			Type: operatorV1.Task_PERIODIC.Enum(),
		},
	})
	require.NoError(t, err, "掩码内更新 type 应成功")
	require.Equal(t, operatorV1.Task_PERIODIC, updated.GetType(), "Update 回显 DTO 应如实呈现更新后的类型 PERIODIC")
	afterRows, err := dbClient.Client().Task.Query().
		Where(task.TypeNameEQ("sqlite_task_readview_delay")).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, afterRows, 1)
	require.Equal(t, task.TypePeriodic, *afterRows[0].Type, "更新后实体行应为 PERIODIC")
	gotAfter, err := repo.Get(ctx, &operatorV1.GetTaskRequest{
		QueryBy: &operatorV1.GetTaskRequest_Id{Id: afterRows[0].ID},
	})
	require.NoError(t, err)
	require.Equal(t, operatorV1.Task_PERIODIC, gotAfter.GetType(), "更新后 Get 读视图应呈现 PERIODIC")
}

// TestTaskRepoSqlite_Delete 验证 Delete 后行数归零。
func TestTaskRepoSqlite_Delete(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	repo := newTaskRepoSqlite(t, dbClient)
	ctx := storetest.NewSystemContext(context.Background())

	_, createErr := repo.Create(ctx, &operatorV1.CreateTaskRequest{
		Data: &operatorV1.Task{TypeName: trans.Ptr("sqlite_task_delete_name")},
	})
	require.NoError(t, createErr)
	rows, err := repo.dbClient.Client().Task.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	require.NoError(t, repo.Delete(ctx, &operatorV1.DeleteTaskRequest{
		QueryBy: &operatorV1.DeleteTaskRequest_Id{Id: rows[0].ID},
	}), "Delete 应成功")
	cnt, err := repo.dbClient.Client().Task.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, cnt, "Delete 后表内行数应为 0")
}
