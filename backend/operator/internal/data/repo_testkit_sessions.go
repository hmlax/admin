package data

import (
	"github.com/tx7do/go-utils/mapper"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"

	"go-admin/operator/internal/data/store"
	"go-admin/operator/internal/data/store/userrole"
	operatorV1 "go-admin/platform/pb/operator"
)

// NewUserRoleRepoForTest uses the production converters with a quiet test logger.
func NewUserRoleRepoForTest(client *store.DBClient) *UserRoleRepo {
	return &UserRoleRepo{
		dbClient: client,
		log:      bLogger.NewHelper(bLogger.NopLogger()),
		statusConverter: mapper.NewEnumTypeConverter[operatorV1.UserRole_Status, userrole.Status](
			operatorV1.UserRole_Status_name,
			operatorV1.UserRole_Status_value,
		),
	}
}
