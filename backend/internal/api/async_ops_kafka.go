package api

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/tracing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const kafkaRetryAttemptHeader = "x-retry-attempt"

// Sentinel errors for async ops Kafka (err113).
var (
	errKafkaAsyncOpsDepsRequired          = errors.New("async ops kafka requires alerts, cases, observables and audits repositories")
	errKafkaAsyncOpsTopicRequired         = errors.New("async ops kafka topic is required")
	errKafkaAsyncOpsGroupIDRequired       = errors.New("async ops kafka group id is required")
	errKafkaAsyncOpsBrokersRequired       = errors.New("async ops kafka brokers are required")
	errKafkaAsyncOpsQueueDisabled         = errors.New("async ops kafka queue is disabled")
	errKafkaAsyncOpsOperationIDRequired   = errors.New("operation id is required")
	errKafkaAsyncOpsOperationTypeRequired = errors.New("operation type is required")
	errKafkaAsyncOpsRequeueMessageNil     = errors.New("requeue message is nil")
)

type KafkaAsyncOpsDependencies struct {
	Alerts          *repository.AlertRepository
	Cases           *repository.CaseRepository
	Observables     *repository.ObservableRepository
	Experience      *repository.ExperienceRepository
	Audits          *repository.AuditRepository
	AsyncOperations *repository.AsyncOperationRepository
	AIAgentQueue    *repository.AIAgentQueueRepository
	Search          interface {
		IndexDocument(ctx context.Context, kind, id string, doc any) error
		DeleteDocument(ctx context.Context, kind, id string) error
	}
}

type KafkaAsyncOps struct {
	enabled        bool
	topic          string
	dlqTopic       string
	pollTimeout    time.Duration
	produceTimeout time.Duration
	consumer       *kafka.Consumer
	producer       *kafka.Producer

	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration

	alerts       *repository.AlertRepository
	cases        *repository.CaseRepository
	observables  *repository.ObservableRepository
	experience   *repository.ExperienceRepository
	audits       *repository.AuditRepository
	operations   *repository.AsyncOperationRepository
	aiAgentQueue *repository.AIAgentQueueRepository
	search       interface {
		IndexDocument(ctx context.Context, kind, id string, doc any) error
		DeleteDocument(ctx context.Context, kind, id string) error
	}

	startOnce sync.Once
}

func NewKafkaAsyncOps(cfg config.AsyncOpsConfig, deps KafkaAsyncOpsDependencies) (*KafkaAsyncOps, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if deps.Alerts == nil || deps.Cases == nil || deps.Observables == nil || deps.Audits == nil {
		return nil, errKafkaAsyncOpsDepsRequired
	}
	topic := strings.TrimSpace(cfg.KafkaTopic)
	if topic == "" {
		return nil, errKafkaAsyncOpsTopicRequired
	}
	groupID := strings.TrimSpace(cfg.KafkaGroupID)
	if groupID == "" {
		return nil, errKafkaAsyncOpsGroupIDRequired
	}
	brokers := cfg.BrokerList()
	if len(brokers) == 0 {
		return nil, errKafkaAsyncOpsBrokersRequired
	}
	bootstrapServers := strings.Join(brokers, ",")
	produceTimeout := positiveDuration(cfg.ProduceTimeout, 5*time.Second)
	pollTimeout := positiveDuration(cfg.PollTimeout, time.Second)
	dlqTopic := strings.TrimSpace(cfg.KafkaDLQTopic)
	if dlqTopic == "" {
		dlqTopic = topic + ".dlq"
	}
	maxAttempts := cfg.RetryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	baseBackoff := positiveDuration(cfg.RetryBaseBackoff, 500*time.Millisecond)
	maxBackoff := positiveDuration(cfg.RetryMaxBackoff, 15*time.Second)

	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"message.timeout.ms": func() int {
			ms := int(produceTimeout / time.Millisecond)
			if ms <= 0 {
				return 5000
			}
			return ms
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("create async ops kafka producer: %w", err)
	}

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("create async ops kafka consumer: %w", err)
	}
	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		_ = consumer.Close()
		producer.Close()
		return nil, fmt.Errorf("subscribe async ops kafka topic: %w", err)
	}

	return &KafkaAsyncOps{
		enabled:        true,
		topic:          topic,
		dlqTopic:       dlqTopic,
		pollTimeout:    pollTimeout,
		produceTimeout: produceTimeout,
		consumer:       consumer,
		producer:       producer,
		maxAttempts:    maxAttempts,
		baseBackoff:    baseBackoff,
		maxBackoff:     maxBackoff,
		alerts:         deps.Alerts,
		cases:          deps.Cases,
		observables:    deps.Observables,
		experience:     deps.Experience,
		audits:         deps.Audits,
		operations:     deps.AsyncOperations,
		aiAgentQueue:   deps.AIAgentQueue,
		search:         deps.Search,
	}, nil
}

