package task

// AiAuditDigestTaskType 是审计日报 AI 摘要任务的类型常量。
// 系统级常驻定时任务：每日早上汇总昨日操作审计（总数/失败/活跃用户/动作分布），
// 调默认模型生成中文摘要，以站内信投递给平台侧用户。
// handler 为 AiDigestService.AsyncAiAuditDigest。
const AiAuditDigestTaskType = "ai_audit_digest"

// AiAuditDigestCronSpec 审计日报的 cron（每天 08:00）。
const AiAuditDigestCronSpec = "0 8 * * *"

// AiAuditDigestTaskData 日报任务载荷（统计区间固定为昨日全天，无参数）。
type AiAuditDigestTaskData struct{}
