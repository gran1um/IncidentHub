package inbound

import (
	"fmt"
	"strings"
)

type inboundAlertSourceProfileTemplate struct {
	Canonical    string
	SourceName   string
	SourceDomain string
	Defaults     map[string]any
}

//nolint:gochecknoglobals // Static inbound source profiles used for normalization defaults.
var inboundAlertSourceProfiles = map[string]inboundAlertSourceProfileTemplate{
	"dlp": {
		Canonical:    "dlp",
		SourceName:   "DLP",
		SourceDomain: "dlp",
		Defaults: map[string]any{
			"id_field":          "incident_id",
			"title_field":       "policy_name",
			"description_field": "description",
			"source_field":      "source",
			"severity_field":    "severity",
			"status_field":      "status",
			"tlp_field":         "tlp",
			"pap_field":         "pap",
			"default_source":    "DLP",
			"default_severity":  "high",
			"default_status":    "new",
			"default_tlp":       "amber",
			"default_pap":       "amber",
		},
	},
	"sb": {
		Canonical:    "sb",
		SourceName:   "Security",
		SourceDomain: "sb",
		Defaults: map[string]any{
			"id_field":          "event_id",
			"title_field":       "scenario",
			"description_field": "details",
			"source_field":      "source",
			"severity_field":    "severity",
			"status_field":      "status",
			"tlp_field":         "tlp",
			"pap_field":         "pap",
			"default_source":    "Security",
			"default_severity":  "medium",
			"default_status":    "new",
			"default_tlp":       "amber",
			"default_pap":       "amber",
		},
	},
	"data_security": {
		Canonical:    "data_security",
		SourceName:   "Data Security",
		SourceDomain: "data_security",
		Defaults: map[string]any{
			"id_field":          "incident_id",
			"title_field":       "incident_type",
			"description_field": "description",
			"source_field":      "system",
			"severity_field":    "criticality",
			"status_field":      "status",
			"tlp_field":         "tlp",
			"pap_field":         "pap",
			"default_source":    "Data Security",
			"default_severity":  "high",
			"default_status":    "new",
			"default_tlp":       "amber",
			"default_pap":       "amber",
		},
	},
	"custom": {
		Canonical:    "custom",
		SourceName:   "Custom Source",
		SourceDomain: "custom",
		Defaults: map[string]any{
			"default_source":   "Custom Source",
			"default_severity": "medium",
			"default_status":   "new",
			"default_tlp":      "amber",
			"default_pap":      "amber",
		},
	},
}

func applyInboundAlertSourceProfile(data map[string]any) (map[string]any, error) {
	if data == nil {
		return nil, nil
	}

	rawProfile := firstString(
		data,
		"source_profile",
		"sourceProfile",
		"alert_source_profile",
		"alertSourceProfile",
		"profile",
	)
	if rawProfile == "" {
		if nested := mapValue(data, "config"); nested != nil {
			rawProfile = firstString(
				nested,
				"source_profile",
				"sourceProfile",
				"alert_source_profile",
				"alertSourceProfile",
				"profile",
			)
		}
	}
	profile := normalizeInboundAlertSourceProfile(rawProfile)
	if profile == "" {
		return data, nil
	}

	template, ok := inboundAlertSourceProfiles[profile]
	if !ok {
		return nil, fmt.Errorf("source profile %q is not supported", strings.TrimSpace(rawProfile))
	}

	out := cloneMapDeep(data)
	target := out
	if nested := mapValue(out, "config"); nested != nil {
		target = nested
	}

	out["source_profile"] = template.Canonical
	target["source_profile"] = template.Canonical
	if template.SourceName != "" {
		setDefaultMapValue(target, "source_name", template.SourceName)
	}
	if template.SourceDomain != "" {
		setDefaultMapValue(out, "source_domain", template.SourceDomain)
		setDefaultMapValue(target, "source_domain", template.SourceDomain)
	}
	for key, value := range template.Defaults {
		setDefaultMapValue(target, key, value)
	}
	return out, nil
}

func normalizeInboundAlertSourceProfile(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "-", "_")
	raw = strings.ReplaceAll(raw, " ", "_")
	switch raw {
	case "dlp", "data_leak_prevention":
		return "dlp"
	case "sb", "security":
		return "sb"
	case "data_security", "datasec", "datasecurity":
		return "data_security"
	case "custom", "other":
		return "custom"
	default:
		return raw
	}
}

func cloneMapDeep(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = cloneAnyDeep(value)
	}
	return out
}

func cloneAnyDeep(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMapDeep(typed)
	case []any:
		out := make([]any, len(typed))
		for idx := range typed {
			out[idx] = cloneAnyDeep(typed[idx])
		}
		return out
	default:
		return value
	}
}

func setDefaultMapValue(target map[string]any, key string, value any) {
	if target == nil || strings.TrimSpace(key) == "" {
		return
	}
	existing, exists := target[key]
	if !exists || isEmptyConfigValue(existing) {
		target[key] = value
	}
}

func isEmptyConfigValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}
