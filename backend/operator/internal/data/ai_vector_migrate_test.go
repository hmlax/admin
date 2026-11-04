package data

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// vectorTestDSN 向量集成测试库（沿用 GUARD_TEST_PG_DSN 的专用测试库模式；
// 连不上 / 无 pgvector 的环境一律 skip，保持缺环境套件绿）。
const vectorTestDSN = "host=127.0.0.1 port=5432 user=postgres password=*Abcd123456 dbname=gwa_guard_test sslmode=disable"

// openVectorTestDB 打开向量集成测试库。门控三段：连接失败、ping 失败、
// pgvector 扩展不可用，均按环境缺失 skip 而非 fail。
func openVectorTestDB(tb testing.TB) *sql.DB {
	tb.Helper()
	db, err := sql.Open("postgres", vectorTestDSNOrDefault())
	if err != nil {
		tb.Skipf("open vector test postgres: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		tb.Skipf("vector test postgres unavailable (set PGVECTOR_TEST_DSN to override): %v", err)
	}
	if _, err = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		_ = db.Close()
		tb.Skipf("pgvector extension unavailable: %v", err)
	}
	tb.Cleanup(func() { _ = db.Close() })
	return db
}

// vectorTestDSNOrDefault 测试库 DSN（PGVECTOR_TEST_DSN 覆盖默认）。
func vectorTestDSNOrDefault() string {
	dsn := os.Getenv("PGVECTOR_TEST_DSN")
	if dsn == "" {
		dsn = vectorTestDSN
	}
	return dsn
}

// vecLiteralOfDim 造 n 维全 0.5 的 pgvector 字面量（测试/基准数据）。
func vecLiteralOfDim(n int) string {
	return "[" + strings.TrimSuffix(strings.Repeat("0.5,", n), ",") + "]"
}

// TestEnsureVectorColumnDim_RejectsInvalidTableName 纯单测（无 DB）：
// 非法表名在触碰数据库前即被拒绝。
func TestEnsureVectorColumnDim_RejectsInvalidTableName(t *testing.T) {
	for _, name := range []string{
		"sys_ai_chunks; DROP TABLE x", "Bad-Name", "sys.ai.chunks", "'quoted'", "with space", "",
	} {
		if _, err := EnsureVectorColumnDim(context.Background(), nil, name); err == nil {
			t.Fatalf("expected rejection for table name %q", name)
		}
	}
}

// TestEnsureVectorColumnDim_MigratesUntypedColumn 集成（PG 门控）：
// 历史非定维 embedding 列被整列重建为 vector(1536)；二次调用幂等 no-op；
// 定维列接受 1536 维写入、按维度不符拒绝异维写入（fail-fast，防混维向量
// 把 <=> 检索在查询期整体打挂——见 docs/ai_module.md「部署要求」）。
func TestEnsureVectorColumnDim_MigratesUntypedColumn(t *testing.T) {
	db := openVectorTestDB(t)
	ctx := context.Background()
	const tbl = "_t_vecmig_test"

	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl); err != nil {
		t.Fatalf("pre-drop: %v", err)
	}
	if _, err := db.ExecContext(ctx, "CREATE TABLE "+tbl+" (id int primary key)"); err != nil {
		t.Fatalf("create scratch table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+tbl)
	})
	// 厺史非定维形态（2026-09-28 定维迁移前的存量列形状）
	if _, err := db.ExecContext(ctx, "ALTER TABLE "+tbl+" ADD COLUMN embedding vector"); err != nil {
		t.Fatalf("add untyped column: %v", err)
	}

	rebuilt, err := EnsureVectorColumnDim(ctx, db, tbl)
	require.NoError(t, err, "migration should succeed")
	require.True(t, rebuilt, "untyped column must be reported as rebuilt")

	var declared string
	if err = db.QueryRowContext(ctx,
		`SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a
		 WHERE a.attrelid = $1::regclass AND a.attname = 'embedding'
		   AND a.attnum > 0 AND NOT a.attisdropped`,
		tbl,
	).Scan(&declared); err != nil {
		t.Fatalf("read declared type: %v", err)
	}
	require.Equal(t, "vector(1536)", declared, "column must be retyped to vector(1536)")

	rebuiltAgain, err := EnsureVectorColumnDim(ctx, db, tbl)
	require.NoError(t, err, "second call should succeed")
	require.False(t, rebuiltAgain, "matching dimension must be a no-op")

	if _, err = db.ExecContext(ctx,
		"INSERT INTO "+tbl+" (id, embedding) VALUES (1, $1::vector)",
		vecLiteralOfDim(1536)); err != nil {
		t.Fatalf("insert matching-dim vector should succeed: %v", err)
	}
	if _, err = db.ExecContext(ctx,
		"INSERT INTO "+tbl+" (id, embedding) VALUES (2, $1::vector)",
		vecLiteralOfDim(384)); err == nil {
		t.Fatal("insert mismatched-dim vector must fail (fail-fast dimension enforcement)")
	}
}

