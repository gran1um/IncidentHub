package inbound

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type inboundConnectorConfig struct {
	DisplayName      string
	SourceType       string
	Schedule         string
	ArrayPath        string
	IDField          string
	TitleField       string
	DescriptionField string
	SourceField      string
	SeverityField    string
	StatusField      string
	TLPField         string
	PAPField         string
	Source           string
	Severity         string
	Status           string
	TLP              string
	PAP              string
	Timeout          time.Duration
	HTTP             inboundHTTPSourceConfig
	SQL              inboundSQLSourceConfig
	S3               inboundS3SourceConfig
	Kafka            inboundKafkaSourceConfig
	Redis            inboundRedisSourceConfig
}

type inboundAuthConfig struct {
	Type            string
	Username        string
	Password        string
	Token           string
	APIKey          string
	APIKeyHeader    string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Headers         map[string]string
}

type inboundHTTPSourceConfig struct {
	URL     string
	Method  string
	Body    string
	Headers map[string]string
	Timeout time.Duration
	Auth    inboundAuthConfig
}

type inboundSQLSourceConfig struct {
	Driver   string
	DSN      string
	Host     string
	Port     int
	Database string
	SSLMode  string
	Query    string
	MaxRows  int
	Timeout  time.Duration
	Auth     inboundAuthConfig
}

type inboundS3SourceConfig struct {
	Endpoint             string
	Region               string
	Bucket               string
	Prefix               string
	ObjectPattern        string
	UseSSL               bool
	PathStyle            bool
	MaxObjects           int
	ObjectSizeLimitBytes int64
	Timeout              time.Duration
	Auth                 inboundAuthConfig
}

type inboundKafkaSourceConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	ClientID      string
	StartOffset   string
	MaxMessages   int
	PollTimeout   time.Duration
	DialTimeout   time.Duration
	Commit        bool
	UseTLS        bool
	SkipTLSVerify bool
	Auth          inboundAuthConfig
}

type inboundRedisSourceConfig struct {
	Addr          string
	DB            int
	Key           string
	MaxMessages   int
	Timeout       time.Duration
	PopFrom       string
	UseTLS        bool
	SkipTLSVerify bool
	Auth          inboundAuthConfig
}

// ValidateConnectorData performs strict inbound connector validation for API create/update.
func ValidateConnectorData(data map[string]any) error {
	if _, err := parseInboundConnectorConfig(map[string]any{"data": data}); err != nil {
		return err
	}
	return nil
}

