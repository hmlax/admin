package service

import (
	"testing"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/stretchr/testify/require"
)

func quotaUsages(pairs map[operatorV1.PlanQuota_QuotaType]uint64) []*operatorV1.QuotaUsage {
	out := make([]*operatorV1.QuotaUsage, 0, len(pairs))
	for t, v := range pairs {
		quotaType := t
		out = append(out, &operatorV1.QuotaUsage{QuotaType: quotaType, QuotaValue: v})
	}
	return out
}

// TestCheckUserQuotaWith USER_LIMIT 纯判定：满拒/未满放/未限售放。
func TestCheckUserQuotaWith(t *testing.T) {
	quotas := quotaUsages(map[operatorV1.PlanQuota_QuotaType]uint64{
		operatorV1.PlanQuota_USER_LIMIT: 10,
		operatorV1.PlanQuota_STORAGE:    1024,
	})

	// 已满（current >= limit）
	err := checkUserQuotaWith(quotas, 10)
	require.Error(t, err)
	require.Contains(t, err.Error(), "USER_LIMIT")

	// 未满
	require.NoError(t, checkUserQuotaWith(quotas, 9))

	// 无该类型配额条目 = 未限售
	require.NoError(t, checkUserQuotaWith(nil, 9999))
}

// TestCheckStorageQuotaWith STORAGE 纯判定：占用+本次上传>上限即超；恰好等于不超。
func TestCheckStorageQuotaWith(t *testing.T) {
	quotas := quotaUsages(map[operatorV1.PlanQuota_QuotaType]uint64{
		operatorV1.PlanQuota_STORAGE: 1000,
	})

	// 占用 900 + 上传 101 = 1001 > 1000 → 超
	err := checkStorageQuotaWith(quotas, 900, 101)
	require.Error(t, err)
	require.Contains(t, err.Error(), "STORAGE")

	// 占用 900 + 上传 100 = 1000 == 上限 → 放行（超过才拒）
	require.NoError(t, checkStorageQuotaWith(quotas, 900, 100))

	// 无配额条目 → 放行
	require.NoError(t, checkStorageQuotaWith(nil, 999999, 999999))
}
