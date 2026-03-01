package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/tracing"
	"net/http"
	"strings"
	"time"
)

type SendRequest struct {
	ThreadID       string         `json:"thread_id"`
	ConversationID string         `json:"conversation_id"`
	Author         string         `json:"author"`
	Message        string         `json:"message"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type SendResponse struct {
	Reply          string         `json:"reply"`
	ConversationID string         `json:"conversation_id"`
	Cursor         string         `json:"cursor"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type PollRequest struct {
	ThreadID       string         `json:"thread_id"`
	ConversationID string         `json:"conversation_id"`
	Cursor         string         `json:"cursor"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type PollMessage struct {
	ExternalID string         `json:"external_id"`
	Author     string         `json:"author"`
	Content    string         `json:"content"`
	Timestamp  string         `json:"timestamp"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type PollResponse struct {
	Messages       []PollMessage  `json:"messages"`
	ConversationID string         `json:"conversation_id"`
	NextCursor     string         `json:"next_cursor"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type Driver interface {
	Kind() string
	Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error)
	Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error)
}

type Service struct {
	drivers    map[string]Driver
	httpClient *http.Client
}

func NewService(cfg config.OutboundConnectorsConfig) *Service {
	_, span, startedAt := tracing.StartModuleOperation(context.Background(), "connectors_outbound", "new_service")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "connectors_outbound", "new_service", err)
	}()

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	svc := &Service{
		httpClient: client,
		drivers:    map[string]Driver{},
	}
	svc.registerDriver(NewMockDriver())
	svc.registerDriver(NewWebhookDriver(client))
	svc.registerDriver(NewEmailDriver(client))
	svc.registerDriver(NewTimeDriver(client))
	svc.registerDriver(NewTelegramDriver(client))
	svc.registerDriver(NewSlackDriver(client))
	svc.registerDriver(NewOutlookDriver(client))
	svc.registerDriver(NewKafkaDriver())
	svc.registerDriver(NewRedisDriver())
	svc.registerDriver(NewSQLDriver())
	svc.registerDriver(NewS3Driver())
	return svc
}

func (s *Service) registerDriver(driver Driver) {
	if s == nil || driver == nil {
		return
	}
	s.drivers[strings.ToLower(strings.TrimSpace(driver.Kind()))] = driver
}

func (s *Service) Send(ctx context.Context, connector models.CatalogItem, req SendRequest) (SendResponse, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "connectors_outbound", "send")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "connectors_outbound", "send", err)
	}()

	driver, cfg, err := s.resolveDriver(connector)
	if err != nil {
		return SendResponse{}, err
	}
	var response SendResponse
	response, err = driver.Send(ctx, cfg, req)
	return response, err
}

func (s *Service) Poll(ctx context.Context, connector models.CatalogItem, req PollRequest) (PollResponse, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "connectors_outbound", "poll")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "connectors_outbound", "poll", err)
	}()

	driver, cfg, err := s.resolveDriver(connector)
	if err != nil {
		return PollResponse{}, err
	}
	var response PollResponse
	response, err = driver.Poll(ctx, cfg, req)
	return response, err
}

func (s *Service) resolveDriver(connector models.CatalogItem) (Driver, map[string]any, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("outbound service is not initialized")
	}

	// Prefer explicit channel on the catalog item, then fall back to type / provider
	rawChannel := strings.ToLower(strings.TrimSpace(firstString(connector.Data, "channel")))
	rawType := strings.ToUpper(strings.TrimSpace(firstString(connector.Data, "type", "provider")))

	channel := strings.TrimSpace(rawChannel)
	if channel == "" {
		switch rawType {
		case "HTTP", "REST", "API":
			channel = "webhook"
		case "SQL", "DATABASE", "DB":
			channel = "sql"
		case "S3", "OBJECT_STORAGE", "OBJECT-STORE", "OBJECT_STORE":
			channel = "object_storage"
		case "KAFKA":
			channel = "kafka"
		case "REDIS":
			channel = "redis"
		case "SMTP", "EMAIL", "MAIL":
			channel = "email"
		case "TELEGRAM":
			channel = "telegram"
		case "SLACK":
			channel = "slack"
		case "OUTLOOK", "OUTLOOK_MAIL", "OUTLOOK MAIL", "MICROSOFT GRAPH", "MICROSOFT_GRAPH", "GRAPH":
			channel = "outlook"
		case "WEBHOOK":
			channel = "webhook"
		case "TIME":
			channel = "time"
		case "CUSTOM":
			channel = "custom"
		default:
			// No explicit mapping; keep empty and fail below so the user sees a clear error
			channel = ""
		}
	}

	if channel == "" {
		return nil, nil, fmt.Errorf("outbound connector channel is required (type=%q)", rawType)
	}

	driver, ok := s.drivers[channel]
	if !ok {
		return nil, nil, fmt.Errorf("outbound connector driver %q is not supported", channel)
	}

	cfg := map[string]any{}
	if value, ok := connector.Data["config"]; ok {
		switch typed := value.(type) {
		case map[string]any:
			cfg = typed
		default:
			raw, _ := json.Marshal(typed)
			_ = json.Unmarshal(raw, &cfg)
		}
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	if _, exists := cfg["channel"]; !exists {
		cfg["channel"] = channel
	}
	// Propagate connector type for drivers that care (e.g. sql engine mapping)
	if rawType != "" {
		if _, exists := cfg["type"]; !exists {
			cfg["type"] = rawType
		}
	}
	// Attach connector id for potential pooling / diagnostics
	if _, exists := cfg["connector_id"]; !exists {
		cfg["connector_id"] = connector.ID.String()
	}
	return driver, cfg, nil
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}
