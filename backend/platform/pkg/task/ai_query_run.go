package task

// AiQueryRunTaskType 是定时问数任务的类型常量。
// 用户经「任务管理」创建（cron + payload.question），系统周期执行问数并将
// 结果以站内信推送给任务创建人。适合定期监控类查询（如"每天早上看昨日失败登录"）。
const AiQueryRunTaskType = "ai_query_run"

// AiQueryRunTaskData 定时问数任务载荷。
type AiQueryRunTaskData struct {
	Question string `json:"question"` // 自然语言问题
	Lang     string `json:"lang"`     // 输出语言（默认 zh-CN）
}
