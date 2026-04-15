package main

import "time"

type authSession struct {
	Token    string
	TenantID string
}

type fixtureData struct {
	CaseID          string
	OpenCaseStatus  string
	SearchProbeTerm string
}

type scenario struct {
	Name          string
	Category      string
	Method        string
	PathTemplate  string
	RequireTenant bool
	Build         func(seq uint64, fixtures fixtureData) (string, []byte, string)
}

type requestSample struct {
	Latency   time.Duration
	Status    int
	Err       string
	StartedAt time.Time
	EndedAt   time.Time
}

type stepResult struct {
	TargetRPS       int                `json:"target_rps"`
	AchievedRPS     float64            `json:"achieved_rps"`
	TotalRequests   int                `json:"total_requests"`
	Successful      int                `json:"successful"`
	Failed          int                `json:"failed"`
	SuccessRate     float64            `json:"success_rate"`
	ErrorRate       float64            `json:"error_rate"`
	P50LatencyMS    float64            `json:"p50_latency_ms"`
	P95LatencyMS    float64            `json:"p95_latency_ms"`
	P99LatencyMS    float64            `json:"p99_latency_ms"`
	AvgLatencyMS    float64            `json:"avg_latency_ms"`
	MinLatencyMS    float64            `json:"min_latency_ms"`
	MaxLatencyMS    float64            `json:"max_latency_ms"`
	DurationMS      int64              `json:"duration_ms"`
	StatusCodeStats map[string]int     `json:"status_code_stats"`
	TopErrors       map[string]int     `json:"top_errors"`
	Passed          bool               `json:"passed"`
	FailReasons     []string           `json:"fail_reasons"`
	Samples         []requestSampleLog `json:"samples,omitempty"`
}

type requestSampleLog struct {
	Status     int     `json:"status"`
	Error      string  `json:"error,omitempty"`
	LatencyMS  float64 `json:"latency_ms"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at"`
}

type scenarioReport struct {
	Name              string       `json:"name"`
	Category          string       `json:"category"`
	Method            string       `json:"method"`
	PathTemplate      string       `json:"path_template"`
	MaxSustainableRPS int          `json:"max_sustainable_rps"`
	Steps             []stepResult `json:"steps"`
}

type loadReport struct {
	GeneratedAtUTC string           `json:"generated_at_utc"`
	BaseURL        string           `json:"base_url"`
	TenantID       string           `json:"tenant_id"`
	StepDuration   string           `json:"step_duration"`
	Cooldown       string           `json:"cooldown"`
	ErrorThreshold float64          `json:"error_threshold"`
	P95ThresholdMS float64          `json:"p95_threshold_ms"`
	Workers        int              `json:"workers"`
	Scenarios      []scenarioReport `json:"scenarios"`
}
