package logger

import (
	"strings"
	"testing"

	"incidenthub/backend/internal/config"
)

func TestInitLoggerInvalidLevel(t *testing.T) {
	cfg := config.App{
		Env: "test",
		Logger: config.LoggerConfig{
			Group:  "g",
			System: "s",
			Level:  "not-a-level",
		},
	}

	err := InitLogger(cfg)
	if err == nil || !strings.Contains(err.Error(), "parse logger level") {
		t.Fatalf("expected invalid level error, got %v", err)
	}
}

func TestInitLoggerAndWrappers(t *testing.T) {
	cfg := config.App{
		Env: "test",
		Logger: config.LoggerConfig{
			Group:  "g",
			System: "s",
			Level:  "info",
		},
	}

	if err := InitLogger(cfg); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	Debugf("debug %s", "message")
	Infof("info %s", "message")
	Warnf("warn %s", "message")
	Errorf("error %s", "message")

	if err := CloseLogger(); err != nil {
		t.Fatalf("close logger: %v", err)
	}
}
