package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
)

var echoPathParamPattern = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

func TestOpenAPISpecCoversRegisteredRoutes(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &spec); err != nil {
		t.Fatalf("unmarshal embedded openapi spec: %v", err)
	}

	cfg := config.App{
		Env: "dev",
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: "http://localhost:5173",
		},
		Auth: config.AuthConfig{
			JWTSecret:  "openapi-drift-test",
			Issuer:     "incidenthub",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: time.Hour,
		},
	}

	srv := NewServer(Dependencies{Config: cfg})
	routes := srv.Echo().Router().Routes()

	registered := map[string]map[string]struct{}{}
	for _, route := range routes {
		method := strings.ToLower(strings.TrimSpace(route.Method))
		if method == "" || method == "options" {
			continue
		}
		path := normalizeEchoPath(route.Path)
		if _, ok := registered[path]; !ok {
			registered[path] = map[string]struct{}{}
		}
		registered[path][method] = struct{}{}
	}

	specOps := map[string]map[string]struct{}{}
	for path, methods := range spec.Paths {
		normalizedPath := strings.TrimSpace(path)
		if normalizedPath == "" {
			continue
		}
		if _, ok := specOps[normalizedPath]; !ok {
			specOps[normalizedPath] = map[string]struct{}{}
		}
		for method := range methods {
			specOps[normalizedPath][strings.ToLower(strings.TrimSpace(method))] = struct{}{}
		}
	}

	missing := make([]string, 0)
	for path, methods := range registered {
		for method := range methods {
			if _, ok := specOps[path][method]; !ok {
				missing = append(missing, fmt.Sprintf("%s %s", strings.ToUpper(method), path))
			}
		}
	}

	extra := make([]string, 0)
	for path, methods := range specOps {
		for method := range methods {
			if _, ok := registered[path][method]; !ok {
				extra = append(extra, fmt.Sprintf("%s %s", strings.ToUpper(method), path))
			}
		}
	}

	if len(missing) > 0 || len(extra) > 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		t.Fatalf("openapi drift detected\nmissing operations: %v\nextra operations: %v", missing, extra)
	}
}

func normalizeEchoPath(path string) string {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		return "/"
	}
	return echoPathParamPattern.ReplaceAllString(normalized, `{$1}`)
}
