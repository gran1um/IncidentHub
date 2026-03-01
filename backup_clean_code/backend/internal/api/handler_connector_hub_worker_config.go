package api

import "time"

func (h *Handler) connectorHubEnabled() bool {
	return h.cfg.Outbound.HubEnabled
}

func (h *Handler) connectorHubPollInterval() time.Duration {
	if h.cfg.Outbound.HubPollInterval > 0 {
		return h.cfg.Outbound.HubPollInterval
	}
	return 3 * time.Second
}

func (h *Handler) connectorHubBatchSize() int {
	if h.cfg.Outbound.HubBatchSize > 0 {
		return h.cfg.Outbound.HubBatchSize
	}
	return 10
}

func (h *Handler) connectorHubRunTimeout() time.Duration {
	if h.cfg.Outbound.HubRunTimeout > 0 {
		return h.cfg.Outbound.HubRunTimeout
	}
	return 45 * time.Second
}

func (h *Handler) connectorHubMaxAttempts() int {
	if h.cfg.Outbound.HubMaxAttempts > 0 {
		return h.cfg.Outbound.HubMaxAttempts
	}
	return 3
}

func (h *Handler) connectorHubRetryBaseBackoff() time.Duration {
	if h.cfg.Outbound.HubRetryBaseBackoff > 0 {
		return h.cfg.Outbound.HubRetryBaseBackoff
	}
	return 2 * time.Second
}

func (h *Handler) connectorHubRetryMaxBackoff() time.Duration {
	if h.cfg.Outbound.HubRetryMaxBackoff > 0 {
		return h.cfg.Outbound.HubRetryMaxBackoff
	}
	return 30 * time.Second
}

func (h *Handler) connectorHubRetryBackoff(attemptCount int) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}
	backoff := h.connectorHubRetryBaseBackoff()
	for i := 1; i < attemptCount; i++ {
		backoff *= 2
		if backoff >= h.connectorHubRetryMaxBackoff() {
			return h.connectorHubRetryMaxBackoff()
		}
	}
	if backoff > h.connectorHubRetryMaxBackoff() {
		return h.connectorHubRetryMaxBackoff()
	}
	return backoff
}
