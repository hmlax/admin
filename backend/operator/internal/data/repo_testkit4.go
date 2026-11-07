// Repository constructors for service tests. Keep fields aligned with production constructors.
package data

import (
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"github.com/tx7do/go-utils/mapper"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/accesskey"
	operatorV1 "go-admin/platform/pb/operator"
)

// NewAccessKeyRepoForTest 与生产 NewAccessKeyRepo 逐字段一致（log 换 NopLogger），并调用 init()。
func NewAccessKeyRepoForTest(dbClient *store.DBClient) *AccessKeyRepo {
	repo := &AccessKeyRepo{
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		dbClient: dbClient,
		mapper:   mapper.NewCopierMapper[operatorV1.AccessKey, store.AccessKey](),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.AccessKey_Status, accesskey.Status](
			operatorV1.AccessKey_Status_name,
			operatorV1.AccessKey_Status_value,
		),
	}

	repo.init()

	return repo
}
