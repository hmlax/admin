package data

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/mapper"
	"github.com/tx7do/go-utils/trans"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data/store"
	modelInternalMessage "go-admin/operator/internal/data/store/internalmessage"
	storetest "go-admin/operator/internal/data/storetest"
	operatorV1 "go-admin/platform/pb/operator"
)

// newInternalMessageRepoSqlite 用 storetest helper 构造一个可直接做 CRUD 的
// InternalMessageRepo，逐字段复刻 NewInternalMessageRepo 的 mapper/converter 初始化，
// 再调用 init()。
func newInternalMessageRepoSqlite(t *testing.T) *InternalMessageRepo {
	t.Helper()
	repo := &InternalMessageRepo{
		dbClient: storetest.NewClientForTest(t),
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		mapper:   mapper.NewCopierMapper[operatorV1.InternalMessage, store.InternalMessage](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Status, modelInternalMessage.Status](
			operatorV1.InternalMessage_Status_name, operatorV1.InternalMessage_Status_value,
		),
		typeConverter: mapper.NewEnumTypeConverter[operatorV1.InternalMessage_Type, modelInternalMessage.Type](
			operatorV1.InternalMessage_Type_name, operatorV1.InternalMessage_Type_value,
		),
	}
	repo.init()
	return repo
}

// TestInternalMessageRepoSqlite_Create 通过 repo.Create 写入一条含全部标量字段的消息，
// GORM client 直查断言各字段按请求落库（status/type 经 converter 转换），
// 并断言 Create 返回的 DTO 读视图对 status/type 如实呈现。
func TestInternalMessageRepoSqlite_Create(t *testing.T) {
	repo := newInternalMessageRepoSqlite(t)
	ctx := storetest.NewSystemContext(context.Background())

	created, err := repo.Create(ctx, &operatorV1.CreateInternalMessageRequest{
		Data: &operatorV1.InternalMessage{
			Title:      trans.Ptr("sqlite消息标题"),
			Content:    trans.Ptr("sqlite消息正文"),
			SenderId:   trans.Ptr(uint32(7)),
			CategoryId: trans.Ptr(uint32(4)),
			Status:     operatorV1.InternalMessage_PUBLISHED.Enum(),
			Type:       operatorV1.InternalMessage_PRIVATE.Enum(),
		},
	})
	require.NoError(t, err, "repo.Create 写入 SQLite 应成功")

	rows, err := repo.dbClient.Client().InternalMessage.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1, "internal_messages 应有 1 条记录")
	row := rows[0]
	require.Equal(t, "sqlite消息标题", *row.Title, "title 应按请求落库")
	require.Equal(t, "sqlite消息正文", *row.Content, "content 应按请求落库")
	require.Equal(t, uint32(7), *row.SenderID, "sender_id 应按请求落库")
	require.Equal(t, uint32(4), *row.CategoryID, "category_id 应按请求落库")
	require.NotNil(t, row.Status, "status 枚举应经 converter 落库")
	require.Equal(t, modelInternalMessage.StatusPublished, *row.Status,
		"proto PUBLISHED 应映射为 GORM StatusPublished")
	require.NotNil(t, row.Type, "type 枚举应经 converter 落库")
	require.Equal(t, modelInternalMessage.TypePrivate, *row.Type,
		"proto PRIVATE 应映射为 GORM TypePrivate")

	// Create 返回的 DTO 走同一 mapper：读视图应如实呈现写入的枚举值。
	require.NotNil(t, created.Status, "Create 返回 DTO 的 status 应如实呈现")
	require.Equal(t, operatorV1.InternalMessage_PUBLISHED, created.GetStatus(),
		"Create 返回 DTO 的 status 应如实呈现")
	require.NotNil(t, created.Type, "Create 返回 DTO 的 type 应如实呈现")
	require.Equal(t, operatorV1.InternalMessage_PRIVATE, created.GetType(),
		"Create 返回 DTO 的 type 应如实呈现")
}

