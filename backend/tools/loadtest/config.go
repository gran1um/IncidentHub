package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type config struct {
	BaseURL           string
	Email             string
	Password          string
	TenantID          string
	OutputPath        string
	StartRPS          int
	StepRPS           int
	MaxRPS            int
	StepDuration      time.Duration
	Cooldown          time.Duration
	Workers           int
	HTTPTimeout       time.Duration
	ErrorThreshold    float64
	P95Threshold      time.Duration
	StopOnFail        bool
	InsecureSkipTLS   bool
	IncludeCaseWrites bool
}

func parseConfig() (config, error) {
	nowStamp := time.Now().UTC().Format("20060102-150405")
	defaultOutput := filepath.Join("loadtest", "results", fmt.Sprintf("report-%s.json", nowStamp))

	cfg := config{}
	flag.StringVar(&cfg.BaseURL, "base-url", firstNonEmptyEnv("LOADTEST_BASE_URL", "http://localhost:8080"), "API base URL")
	flag.StringVar(&cfg.Email, "email", firstNonEmptyEnv("LOADTEST_EMAIL", "admin@incidenthub.local"), "login email")
	flag.StringVar(&cfg.Password, "password", firstNonEmptyEnv("LOADTEST_PASSWORD", "ChangeMeNow123!"), "login password")
	flag.StringVar(&cfg.TenantID, "tenant-id", strings.TrimSpace(os.Getenv("LOADTEST_TENANT_ID")), "tenant ID override")
	flag.StringVar(&cfg.OutputPath, "output", firstNonEmptyEnv("LOADTEST_OUTPUT", defaultOutput), "output JSON report path")
	flag.IntVar(&cfg.StartRPS, "start-rps", envInt("LOADTEST_START_RPS", 25), "initial RPS per route")
	flag.IntVar(&cfg.StepRPS, "step-rps", envInt("LOADTEST_STEP_RPS", 25), "RPS increment per step")
	flag.IntVar(&cfg.MaxRPS, "max-rps", envInt("LOADTEST_MAX_RPS", 500), "max target RPS per route")
	flag.DurationVar(&cfg.StepDuration, "step-duration", envDuration("LOADTEST_STEP_DURATION", 20*time.Second), "single RPS step duration")
	flag.DurationVar(&cfg.Cooldown, "cooldown", envDuration("LOADTEST_COOLDOWN", 2*time.Second), "cooldown between steps")
	flag.IntVar(&cfg.Workers, "workers", envInt("LOADTEST_WORKERS", 128), "concurrent workers")
	flag.DurationVar(&cfg.HTTPTimeout, "http-timeout", envDuration("LOADTEST_HTTP_TIMEOUT", 10*time.Second), "single request timeout")
	flag.Float64Var(&cfg.ErrorThreshold, "error-threshold", envFloat("LOADTEST_ERROR_THRESHOLD", 0.02), "max allowed error rate")
	flag.DurationVar(&cfg.P95Threshold, "p95-threshold", envDuration("LOADTEST_P95_THRESHOLD", 700*time.Millisecond), "max allowed p95 latency")
	flag.BoolVar(&cfg.StopOnFail, "stop-on-fail", envBool("LOADTEST_STOP_ON_FAIL", true), "stop stepping on first failed step")
	flag.BoolVar(&cfg.InsecureSkipTLS, "insecure-skip-tls", envBool("LOADTEST_INSECURE_SKIP_TLS", false), "skip TLS verification")
	flag.BoolVar(&cfg.IncludeCaseWrites, "include-case-writes", envBool("LOADTEST_INCLUDE_CASE_WRITES", true), "include POST /cases in write profile")
	flag.Parse()

	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Email = strings.TrimSpace(cfg.Email)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.TenantID = strings.TrimSpace(cfg.TenantID)
	cfg.OutputPath = strings.TrimSpace(cfg.OutputPath)
	if cfg.BaseURL == "" {
		return config{}, fmt.Errorf("base-url cannot be empty")
	}
	if cfg.Email == "" || cfg.Password == "" {
		return config{}, fmt.Errorf("email and password are required")
	}
	if cfg.StartRPS <= 0 || cfg.StepRPS <= 0 || cfg.MaxRPS <= 0 {
		return config{}, fmt.Errorf("start-rps, step-rps, and max-rps must be > 0")
	}
	if cfg.StartRPS > cfg.MaxRPS {
		return config{}, fmt.Errorf("start-rps cannot be greater than max-rps")
	}
	if cfg.StepDuration <= 0 {
		return config{}, fmt.Errorf("step-duration must be > 0")
	}
	if cfg.Cooldown < 0 {
		return config{}, fmt.Errorf("cooldown cannot be negative")
	}
	if cfg.Workers <= 0 {
		return config{}, fmt.Errorf("workers must be > 0")
	}
	if cfg.HTTPTimeout <= 0 {
		return config{}, fmt.Errorf("http-timeout must be > 0")
	}
	if cfg.ErrorThreshold < 0 || cfg.ErrorThreshold >= 1 {
		return config{}, fmt.Errorf("error-threshold must be in [0,1)")
	}
	if cfg.P95Threshold <= 0 {
		return config{}, fmt.Errorf("p95-threshold must be > 0")
	}
	if cfg.OutputPath == "" {
		return config{}, fmt.Errorf("output cannot be empty")
	}
	return cfg, nil
}

func firstNonEmptyEnv(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
