package connectorhub

import (
	"errors"
	"strings"
	"testing"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/models"
)

func TestNormalizeActionKey(t *testing.T) {
	if got := NormalizeActionKey(" Scan Hash / VT "); got != "scan_hash_vt" {
		t.Fatalf("NormalizeActionKey() = %q, want %q", got, "scan_hash_vt")
	}
}

func TestBuildScopeAndRenderTemplates(t *testing.T) {
	scope := BuildScope(ScopeInput{
		Input: map[string]any{
			"observable": "1.1.1.1",
			"kind":       "ip",
		},
		CaseID: "case-123",
	}, "enrich_observable", "Enrich Observable")

	if got := scope["case_id"]; got != "case-123" {
		t.Fatalf("expected case_id in scope, got %#v", got)
	}
	if got := scope["observable"]; got != "1.1.1.1" {
		t.Fatalf("expected flattened observable in scope, got %#v", got)
	}

	rendered := RenderTemplateValue(map[string]any{
		"message": "Lookup {{observable}}",
		"nested": map[string]any{
			"case": "{{case_id}}",
		},
		"items": []any{"{{kind}}", map[string]any{"value": "{{observable}}"}},
	}, scope)

	if got := rendered["message"]; got != "Lookup 1.1.1.1" {
		t.Fatalf("expected rendered message, got %#v", got)
	}
	nested, _ := rendered["nested"].(map[string]any)
	if got := nested["case"]; got != "case-123" {
		t.Fatalf("expected rendered nested case, got %#v", got)
	}
	items, _ := rendered["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected two rendered items, got %d", len(items))
	}

	merged := MergeMetadata(map[string]any{"source": "manual"}, rendered)
	if got := merged["source"]; got != "manual" {
		t.Fatalf("expected original metadata to survive merge, got %#v", got)
	}
	if got := merged["message"]; got != "Lookup 1.1.1.1" {
		t.Fatalf("expected rendered metadata to merge, got %#v", got)
	}
}

func TestBuildSendResponsePayloadAndRequestFromPayload(t *testing.T) {
	reply := outbound.SendResponse{
		Reply:          " accepted ",
		ConversationID: " conv-1 ",
		Cursor:         " cur-1 ",
		Metadata: map[string]any{
			"external_id":     "msg-7",
			"provider_status": "queued",
			"correlation_id":  "corr-9",
		},
	}
	payload := BuildSendResponsePayload(reply)
	if got := payload["external_id"]; got != "msg-7" {
		t.Fatalf("expected external_id, got %#v", got)
	}
	if got := payload["provider_status"]; got != "queued" {
		t.Fatalf("expected provider_status, got %#v", got)
	}

	request := RequestFromPayload(map[string]any{
		"thread_id":       "thread-1",
		"conversation_id": "conv-1",
		"author":          "analyst",
		"message":         "hello",
		"metadata":        map[string]any{"tenant": "acme"},
	})
	if request.ThreadID != "thread-1" || request.ConversationID != "conv-1" || request.Author != "analyst" || request.Message != "hello" {
		t.Fatalf("unexpected request reconstruction: %#v", request)
	}
	if got := request.Metadata["tenant"]; got != "acme" {
		t.Fatalf("expected metadata to survive reconstruction, got %#v", got)
	}
}

func TestExecutionTerminalHelpers(t *testing.T) {
	if !IsExecutionTerminalStatus(models.ConnectorHubExecutionStatusCompleted) {
		t.Fatalf("completed should be terminal")
	}
	if IsExecutionTerminalStatus(models.ConnectorHubExecutionStatusDispatching) {
		t.Fatalf("dispatching should not be terminal")
	}
	if err := ExecutionTerminalError(&models.ConnectorHubExecution{Status: models.ConnectorHubExecutionStatusCompleted}); err != nil {
		t.Fatalf("completed execution should not return error: %v", err)
	}
	if err := ExecutionTerminalError(&models.ConnectorHubExecution{Status: models.ConnectorHubExecutionStatusCancelled}); err == nil {
		t.Fatalf("canceled execution should return error")
	}
	exec := &models.ConnectorHubExecution{Status: models.ConnectorHubExecutionStatusFailed, Error: "provider timeout"}
	err := ExecutionTerminalError(exec)
	if err == nil || !errors.Is(err, errConnectorHubExecutionFailed) || !strings.Contains(err.Error(), "provider timeout") {
		t.Fatalf("expected failure error to surface provider timeout, got %v", err)
	}
}