func parseInboundConnectorConfig(connector any) (inboundConnectorConfig, error) {
	item := coerceMap(connector)
	if item == nil {
		return inboundConnectorConfig{}, fmt.Errorf("inbound connector: invalid payload")
	}

	data := extractCatalogData(item)
	profiledData, err := applyInboundAlertSourceProfile(data)
	if err != nil {
		return inboundConnectorConfig{}, err
	}
	data = profiledData

	cfgMap := data
	if nested := mapValue(data, "config"); nested != nil {
		cfgMap = nested
	}

	name := strings.TrimSpace(firstString(data, "name"))
	if name == "" {
		name = "Inbound Connector"
	}

	sourceTypeRaw := firstString(
		cfgMap,
		"source_type",
		"sourceType",
		"provider",
		"provider_type",
	)
	if sourceTypeRaw == "" {
		sourceTypeRaw = firstString(data, "source_type", "sourceType", "type", "provider")
	}
	sourceType := normalizeInboundSourceType(sourceTypeRaw)
	if sourceType == "" {
		return inboundConnectorConfig{}, fmt.Errorf("source type is required")
	}

	cfg := inboundConnectorConfig{
		DisplayName:      name,
		SourceType:       sourceType,
		Schedule:         strings.TrimSpace(firstString(cfgMap, "schedule", "cron")),
		ArrayPath:        strings.TrimSpace(firstString(cfgMap, "array_path", "arrayPath", "records_path", "recordsPath")),
		IDField:          defaultString(firstString(cfgMap, "id_field", "idField"), "id"),
		TitleField:       defaultString(firstString(cfgMap, "title_field", "titleField"), "title"),
		DescriptionField: defaultString(firstString(cfgMap, "description_field", "descriptionField"), "description"),
		SourceField:      defaultString(firstString(cfgMap, "source_field", "sourceField"), "source"),
		SeverityField:    defaultString(firstString(cfgMap, "severity_field", "severityField"), "severity"),
		StatusField:      defaultString(firstString(cfgMap, "status_field", "statusField"), "status"),
		TLPField:         defaultString(firstString(cfgMap, "tlp_field", "tlpField"), "tlp"),
		PAPField:         defaultString(firstString(cfgMap, "pap_field", "papField"), "pap"),
		Source:           defaultString(firstString(cfgMap, "default_source", "defaultSource", "source"), ""),
		Severity:         defaultString(firstString(cfgMap, "default_severity", "defaultSeverity"), "medium"),
		Status:           defaultString(firstString(cfgMap, "default_status", "defaultStatus"), "new"),
		TLP:              defaultString(firstString(cfgMap, "default_tlp", "defaultTlp"), "amber"),
		PAP:              defaultString(firstString(cfgMap, "default_pap", "defaultPap"), "amber"),
		Timeout:          parsePositiveSeconds(cfgMap, "timeout", "timeout_seconds"),
	}

	switch sourceType {
	case "http":
		httpCfg, err := parseHTTPSourceConfig(cfgMap)
		if err != nil {
			return inboundConnectorConfig{}, fmt.Errorf("inbound connector %q: %w", name, err)
		}
		cfg.HTTP = httpCfg
	case "sql":
		sqlCfg, err := parseSQLSourceConfig(cfgMap)
		if err != nil {
			return inboundConnectorConfig{}, fmt.Errorf("inbound connector %q: %w", name, err)
		}
		cfg.SQL = sqlCfg
	case "s3":
		s3Cfg, err := parseS3SourceConfig(cfgMap)
		if err != nil {
			return inboundConnectorConfig{}, fmt.Errorf("inbound connector %q: %w", name, err)
		}
		cfg.S3 = s3Cfg
	case "kafka":
		kafkaCfg, err := parseKafkaSourceConfig(cfgMap)
		if err != nil {
			return inboundConnectorConfig{}, fmt.Errorf("inbound connector %q: %w", name, err)
		}
		cfg.Kafka = kafkaCfg
	case "redis":
		redisCfg, err := parseRedisSourceConfig(cfgMap)
		if err != nil {
			return inboundConnectorConfig{}, fmt.Errorf("inbound connector %q: %w", name, err)
		}
		cfg.Redis = redisCfg
	default:
		return inboundConnectorConfig{}, fmt.Errorf("source type %q is not supported", sourceType)
	}

	return cfg, nil
}

func parseHTTPSourceConfig(cfgMap map[string]any) (inboundHTTPSourceConfig, error) {
	section := firstSection(cfgMap, "http", "request")
	maps := chainMaps(section, cfgMap)

	url := strings.TrimSpace(firstStringFromMaps(maps, "url", "endpoint", "feed_url", "feedUrl", "baseUrl"))
	if url == "" {
		return inboundHTTPSourceConfig{}, fmt.Errorf("config.url is required for HTTP source")
	}

	method := strings.ToUpper(strings.TrimSpace(firstStringFromMaps(maps, "method", "http_method")))
	if method == "" {
		method = http.MethodGet
	}

	auth := parseAuthConfig(maps)
	cfg := inboundHTTPSourceConfig{
		URL:     url,
		Method:  method,
		Body:    strings.TrimSpace(firstStringFromMaps(maps, "body", "request_body", "payload")),
		Headers: parseHeadersFromMaps(maps),
		Timeout: parsePositiveSecondsFromMaps(maps, "timeout", "timeout_seconds"),
		Auth:    auth,
	}
	return cfg, nil
}

