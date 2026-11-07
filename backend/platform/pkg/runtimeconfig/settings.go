package runtimeconfig

// Settings holds application options supplied by Nacos server.yaml/data.yaml.
// Bootstrap's protobuf owns database, Redis, transport, JWT, OSS and logger fields.
type Settings struct {
	SMTP              SMTPSettings `yaml:"smtp"`
	SMS               SMSSettings  `yaml:"sms"`
	CryptoKey         string       `yaml:"crypto_key"`
	InstanceID        string       `yaml:"instance_id"`
	OperatorEndpoint  string       `yaml:"operator_endpoint"`
	HTTPAdvertiseAddr string       `yaml:"http_advertise_addr"`
	GRPCAdvertiseAddr string       `yaml:"grpc_advertise_addr"`
	SSO               SSOSettings  `yaml:"sso"`
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

// SMTPSettings configures authentication email delivery for this deployment.
type SMTPSettings struct {
	Host     string `yaml:"host"`
	Port     uint32 `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	FromName string `yaml:"from_name"`
	TLSMode  string `yaml:"tls_mode"`
}

// SMSSettings configures authentication SMS delivery using a provider template.
type SMSSettings struct {
	Provider     string `yaml:"provider"`
	AccessKeyID  string `yaml:"access_key_id"`
	AccessSecret string `yaml:"access_secret"`
	SignName     string `yaml:"sign_name"`
	TemplateCode string `yaml:"template_code"`
	AppID        string `yaml:"app_id"`
	Region       string `yaml:"region"`
}
