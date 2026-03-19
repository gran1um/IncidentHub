package inbound

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx stdlib driver
)

func (w *Worker) fetchSQLRecords(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	sqlCfg := cfg.SQL
	if err := validateReadOnlySQLQuery(sqlCfg.Query); err != nil {
		return nil, fmt.Errorf("inbound connector %q SQL query is not allowed: %w", cfg.DisplayName, err)
	}

	dsn, err := resolveSQLDSN(sqlCfg)
	if err != nil {
		return nil, fmt.Errorf("inbound connector %q: %w", cfg.DisplayName, err)
	}

	db, err := sql.Open(sqlCfg.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQL connection for inbound connector %q: %w", cfg.DisplayName, err)
	}
	defer func() { _ = db.Close() }()

	reqCtx, cancel := withOptionalTimeout(ctx, firstPositiveDuration(sqlCfg.Timeout, cfg.Timeout, 20*time.Second))
	defer cancel()

	if pingErr := db.PingContext(reqCtx); pingErr != nil {
		return nil, fmt.Errorf("ping SQL source for inbound connector %q: %w", cfg.DisplayName, pingErr)
	}

	rows, err := db.QueryContext(reqCtx, sqlCfg.Query)
	if err != nil {
		return nil, fmt.Errorf("execute SQL query for inbound connector %q: %w", cfg.DisplayName, err)
	}
	defer func() { _ = rows.Close() }()

	records, err := scanSQLRows(rows, sqlCfg.MaxRows)
	if err != nil {
		return nil, fmt.Errorf("read SQL rows for inbound connector %q: %w", cfg.DisplayName, err)
	}
	return records, nil
}

func validateReadOnlySQLQuery(query string) error {
	raw := strings.ToLower(strings.TrimSpace(query))
	if raw == "" {
		return fmt.Errorf("query is empty")
	}
	if strings.Contains(raw, ";") {
		return fmt.Errorf("multiple SQL statements are forbidden")
	}
	if !strings.HasPrefix(raw, "select") && !strings.HasPrefix(raw, "with") {
		return fmt.Errorf("only SELECT/CTE queries are allowed")
	}

	padded := " " + raw + " "
	for _, keyword := range []string{
		" insert ",
		" update ",
		" delete ",
		" drop ",
		" alter ",
		" truncate ",
		" create ",
		" grant ",
		" revoke ",
	} {
		if strings.Contains(padded, keyword) {
			return fmt.Errorf("mutating SQL keyword %q is forbidden", strings.TrimSpace(keyword))
		}
	}
	return nil
}

func resolveSQLDSN(cfg inboundSQLSourceConfig) (string, error) {
	if strings.TrimSpace(cfg.DSN) != "" {
		return strings.TrimSpace(cfg.DSN), nil
	}

	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" || driver == "postgresql" || driver == "postgres" {
		driver = "pgx"
	}
	if driver != "pgx" {
		return "", fmt.Errorf("SQL driver %q is not supported", cfg.Driver)
	}

	host := strings.TrimSpace(cfg.Host)
	if host == "" {
		return "", fmt.Errorf("config.host is required when dsn is not provided")
	}
	database := strings.TrimSpace(cfg.Database)
	if database == "" {
		return "", fmt.Errorf("config.database is required when dsn is not provided")
	}
	port := cfg.Port
	if port <= 0 {
		port = 5432
	}
	sslMode := strings.TrimSpace(cfg.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}

	username := url.QueryEscape(strings.TrimSpace(cfg.Auth.Username))
	password := url.QueryEscape(strings.TrimSpace(cfg.Auth.Password))
	if username == "" {
		return fmt.Sprintf("postgres://%s:%d/%s?sslmode=%s", host, port, database, url.QueryEscape(sslMode)), nil
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", username, password, host, port, database, url.QueryEscape(sslMode)), nil
}

func scanSQLRows(rows *sql.Rows, limit int) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("read SQL columns: %w", err)
	}
	if limit <= 0 {
		limit = 500
	}

	records := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if scanErr := rows.Scan(valuePtrs...); scanErr != nil {
			return nil, fmt.Errorf("scan SQL row: %w", scanErr)
		}

		record := make(map[string]any, len(columns))
		for i, column := range columns {
			record[column] = normalizeSQLValue(values[i])
		}
		records = append(records, record)
		if len(records) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate SQL rows: %w", err)
	}
	return records, nil
}

func normalizeSQLValue(value any) any {
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