// TestInternalMessageRepoSqlite_EnumReadback 对 status/type 两个枚举的全部取值逐一
// 建行（含零值 DRAFT/NOTIFICATION 的显式指定，以及未指定时按列默认落库的形态），
// 断言三件事：
//  1. GORM 行侧：显式值经 converter 如实落库；未指定的枚举按列默认（DRAFT/NOTIFICATION）落库；
//  2. List 读视图：DTO 的 status/type 逐行如实呈现行内实际存储值；
//  3. Get 与 ListByIds 读视图：同上逐行如实呈现。
//
// 形态说明：实体侧 status/type 均为可空指针枚举（schema Nillable），DTO 侧为可选
// 指针字段——两侧同为指针时，mapper 注册的枚举转换对（实体枚举名 → proto 枚举值）
// 在 copier 中逐字段命中并如实拷贝，读视图不丢值。本测试钉住该行为：若转换对被
// 注销、或实体侧形态漂移为值型默认枚举（值↔指针不匹配会被 copier 丢弃并退化为
// 零值——岗位仓 type 的历史缺陷形态），读视图的退化将被本测试捕获。
func TestInternalMessageRepoSqlite_EnumReadback(t *testing.T) {
	repo := newInternalMessageRepoSqlite(t)
	ctx := storetest.NewSystemContext(context.Background())

	// 每行携带完整期望：显式指定的枚举按指定值断言；未指定的枚举按列默认断言。
	type enumCase struct {
		title      string
		status     *operatorV1.InternalMessage_Status // nil=未指定 → 落列默认 DRAFT
		typ        *operatorV1.InternalMessage_Type   // nil=未指定 → 落列默认 NOTIFICATION
		wantStatus operatorV1.InternalMessage_Status  // 行内实际存储值的 proto 侧读回期望
		wantType   operatorV1.InternalMessage_Type    // 同上
		wantModelS modelInternalMessage.Status        // GORM 行侧实际存储值
		wantModelT modelInternalMessage.Type          // 同上
	}
	cases := []enumCase{}
	for _, s := range []struct {
		proto     operatorV1.InternalMessage_Status
		modelWant modelInternalMessage.Status
	}{
		{operatorV1.InternalMessage_DRAFT, modelInternalMessage.StatusDraft},
		{operatorV1.InternalMessage_PUBLISHED, modelInternalMessage.StatusPublished},
		{operatorV1.InternalMessage_SCHEDULED, modelInternalMessage.StatusScheduled},
		{operatorV1.InternalMessage_REVOKED, modelInternalMessage.StatusRevoked},
		{operatorV1.InternalMessage_ARCHIVED, modelInternalMessage.StatusArchived},
		{operatorV1.InternalMessage_DELETED, modelInternalMessage.StatusDeleted},
	} {
		cases = append(cases, enumCase{
			title:      fmt.Sprintf("IM_SQLITE_ENUM_S_%d", s.proto),
			status:     s.proto.Enum(),
			typ:        nil,
			wantStatus: s.proto,
			wantType:   operatorV1.InternalMessage_NOTIFICATION,
			wantModelS: s.modelWant,
			wantModelT: modelInternalMessage.TypeNotification,
		})
	}
	for _, tv := range []struct {
		proto     operatorV1.InternalMessage_Type
		modelWant modelInternalMessage.Type
	}{
		{operatorV1.InternalMessage_NOTIFICATION, modelInternalMessage.TypeNotification},
		{operatorV1.InternalMessage_PRIVATE, modelInternalMessage.TypePrivate},
		{operatorV1.InternalMessage_GROUP, modelInternalMessage.TypeGroup},
	} {
		cases = append(cases, enumCase{
			title:      fmt.Sprintf("IM_SQLITE_ENUM_T_%d", tv.proto),
			status:     nil,
			typ:        tv.proto.Enum(),
			wantStatus: operatorV1.InternalMessage_DRAFT,
			wantType:   tv.proto,
			wantModelS: modelInternalMessage.StatusDraft,
			wantModelT: tv.modelWant,
		})
	}
	// 双未指定行：两个枚举都按各自列默认落库。
	cases = append(cases, enumCase{
		title:      "IM_SQLITE_ENUM_BOTH_DEFAULT",
		status:     nil,
		typ:        nil,
		wantStatus: operatorV1.InternalMessage_DRAFT,
		wantType:   operatorV1.InternalMessage_NOTIFICATION,
		wantModelS: modelInternalMessage.StatusDraft,
		wantModelT: modelInternalMessage.TypeNotification,
	})

	for _, c := range cases {
		_, err := repo.Create(ctx, &operatorV1.CreateInternalMessageRequest{
			Data: &operatorV1.InternalMessage{
				Title:  trans.Ptr(c.title),
				Status: c.status,
				Type:   c.typ,
			},
		})
		require.NoError(t, err, "行 %s 建行应成功", c.title)
	}

	// —— GORM 行侧：行内实际存储值（显式值经 converter 落库；未指定按列默认落库）——
	rows, err := repo.dbClient.Client().InternalMessage.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, rows, len(cases), "应有 %d 行", len(cases))
	rowsByTitle := map[string]*store.InternalMessage{}
	for _, row := range rows {
		rowsByTitle[*row.Title] = row
	}
	ids := make([]uint32, 0, len(cases))
	for _, c := range cases {
		row, ok := rowsByTitle[c.title]
		require.True(t, ok, "行 %s 应存在", c.title)
		ids = append(ids, row.ID)
		require.Equal(t, c.wantModelS, *row.Status,
			"行 %s 的 status 应按（显式值或列默认）如实落库", c.title)
		require.Equal(t, c.wantModelT, *row.Type,
			"行 %s 的 type 应按（显式值或列默认）如实落库", c.title)
	}

	// —— List 读视图：DTO 逐行如实呈现行内实际存储值 ——
	listed, err := repo.List(ctx, &paginationV1.PagingRequest{})
	require.NoError(t, err)
	require.Len(t, listed.Items, len(cases), "List 应返回全部 %d 行", len(cases))
	itemsByTitle := map[string]*operatorV1.InternalMessage{}
	for _, item := range listed.Items {
		itemsByTitle[item.GetTitle()] = item
	}
	for _, c := range cases {
		item, ok := itemsByTitle[c.title]
		require.True(t, ok, "行 %s 应出现在 List 结果中", c.title)
		require.NotNil(t, item.Status, "行 %s 的 status 读视图应为非 nil（行内非 NULL）", c.title)
		require.Equal(t, c.wantStatus, item.GetStatus(),
			"行 %s 的 status 读视图应如实呈现行内存储值", c.title)
		require.NotNil(t, item.Type, "行 %s 的 type 读视图应为非 nil（行内非 NULL）", c.title)
		require.Equal(t, c.wantType, item.GetType(),
			"行 %s 的 type 读视图应如实呈现行内存储值", c.title)
	}

	// —— Get 读视图：按主键逐行读取、如实呈现 ——
	for i, c := range cases {
		got, err := repo.Get(ctx, &operatorV1.GetInternalMessageRequest{
			QueryBy: &operatorV1.GetInternalMessageRequest_Id{Id: ids[i]},
		})
		require.NoError(t, err, "行 %s 按主键读取应命中", c.title)
		require.Equal(t, c.wantStatus, got.GetStatus(),
			"行 %s 的 Get 读视图应如实呈现 status", c.title)
		require.Equal(t, c.wantType, got.GetType(),
			"行 %s 的 Get 读视图应如实呈现 type", c.title)
	}

	// —— ListByIds 读视图：按主键批量读取、逐行如实呈现 ——
	byIds, err := repo.ListByIds(ctx, ids)
	require.NoError(t, err)
	require.Len(t, byIds, len(cases), "ListByIds 应返回全部 %d 行", len(cases))
	for i, c := range cases {
		dto, ok := byIds[ids[i]]
		require.True(t, ok, "行 %s 应出现在 ListByIds 结果中", c.title)
		require.Equal(t, c.wantStatus, dto.GetStatus(),
			"行 %s 的 ListByIds 读视图应如实呈现 status", c.title)
		require.Equal(t, c.wantType, dto.GetType(),
			"行 %s 的 ListByIds 读视图应如实呈现 type", c.title)
	}
}
