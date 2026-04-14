package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type runInput struct {
	Config    config
	Client    *apiClient
	Session   authSession
	Scenario  scenario
	Fixtures  fixtureData
	TargetRPS int
}

func runStep(ctx context.Context, input runInput) stepResult {
	workerCount := input.Config.Workers
	if workerCount > input.TargetRPS*2 {
		workerCount = input.TargetRPS * 2
	}
	if workerCount < 1 {
		workerCount = 1
	}

	interval := time.Second / time.Duration(input.TargetRPS)
	tokens := make(chan uint64)
	results := make(chan requestSample, workerCount*4)

	startedAt := time.Now()
	var workerWG sync.WaitGroup
	for idx := 0; idx < workerCount; idx++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for seq := range tokens {
				sample := executeSingleRequest(ctx, input, seq)
				results <- sample
			}
		}()
	}

	go func() {
		defer close(tokens)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		stopTimer := time.NewTimer(input.Config.StepDuration)
		defer stopTimer.Stop()

		var sequence uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopTimer.C:
				return
			case <-ticker.C:
				sequence++
				tokens <- sequence
			}
		}
	}()

	go func() {
		workerWG.Wait()
		close(results)
	}()

	samples := make([]requestSample, 0, input.TargetRPS*int(input.Config.StepDuration.Seconds()))
	for sample := range results {
		samples = append(samples, sample)
	}
	finishedAt := time.Now()
	stepDuration := finishedAt.Sub(startedAt)

	return summarizeStep(samples, stepDuration, input)
}

func executeSingleRequest(ctx context.Context, input runInput, seq uint64) requestSample {
	path, body, contentType := input.Scenario.Build(seq, input.Fixtures)
	start := time.Now()
	resp, err := input.Client.request(
		ctx,
		input.Scenario.Method,
		path,
		body,
		contentType,
		input.Session.Token,
		input.Session.TenantID,
		input.Scenario.RequireTenant,
	)
	if err != nil {
		return requestSample{
			Latency:   time.Since(start),
			Status:    0,
			Err:       err.Error(),
			StartedAt: start,
			EndedAt:   time.Now(),
		}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	errMessage := ""
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMessage = fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	end := time.Now()
	return requestSample{
		Latency:   end.Sub(start),
		Status:    resp.StatusCode,
		Err:       errMessage,
		StartedAt: start,
		EndedAt:   end,
	}
}

func summarizeStep(samples []requestSample, duration time.Duration, input runInput) stepResult {
	statusStats := map[string]int{}
	errorStats := map[string]int{}
	latencies := make([]time.Duration, 0, len(samples))

	successCount := 0
	failedCount := 0
	for _, sample := range samples {
		statusKey := fmt.Sprintf("%d", sample.Status)
		statusStats[statusKey]++
		latencies = append(latencies, sample.Latency)
		if strings.TrimSpace(sample.Err) == "" {
			successCount++
			continue
		}
		failedCount++
		errorStats[sample.Err]++
	}

	if len(latencies) == 0 {
		latencies = append(latencies, 0)
	}
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	totalCount := len(samples)
	if totalCount == 0 {
		totalCount = 1
	}
	successRate := float64(successCount) / float64(totalCount)
	errorRate := float64(failedCount) / float64(totalCount)
	achievedRPS := float64(len(samples)) / math.Max(duration.Seconds(), 0.001)

	p50 := percentile(latencies, 0.50)
	p95 := percentile(latencies, 0.95)
	p99 := percentile(latencies, 0.99)
	avg, minLatency, maxLatency := aggregateLatency(latencies)

	failReasons := make([]string, 0, 4)
	if errorRate > input.Config.ErrorThreshold {
		failReasons = append(failReasons, fmt.Sprintf("error_rate %.4f > threshold %.4f", errorRate, input.Config.ErrorThreshold))
	}
	if p95 > input.Config.P95Threshold {
		failReasons = append(failReasons, fmt.Sprintf("p95 %s > threshold %s", p95, input.Config.P95Threshold))
	}
	achievedRatio := achievedRPS / float64(input.TargetRPS)
	if achievedRatio < 0.9 {
		failReasons = append(failReasons, fmt.Sprintf("achieved_rps %.2f is less than 90%% of target %d", achievedRPS, input.TargetRPS))
	}

	return stepResult{
		TargetRPS:       input.TargetRPS,
		AchievedRPS:     roundFloat(achievedRPS, 2),
		TotalRequests:   len(samples),
		Successful:      successCount,
		Failed:          failedCount,
		SuccessRate:     roundFloat(successRate, 6),
		ErrorRate:       roundFloat(errorRate, 6),
		P50LatencyMS:    roundFloat(durationToMS(p50), 3),
		P95LatencyMS:    roundFloat(durationToMS(p95), 3),
		P99LatencyMS:    roundFloat(durationToMS(p99), 3),
		AvgLatencyMS:    roundFloat(durationToMS(avg), 3),
		MinLatencyMS:    roundFloat(durationToMS(minLatency), 3),
		MaxLatencyMS:    roundFloat(durationToMS(maxLatency), 3),
		DurationMS:      duration.Milliseconds(),
		StatusCodeStats: statusStats,
		TopErrors:       topErrors(errorStats, 5),
		Passed:          len(failReasons) == 0,
		FailReasons:     failReasons,
	}
}

func percentile(values []time.Duration, q float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return values[0]
	}
	if q >= 1 {
		return values[len(values)-1]
	}
	position := int(math.Ceil(float64(len(values))*q)) - 1
	if position < 0 {
		position = 0
	}
	if position >= len(values) {
		position = len(values) - 1
	}
	return values[position]
}

func aggregateLatency(values []time.Duration) (avg time.Duration, minLatency time.Duration, maxLatency time.Duration) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	minLatency = values[0]
	maxLatency = values[0]
	var total time.Duration
	for _, value := range values {
		total += value
		if value < minLatency {
			minLatency = value
		}
		if value > maxLatency {
			maxLatency = value
		}
	}
	avg = time.Duration(int64(total) / int64(len(values)))
	return avg, minLatency, maxLatency
}

func topErrors(input map[string]int, maxItems int) map[string]int {
	if len(input) <= maxItems {
		return input
	}

	type pair struct {
		Key   string
		Count int
	}
	items := make([]pair, 0, len(input))
	for key, count := range input {
		items = append(items, pair{Key: key, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})

	out := make(map[string]int, maxItems)
	for idx := 0; idx < maxItems && idx < len(items); idx++ {
		out[items[idx].Key] = items[idx].Count
	}
	return out
}

func durationToMS(value time.Duration) float64 {
	return float64(value.Microseconds()) / 1000.0
}

func roundFloat(value float64, digits int) float64 {
	if digits <= 0 {
		return math.Round(value)
	}
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
