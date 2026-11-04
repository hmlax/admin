package server

import (
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/go-utils/authorizer"

	"go-admin/operator/internal/data"
	"go-admin/operator/internal/service"
)

// ProviderSet supplies the internal gRPC transport and background worker.
var ProviderSet = wire.NewSet(NewGatewayService, NewGRPCServer, NewAsynqServer)

// NewGatewayService connects service event publishers before transports start.
func NewGatewayService(
	policies authorizer.Provider,
	access *data.GatewayAccessRepo,
	apiAudit *data.ApiAuditLogRepo,
	loginAudit *data.LoginAuditLogRepo,
	operationAudit *data.OperationAuditLogRepo,
	permissionAudit *data.PermissionAuditLogRepo,
	dataAccessAudit *data.DataAccessAuditLogRepo,
	policyAudit *data.PolicyEvaluationLogRepo,
	auditExport *service.AuditExportService,
	dataExport *service.DataExportService,
	files *service.FileTransferService,
	redis *redis.Client,
	messages *service.InternalMessageService,
	chat *service.AiChatService,
	agents *service.AiAgentService,
) *GatewayService {
	gateway := &GatewayService{
		Policies:        policies,
		Access:          access,
		APIAudit:        apiAudit,
		LoginAudit:      loginAudit,
		OperationAudit:  operationAudit,
		PermissionAudit: permissionAudit,
		DataAccessAudit: dataAccessAudit,
		PolicyAudit:     policyAudit,
		AuditExport:     auditExport,
		DataExport:      dataExport,
		Files:           files,
		Redis:           redis,
	}
	messages.RegisterInternalMessagePublisher(gateway)
	chat.RegisterAiChatPublisher(gateway)
	agents.RegisterAiAgentRunPublisher(gateway)
	return gateway
}
