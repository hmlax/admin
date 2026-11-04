package data

import (
	"context"
	"database/sql"
	"time"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/loginauditlog"
	"go-admin/operator/internal/data/store/operationauditlog"
	operatorV1 "go-admin/platform/pb/operator"
)

// DistributionRow scans grouped database values into named fields.
type DistributionRow struct {
	Action string `sql:"action"`
	Status string `sql:"status"`
	Count  int    `sql:"count"`
}

// TrendRow 是登录趋势按日分桶后的结果项（日期 + 次数），由 service 层使用。
type TrendRow struct {
	Date  string
	Count int
}

// DashboardRepo 聚合多张表做只读统计。
// 查询在 store 中按 ViewerContext 自动叠加租户条件。
type DashboardRepo struct {
	dbClient *store.DBClient
	log      *bLogger.Helper
}

func NewDashboardRepo(ctx *bootstrap.Context, dbClient *store.DBClient) *DashboardRepo {
	return &DashboardRepo{
		log:      ctx.NewLoggerHelper("dashboard/repo/admin-service"),
		dbClient: dbClient,
	}
}

// startOfToday 返回今日零点（本地时区）。
func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// CountActiveUsers 统计用户总数（租户隔离由 store 自动处理）。
func (r *DashboardRepo) CountActiveUsers(ctx context.Context) (int, error) {
	count, err := r.dbClient.Client().User.Query().Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count users failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("count users failed")
	}
	return count, nil
}

// CountRoles 统计角色总数。
func (r *DashboardRepo) CountRoles(ctx context.Context) (int, error) {
	count, err := r.dbClient.Client().Role.Query().Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count roles failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("count roles failed")
	}
	return count, nil
}

// CountTodayLogins 统计今日登录次数（action_type = LOGIN 且 created_at >= 今日零点）。
func (r *DashboardRepo) CountTodayLogins(ctx context.Context) (int, error) {
	today := startOfToday()
	count, err := r.dbClient.Client().LoginAuditLog.Query().
		Where(
			loginauditlog.ActionTypeEQ(loginauditlog.ActionTypeLogin),
			loginauditlog.CreatedAtGTE(today),
		).
		Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count today logins failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("count today logins failed")
	}
	return count, nil
}

// CountTodayOperations 统计今日操作审计条数（created_at >= 今日零点）。
func (r *DashboardRepo) CountTodayOperations(ctx context.Context) (int, error) {
	today := startOfToday()
	count, err := r.dbClient.Client().OperationAuditLog.Query().
		Where(operationauditlog.CreatedAtGTE(today)).
		Count(ctx)
	if err != nil {
		r.log.Errorf(ctx, "count today operations failed: %s", err.Error())
		return 0, operatorV1.ErrorAdminInternalServerError("count today operations failed")
	}
	return count, nil
}

