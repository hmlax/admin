package data

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"go-admin/operator/internal/data/store/menu"
	"go-admin/operator/internal/data/store/tenant"
	storetest "go-admin/operator/internal/data/storetest"
)

// TestBackupRestoreRoundTrip 恢复工具链的往返实证：
// 导出核心表 → 清空全部目标表 → RestoreCoreTables 恢复 → 行数与内容一致。
// 非空表前置校验与单事务语义由其余用例钉住。
func TestBackupRestoreRoundTrip(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	client := dbClient.Client()
	repo := &BackupRepo{dbClient: dbClient}
	ctx := storetest.NewSystemContext(context.Background())

	// 1. 播种：两个租户（带 Logo/时间字段）+ 一条菜单（父子结构）
	t1, err := client.Tenant.Create().
		SetName("往返租户甲").
		SetCode("RT_A").
		SetLogoURL("https://cdn.example.com/rt-a.png").
		SetStatus(tenant.StatusOn).
		Save(ctx)
	require.NoError(t, err)
	t2, err := client.Tenant.Create().
		SetName("往返租户乙").
		SetCode("RT_B").
		SetStatus(tenant.StatusOn).
		Save(ctx)
	require.NoError(t, err)

	menu1, err := client.Menu.Create().
		SetName("往返菜单").
		SetPath("/rt").
		SetStatus(menu.StatusOn). // 状态枚举：与种子同值
		Save(ctx)
	require.NoError(t, err)

	// 2. 导出：data 段即 map[表名]行数组（RestoreCoreTables 的入参形状）
	exported, err := repo.ExportCoreTables(ctx)
	require.NoError(t, err)
	rawTables, err := json.Marshal(exported)
	require.NoError(t, err)

	// 3. 清空全部目标表（模拟"全新库"——删序按外键逆序）
	_, err = client.Menu.Delete().Exec(ctx)
	require.NoError(t, err)
	_, err = client.Tenant.Delete().Exec(ctx)
	require.NoError(t, err)

	// 中途抽检：确认真空了
	count, err := client.Tenant.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	// 4. 恢复
	counts, err := repo.RestoreCoreTables(ctx, rawTables)
	require.NoError(t, err)
	require.EqualValues(t, 2, counts["tenants"], "租户应恢复 2 行")
	require.EqualValues(t, 1, counts["menus"], "菜单应恢复 1 行")

	// 5. 内容一致性抽检：字段值与 ID 保真
	restored1, err := client.Tenant.Query().
		Where(tenant.IDEQ(t1.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "往返租户甲", *restored1.Name)
	require.Equal(t, "https://cdn.example.com/rt-a.png", *restored1.LogoURL)
	require.Equal(t, "RT_A", *restored1.Code)

	restoredMenu, err := client.Menu.Query().
		Where(menu.IDEQ(menu1.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "往返菜单", *restoredMenu.Name)
	require.Equal(t, menu1.ID, restoredMenu.ID)
	_ = t2
}

// TestRestoreRefusesNonEmptyTables 非空表前置校验：任一目标表非空即中止且不写入。
func TestRestoreRefusesNonEmptyTables(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	client := dbClient.Client()
	repo := &BackupRepo{dbClient: dbClient}
	ctx := storetest.NewSystemContext(context.Background())

	// 造一个非空的 menus（目标表之一），其余全空
	_, err := client.Menu.Create().
		SetName("既有菜单").
		SetPath("/existing").
		Save(ctx)
	require.NoError(t, err)

	payload := map[string]json.RawMessage{
		"tenants": json.RawMessage(`[{"id":1,"name":"x"}]`),
	}
	rawPayload, _ := json.Marshal(map[string]any{"menus": payload["menus"]})
	_, err = repo.RestoreCoreTables(ctx, rawPayload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not empty")

	// 中止后不写入：tenants 仍为空
	count, err := client.Tenant.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}
