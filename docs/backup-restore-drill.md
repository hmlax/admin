# 备份恢复演练（Backup Restore Drill）

> 配套实现：备份导出（`BackupRepo.ExportCoreTables`，见 docs/task_system.md）
> 与恢复（`BackupRepo.RestoreCoreTables`，仅允许空库、保留原 ID、单事务）。
> 本文档面向运维/交付：如何**安全地**验证"备份确实可用"，以及在灾难场景下如何执行恢复。

## 1. 能覆盖与不覆盖

| 覆盖 | 不覆盖 |
|---|---|
| 8 张核心身份/权限/组织表：tenants、users、roles、permissions、memberships、org_units、positions、menus | 审计日志等大体量表（备份本就不含，走等保归档 JSONL） |
| 保留原 ID 与时间字段 | 增量合并恢复（只支持空库整体恢复） |
| XLSX/JSON 双格式里的 JSON 备份（`*.json.gz`） | 密码外的加密 payload 解密依赖（runtime.crypto_key 未设时明文，设了照常恢复） |

> 等保视角：核心身份/权限数据 ≥6 个月留存由审计归档 JSONL 承担；本工具链解决
> "核心配置表误删/迁移"场景。两者互补，不互相替代。

## 2. 恢复演练（标准流程）

**原则：演练永远不碰活库。** 用一次性全新库（或临时 SQLite 文件库）做导入验证。

### 2.1 前置

- 取到一份备份产物：OSS `backups` 桶对象（`YYYY/MM/DD/<name>-<time>.json.gz`）或本地文件；
- 一个**空的目标数据库**（全新 schema 迁移后、未播种的库）。

### 2.2 步骤（一次性 Go 程序，参照 `backup_restore_test.go` 的往返测试）

```go
// 1) 读备份文件（.json.gz 先 gzip 解压）
raw, _ := os.ReadFile("backup.json.gz")
gzipReader := gzip.NewReader(bytes.NewReader(raw))
doc, _ := io.ReadAll(gzipReader)

// 2) 解出 data 段
var doc0 map[string]json.RawMessage
_ = json.Unmarshal(doc, &doc0)

// 3) 连接空库（迁移 schema 后、未播种）
repo := &data.BackupRepo{EntClient: entClient}
counts, err := repo.RestoreCoreTables(ctx, doc0["data"])
// 成功后 counts 即各表恢复行数；日志核对照份（导出时日志有各表行数）
```

### 2.3 恢复后校验清单

1. `counts` 与备份 `_meta` 记录一致；
2. 管理员账号能登录（密码哈希随备份保真）；
3. 抽查租户/角色/菜单绑定关系完整；
4. 平台超管权限码（`sys:platform_admin`）仍能拉到全部菜单（ID 保真的意义所在）。

### 2.4 频率建议

- 每季度一次演练；每次等保测评前必演；
- 演练记录留存（谁、何时、哪个备份、校验结论）——等保测评要查。

## 3. 灾难恢复（真实场景）

1. 定位故障（误删/迁移）→ 准备全新库（同版本 schema 迁移、未播种）；
2. 选最近一次成功备份（AsyncBackup 日志 `backup: completed successfully, object=...`）；
3. 按第 2 节流程恢复到新库 → 校验 → 把应用配置指到新库 → 启动；
4. 事后把误操作期间的业务数据人工补录（恢复点是备份时刻，之后的增量不在覆盖范围）。

## 4. 明确不做

- **不做活库合并恢复**：RestoreCoreTables 前置校验任一目标表非空即整体中止
  （这是安全设计：活库合并涉及 ID 冲突/外键顺序/序列回填，脚本化极易产出脏数据）；
- 不提供管理页/RPC 恢复入口（避免误触；恢复是运维动作，走一次性程序）；
- 不自动恢复（自动=把误删快速放大，人工确认是必要闸门）。
