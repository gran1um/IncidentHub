package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsCacheableRequest(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		target   string
		expected bool
	}{
		{name: "cacheable get", method: http.MethodGet, target: "/api/v1/alerts?severity=high", expected: true},
		{name: "skip post", method: http.MethodPost, target: "/api/v1/alerts", expected: false},
		{name: "skip health", method: http.MethodGet, target: "/healthz", expected: false},
		{name: "skip metrics", method: http.MethodGet, target: "/metrics", expected: false},
		{name: "skip swagger", method: http.MethodGet, target: "/dev/swagger/openapi.json", expected: false},
		{name: "skip async operation status", method: http.MethodGet, target: "/api/v1/operations/11111111-1111-1111-1111-111111111111", expected: false},
		{name: "skip no_cache flag", method: http.MethodGet, target: "/api/v1/cases?no_cache=1", expected: false},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.target, http.NoBody)
		got := isCacheableRequest(req)
		if got != tc.expected {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.expected, got)
		}
	}
}

func TestIsInvalidationCandidate(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{method: http.MethodGet, expected: false},
		{method: http.MethodHead, expected: false},
		{method: http.MethodOptions, expected: false},
		{method: http.MethodPost, expected: true},
		{method: http.MethodPatch, expected: true},
		{method: http.MethodDelete, expected: true},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, "/api/v1/test", http.NoBody)
		got := isInvalidationCandidate(req)
		if got != tc.expected {
			t.Fatalf("method %s expected %v, got %v", tc.method, tc.expected, got)
		}
	}
}
