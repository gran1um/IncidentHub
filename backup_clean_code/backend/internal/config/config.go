package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

const TenantHeader = "X-Tenant-ID"

type LoggerConfig struct {
	Group  string `envconfig:"LOGGER_GROUP" default:"soc_dev"`
	System string `envconfig:"LOGGER_SYSTEM" default:"incidenthub"`
	Level  string `envconfig:"LOG_LEVEL" default:"info"`
}

type TraceConfig struct {
	Enabled    bool   `envconfig:"TRACE_ENABLED" default:"true"`
	Tenant     string `envconfig:"TRACE_TENANT" default:"soc-ir"`
	ServiceKey string `envconfig:"TRACE_SERVICE_KEY" default:"incidenthub"`
}

type HTTPConfig struct {
	Addr             string        `envconfig:"HTTP_ADDR" default:":8080"`
	ReadTimeout      time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"10s"`
	WriteTimeout     time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"60s"`
	ShutdownTimeout  time.Duration `envconfig:"HTTP_SHUTDOWN_TIMEOUT" default:"15s"`
	CORSAllowOrigins string        `envconfig:"HTTP_CORS_ALLOW_ORIGINS" default:"http://localhost:5173"`
	SecureCookies    bool          `envconfig:"HTTP_SECURE_COOKIES" default:"false"`
	ExposeSwagger    bool          `envconfig:"HTTP_EXPOSE_SWAGGER" default:"false"`
}

type AuthConfig struct {
	JWTSecret  string        `envconfig:"AUTH_JWT_SECRET" default:"change-me-please"`
	Issuer     string        `envconfig:"AUTH_ISSUER" default:"incidenthub"`
	AccessTTL  time.Duration `envconfig:"AUTH_ACCESS_TTL" default:"15m"`
	RefreshTTL time.Duration `envconfig:"AUTH_REFRESH_TTL" default:"24h"`
	CookieName string        `envconfig:"AUTH_REFRESH_COOKIE_NAME" default:"ih_refresh_token"`
}

type SecurityConfig struct {
	PasswordMinLength int           `envconfig:"SEC_PASSWORD_MIN_LENGTH" default:"12"`
	LoginRateLimit    int           `envconfig:"SEC_LOGIN_RATE_LIMIT" default:"10"`
	LoginRateWindow   time.Duration `envconfig:"SEC_LOGIN_RATE_WINDOW" default:"5m"`
}

type ArtifactsConfig struct {
	MaxUploadMB int           `envconfig:"ARTIFACTS_MAX_UPLOAD_MB" default:"50"`
	PresignTTL  time.Duration `envconfig:"ARTIFACTS_PRESIGN_TTL" default:"15m"`
}

type PostgresConfig struct {
	Host     string `envconfig:"POSTGRES_HOST" default:"localhost"`
	Port     int    `envconfig:"POSTGRES_PORT" default:"5432"`
	User     string `envconfig:"POSTGRES_USER" default:"incidenthub"`
	Password string `envconfig:"POSTGRES_PASSWORD" default:"incidenthub"`
	DBName   string `envconfig:"POSTGRES_DB" default:"incidenthub"`
	SSLMode  string `envconfig:"POSTGRES_SSLMODE" default:"disable"`
	MaxConns int32  `envconfig:"POSTGRES_MAX_CONNS" default:"20"`
	MinConns int32  `envconfig:"POSTGRES_MIN_CONNS" default:"2"`
}

type RedisConfig struct {
	Addr     string `envconfig:"REDIS_ADDR" default:"localhost:6379"`
	Username string `envconfig:"REDIS_USERNAME" default:""`
	Password string `envconfig:"REDIS_PASSWORD" default:""`
	DB       int    `envconfig:"REDIS_DB" default:"0"`
}

type APICacheConfig struct {
	Enabled      bool          `envconfig:"API_CACHE_ENABLED" default:"true"`
	TTL          time.Duration `envconfig:"API_CACHE_TTL" default:"30s"`
	MaxBodyBytes int           `envconfig:"API_CACHE_MAX_BODY_BYTES" default:"1048576"`
}