func (q *KafkaAsyncOps) Enabled() bool {
	return q != nil && q.enabled
}

func (q *KafkaAsyncOps) MaxAttempts() int {
	if q == nil || q.maxAttempts <= 0 {
		return 1
	}
	return q.maxAttempts
}

func (q *KafkaAsyncOps) Enqueue(ctx context.Context, operation AsyncOperation) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "async_ops", "enqueue")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "async_ops", "enqueue", err)
	}()

	if !q.Enabled() {
		err = errKafkaAsyncOpsQueueDisabled
		return err
	}
	if strings.TrimSpace(operation.OperationID) == "" {
		err = errKafkaAsyncOpsOperationIDRequired
		return err
	}
	if strings.TrimSpace(string(operation.Type)) == "" {
		err = errKafkaAsyncOpsOperationTypeRequired
		return err
	}
	payload, err := json.Marshal(operation)
	if err != nil {
		return fmt.Errorf("marshal async operation: %w", err)
	}
	headers := []kafka.Header{{Key: kafkaRetryAttemptHeader, Value: []byte("0")}}
	err = q.produceMessage(ctx, q.topic, []byte(operation.OperationID), payload, headers)
	if err != nil {
		return fmt.Errorf("enqueue async operation to kafka: %w", err)
	}
	return nil
}

func (q *KafkaAsyncOps) Start(ctx context.Context) {
	if !q.Enabled() {
		return
	}
	q.startOnce.Do(func() {
		go q.consumeLoop(ctx)
	})
}

func (q *KafkaAsyncOps) Close() error {
	if q == nil {
		return nil
	}
	if q.consumer != nil {
		if err := q.consumer.Close(); err != nil {
			return err
		}
	}
	if q.producer != nil {
		timeoutMs := int((2 * time.Second) / time.Millisecond)
		q.producer.Flush(timeoutMs)
		q.producer.Close()
	}
	return nil
}

func (q *KafkaAsyncOps) consumeLoop(ctx context.Context) {
	pollTimeoutMs := int(q.pollTimeout / time.Millisecond)
	if pollTimeoutMs <= 0 {
		pollTimeoutMs = 1000
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event := q.consumer.Poll(pollTimeoutMs)
		if event == nil {
			continue
		}

		switch typed := event.(type) {
		case *kafka.Message:
			operation, ok := parseAsyncOperationMessage(typed)
			if !ok {
				metrics.ObserveModuleOperation("async_ops", "parse_operation", "error", 0)
				q.commitConsumedMessage(typed)
				continue
			}
			attempt := kafkaHeaderInt(typed.Headers) + 1
			q.markOperationProcessing(operation, attempt)

			retryableErr := q.processOperation(operation)
			if retryableErr == nil {
				q.markOperationDone(operation)
				q.commitConsumedMessage(typed)
				continue
			}
			q.handleProcessingError(ctx, typed, operation, attempt, retryableErr)
		case kafka.Error:
			if typed.IsTimeout() || typed.Code() == kafka.ErrTimedOut {
				continue
			}
			metrics.ObserveModuleOperation("async_ops", "consumer_poll", "error", 0)
			logger.Errorf("async kafka consumer error: %v", typed)
		}
	}
}

