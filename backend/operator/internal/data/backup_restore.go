package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 备份恢复：RestoreCoreTables 是 ExportCoreTables 的逆操作。
//
// 安全性：**仅允许向空库恢复**——任一目标表非空即整体中止（disaster recovery
// 语义是"全新库 + 恢复 + 启动"，不是对活库做合并；活库合并涉及 ID 冲突/
// 外键顺序/序列回填，超出工具职责）。恢复保留原 ID，按 ID 升序逐表插入，
// 单事务：任一表失败整体回滚，不留半恢复状态。
//
// 实现取表无关的动态 INSERT（备份 JSON 的键即列名——GORM 实体 JSON tag 与
// 列名一致，且不含边字段），避免逐表手写字段映射漏列。表名走固定白名单，
// 列名经引号包裹，无注入面。

// restoreTableOrder 恢复顺序：被引用表先于引用表（外键依赖），与导出表集一致。
// 键为备份 JSON 的逻辑名（ExportCoreTables 的键），值带 sys_ 前缀物理表名。
var restoreTableOrder = []string{
	"tenants",
	"org_units",
	"positions",
	"users",
	"roles",
	"permissions",
	"memberships",
	"menus",
}

// restorePhysicalTables 逻辑名 → 物理表名（GORM 迁移生成的物理表带 sys_ 前缀）。
var restorePhysicalTables = map[string]string{
	"tenants":     "sys_tenants",
	"org_units":   "sys_org_units",
	"positions":   "sys_positions",
	"users":       "sys_users",
	"roles":       "sys_roles",
	"permissions": "sys_permissions",
	"memberships": "sys_memberships",
	"menus":       "sys_menus",
}

// RestoreCoreTables 把备份 JSON 的 data 段恢复进核心表，返回各表行数。
func (r *BackupRepo) RestoreCoreTables(ctx context.Context, rawData json.RawMessage) (map[string]int64, error) {
	var tables map[string]json.RawMessage
	if err := json.Unmarshal(rawData, &tables); err != nil {
		return nil, fmt.Errorf("decode backup data failed: %w", err)
	}

	db := r.dbClient.DB()

	// 前置校验：全部目标表必须为空（任一非空即中止）
	for _, name := range restoreTableOrder {
		count, err := countRestoreTable(ctx, db, restorePhysicalTables[name])
		if err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, fmt.Errorf("table %q is not empty (%d rows): restore only allowed into empty tables", name, count)
		}
	}

	// 原生 SQL 事务（动态 INSERT 不经 GORM builder）
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin restore transaction failed: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	counts := make(map[string]int64, len(restoreTableOrder))
	for _, name := range restoreTableOrder {
		raw, ok := tables[name]
		if !ok || strings.TrimSpace(string(raw)) == "" || string(raw) == "null" {
			counts[name] = 0
			continue
		}
		n, rerr := restoreOneTable(ctx, tx, restorePhysicalTables[name], raw)
		if rerr != nil {
			return nil, fmt.Errorf("restore table %q failed: %w", name, rerr)
		}
		counts[name] = n
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit restore failed: %w", err)
	}
	committed = true

	return counts, nil
}

// restoreOneTable 动态 INSERT 单表：行按 id 升序稳定排序。
func restoreOneTable(ctx context.Context, tx *sql.Tx, name string, raw json.RawMessage) (int64, error) {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("decode rows failed: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// 行内列集取并集（GORM 实体 JSON 键一致，但个别行可能缺 omitempty 字段）。
	// "edges" 键丢弃：GORM 实体序列化时带空 edges 对象，非表列。
	colSet := map[string]bool{}
	for _, row := range rows {
		for col := range row {
			if col == "edges" {
				continue
			}
			colSet[col] = true
		}
	}
	cols := make([]string, 0, len(colSet))
	for col := range colSet {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	// 按 id 升序稳定排序（被引用行先行；无 id 键保持原序）
	sort.SliceStable(rows, func(i, j int) bool {
		a, aok := rows[i]["id"].(float64)
		b, bok := rows[j]["id"].(float64)
		if aok && bok {
			return a < b
		}
		return false
	})

	quotedCols := make([]string, 0, len(cols))
	for _, c := range cols {
		quotedCols = append(quotedCols, `"`+c+`"`)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	insertSQL := fmt.Sprintf(`INSERT INTO %q (%s) VALUES (%s)`,
		name, strings.Join(quotedCols, ", "), placeholders)

	var total int64
	for _, row := range rows {
		args := make([]any, 0, len(cols))
		for _, c := range cols {
			args = append(args, row[c])
		}
		if _, err := tx.ExecContext(ctx, insertSQL, args...); err != nil {
			return total, fmt.Errorf("insert row (id=%v) failed: %w", row["id"], err)
		}
		total++
	}
	return total, nil
}

func countRestoreTable(ctx context.Context, db *sql.DB, name string) (int, error) {
	// name 走固定白名单（restoreTableOrder），无注入面
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+name+`"`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count table %q failed: %w", name, err)
	}
	return count, nil
}
