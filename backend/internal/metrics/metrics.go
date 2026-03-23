package metrics

import (
	"incidenthub/backend/internal/models"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collector struct {
	HTTPRequestsTotal      *prometheus.CounterVec
	HTTPRequestDuration    *prometheus.HistogramVec
	AuthAttemptsTotal      *prometheus.CounterVec
	DataAccessDenied       *prometheus.CounterVec
	ModuleOperations       *prometheus.CounterVec
	ModuleDuration         *prometheus.HistogramVec
	ModuleHealthStatus     *prometheus.GaugeVec
	ModuleHealthLatency    *prometheus.GaugeVec
	registry               *prometheus.Registry
	reqMu                  sync.Mutex
	requestBuckets         map[string]map[int64]uint64
	moduleMu               sync.Mutex
	moduleBuckets          map[string]map[int64]moduleOperationBucket
	moduleOperationBuckets map[string]map[string]map[int64]moduleOperationBucket
}

//nolint:gochecknoglobals // Global collector simplifies metrics access across middleware.
var globalCollector atomic.Pointer[Collector]

type moduleOperationBucket struct {
	Count          uint64
	ErrorCount     uint64
	DurationMicros uint64
}

type ModuleOperationSnapshot struct {
	Module            string
	WindowSeconds     int64
	Operations        int64
	Errors            int64
	RequestsPerSecond float64
	AvgLatencyMs      float64
}

func New(namespace string) *Collector {
	r := prometheus.NewRegistry()
	c := &Collector{
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration seconds",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		}, []string{"method", "path"}),
		AuthAttemptsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "auth_attempts_total",
			Help:      "Authentication attempts",
		}, []string{"result"}),
		DataAccessDenied: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "access_denied_total",
			Help:      "Denied access attempts",
		}, []string{"reason"}),
		ModuleOperations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "module_operations_total",
			Help:      "Module operation calls by module, operation and status",
		}, []string{"module", "operation", "status"}),
		ModuleDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "module_operation_duration_seconds",
			Help:      "Duration of module operations in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30},
		}, []string{"module", "operation"}),
		ModuleHealthStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "module_health_status",
			Help:      "Module health status: ok=1, disabled=0, error=-1",
		}, []string{"module"}),
		ModuleHealthLatency: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "module_health_response_ms",
			Help:      "Module health check response time in milliseconds",
		}, []string{"module"}),
		registry:               r,
		requestBuckets:         map[string]map[int64]uint64{},
		moduleBuckets:          map[string]map[int64]moduleOperationBucket{},
		moduleOperationBuckets: map[string]map[string]map[int64]moduleOperationBucket{},
	}

	r.MustRegister(
		c.HTTPRequestsTotal,
		c.HTTPRequestDuration,
		c.AuthAttemptsTotal,
		c.DataAccessDenied,
		c.ModuleOperations,
		c.ModuleDuration,
		c.ModuleHealthStatus,
		c.ModuleHealthLatency,
	)
	return c
}

func (c *Collector) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx *echo.Context) error {
			start := time.Now()
			err := next(ctx)
			status := http.StatusOK
			if resp, unwrapErr := echo.UnwrapResponse(ctx.Response()); unwrapErr == nil && resp.Status > 0 {
				status = resp.Status
			}
			method := ctx.Request().Method
			path := ctx.Path()
			if path == "" {
				path = "unmatched"
			}

			c.HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
			c.HTTPRequestDuration.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
			if strings.HasPrefix(ctx.Request().URL.Path, "/api/") {
				c.recordRequest(resolveRequestTenantID(ctx), time.Now().UTC())
			}
			return err
		}
	}
}

func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

func (c *Collector) RequestsLast24h(tenantID string) int64 {
	if c == nil {
		return 0
	}
	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	currentHour := hourBucket(time.Now().UTC())
	oldest := currentHour - 23
	target := normalizeTenantBucket(tenantID)

	var total uint64
	if target == globalTenantBucket {
		for key := range c.requestBuckets {
			total += sumBucketRange(c.requestBuckets[key], oldest, currentHour)
		}
		return safeInt64FromUint64(total)
	}
	return safeInt64FromUint64(sumBucketRange(c.requestBuckets[target], oldest, currentHour))
}

func (c *Collector) recordRequest(tenantID string, ts time.Time) {
	if c == nil {
		return
	}
	key := normalizeTenantBucket(tenantID)
	bucket := hourBucket(ts.UTC())
	cutoff := bucket - 48

	c.reqMu.Lock()
	defer c.reqMu.Unlock()

	tenantBuckets, ok := c.requestBuckets[key]
	if !ok {
		tenantBuckets = map[int64]uint64{}
		c.requestBuckets[key] = tenantBuckets
	}
	tenantBuckets[bucket]++

	for hour := range tenantBuckets {
		if hour < cutoff {
			delete(tenantBuckets, hour)
		}
	}
}