func (q *KafkaAsyncOps) handleProcessingError(ctx context.Context, message *kafka.Message, operation AsyncOperation, attempt int, retryableErr error) {
	logger.Errorf("async operation processing failed (attempt %d/%d): %v", attempt, q.MaxAttempts(), retryableErr)
	if attempt >= q.MaxAttempts() {
		q.markOperationFailed(operation, attempt, retryableErr)
		if err := q.publishToDLQ(ctx, message, operation, attempt, retryableErr); err != nil {
			logger.Errorf("async operation DLQ publish failed: %v", err)
		}
		q.commitConsumedMessage(message)
		return
	}

	backoff := q.retryBackoff(attempt)
	if !sleepWithContext(ctx, backoff) {
		return
	}

	q.markOperationRetryQueued(operation, attempt, retryableErr)
	if err := q.requeueMessage(ctx, message, attempt); err != nil {
		logger.Errorf("async operation requeue failed: %v", err)
		return
	}
	q.commitConsumedMessage(message)
}

func (q *KafkaAsyncOps) processOperation(operation AsyncOperation) error {
	processCtx, span, startedAt := tracing.StartModuleOperation(context.Background(), "async_ops", "process_operation")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "async_ops", "process_operation", err)
	}()

	processCtx, cancel := context.WithTimeout(processCtx, 15*time.Second)
	defer cancel()

	switch operation.Type {
	case AsyncOperationAlertCreate:
		err = q.processAlertCreate(processCtx, operation)
		return err
	case AsyncOperationAlertUpdate:
		err = q.processAlertUpdate(processCtx, operation)
		return err
	case AsyncOperationAlertDelete:
		err = q.processAlertDelete(processCtx, operation)
		return err
	case AsyncOperationCaseCreate:
		err = q.processCaseCreate(processCtx, operation)
		return err
	case AsyncOperationCaseUpdate:
		err = q.processCaseUpdate(processCtx, operation)
		return err
	case AsyncOperationCaseDelete:
		err = q.processCaseDelete(processCtx, operation)
		return err
	default:
		logger.Warnf("drop unsupported async operation type %q (operation_id=%s)", operation.Type, operation.OperationID)
		return nil
	}
}