func parseSQLSourceConfig(cfgMap map[string]any) (inboundSQLSourceConfig, error) {
	section := firstSection(cfgMap, "sql")
	maps := chainMaps(section, cfgMap)

	query := strings.TrimSpace(firstStringFromMaps(maps, "query", "sql", "statement"))
	if query == "" {
		return inboundSQLSourceConfig{}, fmt.Errorf("config.query is required for SQL source")
	}

	auth := parseAuthConfig(maps)
	driver := strings.ToLower(strings.TrimSpace(firstStringFromMaps(maps, "driver", "dialect", "type")))
	if driver == "" || driver == "postgresql" || driver == "postgres" {
		driver = "pgx"
	}

	cfg := inboundSQLSourceConfig{
		Driver:   driver,
		DSN:      strings.TrimSpace(firstStringFromMaps(maps, "dsn", "connection_string", "connectionString", "url")),
		Host:     strings.TrimSpace(firstStringFromMaps(maps, "host")),
		Port:     parseIntFromMaps(maps, 5432, "port"),
		Database: strings.TrimSpace(firstStringFromMaps(maps, "database", "db", "dbname")),
		SSLMode:  defaultString(firstStringFromMaps(maps, "ssl_mode", "sslMode"), "disable"),
		Query:    query,
		MaxRows:  parseIntFromMaps(maps, 500, "max_rows", "maxRows", "limit"),
		Timeout:  parsePositiveSecondsFromMaps(maps, "timeout", "timeout_seconds"),
		Auth:     auth,
	}
	return cfg, nil
}

func parseS3SourceConfig(cfgMap map[string]any) (inboundS3SourceConfig, error) {
	section := firstSection(cfgMap, "s3", "storage")
	maps := chainMaps(section, cfgMap)

	endpoint := strings.TrimSpace(firstStringFromMaps(maps, "endpoint", "endpoint_url", "endpointUrl"))
	if endpoint == "" {
		return inboundS3SourceConfig{}, fmt.Errorf("config.endpoint is required for S3 source")
	}

	bucket := strings.TrimSpace(firstStringFromMaps(maps, "bucket"))
	if bucket == "" {
		return inboundS3SourceConfig{}, fmt.Errorf("config.bucket is required for S3 source")
	}

	auth := parseAuthConfig(maps)
	if strings.TrimSpace(auth.AccessKeyID) == "" {
		auth.AccessKeyID = strings.TrimSpace(firstStringFromMaps(maps, "accessKey", "access_key", "accessKeyId", "access_key_id"))
	}
	if strings.TrimSpace(auth.SecretAccessKey) == "" {
		auth.SecretAccessKey = strings.TrimSpace(firstStringFromMaps(maps, "secretKey", "secret_key", "secretAccessKey", "secret_access_key"))
	}
	if strings.TrimSpace(auth.SessionToken) == "" {
		auth.SessionToken = strings.TrimSpace(firstStringFromMaps(maps, "sessionToken", "session_token"))
	}
	if auth.Type == "none" && strings.TrimSpace(auth.AccessKeyID) != "" {
		auth.Type = "access_key"
	}

	cfg := inboundS3SourceConfig{
		Endpoint:             endpoint,
		Region:               defaultString(firstStringFromMaps(maps, "region"), "us-east-1"),
		Bucket:               bucket,
		Prefix:               strings.TrimSpace(firstStringFromMaps(maps, "prefix", "path", "folder")),
		ObjectPattern:        strings.TrimSpace(firstStringFromMaps(maps, "object_pattern", "objectPattern", "pattern", "glob")),
		UseSSL:               parseBoolFromMaps(maps, false, "use_ssl", "useSsl", "secure", "ssl"),
		PathStyle:            parseBoolFromMaps(maps, true, "path_style", "pathStyle", "force_path_style", "forcePathStyle"),
		MaxObjects:           parseIntFromMaps(maps, 100, "max_objects", "maxObjects", "limit"),
		ObjectSizeLimitBytes: int64(parseIntFromMaps(maps, 5*1024*1024, "object_size_limit_bytes", "objectSizeLimitBytes", "max_object_size", "maxObjectSize")),
		Timeout:              parsePositiveSecondsFromMaps(maps, "timeout", "timeout_seconds"),
		Auth:                 auth,
	}
	return cfg, nil
}

