package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
	operatorV1 "go-admin/platform/pb/operator"
)

// TestServiceTagToBusinessModuleExactMapping 钉死服务 tag → 业务模块映射表的
// 全量内容。该表驱动 sys_apis.business_module 回填，是租户套餐模块白名单
// （模块白名单过滤）的判定来源：任何一个条目漂移都会导致对应服务的 API
// 在租户侧被误放行或误拒绝，因此任何改动都必须是有意识的、显式更新本测试的改动。
func TestServiceTagToBusinessModuleExactMapping(t *testing.T) {
	expected := map[string]operatorV1.Module_Module{
		"AdminPortalService":    operatorV1.Module_DASHBOARD,
		"DashboardService":      operatorV1.Module_DASHBOARD,
		"AuthenticationService": operatorV1.Module_DASHBOARD,

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
	}
	assert.Equal(t, expected, ServiceTagToBusinessModule,
		"ServiceTagToBusinessModule 内容漂移；如为有意变更请同步更新本测试")
}

// TestServiceTagToBusinessModuleReverseMapping 双向一致性：由正向映射推导
// 反向映射（模块 → 服务 tag 集合），必须与预期完全一致。防止只看正向表时
// 某模块的登记被悄悄增删。
func TestServiceTagToBusinessModuleReverseMapping(t *testing.T) {
	expected := map[operatorV1.Module_Module][]string{
		operatorV1.Module_DASHBOARD:        {"AdminPortalService", "DashboardService", "AuthenticationService", "BiDashboardService"},
		operatorV1.Module_OPM:              {"UserService", "OrgUnitService", "PositionService", "UserProfileService", "RoleService", "MfaService"},
		operatorV1.Module_PERMISSION:       {"MenuService", "ApiService", "PermissionService", "PermissionGroupService"},
		operatorV1.Module_DICT:             {"DictTypeService", "DictEntryService"},
		operatorV1.Module_SYSTEM:           {"LanguageService", "LoginPolicyService", "ConfigService", "AccessKeyService", "ServerMonitorService", "NotificationChannelService", "NotificationService", "NotificationRuleService", "NotificationTemplateService", "MonitorAlertService", "OnlineSessionService", "PrintTemplateService", "ScriptService", "ScriptLogService"},
		operatorV1.Module_FILE:             {"FileService", "FileTransferService", "MediaService"},
		operatorV1.Module_TASK:             {"TaskService", "TaskMonitorService"},
		operatorV1.Module_TENANT:           {"TenantService", "PlanService", "PlanQuotaService", "MyTenantUsageService", "PlanModuleService"},
		operatorV1.Module_LOG:              {"ApiAuditLogService", "LoginAuditLogService", "OperationAuditLogService", "DataAccessAuditLogService", "PermissionAuditLogService", "PolicyEvaluationLogService", "RedisCacheMonitorService"},
		operatorV1.Module_INTERNAL_MESSAGE: {"InternalMessageService", "InternalMessageCategoryService", "InternalMessageRecipientService", "NotificationPreferenceService"},
		operatorV1.Module_AI:               {"AiProviderService", "AiConversationService", "AiMessageService", "AiChatService", "AiUsageLogService", "AiKnowledgeBaseService", "AiQueryService", "AiAgentService", "AiContentService"},
	}

	actual := make(map[operatorV1.Module_Module][]string)
	for tag, module := range ServiceTagToBusinessModule {
		actual[module] = append(actual[module], tag)
	}
	assert.Len(t, actual, len(expected), "反向映射的模块数量与预期不符")
	for module, expectedTags := range expected {
		assert.Contains(t, actual, module, "模块 %v 应有登记的服务", module)
		assert.ElementsMatch(t, expectedTags, actual[module],
			"模块 %v 登记的服务集合与预期不符", module)
	}
}

// TestServiceTagToBusinessModuleValuesValid 映射值合法性：每个 tag 必须映射到
// 一个有定义且非 UNSPECIFIED 的模块。UNSPECIFIED 会让该服务的全部 API 被
// 租户白名单当作"不在任何白名单内"而拒绝（fail-closed），等于整块业务
// 对租户不可用——必须显式登记。
func TestServiceTagToBusinessModuleValuesValid(t *testing.T) {
	for tag, module := range ServiceTagToBusinessModule {
		assert.NotEmpty(t, tag, "映射键不允许为空串")
		name, defined := operatorV1.Module_Module_name[int32(module)]
		assert.True(t, defined, "tag %q 映射到未定义模块值 %d", tag, module)
		assert.NotEqual(t, "MODULE_UNSPECIFIED", name,
			"tag %q 映射到 UNSPECIFIED（等于未登记，租户侧将被 fail-closed 拒绝）", tag)
	}
}

// TestServiceTagToBusinessModuleCoversAllDefinedModules 覆盖完备性：每个已定义的
// 非 UNSPECIFIED 模块都必须有至少一个服务 tag 登记。模块枚举新增值而映射表
// 未同步登记，是租户白名单误伤整模块的典型来源，此测试让它当场暴露。
func TestServiceTagToBusinessModuleCoversAllDefinedModules(t *testing.T) {
	registered := make(map[operatorV1.Module_Module]bool)
	for _, module := range ServiceTagToBusinessModule {
		registered[module] = true
	}
	for value, name := range operatorV1.Module_Module_name {
		if name == "MODULE_UNSPECIFIED" {
			continue
		}
		assert.True(t, registered[operatorV1.Module_Module(value)],
			"模块 %s(%d) 已在枚举中定义但映射表无任何服务登记；若为新增模块请在 ServiceTagToBusinessModule 显式登记各服务", name, value)
	}
}
