package inbound

import (
	"strings"
	"testing"
)

func TestParseInboundConnectorConfig_AppliesDLPSourceProfileDefaults(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name":           "DLP Feed",
			"source_type":    "http",
			"source_profile": "dlp",
			"url":            "https://dlp.example.local/feed",
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.Source != "DLP" {
		t.Fatalf("expected default source DLP, got %q", cfg.Source)
	}
	if cfg.IDField != "incident_id" {
		t.Fatalf("expected DLP id_field incident_id, got %q", cfg.IDField)
	}
	if cfg.TitleField != "policy_name" {
		t.Fatalf("expected DLP title_field policy_name, got %q", cfg.TitleField)
	}
	if cfg.Severity != "high" {
		t.Fatalf("expected DLP default severity high, got %q", cfg.Severity)
	}
}

func TestParseInboundConnectorConfig_SourceProfileDoesNotOverrideExplicitConfig(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Custom Feed",
			"config": map[string]any{
				"source_profile":   "custom",
				"source_type":      "http",
				"url":              "https://custom.example.local/feed",
				"id_field":         "custom_event_id",
				"default_severity": "critical",
				"default_source":   "Custom Core",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.IDField != "custom_event_id" {
		t.Fatalf("expected explicit id_field to be preserved, got %q", cfg.IDField)
	}
	if cfg.Severity != "critical" {
		t.Fatalf("expected explicit default_severity to be preserved, got %q", cfg.Severity)
	}
	if cfg.Source != "Custom Core" {
		t.Fatalf("expected explicit default_source to be preserved, got %q", cfg.Source)
	}
}

func TestParseInboundConnectorConfig_UnknownSourceProfileFails(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name":           "Unknown Profile",
			"source_type":    "http",
			"source_profile": "unknown-profile",
			"url":            "https://example.local/feed",
		},
	})
	if err == nil {
		t.Fatal("expected source profile validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "source profile") {
		t.Fatalf("expected source profile error, got %v", err)
	}
}

func TestParseInboundConnectorConfig_NormalizesDataSecuritySourceProfile(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name":           "Data Security Feed",
			"source_type":    "http",
			"source_profile": "data security",
			"url":            "https://datasec.example.local/feed",
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.Source != "Data Security" {
		t.Fatalf("expected Data Security default source, got %q", cfg.Source)
	}
	if cfg.SeverityField != "criticality" {
		t.Fatalf("expected Data Security severity_field criticality, got %q", cfg.SeverityField)
	}
}
