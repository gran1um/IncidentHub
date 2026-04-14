package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "loadtest failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}

	client := newAPIClient(cfg)

	session, err := client.login(ctx, cfg.Email, cfg.Password)
	if err != nil {
		return err
	}
	if cfg.TenantID != "" {
		session.TenantID = cfg.TenantID
	}
	if session.TenantID == "" {
		return fmt.Errorf("tenant_id is empty in login response; set -tenant-id explicitly")
	}

	fixtures, err := prepareFixtures(ctx, client, session, cfg)
	if err != nil {
		return err
	}

	scenarios := buildScenarios(cfg)
	reports := make([]scenarioReport, 0, len(scenarios))

	for _, sc := range scenarios {
		sr := scenarioReport{
			Name:         sc.Name,
			Category:     sc.Category,
			Method:       sc.Method,
			PathTemplate: sc.PathTemplate,
			Steps:        make([]stepResult, 0, 16),
		}

		for rps := cfg.StartRPS; rps <= cfg.MaxRPS; rps += cfg.StepRPS {
			step := runStep(ctx, runInput{
				Config:    cfg,
				Client:    client,
				Session:   session,
				Scenario:  sc,
				Fixtures:  fixtures,
				TargetRPS: rps,
			})
			sr.Steps = append(sr.Steps, step)
			if step.Passed && rps > sr.MaxSustainableRPS {
				sr.MaxSustainableRPS = rps
			}

			if cfg.StopOnFail && !step.Passed {
				break
			}
			if cfg.Cooldown > 0 {
				time.Sleep(cfg.Cooldown)
			}
		}
		reports = append(reports, sr)
	}

	report := loadReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		BaseURL:        cfg.BaseURL,
		TenantID:       session.TenantID,
		StepDuration:   cfg.StepDuration.String(),
		Cooldown:       cfg.Cooldown.String(),
		ErrorThreshold: cfg.ErrorThreshold,
		P95ThresholdMS: roundFloat(durationToMS(cfg.P95Threshold), 3),
		Workers:        cfg.Workers,
		Scenarios:      reports,
	}

	jsonPath, mdPath, err := writeReportFiles(cfg.OutputPath, report)
	if err != nil {
		return err
	}

	fmt.Printf("Load test report written:\n- %s\n- %s\n", jsonPath, mdPath)
	for _, item := range report.Scenarios {
		fmt.Printf("[%s] %s: max sustainable RPS=%d\n", item.Category, item.Name, item.MaxSustainableRPS)
	}
	return nil
}
