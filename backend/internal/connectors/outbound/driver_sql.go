package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLDriver implements outbound connector support for SQL databases.
//
// P0 focuses on Postgres using pgxpool with read-only SELECT queries.
// Other engines can be added incrementally by extending the engine switch.
type SQLDriver struct {
	mu    sync.Mutex
	pools map[string]*pgxpool.Pool
}

func NewSQLDriver() *SQLDriver {
	return &SQLDriver{pools: make(map[string]*pgxpool.Pool)}
}

func (d *SQLDriver) Kind() string {
	return "sql"
}

func (d *SQLDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	pool, engine, err := d.getPool(ctx, cfg)
	if err != nil {
		return SendResponse{}, err
	}

	query := strings.TrimSpace(firstString(req.Metadata, "query"))
	if query == "" {
		query = strings.TrimSpace(req.Message)
	}
	if query == "" {
		return SendResponse{}, fmt.Errorf("sql connector query is required")
	}

	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
		return SendResponse{}, fmt.Errorf("sql connector only supports SELECT queries for now")
	}

	// Optional timeout to avoid hanging connections.
	timeout := sqlDriverConfigDuration(cfg, 15*time.Second, "timeout", "timeout_seconds", "queryTimeout", "query_timeout")
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := pool.Query(queryCtx, query)
	if err != nil {
		return SendResponse{}, fmt.Errorf("sql connector query failed: %w", err)
	}
	defer rows.Close()

	columnDescs := rows.FieldDescriptions()
	columnNames := make([]string, len(columnDescs))
	for index, desc := range columnDescs {
		columnNames[index] = desc.Name
	}

	resultRows := make([]map[string]any, 0, 64)
	for rows.Next() {
		values, scanErr := rows.Values()
		if scanErr != nil {
			return SendResponse{}, fmt.Errorf("sql connector scan row: %w", scanErr)
		}
		row := make(map[string]any, len(values))
		for index, value := range values {
			name := columnNames[index]
			if name == "" {
				name = fmt.Sprintf("col_%d", index)
			}
			row[name] = sqlDriverNormalizeValue(value)
		}
		resultRows = append(resultRows, row)
	}
	if rowErr := rows.Err(); rowErr != nil {
		return SendResponse{}, fmt.Errorf("sql connector iterate rows: %w", rowErr)
	}

	raw, err := json.Marshal(resultRows)
	if err != nil {
		return SendResponse{}, fmt.Errorf("sql connector encode rows: %w", err)
	}

	metadata := map[string]any{
		"engine":    engine,
		"row_count": len(resultRows),
		"rows":      resultRows,
	}

	return SendResponse{
		Reply:          string(raw),
		ConversationID: req.ConversationID,
		Metadata:       metadata,
	}, nil
}

func (d *SQLDriver) Poll(context.Context, map[string]any, PollRequest) (PollResponse, error) {
	// SQL connectors are request/response only for v1.5.
	return PollResponse{}, nil
}

func (d *SQLDriver) getPool(ctx context.Context, cfg map[string]any) (*pgxpool.Pool, string, error) {
	if d == nil {
		return nil, "", fmt.Errorf("sql driver is not initialized")
	}

	engineKey := strings.ToLower(strings.TrimSpace(firstString(cfg, "engine")))
	if engineKey == "" {
		engineKey = strings.ToLower(strings.TrimSpace(firstString(cfg, "type")))
	}
	var engine string
	switch engineKey {
	case "", "postgres", "postgresql", "sql":
		engine = "postgres"
	default:
		return nil, "", fmt.Errorf("sql engine not supported yet: %s", engineKey)
	}

	dsn := strings.TrimSpace(firstString(cfg, "dsn", "url"))
	if dsn == "" {
		var buildErr error
		dsn, buildErr = sqlDriverBuildPostgresDSN(cfg)
		if buildErr != nil {
			return nil, "", buildErr
		}
	}

	connectorID := strings.TrimSpace(fmt.Sprint(cfg["connector_id"]))
	key := engine + ":" + dsn
	if connectorID != "" {
		key = engine + ":" + connectorID
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if pool, exists := d.pools[key]; exists && pool != nil {
		return pool, engine, nil
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, "", fmt.Errorf("sql connector parse config: %w", err)
	}
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, "", fmt.Errorf("sql connector create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, "", fmt.Errorf("sql connector ping database: %w", err)
	}

	if d.pools == nil {
		d.pools = make(map[string]*pgxpool.Pool)
	}
	d.pools[key] = pool
	return pool, engine, nil
}

func sqlDriverBuildPostgresDSN(cfg map[string]any) (string, error) {
	host := strings.TrimSpace(firstString(cfg, "host"))
	port := strings.TrimSpace(firstString(cfg, "port"))
	database := strings.TrimSpace(firstString(cfg, "database", "dbname"))
	user := strings.TrimSpace(firstString(cfg, "username", "user"))
	password := strings.TrimSpace(firstString(cfg, "password", "pass"))
	sslMode := strings.TrimSpace(firstString(cfg, "sslmode", "ssl_mode", "sslMode"))
	if sslMode == "" {
		sslMode = "disable"
	}

	if host == "" {
		return "", fmt.Errorf("sql connector host is required when DSN is not provided")
	}
	if database == "" {
		return "", fmt.Errorf("sql connector database is required when DSN is not provided")
	}
	if port == "" {
		port = "5432"
	}

	u := &url.URL{Scheme: "postgres"}
	if user != "" {
		if password != "" {
			u.User = url.UserPassword(user, password)
		} else {
			u.User = url.User(user)
		}
	}
	u.Host = fmt.Sprintf("%s:%s", host, port)
	u.Path = "/" + database
	q := u.Query()
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func sqlDriverNormalizeValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	default:
		return typed
	}
}

func sqlDriverConfigDuration(cfg map[string]any, fallback time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case time.Duration:
			if typed > 0 {
				return typed
			}
		case int:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case int32:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case int64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case float64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case float32:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if parsed, err := time.ParseDuration(trimmed); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return fallback
}
