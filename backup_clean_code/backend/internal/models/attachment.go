package models

import (
	"time"

	"github.com/google/uuid"
)

type Attachment struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	CaseID         uuid.UUID  `json:"case_id"`
	FileName       string     `json:"file_name"`
	ContentType    string     `json:"content_type"`
	FileSizeBytes  int64      `json:"file_size_bytes"`
	StorageKey     string     `json:"storage_key"`
	ChecksumSHA256 string     `json:"checksum_sha256"`
	UploadedBy     *uuid.UUID `json:"uploaded_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}
