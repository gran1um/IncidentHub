package api

import (
	"context"
	"time"

	"incidenthub/backend/internal/logger"
)

func (h *Handler) startConnectorHubWorker(ctx context.Context) {
	if h.connectorHubExecutions == nil {
		return
	}
	if !h.connectorHubEnabled() {
		return
	}
	go h.runConnectorHubLoop(ctx)
}

func (h *Handler) runConnectorHubLoop(ctx context.Context) {
	h.processNextConnectorHubBatch(ctx, h.connectorHubBatchSize())
	ticker := time.NewTicker(h.connectorHubPollInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.processNextConnectorHubBatch(ctx, h.connectorHubBatchSize())
		}
	}
}

func (h *Handler) processNextConnectorHubBatch(ctx context.Context, limit int) int {
	if h == nil || h.connectorHubExecutions == nil {
		return 0
	}
	h.recoverStaleConnectorHubDispatches(ctx)
	executions, err := h.connectorHubExecutions.ClaimReadyBatch(ctx, limit)
	if err != nil {
		logger.Warnf("connector hub claim batch failed: %v", err)
		return 0
	}
	processed := 0
	for _, execution := range executions {
		runCtx, cancel := context.WithTimeout(ctx, h.connectorHubRunTimeout())
		if err := h.processConnectorHubExecution(runCtx, execution); err != nil {
			logger.Warnf("connector hub execution failed: execution=%s err=%v", execution.ID.String(), err)
		}
		cancel()
		processed++
	}
	return processed
}