type S3Config struct {
	Enabled          bool   `envconfig:"S3_ENABLED" default:"false"`
	Endpoint         string `envconfig:"S3_ENDPOINT" default:"http://localhost:9000"`
	PublicEndpoint   string `envconfig:"S3_PUBLIC_ENDPOINT" default:""`
	Region           string `envconfig:"S3_REGION" default:"us-east-1"`
	Bucket           string `envconfig:"S3_BUCKET" default:"incidenthub-artifacts"`
	AccessKey        string `envconfig:"S3_ACCESS_KEY" default:""`
	SecretKey        string `envconfig:"S3_SECRET_KEY" default:""`
	UseSSL           bool   `envconfig:"S3_USE_SSL" default:"false"`
	AutoCreateBucket bool   `envconfig:"S3_AUTO_CREATE_BUCKET" default:"true"`
}

type ElasticConfig struct {
	Enabled     bool   `envconfig:"ELASTIC_ENABLED" default:"true"`
	Addresses   string `envconfig:"ELASTIC_ADDRESSES" default:"http://localhost:9200"`
	Username    string `envconfig:"ELASTIC_USERNAME" default:"elastic"`
	Password    string `envconfig:"ELASTIC_PASSWORD" default:"elastic"`
	IndexPrefix string `envconfig:"ELASTIC_INDEX_PREFIX" default:"incidenthub"`
}

type LDAPConfig struct {
	Enabled      bool   `envconfig:"LDAP_ENABLED" default:"false"`
	URL          string `envconfig:"LDAP_URL" default:"ldap://localhost:389"`
	BaseDN       string `envconfig:"LDAP_BASE_DN" default:"dc=example,dc=org"`
	BindDN       string `envconfig:"LDAP_BIND_DN" default:""`
	BindPassword string `envconfig:"LDAP_BIND_PASSWORD" default:""`
	UserFilter   string `envconfig:"LDAP_USER_FILTER" default:"(uid=%s)"`
}

type BootstrapConfig struct {
	Enabled            bool   `envconfig:"BOOTSTRAP_ENABLED" default:"true"`
	SeedDemoData       bool   `envconfig:"BOOTSTRAP_SEED_DEMO_DATA" default:"false"`
	PlatformAdminEmail string `envconfig:"BOOTSTRAP_PLATFORM_ADMIN_EMAIL" default:"admin@incidenthub.local"`
	PlatformAdminUser  string `envconfig:"BOOTSTRAP_PLATFORM_ADMIN_USERNAME" default:"platform-admin"`
	PlatformAdminPass  string `envconfig:"BOOTSTRAP_PLATFORM_ADMIN_PASSWORD" default:"ChangeMeNow123!"`
	PlatformAdminName  string `envconfig:"BOOTSTRAP_PLATFORM_ADMIN_FULL_NAME" default:"Platform Administrator"`
}

type MetricsConfig struct {
	Namespace string `envconfig:"METRICS_NAMESPACE" default:"incidenthub"`
}

type InboundConnectorsConfig struct {
	Enabled      bool          `envconfig:"CONNECTORS_INBOUND_ENABLED" default:"true"`
	PollInterval time.Duration `envconfig:"CONNECTORS_INBOUND_POLL_INTERVAL" default:"5m"`
	HTTPTimeout  time.Duration `envconfig:"CONNECTORS_INBOUND_HTTP_TIMEOUT" default:"20s"`
	BatchLimit   int           `envconfig:"CONNECTORS_INBOUND_BATCH_LIMIT" default:"200"`
}

type OutboundConnectorsConfig struct {
	HTTPTimeout         time.Duration `envconfig:"CONNECTORS_OUTBOUND_HTTP_TIMEOUT" default:"15s"`
	TelegramAPIBaseURL  string        `envconfig:"CONNECTORS_OUTBOUND_TELEGRAM_API_BASE_URL" default:"https://api.telegram.org"`
	HubEnabled          bool          `envconfig:"CONNECTOR_HUB_ENABLED" default:"true"`
	HubPollInterval     time.Duration `envconfig:"CONNECTOR_HUB_POLL_INTERVAL" default:"3s"`
	HubBatchSize        int           `envconfig:"CONNECTOR_HUB_BATCH_SIZE" default:"10"`
	HubRunTimeout       time.Duration `envconfig:"CONNECTOR_HUB_RUN_TIMEOUT" default:"45s"`
	HubMaxAttempts      int           `envconfig:"CONNECTOR_HUB_MAX_ATTEMPTS" default:"3"`
	HubRetryBaseBackoff time.Duration `envconfig:"CONNECTOR_HUB_RETRY_BASE_BACKOFF" default:"2s"`
	HubRetryMaxBackoff  time.Duration `envconfig:"CONNECTOR_HUB_RETRY_MAX_BACKOFF" default:"30s"`
}

