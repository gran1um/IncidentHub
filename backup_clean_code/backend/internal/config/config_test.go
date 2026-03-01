package config

import "testing"

func TestPostgresDSN(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "pg.local",
		Port:     5433,
		User:     "user",
		Password: "pass",
		DBName:   "ih",
		SSLMode:  "disable",
	}

	got := cfg.DSN()
	want := "postgres://user:pass@pg.local:5433/ih?sslmode=disable"
	if got != want {
		t.Fatalf("unexpected dsn: %s", got)
	}
}

func TestElasticAddressList(t *testing.T) {
	cfg := ElasticConfig{Addresses: " http://a:9200, ,http://b:9200 "}
	addresses := cfg.AddressList()
	if len(addresses) != 2 || addresses[0] != "http://a:9200" || addresses[1] != "http://b:9200" {
		t.Fatalf("unexpected addresses: %#v", addresses)
	}

	fallback := (ElasticConfig{Addresses: " , "}).AddressList()
	if len(fallback) != 1 || fallback[0] != "http://localhost:9200" {
		t.Fatalf("unexpected fallback addresses: %#v", fallback)
	}
}

func TestHTTPCORSOrigins(t *testing.T) {
	cfg := HTTPConfig{CORSAllowOrigins: "http://localhost:5173, https://x.local "}
	origins := cfg.CORSOrigins()
	if len(origins) != 2 || origins[0] != "http://localhost:5173" || origins[1] != "https://x.local" {
		t.Fatalf("unexpected cors origins: %#v", origins)
	}

	fallback := (HTTPConfig{CORSAllowOrigins: "  "}).CORSOrigins()
	if len(fallback) != 1 || fallback[0] != "http://localhost:5173" {
		t.Fatalf("unexpected fallback cors origins: %#v", fallback)
	}
}

func TestLoadPanicsOnEmptyJWTSecret(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "")

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for empty auth jwt secret")
		}
	}()
	_ = Load()
}

func TestLoadReadsExplicitJWTSecret(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "test-secret")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HTTP_CORS_ALLOW_ORIGINS", "http://localhost:5173,https://example.local")

	cfg := Load()
	if cfg.Auth.JWTSecret != "test-secret" {
		t.Fatalf("expected jwt secret from env")
	}
	if cfg.Logger.Level != "debug" {
		t.Fatalf("expected logger level debug, got %s", cfg.Logger.Level)
	}
	if got := cfg.HTTP.CORSOrigins(); len(got) != 2 {
		t.Fatalf("expected two cors origins, got %#v", got)
	}
}