func parseKafkaSourceConfig(cfgMap map[string]any) (inboundKafkaSourceConfig, error) {
	section := firstSection(cfgMap, "kafka")
	maps := chainMaps(section, cfgMap)

	brokers := parseStringListFromMaps(maps, "brokers", "bootstrap_servers", "bootstrapServers", "broker", "brokers_csv")
	if len(brokers) == 0 {
		return inboundKafkaSourceConfig{}, fmt.Errorf("config.brokers is required for Kafka source")
	}

	topic := strings.TrimSpace(firstStringFromMaps(maps, "topic"))
	if topic == "" {
		return inboundKafkaSourceConfig{}, fmt.Errorf("config.topic is required for Kafka source")
	}

	auth := parseAuthConfig(maps)
	startOffset := strings.ToLower(defaultString(firstStringFromMaps(maps, "start_offset", "startOffset", "offset"), "latest"))
	if startOffset != "latest" && startOffset != "earliest" {
		return inboundKafkaSourceConfig{}, fmt.Errorf("config.start_offset must be latest or earliest")
	}

	cfg := inboundKafkaSourceConfig{
		Brokers:       brokers,
		Topic:         topic,
		GroupID:       strings.TrimSpace(firstStringFromMaps(maps, "group_id", "groupId", "consumer_group", "consumerGroup")),
		ClientID:      strings.TrimSpace(firstStringFromMaps(maps, "client_id", "clientId")),
		StartOffset:   startOffset,
		MaxMessages:   parseIntFromMaps(maps, 100, "max_messages", "maxMessages", "batch_limit", "batchLimit", "limit"),
		PollTimeout:   parsePositiveSecondsFromMaps(maps, "poll_timeout", "poll_timeout_seconds", "pollTimeout", "pollTimeoutSeconds"),
		DialTimeout:   parsePositiveSecondsFromMaps(maps, "dial_timeout", "dial_timeout_seconds", "dialTimeout", "dialTimeoutSeconds"),
		Commit:        parseBoolFromMaps(maps, true, "commit", "auto_commit", "autoCommit"),
		UseTLS:        parseBoolFromMaps(maps, false, "use_tls", "useTls", "tls", "ssl"),
		SkipTLSVerify: parseBoolFromMaps(maps, false, "skip_tls_verify", "skipTlsVerify", "insecure_skip_verify", "insecureSkipVerify"),
		Auth:          auth,
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 100
	}
	return cfg, nil
}

func parseRedisSourceConfig(cfgMap map[string]any) (inboundRedisSourceConfig, error) {
	section := firstSection(cfgMap, "redis")
	maps := chainMaps(section, cfgMap)

	addr := strings.TrimSpace(firstStringFromMaps(maps, "addr", "address"))
	if addr == "" {
		host := strings.TrimSpace(firstStringFromMaps(maps, "host"))
		if host != "" {
			addr = fmt.Sprintf("%s:%d", host, parseIntFromMaps(maps, 6379, "port"))
		}
	}
	if addr == "" {
		return inboundRedisSourceConfig{}, fmt.Errorf("config.addr is required for Redis source")
	}

	key := strings.TrimSpace(firstStringFromMaps(maps, "key", "queue", "queue_key", "queueKey", "list", "list_key", "listKey"))
	if key == "" {
		return inboundRedisSourceConfig{}, fmt.Errorf("config.key is required for Redis source")
	}

	popFrom := strings.ToLower(defaultString(firstStringFromMaps(maps, "pop_from", "popFrom", "side"), "left"))
	if popFrom != "left" && popFrom != "right" {
		return inboundRedisSourceConfig{}, fmt.Errorf("config.pop_from must be left or right")
	}

	auth := parseAuthConfig(maps)
	cfg := inboundRedisSourceConfig{
		Addr:          addr,
		DB:            parseIntFromMaps(maps, 0, "db", "database"),
		Key:           key,
		MaxMessages:   parseIntFromMaps(maps, 100, "max_messages", "maxMessages", "batch_limit", "batchLimit", "limit"),
		Timeout:       parsePositiveSecondsFromMaps(maps, "timeout", "timeout_seconds"),
		PopFrom:       popFrom,
		UseTLS:        parseBoolFromMaps(maps, false, "use_tls", "useTls", "tls", "ssl"),
		SkipTLSVerify: parseBoolFromMaps(maps, false, "skip_tls_verify", "skipTlsVerify", "insecure_skip_verify", "insecureSkipVerify"),
		Auth:          auth,
	}
	if cfg.MaxMessages <= 0 {
		cfg.MaxMessages = 100
	}
	return cfg, nil
}