type AIConfig struct {
	Enabled         bool          `envconfig:"AI_ENABLED" default:"true"`
	Provider        string        `envconfig:"AI_PROVIDER" default:"ollama"`
	Endpoint        string        `envconfig:"AI_ENDPOINT" default:"http://localhost:11434"`
	APIKey          string        `envconfig:"AI_API_KEY" default:""`
	OpenAIAPIKey    string        `envconfig:"OPENAI_API_KEY" default:""`
	OpenAIBaseURL   string        `envconfig:"OPENAI_BASE_URL" default:"https://api.openai.com/v1"`
	Model           string        `envconfig:"AI_MODEL" default:"llama3.2:3b"`
	Timeout         time.Duration `envconfig:"AI_TIMEOUT" default:"60s"`
	TopK            int           `envconfig:"AI_TOP_K" default:"6"`
	MaxContextChars int           `envconfig:"AI_MAX_CONTEXT_CHARS" default:"5000"`
	HistoryMessages int           `envconfig:"AI_HISTORY_MESSAGES" default:"20"`
	MCPEnabled      bool          `envconfig:"AI_MCP_ENABLED" default:"true"`
	MCPTools        string        `envconfig:"AI_MCP_TOOLS_ALLOWLIST" default:"open_case_statuses,cases_in_work,recent_cases,recent_alerts,dashboard_stats"`
	MCPPerToolLimit int           `envconfig:"AI_MCP_PER_TOOL_LIMIT" default:"5"`
	SystemPrompt    string        `envconfig:"AI_SYSTEM_PROMPT" default:"You are a SOC assistant for security incident response. Provide concise, actionable analysis based on the tenant data."`
	MaxConcurrent   int           `envconfig:"AI_MAX_CONCURRENT_SESSIONS" default:"2"`
	QueueEnabled    bool          `envconfig:"AI_AGENT_QUEUE_ENABLED" default:"true"`
	QueuePoll       time.Duration `envconfig:"AI_AGENT_QUEUE_POLL_INTERVAL" default:"3s"`
	QueueBatchSize  int           `envconfig:"AI_AGENT_QUEUE_BATCH_SIZE" default:"20"`
	QueueRunTimeout time.Duration `envconfig:"AI_AGENT_QUEUE_RUN_TIMEOUT" default:"90s"`
	QueueMaxRetries int           `envconfig:"AI_AGENT_QUEUE_MAX_RETRIES" default:"3"`
}

type AsyncOpsConfig struct {
	Enabled          bool          `envconfig:"ASYNC_OPS_ENABLED" default:"false"`
	KafkaBrokers     string        `envconfig:"ASYNC_OPS_KAFKA_BROKERS" default:"localhost:9092"`
	KafkaTopic       string        `envconfig:"ASYNC_OPS_KAFKA_TOPIC" default:"incidenthub.async.ops"`
	KafkaDLQTopic    string        `envconfig:"ASYNC_OPS_KAFKA_DLQ_TOPIC" default:"incidenthub.async.ops.dlq"`
	KafkaGroupID     string        `envconfig:"ASYNC_OPS_KAFKA_GROUP_ID" default:"incidenthub-api-async-ops"`
	ProduceTimeout   time.Duration `envconfig:"ASYNC_OPS_PRODUCE_TIMEOUT" default:"5s"`
	PollTimeout      time.Duration `envconfig:"ASYNC_OPS_POLL_TIMEOUT" default:"1s"`
	RetryMaxAttempts int           `envconfig:"ASYNC_OPS_RETRY_MAX_ATTEMPTS" default:"5"`
	RetryBaseBackoff time.Duration `envconfig:"ASYNC_OPS_RETRY_BASE_BACKOFF" default:"500ms"`
	RetryMaxBackoff  time.Duration `envconfig:"ASYNC_OPS_RETRY_MAX_BACKOFF" default:"15s"`
}

