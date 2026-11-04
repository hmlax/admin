package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go-admin/operator/internal/data"
	"go-admin/platform/pkg/mailtext"
)

// 文案表两语言都必须完整、且真实分叉（防"翻译"只是复制中文）；未识别语言与
// 未标注一样回落中文，标签按归一化命中（归一是 LocaleOfTag 的职责，投递路径
// 先归一再查表）。
func TestWatermarkCopyTableBothLocales(t *testing.T) {
	zh := watermarkCopyOf(mailtext.LocaleZhCN)
	en := watermarkCopyOf(mailtext.LocaleEnUS)
	for _, cp := range []watermarkCopy{zh, en} {
		for _, f := range []string{
			cp.title, cp.tenantLine, cp.quotaHead, cp.quotaLine, cp.footer,
		} {
			require.NotEmpty(t, f)
		}
	}
	require.NotEqual(t, zh, en)
	require.Equal(t, zh, watermarkCopyOf("zh-CN"))
	require.Equal(t, en, watermarkCopyOf("en-US"))
	require.Equal(t, zh, watermarkCopyOf("xx"))
	require.Equal(t, zh, watermarkCopyOf("zh"))
	require.Equal(t, zh, watermarkCopyOf("en"))
}

func TestWatermarkTitleLocalized(t *testing.T) {
	require.Equal(t, "套餐配额水位告警（甲租户）", watermarkTitle(mailtext.LocaleZhCN, "甲租户"))
	require.Equal(t, "Plan Quota Watermark Alert (tenantA)", watermarkTitle(mailtext.LocaleEnUS, "tenantA"))
}

func TestWatermarkBodyLocalized(t *testing.T) {
	hits := []data.QuotaWatermarkHit{
		{QuotaType: "API_CALL", Used: 9, Limit: 10, RatioPct: 90},
		{QuotaType: "STORAGE", Used: 900, Limit: 1000, RatioPct: 90},
	}
	require.Equal(t,
		"租户：甲租户\n以下配额的用量已达到 80% 水位：\n  - API_CALL：已用 9 / 上限 10（90%）\n  - STORAGE：已用 900 / 上限 1000（90%）\n请尽快检查用量并与平台管理员联系调整套餐配额；用量到达上限后，相关操作将被拒绝。\n",
		watermarkBody(mailtext.LocaleZhCN, "甲租户", hits))
	require.Equal(t,
		"Tenant: tenantA\nThe usage of the following quotas has reached the 80% watermark:\n  - API_CALL: used 9 / limit 10 (90%)\n  - STORAGE: used 900 / limit 1000 (90%)\nPlease review your usage soon and contact the platform administrator to adjust the plan quotas; once a limit is reached, the related operations will be rejected.\n",
		watermarkBody(mailtext.LocaleEnUS, "tenantA", hits))
}
