package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AttachmentRepository struct {
	pool *pgxpool.Pool
}

type CreateAttachmentParams struct {
	TenantID       uuid.UUID
	CaseID         uuid.UUID
	FileName       string
	ContentType    string
	FileSizeBytes  int64
	StorageKey     string
	ChecksumSHA256 string
	UploadedBy     uuid.UUID
}

func NewAttachmentRepository(pool *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{pool: pool}
}

func (r *AttachmentRepository) Create(ctx context.Context, p CreateAttachmentParams) (*models.Attachment, error) {
	q := `
		INSERT INTO case_attachments(tenant_id, case_id, file_name, content_type, file_size_bytes, storage_key, checksum_sha256, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, tenant_id, case_id, file_name, content_type, file_size_bytes, storage_key, checksum_sha256, uploaded_by, created_at
	`

	var item models.Attachment
	if err := r.pool.QueryRow(ctx, q,
		p.TenantID,
		p.CaseID,
		p.FileName,
		p.ContentType,
		p.FileSizeBytes,
		p.StorageKey,
		p.ChecksumSHA256,
		p.UploadedBy,
	).Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.FileName,
		&item.ContentType,
		&item.FileSizeBytes,
		&item.StorageKey,
		&item.ChecksumSHA256,
		&item.UploadedBy,
		&item.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("create attachment: %w", err)
	}

	return &item, nil
}

func (r *AttachmentRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.Attachment, error) {
	return listByCase(
		ctx,
		r.pool,
		`
			SELECT id, tenant_id, case_id, file_name, content_type, file_size_bytes, storage_key, checksum_sha256, uploaded_by, created_at
			FROM case_attachments
			WHERE tenant_id=$1 AND case_id=$2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`,
		tenantID,
		caseID,
		limit,
		offset,
		scanAttachment,
		"list attachments",
		"scan attachment",
		"iterate attachments",
	)
}

func (r *AttachmentRepository) GetByID(ctx context.Context, tenantID, caseID, attachmentID uuid.UUID) (*models.Attachment, error) {
	q := `
		SELECT id, tenant_id, case_id, file_name, content_type, file_size_bytes, storage_key, checksum_sha256, uploaded_by, created_at
		FROM case_attachments
		WHERE tenant_id=$1 AND case_id=$2 AND id=$3
	`
	var item models.Attachment
	if err := r.pool.QueryRow(ctx, q, tenantID, caseID, attachmentID).Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.FileName,
		&item.ContentType,
		&item.FileSizeBytes,
		&item.StorageKey,
		&item.ChecksumSHA256,
		&item.UploadedBy,
		&item.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("get attachment by id: %w", err)
	}
	return &item, nil
}

func scanAttachment(scanner rowScanner) (*models.Attachment, error) {
	var item models.Attachment
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.FileName,
		&item.ContentType,
		&item.FileSizeBytes,
		&item.StorageKey,
		&item.ChecksumSHA256,
		&item.UploadedBy,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
