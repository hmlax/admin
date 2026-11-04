package service

import (
	"context"
	"fmt"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"go-admin/operator/internal/data"
	"go-admin/operator/internal/data/channel"
	"go-admin/operator/internal/data/store"
	"go-admin/platform/pkg/oss"
	"go-admin/platform/pkg/runtimeconfig"
)

// ProviderSet supplies business services and their application lifecycle hooks.
var ProviderSet = wire.NewSet(
	NewMyTenantUsageService,
	NewNotificationTemplateService,
	NewAuditExportService,
	NewDataExportService,
	NewTaskMonitorService,
	NewAuthenticationService,
	NewMfaService,
	NewLoginPolicyService,
	NewUserService,
	NewUserProfileService,
	NewPositionService,
	NewOrgUnitService,
	NewRoleService,
	NewMenuService,
	NewApiService,
	NewPermissionService,
	NewPermissionGroupService,
	NewTenantService,
	NewPlanService,
	NewPlanQuotaService,
	NewPlanModuleService,
	NewDictTypeService,
	NewDictEntryService,
	NewLanguageService,
	NewFileService,
	NewFileTransferService,
	NewTaskService,
	NewLoginAuditLogService,
	NewApiAuditLogService,
	NewOperationAuditLogService,
	NewDataAccessAuditLogService,
	NewPermissionAuditLogService,
	NewPolicyEvaluationLogService,
	NewRedisCacheMonitorService,
	NewServerMonitorService,
	NewNotificationChannelService,
	NewOnlineSessionService,
	NewAdminPortalService,
	NewInternalMessageService,
	NewInternalMessageCategoryService,
	NewInternalMessageRecipientService,
	NewNotificationPreferenceService,
	NewScriptService,
	NewScriptLogService,
	NewDashboardService,
	NewAiDigestService,
	NewPlanQuotaWatermarkService,
	NewAiQueryService,
	NewAiContentService,
	NewBiDashboardService,
	NewPrintTemplateService,
	NewNotificationRuleService,
	NewAccessKeyService,
	NewConfigService,
	NewAiProviderService,
	NewAiConversationService,
	NewAiMessageService,
	NewAiUsageLogService,
	NewAiKnowledgeService,
	NewAiChatService,
	NewAiAgentService,
	ProvideNotificationService,
	ProvideMonitorAlertService,
	ProvideScriptRuntime,
	ProvideChannelRegistry,
	wire.Bind(new(Notifier), new(*NotificationService)),
	wire.Struct(new(TransactionMailer), "TemplateRepo"),
	// register:service
)

// ProvideChannelRegistry registers all configured delivery adapters, including
// the live message service whose publisher is connected by the gateway provider.
func ProvideChannelRegistry(
	ctx *bootstrap.Context,
	repo *data.NotificationChannelRepo,
	messages *InternalMessageService,
) *channel.Registry {
	registry := channel.NewRegistry()
	registry.Register(channel.NewEmailSender(repo))
	registry.Register(channel.NewWebhookSender(repo, runtimeconfig.SettingsFromContext(ctx)))
	registry.Register(channel.NewSmsSender(repo))
	registry.Register(channel.NewSettingsSender(repo))
	registry.Register(NewInternalMessageSender(messages))
	return registry
}

// ProvideNotificationService completes the notification/message feedback edge
// after both constructors have run; neither constructor depends on that edge.
func ProvideNotificationService(
	ctx *bootstrap.Context,
	delivery *data.NotificationDeliveryRepo,
	rules *data.NotificationRuleRepo,
	templates *data.NotificationTemplateRepo,
	registry *channel.Registry,
	messages *InternalMessageService,
) *NotificationService {
	notifications := NewNotificationService(ctx, delivery, rules, templates, registry)
	messages.RegisterNotifier(notifications)
	return notifications
}

// ProvideMonitorAlertService connects the same notification service to alerts.
func ProvideMonitorAlertService(
	ctx *bootstrap.Context,
	rules *data.MonitorAlertRuleRepo,
	server *data.ServerMonitorRepo,
	redis *data.RedisCacheMonitorRepo,
	db *store.DBClient,
	notifier Notifier,
) *MonitorAlertService {
	alerts := NewMonitorAlertService(ctx, rules, server, redis, db)
	alerts.RegisterNotifier(notifier)
	return alerts
}

// ProvideScriptRuntime owns the engines and resync listener. Wire calls its
// cleanup before closing the database and Redis, including on injection errors.
func ProvideScriptRuntime(
	ctx *bootstrap.Context,
	scripts *data.ScriptRepo,
	redis *redis.Client,
	minio *oss.MinIOClient,
	logs *data.ScriptLogRepo,
	providers *data.AiProviderRepo,
	usage *data.AiUsageLogRepo,
	db *store.DBClient,
) (*ScriptRuntime, func()) {
	runtime := NewScriptRuntime(ctx, scripts, redis, minio, logs, providers, usage)
	// Startup resync remains best-effort: a bad script is logged and must not
	// prevent the rest of the administration service from starting.
	if err := runtime.Resync(ctx.Context()); err != nil {
		ctx.GetLogger().Error(ctx.Context(), fmt.Sprintf("script runtime startup resync failed: %v", err))
	}
	runtime.StartResyncListener(ctx.Context())
	AttachEntityHooks(db.Client(),
		func(hookCtx context.Context, hookPoint string, payload map[string]any) {
			if err := runtime.InvokeEntityHook(hookPoint, payload); err != nil {
				ctx.GetLogger().Error(hookCtx, fmt.Sprintf("script entity hook %s failed: %v", hookPoint, err))
			}
		},
		func(_ context.Context, hookPoint string, payload map[string]any) error {
			return runtime.InvokeEntityHookVeto(hookPoint, payload)
		},
	)
	return runtime, func() {
		runtime.StopResyncListener()
		runtime.Close()
	}
}