func parseAuthConfig(maps []map[string]any) inboundAuthConfig {
	authMap := map[string]any{}
	for _, source := range maps {
		if nested := mapValue(source, "auth"); nested != nil {
			authMap = mergeMaps(authMap, nested)
		}
	}

	auth := inboundAuthConfig{
		Type:            strings.ToLower(strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "type", "authType", "mode"))),
		Username:        strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "username", "user", "login")),
		Password:        strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "password", "pass")),
		Token:           strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "token", "bearer", "bearerToken", "accessToken")),
		APIKey:          strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "apiKey", "api_key", "key")),
		APIKeyHeader:    strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "apiKeyHeader", "api_key_header", "header", "headerName")),
		AccessKeyID:     strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "accessKey", "access_key", "accessKeyId", "access_key_id")),
		SecretAccessKey: strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "secretKey", "secret_key", "secretAccessKey", "secret_access_key")),
		SessionToken:    strings.TrimSpace(firstStringFromMaps([]map[string]any{authMap}, "sessionToken", "session_token")),
		Headers:         parseHeadersMap(authMap),
	}

	if auth.Type == "" {
		auth.Type = strings.ToLower(strings.TrimSpace(firstStringFromMaps(maps, "authType", "auth_type")))
	}
	if auth.Token == "" {
		auth.Token = strings.TrimSpace(firstStringFromMaps(maps, "bearerToken", "bearer_token", "token"))
	}
	if auth.APIKey == "" {
		auth.APIKey = strings.TrimSpace(firstStringFromMaps(maps, "apiKey", "api_key"))
	}
	if auth.Username == "" {
		auth.Username = strings.TrimSpace(firstStringFromMaps(maps, "username", "user"))
	}
	if auth.Password == "" {
		auth.Password = strings.TrimSpace(firstStringFromMaps(maps, "password", "pass"))
	}
	if auth.APIKeyHeader == "" {
		auth.APIKeyHeader = strings.TrimSpace(firstStringFromMaps(maps, "apiKeyHeader", "api_key_header", "headerName"))
	}
	if auth.APIKeyHeader == "" {
		auth.APIKeyHeader = "X-API-Key"
	}
	if auth.Type == "" {
		switch {
		case auth.Token != "":
			auth.Type = "bearer"
		case auth.APIKey != "":
			auth.Type = "api_key"
		case auth.Username != "" || auth.Password != "":
			auth.Type = "basic"
		case auth.AccessKeyID != "" || auth.SecretAccessKey != "":
			auth.Type = "access_key"
		default:
			auth.Type = "none"
		}
	}
	return auth
}

func normalizeInboundSourceType(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	switch raw {
	case "", "http", "https", "webhook", "telegram", "smtp", "ldap", "syslog", "custom":
		return "http"
	case "sql", "postgres", "postgresql", "pg", "pgx":
		return "sql"
	case "s3", "minio", "object_storage", "object-store", "objectstorage":
		return "s3"
	case "kafka":
		return "kafka"
	case "redis", "valkey":
		return "redis"
	default:
		return raw
	}
}

func extractCatalogData(item map[string]any) map[string]any {
	if data := mapValue(item, "Data"); data != nil {
		return data
	}
	if data := mapValue(item, "data"); data != nil {
		return data
	}
	return item
}

