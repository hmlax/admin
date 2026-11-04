package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"
)

type fakeBackupCleaner struct {
	metas   []backupObjectMeta
	removed []string
	listErr error
	delErr  map[string]error
}

func (f *fakeBackupCleaner) listObjects(_ context.Context, _, _ string) ([]backupObjectMeta, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.metas, nil
}

func (f *fakeBackupCleaner) removeObject(_ context.Context, _, key string) error {
	if f.delErr != nil {
		if err, ok := f.delErr[key]; ok {
			return err
		}
	}
	f.removed = append(f.removed, key)
	return nil
}

func TestExpiredBackupKeys(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.Local)
	metas := []backupObjectMeta{
		{
			Key:          "2026/09/01/old.json.gz",
			LastModified: now.AddDate(0, 0, -40),
		},
		{
			Key:          "2026/10/02/fresh.json.gz",
			LastModified: now.AddDate(0, 0, -1),
		},
		{
			Key:          "2026/09/03/boundary.json.gz",
			LastModified: now.Add(-30 * 24 * time.Hour),
		},
	}

	// cutoff = now - 30d；恰好等于 cutoff 的不算过期（Before 语义）
	expired := expiredBackupKeys(metas, now.AddDate(0, 0, -30))
	require.Equal(t, []string{"2026/09/01/old.json.gz"}, expired, "只挑出严格早于 cutoff 的对象")
}

func TestCleanupOldBackups(t *testing.T) {
	now := time.Now()
	cleaner := &fakeBackupCleaner{
		metas: []backupObjectMeta{
			{
				Key:          "2026/09/01/old.json.gz",
				LastModified: now.AddDate(0, 0, -40),
			},
			{
				Key:          "2026/10/02/fresh.json.gz",
				LastModified: now.AddDate(0, 0, -1),
			},
			{
				Key:          "2026/08/01/older.json.gz",
				LastModified: now.AddDate(0, 0, -60),
			},
		},
	}

	svc := &TaskService{log: bLogger.NewHelper(bLogger.NopLogger())}
	svc.cleanupOldBackups(context.Background(), cleaner)

	require.ElementsMatch(t,
		[]string{"2026/09/01/old.json.gz", "2026/08/01/older.json.gz"},
		cleaner.removed,
		"过期对象应被删除，未过期保留")
}

func TestCleanupOldBackupsDisabled(t *testing.T) {

	now := time.Now()
	cleaner := &fakeBackupCleaner{
		metas: []backupObjectMeta{
			{
				Key:          "2026/09/01/old.json.gz",
				LastModified: now.AddDate(0, 0, -400),
			},
		},
	}

	svc := &TaskService{log: bLogger.NewHelper(bLogger.NopLogger())}
	days := 0
	svc.settings.BackupRetentionDays = &days
	svc.cleanupOldBackups(context.Background(), cleaner)

	require.Empty(t, cleaner.removed, "保留期 <=0 应关闭清理")
}

func TestCleanupOldBackupsListError(t *testing.T) {
	cleaner := &fakeBackupCleaner{listErr: errors.New("minio unreachable")}

	svc := &TaskService{log: bLogger.NewHelper(bLogger.NopLogger())}
	svc.cleanupOldBackups(context.Background(), cleaner)

	require.Empty(t, cleaner.removed, "列举失败不应触发任何删除")
}

func TestCleanupOldBackupsPartialDeleteFailure(t *testing.T) {
	now := time.Now()
	cleaner := &fakeBackupCleaner{
		metas: []backupObjectMeta{
			{
				Key:          "a.json.gz",
				LastModified: now.AddDate(0, 0, -40),
			},
			{
				Key:          "b.json.gz",
				LastModified: now.AddDate(0, 0, -50),
			},
		},
		delErr: map[string]error{"a.json.gz": errors.New("access denied")},
	}

	svc := &TaskService{log: bLogger.NewHelper(bLogger.NopLogger())}
	svc.cleanupOldBackups(context.Background(), cleaner)

	require.Equal(t, []string{"b.json.gz"}, cleaner.removed,
		"单个对象删除失败应留日志并继续删其余（best-effort，不影响备份结论）")
}