func (q *KafkaAsyncOps) processAlertCreate(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid alert.create async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncAlertCreatePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed alert.create payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	alertID, err := uuid.Parse(strings.TrimSpace(payload.AlertID))
	if err != nil {
		logger.Warnf("drop alert.create with invalid alert_id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}

	item, err := q.alerts.Create(ctx, repository.CreateAlertParams{
		ID:          &alertID,
		TenantID:    tenantID,
		Title:       payload.Title,
		Description: payload.Description,
		Source:      payload.Source,
		Status:      payload.Status,
		Severity:    payload.Severity,
		TLP:         payload.TLP,
		PAP:         payload.PAP,
		CreatedBy:   &actorID,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("process alert.create: %w", err)
	}
	if q.search != nil {
		_ = q.search.IndexDocument(ctx, "alerts", item.ID.String(), item)
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "alert_create", "alert", &item.ID, map[string]any{
		"title": item.Title,
		"async": true,
	})
	if q.aiAgentQueue != nil {
		_, _, _ = q.aiAgentQueue.Enqueue(ctx, repository.EnqueueAIAgentQueueEventParams{
			TenantID:   tenantID,
			ActorID:    &actorID,
			EntityType: "alert",
			EntityID:   item.ID,
			Source:     "async_kafka",
		})
	}
	return nil
}

func (q *KafkaAsyncOps) processAlertUpdate(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid alert.update async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncAlertUpdatePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed alert.update payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	alertID, err := uuid.Parse(strings.TrimSpace(payload.AlertID))
	if err != nil {
		logger.Warnf("drop alert.update with invalid alert_id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	assignedTo, err := parseOptionalUUID(payload.AssignedTo)
	if err != nil {
		logger.Warnf("drop alert.update with invalid assigned_to (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}

	item, err := q.alerts.Update(ctx, tenantID, alertID, repository.UpdateAlertParams{
		Title:       payload.Title,
		Description: payload.Description,
		Source:      payload.Source,
		Status:      payload.Status,
		Severity:    payload.Severity,
		TLP:         payload.TLP,
		PAP:         payload.PAP,
		AssignedTo:  assignedTo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warnf("drop alert.update for missing alert (operation_id=%s alert_id=%s)", operation.OperationID, alertID.String())
			return nil
		}
		return fmt.Errorf("process alert.update: %w", err)
	}
	if q.search != nil {
		_ = q.search.IndexDocument(ctx, "alerts", item.ID.String(), item)
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "alert_update", "alert", &item.ID, map[string]any{
		"async": true,
	})
	return nil
}

func (q *KafkaAsyncOps) processAlertDelete(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid alert.delete async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncDeletePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed alert.delete payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	alertID, err := uuid.Parse(strings.TrimSpace(payload.ID))
	if err != nil {
		logger.Warnf("drop alert.delete with invalid id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	deleted, err := q.alerts.Delete(ctx, tenantID, alertID)
	if err != nil {
		return fmt.Errorf("process alert.delete: %w", err)
	}
	if q.search != nil {
		_ = q.search.DeleteDocument(ctx, "alerts", alertID.String())
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "alert_delete", "alert", &alertID, map[string]any{
		"deleted": deleted,
		"async":   true,
	})
	return nil
}

func (q *KafkaAsyncOps) processCaseCreate(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid case.create async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncCaseCreatePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed case.create payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	caseID, err := uuid.Parse(strings.TrimSpace(payload.CaseID))
	if err != nil {
		logger.Warnf("drop case.create with invalid case_id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	assignedTo, err := parseOptionalUUID(payload.AssignedTo)
	if err != nil {
		logger.Warnf("drop case.create with invalid assigned_to (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	item, err := q.cases.Create(ctx, repository.CreateCaseParams{
		ID:                &caseID,
		TenantID:          tenantID,
		CaseNumber:        payload.CaseNumber,
		Title:             payload.Title,
		Description:       payload.Description,
		Source:            payload.Source,
		IncidentType:      payload.IncidentType,
		Status:            payload.Status,
		Priority:          payload.Priority,
		Impact:            payload.Impact,
		Confidence:        payload.Confidence,
		Severity:          payload.Severity,
		TLP:               payload.TLP,
		PAP:               payload.PAP,
		DetectedAt:        optionalStringPtr(payload.DetectedAt),
		OccurredAt:        optionalStringPtr(payload.OccurredAt),
		ClosedAt:          optionalStringPtr(payload.ClosedAt),
		ResolutionSummary: payload.ResolutionSummary,
		CreatedBy:         actorID,
		AssignedTo:        assignedTo,
	})
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil
		}
		return fmt.Errorf("process case.create: %w", err)
	}
	copiedObservables := 0
	if payload.IncludeObservables {
		copiedObservables = q.copyAsyncCaseObservables(ctx, tenantID, strings.TrimSpace(payload.SourceCaseID), item.ID, actorID)
	}
	if q.search != nil {
		_ = q.search.IndexDocument(ctx, "cases", item.ID.String(), item)
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "case_create", "case", &item.ID, map[string]any{
		"title":               item.Title,
		"async":               true,
		"include_observables": payload.IncludeObservables,
		"copied_observables":  copiedObservables,
	})
	if q.aiAgentQueue != nil {
		_, _, _ = q.aiAgentQueue.Enqueue(ctx, repository.EnqueueAIAgentQueueEventParams{
			TenantID:   tenantID,
			ActorID:    &actorID,
			EntityType: "case",
			EntityID:   item.ID,
			Source:     "async_kafka",
		})
	}
	return nil
}

func (q *KafkaAsyncOps) copyAsyncCaseObservables(ctx context.Context, tenantID uuid.UUID, sourceCaseIDRaw string, targetCaseID, actorID uuid.UUID) int {
	if q == nil || q.observables == nil {
		return 0
	}
	if strings.TrimSpace(sourceCaseIDRaw) == "" {
		return 0
	}
	sourceCaseID, err := uuid.Parse(strings.TrimSpace(sourceCaseIDRaw))
	if err != nil {
		return 0
	}
	observables, err := q.observables.ListByCase(ctx, tenantID, sourceCaseID, 2000, 0)
	if err != nil {
		return 0
	}
	copied := 0
	for idx := range observables {
		sourceObservable := observables[idx]
		createdObservable, createErr := q.observables.Create(ctx, repository.CreateObservableParams{
			TenantID:  tenantID,
			CaseID:    targetCaseID,
			Type:      sourceObservable.Type,
			Value:     sourceObservable.Value,
			Verdict:   sourceObservable.Verdict,
			Source:    sourceObservable.Source,
			Tags:      sourceObservable.Tags,
			CreatedBy: actorID,
		})
		if createErr != nil {
			continue
		}
		copied++
		if q.search != nil {
			_ = q.search.IndexDocument(ctx, "observables", createdObservable.ID.String(), createdObservable)
		}
	}
	return copied
}

func (q *KafkaAsyncOps) processCaseUpdate(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid case.update async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncCaseUpdatePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed case.update payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	caseID, err := uuid.Parse(strings.TrimSpace(payload.CaseID))
	if err != nil {
		logger.Warnf("drop case.update with invalid case_id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	assignedTo, err := parseOptionalUUID(payload.AssignedTo)
	if err != nil {
		logger.Warnf("drop case.update with invalid assigned_to (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}

	item, err := q.cases.Update(ctx, tenantID, caseID, repository.UpdateCaseParams{
		CaseNumber:        payload.CaseNumber,
		Title:             payload.Title,
		Description:       payload.Description,
		Source:            payload.Source,
		IncidentType:      payload.IncidentType,
		Status:            payload.Status,
		Priority:          payload.Priority,
		Impact:            payload.Impact,
		Confidence:        payload.Confidence,
		Severity:          payload.Severity,
		TLP:               payload.TLP,
		PAP:               payload.PAP,
		DetectedAt:        payload.DetectedAt,
		OccurredAt:        payload.OccurredAt,
		ClosedAt:          payload.ClosedAt,
		ResolutionSummary: payload.ResolutionSummary,
		AssignedTo:        assignedTo,
		ClearAssignedTo:   payload.ClearAssignedTo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Warnf("drop case.update for missing case (operation_id=%s case_id=%s)", operation.OperationID, caseID.String())
			return nil
		}
		return fmt.Errorf("process case.update: %w", err)
	}
	if q.search != nil {
		_ = q.search.IndexDocument(ctx, "cases", item.ID.String(), item)
	}
	if payload.RewardCaseClosureEvent {
		rewardUserID := actorID
		if strings.TrimSpace(payload.RewardCaseClosureToID) != "" {
			if parsedUserID, parseErr := uuid.Parse(strings.TrimSpace(payload.RewardCaseClosureToID)); parseErr == nil {
				rewardUserID = parsedUserID
			}
		}
		q.rewardCaseClosure(ctx, tenantID, rewardUserID, item.ID, item.Severity)
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "case_update", "case", &item.ID, map[string]any{
		"async": true,
	})
	return nil
}

func (q *KafkaAsyncOps) processCaseDelete(ctx context.Context, operation AsyncOperation) error {
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		logger.Warnf("drop invalid case.delete async operation (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}
	var payload asyncDeletePayload
	if unmarshalErr := json.Unmarshal(operation.Payload, &payload); unmarshalErr != nil {
		logger.Warnf("drop malformed case.delete payload (operation_id=%s): %v", operation.OperationID, unmarshalErr)
		return nil
	}
	caseID, err := uuid.Parse(strings.TrimSpace(payload.ID))
	if err != nil {
		logger.Warnf("drop case.delete with invalid id (operation_id=%s): %v", operation.OperationID, err)
		return nil
	}

	linkedObservableIDs := make([]uuid.UUID, 0)
	if q.observables != nil {
		if linkedObservables, listErr := q.observables.ListByCase(ctx, tenantID, caseID, 2000, 0); listErr == nil {
			for idx := range linkedObservables {
				linkedObservableIDs = append(linkedObservableIDs, linkedObservables[idx].ID)
			}
		}
	}

	linkedAlertIDs := make([]uuid.UUID, 0)
	if linkedAlerts, listErr := q.alerts.ListByCase(ctx, tenantID, caseID, 2000, 0); listErr == nil {
		for idx := range linkedAlerts {
			linkedAlertIDs = append(linkedAlertIDs, linkedAlerts[idx].ID)
		}
	}

	deleted, err := q.cases.Delete(ctx, tenantID, caseID)
	if err != nil {
		return fmt.Errorf("process case.delete: %w", err)
	}
	if q.search != nil {
		_ = q.search.DeleteDocument(ctx, "cases", caseID.String())
		for _, observableID := range linkedObservableIDs {
			_ = q.search.DeleteDocument(ctx, "observables", observableID.String())
		}
		if len(linkedAlertIDs) > 0 {
			if affectedAlerts, listErr := q.alerts.ListByIDs(ctx, tenantID, linkedAlertIDs); listErr == nil {
				for idx := range affectedAlerts {
					item := affectedAlerts[idx]
					_ = q.search.IndexDocument(ctx, "alerts", item.ID.String(), item)
				}
			}
		}
	}
	_ = q.audits.Log(ctx, &tenantID, &actorID, "case_delete", "case", &caseID, map[string]any{
		"deleted": deleted,
		"async":   true,
	})
	return nil
}

func parseAsyncOperationMessage(message *kafka.Message) (AsyncOperation, bool) {
	if message == nil {
		return AsyncOperation{}, false
	}
	var operation AsyncOperation
	if err := json.Unmarshal(message.Value, &operation); err != nil {
		logger.Warnf("drop malformed async operation payload: %v", err)
		return AsyncOperation{}, false
	}
	if strings.TrimSpace(operation.OperationID) == "" {
		logger.Warnf("drop async operation payload without operation_id")
		return AsyncOperation{}, false
	}
	return operation, true
}

func parseAsyncTenantAndActor(operation AsyncOperation) (tenantID uuid.UUID, actorID uuid.UUID, err error) {
	tenantID, err = uuid.Parse(strings.TrimSpace(operation.TenantID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid tenant id: %w", err)
	}
	actorID, err = uuid.Parse(strings.TrimSpace(operation.ActorID))
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid actor id: %w", err)
	}
	return tenantID, actorID, nil
}

func (q *KafkaAsyncOps) produceMessage(ctx context.Context, topic string, key, value []byte, headers []kafka.Header) error {
	return produceKafkaMessage(ctx, q.producer, topic, key, value, headers, q.produceTimeout)
}

func (q *KafkaAsyncOps) requeueMessage(ctx context.Context, original *kafka.Message, attempt int) error {
	if original == nil {
		return errKafkaAsyncOpsRequeueMessageNil
	}
	headers := withRetryHeader(original.Headers, attempt)
	key := append([]byte(nil), original.Key...)
	value := append([]byte(nil), original.Value...)
	return q.produceMessage(ctx, q.topic, key, value, headers)
}

func (q *KafkaAsyncOps) publishToDLQ(ctx context.Context, original *kafka.Message, operation AsyncOperation, attempt int, failureErr error) error {
	key := []byte(operation.OperationID)
	if strings.TrimSpace(operation.OperationID) == "" {
		key = nil
	}
	return publishKafkaDLQMessage(
		ctx,
		q.producer,
		q.produceTimeout,
		q.dlqTopic,
		q.topic,
		original,
		key,
		"operation",
		operation,
		attempt,
		failureErr,
		"marshal dlq payload",
	)
}

func (q *KafkaAsyncOps) retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := q.baseBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= q.maxBackoff {
			backoff = q.maxBackoff
			break
		}
	}
	if backoff <= 0 {
		backoff = q.baseBackoff
	}
	jitterCap := backoff / 4
	if jitterCap > 0 {
		jitter, err := crand.Int(crand.Reader, big.NewInt(int64(jitterCap)+1))
		if err == nil {
			backoff += time.Duration(jitter.Int64())
		}
	}
	if backoff > q.maxBackoff {
		return q.maxBackoff
	}
	return backoff
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	if ctx == nil {
		time.Sleep(delay)
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func kafkaHeaderInt(headers []kafka.Header) int {
	for _, header := range headers {
		if !strings.EqualFold(strings.TrimSpace(header.Key), kafkaRetryAttemptHeader) {
			continue
		}
		value := strings.TrimSpace(string(header.Value))
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0
		}
		if parsed < 0 {
			return 0
		}
		return parsed
	}
	return 0
}

func withRetryHeader(headers []kafka.Header, attempt int) []kafka.Header {
	if attempt < 0 {
		attempt = 0
	}
	out := make([]kafka.Header, 0, len(headers)+1)
	for _, header := range headers {
		if strings.EqualFold(strings.TrimSpace(header.Key), kafkaRetryAttemptHeader) {
			continue
		}
		out = append(out, header)
	}
	out = append(out, kafka.Header{Key: kafkaRetryAttemptHeader, Value: []byte(strconv.Itoa(attempt))})
	return out
}

func (q *KafkaAsyncOps) commitConsumedMessage(message *kafka.Message) {
	if message == nil {
		return
	}
	if _, err := q.consumer.CommitMessage(message); err != nil {
		logger.Errorf("async operation commit failed: %v", err)
	}
}

func (q *KafkaAsyncOps) markOperationProcessing(operation AsyncOperation, attempt int) {
	if q.operations == nil {
		return
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return
	}
	if err := q.operations.MarkProcessing(context.Background(), operationID, attempt); err != nil {
		logger.Warnf("mark operation processing failed: %v", err)
	}
}

func (q *KafkaAsyncOps) markOperationRetryQueued(operation AsyncOperation, attempt int, operationErr error) {
	if q.operations == nil {
		return
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return
	}
	lastError := ""
	if operationErr != nil {
		lastError = operationErr.Error()
	}
	if err := q.operations.MarkRetryQueued(context.Background(), operationID, attempt, lastError); err != nil {
		logger.Warnf("mark operation retry queued failed: %v", err)
	}
}

func (q *KafkaAsyncOps) markOperationDone(operation AsyncOperation) {
	if q.operations == nil {
		return
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return
	}
	if err := q.operations.MarkDone(context.Background(), operationID); err != nil {
		logger.Warnf("mark operation done failed: %v", err)
	}
}

func (q *KafkaAsyncOps) markOperationFailed(operation AsyncOperation, attempt int, operationErr error) {
	if q.operations == nil {
		return
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return
	}
	lastError := ""
	if operationErr != nil {
		lastError = operationErr.Error()
	}
	if err := q.operations.MarkFailed(context.Background(), operationID, attempt, lastError); err != nil {
		logger.Warnf("mark operation failed failed: %v", err)
	}
}

func (q *KafkaAsyncOps) rewardCaseClosure(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, caseID uuid.UUID, severity string) {
	if q.experience == nil {
		return
	}

	ruleKey := xpRuleKeyForSeverity(severity)
	points, err := q.experience.RulePoints(ctx, ruleKey)
	if err != nil {
		points = fallbackRewardRulePoints(ruleKey)
	}
	if points <= 0 {
		return
	}

	details := map[string]any{
		"case_id":   caseID.String(),
		"severity":  normalizeSeverityForXP(severity),
		"rule_key":  ruleKey,
		"triggered": xpEventTypeCaseClosed,
		"async":     true,
	}
	_, err = q.experience.Award(ctx, repository.AwardExperienceParams{
		TenantID:    &tenantID,
		UserID:      userID,
		EventKey:    "case_closed:" + caseID.String(),
		EventType:   xpEventTypeCaseClosed,
		Points:      points,
		Description: defaultExperienceDescription(xpEventTypeCaseClosed, details),
		Details:     details,
	})
	if err != nil {
		logger.Warnf("async case closure reward failed (case_id=%s user_id=%s): %v", caseID.String(), userID.String(), err)
	}
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalStringPtr(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func isPGUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}