// TestCountTableRows_RejectsInvalidTableName 纯单测（无 DB）：
// 非法表名在触碰数据库前即被拒绝。
func TestCountTableRows_RejectsInvalidTableName(t *testing.T) {
	for _, name := range []string{
		"sys_ai_chunks; DROP TABLE x", "Bad-Name", "sys.ai.chunks", "'quoted'", "with space", "",
	} {
		if _, err := CountTableRows(context.Background(), nil, name); err == nil {
			t.Fatalf("expected rejection for table name %q", name)
		}
	}
}

// TestEnsureHnswIndexIfLarge_ThresholdGate 集成（PG 门控）：HNSW 门槛三段——
// 阈下（含边界值-1）不建索引；越线（含边界值）建 HNSW cosine 索引（pg_indexes
// 断言）；二次调用幂等 no-op；建后按产品查询形状（embedding <=> $n::vector +
// ORDER BY + LIMIT）仍可执行且命中（证明 ops class 与 <=> 匹配、索引真实可用，
// 不是只存在于目录里）。CountTableRows 在真实种子行上断言精确性。
func TestEnsureHnswIndexIfLarge_ThresholdGate(t *testing.T) {
	db := openVectorTestDB(t)
	ctx := context.Background()
	const tbl = "_t_hnswgate_test"
	const idxName = "_t_hnswgate_test_embedding_hnsw"

	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl); err != nil {
		t.Fatalf("pre-drop: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"CREATE TABLE "+tbl+" (id int primary key, embedding vector(1536))"); err != nil {
		t.Fatalf("create scratch table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+tbl)
	})
	if _, err := db.ExecContext(ctx,
		"INSERT INTO "+tbl+" (id, embedding) VALUES (1, $1::vector)",
		vecLiteralOfDim(1536)); err != nil {
		t.Fatalf("seed row: %v", err)
	}

	// CountTableRows 精确性：种子 1 行即计 1。
	n, err := CountTableRows(ctx, db, tbl)
	require.NoError(t, err)
	require.EqualValues(t, 1, n, "CountTableRows must report exact seeded row count")

	hnswIndexDef := func() string {
		var def string
		err = db.QueryRowContext(ctx,
			`SELECT indexdef FROM pg_indexes WHERE tablename = $1 AND indexname = $2`,
			tbl, idxName).Scan(&def)
		if err == sql.ErrNoRows {
			return ""
		}
		require.NoError(t, err)
		return def
	}

	// 阈下（边界值-1）：不建。
	created, err := EnsureHnswIndexIfLarge(ctx, db, tbl, HnswIndexRowThreshold-1)
	require.NoError(t, err)
	require.False(t, created, "below-threshold call must not create the index")
	require.Empty(t, hnswIndexDef(), "no hnsw index should exist below threshold")

	// 越线（边界值）：建，目录里是 hnsw + cosine ops。
	created, err = EnsureHnswIndexIfLarge(ctx, db, tbl, HnswIndexRowThreshold)
	require.NoError(t, err)
	require.True(t, created, "at-threshold call must create the index")
	require.Contains(t, hnswIndexDef(), "USING hnsw", "index must be an hnsw index")
	require.Contains(t, hnswIndexDef(), "vector_cosine_ops", "index must use the cosine ops class")

	// 二次调用幂等：索引已存在即 no-op。
	created, err = EnsureHnswIndexIfLarge(ctx, db, tbl, HnswIndexRowThreshold)
	require.NoError(t, err)
	require.False(t, created, "existing index must make the call a no-op")
	require.Contains(t, hnswIndexDef(), "USING hnsw")

	// 建后产品查询形状可执行且命中自匹配种子行（cosine 距离 0 的最近邻）。
	var hitID int
	err = db.QueryRowContext(ctx,
		"SELECT id FROM "+tbl+" ORDER BY embedding <=> $1::vector LIMIT 1",
		vecLiteralOfDim(1536)).Scan(&hitID)
	require.NoError(t, err, "post-index query in product shape must execute")
	require.Equal(t, 1, hitID, "self-nearest seeded row must be returned")
}

