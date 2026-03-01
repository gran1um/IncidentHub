package api

import "context"

type NotificationDeliveryEvent struct {
	EventID        string `json:"event_id"`
	NotificationID string `json:"notification_id"`
	TenantID       string `json:"tenant_id"`
	UserID         string `json:"user_id"`
	Title          string `json:"title"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	CreatedAt      string `json:"created_at"`
}

type NotificationDeliveryQueue interface {
	Enabled() bool
	Enqueue(ctx context.Context, event NotificationDeliveryEvent) error
}
