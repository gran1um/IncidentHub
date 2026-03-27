package models

import (
	"time"

	"github.com/google/uuid"
)

type InboundConnectorRun struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	ConnectorID       uuid.UUID  `json:"connector_id"`
	ScheduledFor      *time.Time `json:"scheduled_for,omitempty"`
	Trigger           string     `json:"trigger"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at,omitempty"`
	Status            string     `json:"status"`
	RecordsSeen       int        `json:"records_seen"`
	AlertsCreated     int        `json:"alerts_created"`
	DuplicatesSkipped int        `json:"duplicates_skipped"`
	Errors            int        `json:"errors"`
	Message           string     `json:"message"`
	Logs              string     `json:"logs"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
