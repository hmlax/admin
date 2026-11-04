package runtimeconfig

// Settings holds application options supplied by Nacos server.yaml/data.yaml.
// Bootstrap's protobuf owns database, Redis, transport, JWT, OSS and logger fields.
type Settings struct {
	CryptoKey                        string      `yaml:"crypto_key"`
	InstanceID                       string      `yaml:"instance_id"`
	OperatorEndpoint                 string      `yaml:"operator_endpoint"`
	HTTPAdvertiseAddr                string      `yaml:"http_advertise_addr"`
	SSEAdvertiseAddr                 string      `yaml:"sse_advertise_addr"`
	GRPCAdvertiseAddr                string      `yaml:"grpc_advertise_addr"`
	AuditArchiveDir                  string      `yaml:"audit_archive_dir"`
	AuditRetentionDays               int         `yaml:"audit_retention_days"`
	BackupRetentionDays              *int        `yaml:"backup_retention_days"`
	NotificationDeliveryStaleMinutes int         `yaml:"notification_delivery_stale_minutes"`
	AllowPrivateWebhook              bool        `yaml:"allow_private_webhook"`
	ChromePath                       string      `yaml:"chrome_path"`
	PrintNoSandbox                   *bool       `yaml:"print_no_sandbox"`
	SSO                              SSOSettings `yaml:"sso"`
}

type SSOSettings struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
	Scopes       string `yaml:"scopes"`
	AutoCreate   bool   `yaml:"auto_create"`
	TenantCode   string `yaml:"tenant_code"`
}