func coerceMap(input any) map[string]any {
	switch typed := input.(type) {
	case map[string]any:
		return typed
	case nil:
		return nil
	default:
		raw, err := json.Marshal(input)
		if err != nil {
			return nil
		}
		result := map[string]any{}
		if json.Unmarshal(raw, &result) != nil {
			return nil
		}
		return result
	}
}

func mapValue(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok {
		return nil
	}
	return coerceMap(value)
}

func firstSection(payload map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if section := mapValue(payload, key); section != nil {
			return section
		}
	}
	return map[string]any{}
}

func chainMaps(primary map[string]any, fallback ...map[string]any) []map[string]any {
	maps := make([]map[string]any, 0, len(fallback)+1)
	if primary != nil {
		maps = append(maps, primary)
	}
	maps = append(maps, fallback...)
	return maps
}

func mergeMaps(base map[string]any, override map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func firstStringFromMaps(maps []map[string]any, keys ...string) string {
	for _, payload := range maps {
		value := firstString(payload, keys...)
		if value != "" {
			return value
		}
	}
	return ""
}

func parseHeadersFromMaps(maps []map[string]any) map[string]string {
	for _, payload := range maps {
		headers := parseHeadersMap(payload)
		if len(headers) > 0 {
			return headers
		}
	}
	return map[string]string{}
}

func parsePositiveSeconds(payload map[string]any, keys ...string) time.Duration {
	seconds := parseInt(payload, -1, keys...)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func parsePositiveSecondsFromMaps(maps []map[string]any, keys ...string) time.Duration {
	for _, payload := range maps {
		value := parsePositiveSeconds(payload, keys...)
		if value > 0 {
			return value
		}
	}
	return 0
}

func parseIntFromMaps(maps []map[string]any, fallback int, keys ...string) int {
	for _, payload := range maps {
		value := parseInt(payload, fallback, keys...)
		if value != fallback {
			return value
		}
	}
	return fallback
}

func parseStringListFromMaps(maps []map[string]any, keys ...string) []string {
	for _, payload := range maps {
		value := parseStringList(payload, keys...)
		if len(value) > 0 {
			return value
		}
	}
	return nil
}

func parseBoolFromMaps(maps []map[string]any, fallback bool, keys ...string) bool {
	for _, payload := range maps {
		if value, ok := parseBool(payload, keys...); ok {
			return value
		}
	}
	return fallback
}

func parseStringList(payload map[string]any, keys ...string) []string {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		raw, exists := payload[key]
		if !exists {
			continue
		}

		switch typed := raw.(type) {
		case string:
			parts := strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
			})
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				item := strings.TrimSpace(part)
				if item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []string:
			out := make([]string, 0, len(typed))
			for _, value := range typed {
				item := strings.TrimSpace(value)
				if item != "" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []any:
			out := make([]string, 0, len(typed))
			for _, value := range typed {
				item := strings.TrimSpace(fmt.Sprint(value))
				if item != "" && item != "<nil>" {
					out = append(out, item)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func parseInt(payload map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		raw, exists := payload[key]
		if !exists {
			continue
		}
		switch typed := raw.(type) {
		case int:
			return typed
		case int32:
			return int(typed)
		case int64:
			return int(typed)
		case float32:
			return int(typed)
		case float64:
			return int(typed)
		case json.Number:
			if number, err := typed.Int64(); err == nil {
				return int(number)
			}
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func parseBool(payload map[string]any, keys ...string) (flag bool, ok bool) {
	for _, key := range keys {
		if payload == nil {
			continue
		}
		raw, exists := payload[key]
		if !exists {
			continue
		}
		switch typed := raw.(type) {
		case bool:
			return typed, true
		case string:
			value := strings.ToLower(strings.TrimSpace(typed))
			switch value {
			case "true", "1", "yes", "on":
				return true, true
			case "false", "0", "no", "off":
				return false, true
			}
		}
	}
	return false, false
}
