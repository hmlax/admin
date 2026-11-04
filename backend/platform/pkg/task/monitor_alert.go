package task

// MonitorAlertScanTaskType 是监控告警扫描任务的类型常量。
// 系统级常驻任务（不写 sys_tasks 表），handler 属监控告警域（MonitorAlertService）。
const MonitorAlertScanTaskType = "monitor_alert_scan"

// MonitorAlertScanCronSpec 每 5 分钟评估一轮全部启用的告警规则。
const MonitorAlertScanCronSpec = "*/5 * * * *"

// MonitorAlertScanTaskData 是监控告警扫描任务的载荷（当前无参数，留空占位）。
type MonitorAlertScanTaskData struct{}
