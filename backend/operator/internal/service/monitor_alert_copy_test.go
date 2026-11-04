package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	storetest "go-admin/operator/internal/data/storetest"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/mailtext"
)

// TestAlertCopyTableBothLocales 文案表两语言完整且真实分叉；未识别/非规范标签
// 一律回落中文（归一是 LocaleOfTag 的职责，解析路径先归一再查表）。
func TestAlertCopyTableBothLocales(t *testing.T) {
	zh := alertCopyOf(mailtext.LocaleZhCN)
	en := alertCopyOf(mailtext.LocaleEnUS)
	for _, cp := range []alertCopy{zh, en} {
		for _, f := range []string{cp.resolvedTitle, cp.resolvedBody, cp.alertTitle, cp.alertBody} {
			require.NotEmpty(t, f)
		}
	}
	require.NotEqual(t, zh, en)
	require.Equal(t, zh, alertCopyOf("zh-CN"))
	require.Equal(t, en, alertCopyOf("en-US"))
	require.Equal(t, zh, alertCopyOf("xx"))
	require.Equal(t, zh, alertCopyOf("zh"))
	require.Equal(t, zh, alertCopyOf("en"))
}

// TestAlertTextLocalized 中英双语的告警/恢复文案逐字节钉死；
// 指标与比较符保持枚举字面量（两语言一致，与闸门报错/用量页同口径）。
func TestAlertTextLocalized(t *testing.T) {
	rule := &operatorV1.MonitorAlertRule{
		Name:            trans.Ptr("数据库连通"),
		Metric:          trans.Ptr(operatorV1.MonitorMetric_DB_PING_FAIL),
		Op:              trans.Ptr(operatorV1.AlertOp_GE),
		Threshold:       trans.Ptr(float64(1)),
		CooldownMinutes: trans.Ptr(uint32(30)),
	}
	now := time.Date(2026, 10, 4, 9, 30, 0, 0, time.Local)

	title, content := alertTextFor(mailtext.LocaleZhCN, rule, 1, false, now)
	require.Equal(t, "[告警] 数据库连通", title)
	require.Equal(t,
		"监控指标 DB_PING_FAIL 当前值 1.00，越过阈值 GE 1.00，时间 2026-10-04 09:30:00。持续越限时每 30 分钟重发一次。",
		content)

	title, content = alertTextFor(mailtext.LocaleEnUS, rule, 1, false, now)
	require.Equal(t, "[Alert] 数据库连通", title)
	require.Equal(t,
		"Metric DB_PING_FAIL current value 1.00 crossed the threshold GE 1.00 at 2026-10-04 09:30:00. Re-alerts every 30 minutes while it keeps crossing.",
		content)

	title, content = alertTextFor(mailtext.LocaleZhCN, rule, 0, true, now)
	require.Equal(t, "[已恢复] 数据库连通", title)
	require.Equal(t,
		"监控指标 DB_PING_FAIL 已回到阈值内（当前值 0.00，阈值 GE 1.00），时间 2026-10-04 09:30:00。",
		content)

	title, content = alertTextFor(mailtext.LocaleEnUS, rule, 0, true, now)
	require.Equal(t, "[Resolved] 数据库连通", title)
	require.Equal(t,
		"Metric DB_PING_FAIL is back within threshold (current value 0.00, threshold GE 1.00), at 2026-10-04 09:30:00.",
		content)
}

// TestAlertLocaleResolution 收件人语言解析（SQLite 内存库，白盒）：
// INTERNAL 按用户 ID 读 locale、EMAIL 按邮箱反查、target 形态不符/查无此人/
// WEBHOOK/未注入 GORM 一律回落中文。
func TestAlertLocaleResolution(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	ctx := storetest.NewSystemContext(context.Background())
	client := dbClient.Client()

	require.NoError(t, client.User.Create().
		SetUsername("alert_locale_en").
		SetEmail("en-ops@example.com").
		SetLocale("en-US").
		Exec(ctx))
	require.NoError(t, client.User.Create().
		SetUsername("alert_locale_zh").
		SetEmail("zh-ops@example.com").
		Exec(ctx))
	require.NoError(t, client.User.Create().
		SetUsername("alert_locale_zh_b").
		SetLocale("fr").
		Exec(ctx))

	svc := &MonitorAlertService{dbClient: dbClient, log: bLogger.NewHelper(bLogger.NopLogger())}

	// INTERNAL：用户 ID 命中偏好
	require.Equal(t, mailtext.LocaleEnUS, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("1"),
	}))
	// 用户无 locale / locale 未识别 → 回落
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("2"),
	}))
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("3"),
	}))
	// target 非用户 ID 形态 / 查无此人 → 回落
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("not-a-number"),
	}))
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("99999"),
	}))

	// EMAIL：按邮箱反查命中 / 未知邮箱回落
	require.Equal(t, mailtext.LocaleEnUS, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_EMAIL),
		Target:  trans.Ptr("en-ops@example.com"),
	}))
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_EMAIL),
		Target:  trans.Ptr("nobody@example.com"),
	}))

	// WEBHOOK：对端无用户身份
	require.Equal(t, mailtext.LocaleZhCN, svc.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_WEBHOOK),
		Target:  trans.Ptr("https://hooks.example.com/x"),
	}))

	// 旧测试装配未注入 GORM：回落而不 panic
	bare := &MonitorAlertService{log: bLogger.NewHelper(bLogger.NopLogger())}
	require.Equal(t, mailtext.LocaleZhCN, bare.alertLocale(ctx, &operatorV1.MonitorAlertRule{
		Channel: trans.Ptr(operatorV1.Channel_INTERNAL),
		Target:  trans.Ptr("1"),
	}))
}