// LoginTrend 统计近 days 天每日登录次数（action_type = LOGIN）。
// GORM 的 GroupBy 只支持已有列、不支持 DATE() 等原生表达式，且 prepareQuery 会校验列名，
// 故这里只 Select 出 created_at 单列，在内存按天分桶补零。
// 近 N 天登录审计数据量可控，单列查询开销可接受。
func (r *DashboardRepo) LoginTrend(ctx context.Context, days int) ([]TrendRow, error) {
	if days <= 0 {
		days = 7
	}
	today := startOfToday()
	start := today.AddDate(0, 0, -(days - 1))

	type onlyCreated struct {
		CreatedAt time.Time `sql:"created_at"`
	}
	var logs []onlyCreated
	err := r.dbClient.Client().LoginAuditLog.Query().
		Where(
			loginauditlog.ActionTypeEQ(loginauditlog.ActionTypeLogin),
			loginauditlog.CreatedAtGTE(start),
		).
		Select(loginauditlog.FieldCreatedAt).
		Scan(ctx, &logs)
	if err != nil {
		r.log.Errorf(ctx, "login trend select failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("login trend query failed")
	}

	// 按日期初始化桶，保证无记录的日期也有零值，结果按日期升序。
	loc := today.Location()
	buckets := make([]TrendRow, 0, days)
	idx := make(map[string]int, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		idx[d] = i
		buckets = append(buckets, TrendRow{Date: d, Count: 0})
	}
	for _, lg := range logs {
		d := lg.CreatedAt.In(loc).Format("2006-01-02")
		if i, ok := idx[d]; ok {
			buckets[i].Count++
		}
	}
	return buckets, nil
}

// OperationActionDistribution 按操作类型（action）分组统计。
// 返回的 Label 是 action 枚举值字符串（CREATE/UPDATE/...），由 service 层透传，前端做 i18n 映射。
// GroupBy 的字段会被自动 select；用 store.As(store.Count(),"count") 给计数列起别名，
// 配合 struct 的 sql tag 映射；分组列名即字段名 "action"，故 struct 用 Label `sql:"action"`。
func (r *DashboardRepo) OperationActionDistribution(ctx context.Context) ([]DistributionRow, error) {
	var rows []DistributionRow
	err := r.dbClient.Client().OperationAuditLog.Query().
		GroupBy(operationauditlog.FieldAction).
		Aggregate(store.As(store.Count(), "count")).
		Scan(ctx, &rows)
	if err != nil {
		r.log.Errorf(ctx, "operation action distribution failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("operation action distribution query failed")
	}
	return rows, nil
}

// LoginStatusDistribution 按登录状态（status）分组统计。
// Label 为 status 枚举值字符串（SUCCESS/FAILED/...）。
func (r *DashboardRepo) LoginStatusDistribution(ctx context.Context) ([]DistributionRow, error) {
	var rows []DistributionRow
	err := r.dbClient.Client().LoginAuditLog.Query().
		GroupBy(loginauditlog.FieldStatus).
		Aggregate(store.As(store.Count(), "count")).
		Scan(ctx, &rows)
	if err != nil {
		r.log.Errorf(ctx, "login status distribution failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("login status distribution query failed")
	}
	return rows, nil
}

// ── 安全与异常洞察（审计明细的行为模式挖掘，非页面聚合数字的复述） ──────

// UserBehaviorRow 按用户聚合的 24h 操作行为。
type UserBehaviorRow struct {
	UserID    uint32 `sql:"user_id"`
	Username  string `sql:"username"`
	Total     int    `sql:"total"`
	Failed    int    `sql:"failed"`
	Sensitive int    `sql:"sensitive"`
	Night     int    `sql:"night"`
}

// LoginFailRow 24h 内登录失败集中的账号。
type LoginFailRow struct {
	Username string `sql:"username"`
	Fails    int    `sql:"fails"`
	LastIP   string `sql:"last_ip"`
}

// SensitiveOpRow 24h 内的敏感操作明细（DELETE/EXPORT/ASSIGN）。
type SensitiveOpRow struct {
	Username     string    `sql:"username"`
	Action       string    `sql:"action"`
	ResourceType string    `sql:"resource_type"`
	ResourceID   string    `sql:"resource_id"`
	CreatedAt    time.Time `sql:"created_at"`
}

// UserBehavior24h 按用户聚合近 24h 操作行为：
// 总数 / 失败 / 敏感（DELETE/EXPORT/ASSIGN）/ 深夜（本地 0-6 点）。
func (r *DashboardRepo) UserBehavior24h(ctx context.Context) ([]UserBehaviorRow, error) {
	rows, err := r.dbClient.DB().QueryContext(ctx,
		`SELECT user_id, MAX(username) AS username,
		        COUNT(*) AS total,
		        SUM(CASE WHEN success = false THEN 1 ELSE 0 END) AS failed,
		        SUM(CASE WHEN action IN ('DELETE','EXPORT','ASSIGN') THEN 1 ELSE 0 END) AS sensitive,
		        SUM(CASE WHEN EXTRACT(HOUR FROM created_at AT TIME ZONE 'Asia/Shanghai') < 6 THEN 1 ELSE 0 END) AS night
		 FROM sys_operation_audit_logs
		 WHERE created_at >= NOW() - INTERVAL '24 hours' AND user_id IS NOT NULL
		 GROUP BY user_id
		 ORDER BY total DESC`,
	)
	if err != nil {
		r.log.Errorf(ctx, "user behavior 24h query failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("query user behavior failed")
	}
	defer func() { _ = rows.Close() }()

	out := make([]UserBehaviorRow, 0, 16)
	for rows.Next() {
		var row UserBehaviorRow
		var username sql.NullString
		if err = rows.Scan(&row.UserID, &username, &row.Total, &row.Failed, &row.Sensitive, &row.Night); err != nil {
			r.log.Errorf(ctx, "scan user behavior failed: %s", err.Error())
			return nil, operatorV1.ErrorAdminInternalServerError("scan user behavior failed")
		}
		row.Username = username.String
		out = append(out, row)
	}
	return out, rows.Err()
}

// LoginFailAccounts 24h 内登录失败次数达到阈值的账号（疑似口令尝试）。
func (r *DashboardRepo) LoginFailAccounts(ctx context.Context, minFails int) ([]LoginFailRow, error) {
	rows, err := r.dbClient.DB().QueryContext(ctx,
		`SELECT COALESCE(username, ''), COUNT(*) AS fails, MAX(ip_address) AS last_ip
		 FROM sys_login_audit_logs
		 WHERE created_at >= NOW() - INTERVAL '24 hours' AND status = 'FAILED'
		 GROUP BY COALESCE(username, '') HAVING COUNT(*) >= $1
		 ORDER BY fails DESC`,
		minFails,
	)
	if err != nil {
		r.log.Errorf(ctx, "login fail accounts query failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("query login fails failed")
	}
	defer func() { _ = rows.Close() }()

	out := make([]LoginFailRow, 0, 4)
	for rows.Next() {
		var row LoginFailRow
		if err = rows.Scan(&row.Username, &row.Fails, &row.LastIP); err != nil {
			r.log.Errorf(ctx, "scan login fail failed: %s", err.Error())
			return nil, operatorV1.ErrorAdminInternalServerError("scan login fails failed")
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SensitiveOps24h 24h 内敏感操作明细（DELETE/EXPORT/ASSIGN，按时间倒序，最多 limit 条）。
func (r *DashboardRepo) SensitiveOps24h(ctx context.Context, limit int) ([]SensitiveOpRow, error) {
	rows, err := r.dbClient.DB().QueryContext(ctx,
		`SELECT COALESCE(username,''), action, resource_type, COALESCE(resource_id,''), created_at
		 FROM sys_operation_audit_logs
		 WHERE created_at >= NOW() - INTERVAL '24 hours' AND action IN ('DELETE','EXPORT','ASSIGN')
		 ORDER BY created_at DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		r.log.Errorf(ctx, "sensitive ops query failed: %s", err.Error())
		return nil, operatorV1.ErrorAdminInternalServerError("query sensitive ops failed")
	}
	defer func() { _ = rows.Close() }()

	out := make([]SensitiveOpRow, 0, limit)
	for rows.Next() {
		var row SensitiveOpRow
		if err = rows.Scan(&row.Username, &row.Action, &row.ResourceType, &row.ResourceID, &row.CreatedAt); err != nil {
			r.log.Errorf(ctx, "scan sensitive op failed: %s", err.Error())
			return nil, operatorV1.ErrorAdminInternalServerError("scan sensitive ops failed")
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