func (c *Collector) ObserveModuleOperation(module, operation, status string, duration time.Duration) {
	if c == nil {
		return
	}
	module = normalizeMetricLabel(module, "unknown")
	operation = normalizeMetricLabel(operation, "unknown")
	status = normalizeMetricLabel(status, "ok")
	c.ModuleOperations.WithLabelValues(module, operation, status).Inc()
	if duration < 0 {
		duration = 0
	}
	c.ModuleDuration.WithLabelValues(module, operation).Observe(duration.Seconds())
	c.recordModuleOperation(module, operation, status, duration, time.Now().UTC())
}

func (c *Collector) SetModuleHealth(module, status string, latencyMs float64) {
	if c == nil {
		return
	}
	module = normalizeMetricLabel(module, "unknown")
	status = normalizeMetricLabel(status, "ok")
	healthValue := -1.0
	switch status {
	case "ok":
		healthValue = 1
	case "disabled":
		healthValue = 0
	case "error":
		healthValue = -1
	}
	if latencyMs < 0 {
		latencyMs = 0
	}
	c.ModuleHealthStatus.WithLabelValues(module).Set(healthValue)
	c.ModuleHealthLatency.WithLabelValues(module).Set(latencyMs)
}

func SetGlobal(collector *Collector) {
	globalCollector.Store(collector)
}

func Global() *Collector {
	return globalCollector.Load()
}

func ObserveModuleOperation(module, operation, status string, duration time.Duration) {
	if collector := globalCollector.Load(); collector != nil {
		collector.ObserveModuleOperation(module, operation, status, duration)
	}
}

func SetModuleHealth(module, status string, latencyMs float64) {
	if collector := globalCollector.Load(); collector != nil {
		collector.SetModuleHealth(module, status, latencyMs)
	}
}

