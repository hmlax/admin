package serviceid

const (
	AdminService    = "admin"    // Hertz HTTP API
	OperatorService = "operator" // Kratos gRPC business service
)

// NewDiscoveryName 构建服务发现名称
func NewDiscoveryName(serviceName string) string {
	return serviceName
}