// TestSetHnswIterativeScanDefault_RoundTrip 集成（PG 门控）：库级
// hnsw.iterative_scan 默认值往返——设为 relaxed_order 后新会话可见、RESET 后
// 回到 off。库级默认只对新会话生效（实测），故断言用第二条新连接；连接里先触碰
// vector 类型再 SHOW（GUC 定义随扩展 .so 会话内加载，实测空会话直接 SHOW 会报
// unrecognized）。收尾必须 RESET，否则把 relaxed_order 泄漏给同库其他门控测试。
func TestSetHnswIterativeScanDefault_RoundTrip(t *testing.T) {
	db := openVectorTestDB(t)
	ctx := context.Background()

	var dbName string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		t.Fatalf("read current database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`ALTER DATABASE "`+dbName+`" RESET hnsw.iterative_scan`)
	})

	if err := SetHnswIterativeScanDefault(ctx, db); err != nil {
		t.Fatalf("set default: %v", err)
	}

	freshSessionShow := func() string {
		fresh, err := sql.Open("postgres", vectorTestDSNOrDefault())
		if err != nil {
			t.Fatalf("open fresh session: %v", err)
		}
		defer func() { _ = fresh.Close() }()
		if _, err = fresh.ExecContext(context.Background(), `SELECT '[1,2]'::vector`); err != nil {
			t.Fatalf("load vector lib in fresh session: %v", err)
		}
		var val string
		if err = fresh.QueryRowContext(context.Background(), `SHOW hnsw.iterative_scan`).Scan(&val); err != nil {
			t.Fatalf("show in fresh session: %v", err)
		}
		return val
	}

	require.Equal(t, "relaxed_order", freshSessionShow(),
		"fresh session must see the database-level default after the gate set it")

	if _, err := db.ExecContext(ctx, `ALTER DATABASE "`+dbName+`" RESET hnsw.iterative_scan`); err != nil {
		t.Fatalf("reset default: %v", err)
	}
	require.Equal(t, "off", freshSessionShow(),
		"fresh session must see the built-in default after reset")
}

// BenchmarkRagVectorScan1536 基准（PG 门控）：RAG 检索查询形状（tenant/base
// 过滤 + <=> 排序 + LIMIT）在全行命中最坏情形下的扫描成本。行数是主导变量：
// 2026-09-28 本机实测 1 万行 ≈ 28ms、2 万行 ≈ 125ms（超线性，疑向量存储超出
// 内存缓存后打盘）——扩容承受力见 docs/ai_module.md。基准取 2000 行使默认
// benchtime 可在秒级完成。
func BenchmarkRagVectorScan1536(b *testing.B) {
	db := openVectorTestDB(b)
	ctx := context.Background()
	const tbl = "_t_vecbench_test"
	const rows = 2000

	if _, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl); err != nil {
		b.Fatalf("pre-drop: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		"CREATE TABLE "+tbl+" (id bigserial primary key, tenant_id int, base_id int, embedding vector(1536))"); err != nil {
		b.Fatalf("create bench table: %v", err)
	}
	b.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+tbl)
	})
	if _, err := db.ExecContext(ctx,
		`INSERT INTO `+tbl+` (tenant_id, base_id, embedding)
		 SELECT 1, 1, ('[' || array_to_string(array(SELECT 0.5::float8 FROM generate_series(1,1536)), ',') || ']')::vector
		 FROM generate_series(1,`+strconv.Itoa(rows)+`)`); err != nil {
		b.Fatalf("seed bench rows: %v", err)
	}

	vec := vecLiteralOfDim(1536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rs, err := db.QueryContext(ctx,
			"SELECT id FROM "+tbl+" WHERE tenant_id = 1 AND base_id = 1 ORDER BY embedding <=> $1::vector LIMIT 3",
			vec)
		if err != nil {
			b.Fatalf("query: %v", err)
		}
		_ = rs.Close()
	}
}
