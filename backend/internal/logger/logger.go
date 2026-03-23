package logger

import (
	"fmt"
	"incidenthub/backend/internal/config"

	"gitlab.anyinfra.ru/golang-core/logster"
)

//nolint:gochecknoglobals // Global logger instance is configured once during boot.
var Logger *logster.Logger

func InitLogger(cfg config.App) error {
	minLevel, err := logster.LevelFromString(cfg.Logger.Level)
	if err != nil {
		return fmt.Errorf("parse logger level: %w", err)
	}

	l, err := logster.NewLogger(logster.Config{
		MinLevel: minLevel,
		Format:   logster.FormatJSON,
		Sage: logster.Sage{
			Env:    cfg.Env,
			Group:  cfg.Logger.Group,
			System: cfg.Logger.System,
		},
	})
	if err != nil {
		return fmt.Errorf("new logger: %w", err)
	}

	Logger = l
	return nil
}

func CloseLogger() error {
	if Logger == nil {
		return nil
	}
	_ = Logger.Close()
	Logger = nil
	return nil
}

func Debugf(format string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Debugf(format, args...)
}

func Infof(format string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Infof(format, args...)
}

func Warnf(format string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Warnf(format, args...)
}

func Errorf(format string, args ...any) {
	if Logger == nil {
		return
	}
	Logger.Errorf(format, args...)
}