type NotificationDeliveryConfig struct {
	Enabled          bool          `envconfig:"NOTIFICATIONS_KAFKA_ENABLED" default:"true"`
	KafkaBrokers     string        `envconfig:"NOTIFICATIONS_KAFKA_BROKERS" default:"localhost:9092"`
	KafkaTopic       string        `envconfig:"NOTIFICATIONS_KAFKA_TOPIC" default:"incidenthub.notifications"`
	KafkaDLQTopic    string        `envconfig:"NOTIFICATIONS_KAFKA_DLQ_TOPIC" default:"incidenthub.notifications.dlq"`
	KafkaGroupID     string        `envconfig:"NOTIFICATIONS_KAFKA_GROUP_ID" default:"incidenthub-api-notifications"`
	ProduceTimeout   time.Duration `envconfig:"NOTIFICATIONS_PRODUCE_TIMEOUT" default:"5s"`
	PollTimeout      time.Duration `envconfig:"NOTIFICATIONS_POLL_TIMEOUT" default:"1s"`
	RetryMaxAttempts int           `envconfig:"NOTIFICATIONS_RETRY_MAX_ATTEMPTS" default:"5"`
	RetryBaseBackoff time.Duration `envconfig:"NOTIFICATIONS_RETRY_BASE_BACKOFF" default:"500ms"`
	RetryMaxBackoff  time.Duration `envconfig:"NOTIFICATIONS_RETRY_MAX_BACKOFF" default:"15s"`
}

type ServiceAlertingConfig struct {
	Enabled            bool          `envconfig:"SERVICE_ALERTING_ENABLED" default:"true"`
	EvaluationInterval time.Duration `envconfig:"SERVICE_ALERTING_EVALUATION_INTERVAL" default:"30s"`
	DefaultWindow      time.Duration `envconfig:"SERVICE_ALERTING_DEFAULT_WINDOW" default:"5m"`
	DefaultCooldown    time.Duration `envconfig:"SERVICE_ALERTING_DEFAULT_COOLDOWN" default:"15m"`
	RulesLimit         int           `envconfig:"SERVICE_ALERTING_RULES_LIMIT" default:"500"`
	RecipientsLimit    int           `envconfig:"SERVICE_ALERTING_RECIPIENTS_LIMIT" default:"300"`
}

type WorkflowConfig struct {
	Engine   string `envconfig:"WORKFLOW_ENGINE" default:"internal"`
	VaultKey string `envconfig:"WORKFLOW_VAULT_KEY" default:""`
}

type App struct {
	Env         string `envconfig:"ENV" default:"dev"`
	ServiceName string `envconfig:"SERVICE_NAME" default:"incidenthub-api"`

	Logger        LoggerConfig
	Trace         TraceConfig
	HTTP          HTTPConfig
	Auth          AuthConfig
	Security      SecurityConfig
	Artifacts     ArtifactsConfig
	Postgres      PostgresConfig
	Redis         RedisConfig
	APICache      APICacheConfig
	S3            S3Config
	Elastic       ElasticConfig
	LDAP          LDAPConfig
	Bootstrap     BootstrapConfig
	Metrics       MetricsConfig
	Inbound       InboundConnectorsConfig
	Outbound      OutboundConnectorsConfig
	AI            AIConfig
	AsyncOps      AsyncOpsConfig
	Notifications NotificationDeliveryConfig
	ServiceAlerts ServiceAlertingConfig
	Workflow      WorkflowConfig
}

func Load() App {
	_ = godotenv.Load()

	var cfg App
	envconfig.MustProcess("", &cfg)

	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		panic("AUTH_JWT_SECRET must not be empty")
	}
	return cfg
}

func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User,
		p.Password,
		p.Host,
		p.Port,
		p.DBName,
		p.SSLMode,
	)
}

func (e ElasticConfig) AddressList() []string {
	parts := strings.Split(e.Addresses, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"http://localhost:9200"}
	}
	return out
}

func (h HTTPConfig) CORSOrigins() []string {
	parts := strings.Split(h.CORSAllowOrigins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"http://localhost:5173"}
	}
	return out
}

func brokerListFromString(brokers string) []string {
	raw := strings.NewReplacer(";", ",", "\n", ",", "\t", ",").Replace(brokers)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker == "" {
			continue
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		out = append(out, broker)
	}
	return out
}

func (a AsyncOpsConfig) BrokerList() []string {
	return brokerListFromString(a.KafkaBrokers)
}

func (c NotificationDeliveryConfig) BrokerList() []string {
	return brokerListFromString(c.KafkaBrokers)
}