func normalizeMetricLabel(value string, fallback string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func resolveRequestTenantID(ctx *echo.Context) string {
	if ctx == nil {
		return ""
	}
	if headerTenant := strings.TrimSpace(ctx.Request().Header.Get("X-Tenant-ID")); headerTenant != "" {
		return headerTenant
	}
	if contextTenant := tenantIDFromContextValue(ctx.Get("tenant_id")); contextTenant != "" {
		return contextTenant
	}
	if identity, ok := ctx.Get("identity").(models.Identity); ok && identity.TenantID != nil && *identity.TenantID != uuid.Nil {
		return identity.TenantID.String()
	}
	return ""
}

func tenantIDFromContextValue(raw any) string {
	switch typed := raw.(type) {
	case uuid.UUID:
		if typed == uuid.Nil {
			return ""
		}
		return typed.String()
	case *uuid.UUID:
		if typed == nil || *typed == uuid.Nil {
			return ""
		}
		return typed.String()
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func sumBucketRange(buckets map[int64]uint64, startHour, endHour int64) uint64 {
	if len(buckets) == 0 {
		return 0
	}
	var total uint64
	for hour, value := range buckets {
		if hour < startHour || hour > endHour {
			continue
		}
		total += value
	}
	return total
}

func hourBucket(ts time.Time) int64 {
	return ts.Unix() / 3600
}

func secondBucket(ts time.Time) int64 {
	return ts.Unix()
}

func safeUint64FromInt64(value int64) uint64 {
	if value <= 0 {
		return 0
	}
	return uint64(value)
}

func safeInt64FromUint64(value uint64) int64 {
	if value > uint64(math.MaxInt64) {
		return math.MaxInt64
	}
	return int64(value)
}

func (c *Collector) recordModuleOperation(module, operation, status string, duration time.Duration, ts time.Time) {
	if c == nil {
		return
	}
	bucket := secondBucket(ts.UTC())
	cutoff := bucket - 3600
	isError := strings.EqualFold(strings.TrimSpace(status), "error")
	durationMicros := duration.Microseconds()
	if durationMicros < 0 {
		durationMicros = 0
	}

	c.moduleMu.Lock()
	defer c.moduleMu.Unlock()

	series, ok := c.moduleBuckets[module]
	if !ok {
		series = map[int64]moduleOperationBucket{}
		c.moduleBuckets[module] = series
	}
	point := series[bucket]
	point.Count++
	if isError {
		point.ErrorCount++
	}
	point.DurationMicros += safeUint64FromInt64(durationMicros)
	series[bucket] = point

	for sec := range series {
		if sec < cutoff {
			delete(series, sec)
		}
	}

	operationSeriesByModule, ok := c.moduleOperationBuckets[module]
	if !ok {
		operationSeriesByModule = map[string]map[int64]moduleOperationBucket{}
		c.moduleOperationBuckets[module] = operationSeriesByModule
	}
	operationSeries, ok := operationSeriesByModule[operation]
	if !ok {
		operationSeries = map[int64]moduleOperationBucket{}
		operationSeriesByModule[operation] = operationSeries
	}
	operationPoint := operationSeries[bucket]
	operationPoint.Count++
	if isError {
		operationPoint.ErrorCount++
	}
	operationPoint.DurationMicros += safeUint64FromInt64(durationMicros)
	operationSeries[bucket] = operationPoint

	for sec := range operationSeries {
		if sec < cutoff {
			delete(operationSeries, sec)
		}
	}
}

func (c *Collector) ModuleOperationSnapshot(module string, window time.Duration) ModuleOperationSnapshot {
	if c == nil {
		return ModuleOperationSnapshot{}
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	windowSeconds := int64(window / time.Second)
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	now := time.Now().UTC()
	startBucket := secondBucket(now) - windowSeconds + 1
	endBucket := secondBucket(now)

	normalizedModule := normalizeMetricLabel(module, "unknown")
	var (
		totalCount          uint64
		totalErrors         uint64
		totalDurationMicros uint64
	)

	c.moduleMu.Lock()
	defer c.moduleMu.Unlock()

	accumulate := func(series map[int64]moduleOperationBucket) {
		for sec, point := range series {
			if sec < startBucket || sec > endBucket {
				continue
			}
			totalCount += point.Count
			totalErrors += point.ErrorCount
			totalDurationMicros += point.DurationMicros
		}
	}

	switch normalizedModule {
	case "", "*", "all", "__all__":
		for moduleName := range c.moduleBuckets {
			accumulate(c.moduleBuckets[moduleName])
		}
		normalizedModule = "*"
	default:
		accumulate(c.moduleBuckets[normalizedModule])
	}

	snapshot := ModuleOperationSnapshot{
		Module:        normalizedModule,
		WindowSeconds: windowSeconds,
		Operations:    safeInt64FromUint64(totalCount),
		Errors:        safeInt64FromUint64(totalErrors),
	}
	snapshot.RequestsPerSecond = float64(totalCount) / float64(windowSeconds)
	if totalCount > 0 {
		snapshot.AvgLatencyMs = (float64(totalDurationMicros) / float64(totalCount)) / 1000.0
	}
	return snapshot
}

func (c *Collector) ModuleOperationSnapshotByOperation(module, operation string, window time.Duration) ModuleOperationSnapshot {
	if c == nil {
		return ModuleOperationSnapshot{}
	}
	if strings.TrimSpace(operation) == "" {
		return c.ModuleOperationSnapshot(module, window)
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	windowSeconds := int64(window / time.Second)
	if windowSeconds < 1 {
		windowSeconds = 1
	}
	now := time.Now().UTC()
	startBucket := secondBucket(now) - windowSeconds + 1
	endBucket := secondBucket(now)

	normalizedModule := normalizeMetricLabel(module, "unknown")
	normalizedOperation := normalizeMetricLabel(operation, "unknown")
	var (
		totalCount          uint64
		totalErrors         uint64
		totalDurationMicros uint64
	)

	c.moduleMu.Lock()
	defer c.moduleMu.Unlock()

	operationSeriesByModule := c.moduleOperationBuckets[normalizedModule]
	for sec, point := range operationSeriesByModule[normalizedOperation] {
		if sec < startBucket || sec > endBucket {
			continue
		}
		totalCount += point.Count
		totalErrors += point.ErrorCount
		totalDurationMicros += point.DurationMicros
	}

	snapshot := ModuleOperationSnapshot{
		Module:        normalizedModule,
		WindowSeconds: windowSeconds,
		Operations:    safeInt64FromUint64(totalCount),
		Errors:        safeInt64FromUint64(totalErrors),
	}
	snapshot.RequestsPerSecond = float64(totalCount) / float64(windowSeconds)
	if totalCount > 0 {
		snapshot.AvgLatencyMs = (float64(totalDurationMicros) / float64(totalCount)) / 1000.0
	}
	return snapshot
}

const globalTenantBucket = "__all__"

func normalizeTenantBucket(tenantID string) string {
	trimmed := strings.TrimSpace(tenantID)
	if trimmed == "" {
		return globalTenantBucket
	}
	return trimmed
}
