package constants

import operatorV1 "go-admin/platform/pb/operator"

// ServiceTagToBusinessModule 将 OpenAPI tag 名（gRPC 服务名）映射到业务功能模块。
// 用于 sys_apis.business_module 的回填，供套餐模块白名单过滤使用。
// 新增服务时必须在此登记，否则其 API 的 business_module 将为 UNSPECIFIED，
// 租户白名单过滤会把 UNSPECIFIED 视为"不在任何白名单内"而拒绝访问。
//
// 反过来说：登记成哪个模块，就等于把该服务的全部端点交给那个模块的白名单。
// 通知域三个服务登记为 SYSTEM（与套餐白名单联动），而它们管的是**平台全局**配置与台账，
// 因此租户侧的拒绝由 service 层的 requirePlatformAdmin 显式承担
// （operator/internal/service/notification_platform_guard.go），不在这里做。
var ServiceTagToBusinessModule = map[string]operatorV1.Module_Module{
	"AdminPortalService": operatorV1.Module_DASHBOARD,
	"DashboardService":   operatorV1.Module_DASHBOARD,

	"UserService":          operatorV1.Module_OPM,
	"OrgUnitService":       operatorV1.Module_OPM,
	"PositionService":      operatorV1.Module_OPM,
	"UserProfileService":   operatorV1.Module_OPM,
	"RoleService":          operatorV1.Module_OPM,
	"MfaService":           operatorV1.Module_OPM,
	"PrintTemplateService": operatorV1.Module_SYSTEM,
	"ScriptService":        operatorV1.Module_SYSTEM,
	"ScriptLogService":     operatorV1.Module_SYSTEM,
	"MediaService":         operatorV1.Module_FILE,
	"BiDashboardService":   operatorV1.Module_DASHBOARD,
	"AiAgentService":       operatorV1.Module_AI,
	"AiContentService":     operatorV1.Module_AI,
	"PlanModuleService":    operatorV1.Module_TENANT,

	"MenuService":            operatorV1.Module_PERMISSION,
	"ApiService":             operatorV1.Module_PERMISSION,
	"PermissionService":      operatorV1.Module_PERMISSION,
	"PermissionGroupService": operatorV1.Module_PERMISSION,

	"DictTypeService":      operatorV1.Module_DICT,
	"DictEntryService":     operatorV1.Module_DICT,
	"LanguageService":      operatorV1.Module_SYSTEM,
	"FileService":          operatorV1.Module_FILE,
	"FileTransferService":  operatorV1.Module_FILE,
	"TaskService":          operatorV1.Module_TASK,
	"TaskMonitorService":   operatorV1.Module_TASK,
	"MyTenantUsageService": operatorV1.Module_TENANT,
	"LoginPolicyService":   operatorV1.Module_SYSTEM,
	"ConfigService":        operatorV1.Module_SYSTEM,
	"AccessKeyService":     operatorV1.Module_SYSTEM,

	"AiProviderService":      operatorV1.Module_AI,
	"AiConversationService":  operatorV1.Module_AI,
	"AiMessageService":       operatorV1.Module_AI,
	"AiChatService":          operatorV1.Module_AI,
	"AiUsageLogService":      operatorV1.Module_AI,
	"AiKnowledgeBaseService": operatorV1.Module_AI,
	"AiQueryService":         operatorV1.Module_AI,

	"TenantService":    operatorV1.Module_TENANT,
	"PlanService":      operatorV1.Module_TENANT,
	"PlanQuotaService": operatorV1.Module_TENANT,

	"ApiAuditLogService":          operatorV1.Module_LOG,
	"LoginAuditLogService":        operatorV1.Module_LOG,
	"OperationAuditLogService":    operatorV1.Module_LOG,
	"DataAccessAuditLogService":   operatorV1.Module_LOG,
	"PermissionAuditLogService":   operatorV1.Module_LOG,
	"PolicyEvaluationLogService":  operatorV1.Module_LOG,
	"RedisCacheMonitorService":    operatorV1.Module_LOG,
	"ServerMonitorService":        operatorV1.Module_SYSTEM,
	"MonitorAlertService":         operatorV1.Module_SYSTEM,
	"NotificationChannelService":  operatorV1.Module_SYSTEM,
	"NotificationService":         operatorV1.Module_SYSTEM,
	"NotificationRuleService":     operatorV1.Module_SYSTEM,
	"NotificationTemplateService": operatorV1.Module_SYSTEM,
	"OnlineSessionService":        operatorV1.Module_SYSTEM,

	"InternalMessageService":          operatorV1.Module_INTERNAL_MESSAGE,
	"InternalMessageCategoryService":  operatorV1.Module_INTERNAL_MESSAGE,
	"InternalMessageRecipientService": operatorV1.Module_INTERNAL_MESSAGE,
	"NotificationPreferenceService":   operatorV1.Module_INTERNAL_MESSAGE,

	"AuthenticationService": operatorV1.Module_DASHBOARD,
}
