package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithPasswordFlag(t *testing.T) {
	var out bytes.Buffer
	if err := run(strings.NewReader(""), &out, []string{"-password", "Password123!"}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	hash := strings.TrimSpace(out.String())
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id hash, got %q", hash)
	}
}

func TestRunWithStdin(t *testing.T) {
	var out bytes.Buffer
	if err := run(strings.NewReader("Password123!\n"), &out, []string{"-stdin"}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	hash := strings.TrimSpace(out.String())
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id hash, got %q", hash)
	}
}

func TestRunRequiresPassword(t *testing.T) {
	var out bytes.Buffer
	err := run(strings.NewReader(""), &out, []string{})
	if err == nil {
		t.Fatalf("expected password-required error")
	}
}
