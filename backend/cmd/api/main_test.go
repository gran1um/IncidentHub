package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRunFailsOnInvalidLoggerLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "not-a-level")
	t.Setenv("TRACE_ENABLED", "false")

	err := run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "init runtime") {
		t.Fatalf("expected runtime init error, got %v", err)
	}
}

func TestRunFailsOnInvalidPostgresConfig(t *testing.T) {
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("TRACE_ENABLED", "false")
	t.Setenv("POSTGRES_HOST", "bad host")
	t.Setenv("POSTGRES_PORT", "5432")

	err := run(context.Background())
	if err == nil || (!strings.Contains(err.Error(), "init postgres") && !strings.Contains(err.Error(), "parse postgres config")) {
		t.Fatalf("expected postgres init error, got %v", err)
	}
}

func TestRunFailsAfterMigrationsOnRedisInit(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if chdirErr := os.Chdir("../.."); chdirErr != nil {
		t.Fatalf("chdir to backend root: %v", chdirErr)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("TRACE_ENABLED", "false")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "incidenthub")
	t.Setenv("POSTGRES_PASSWORD", "incidenthub")
	t.Setenv("POSTGRES_DB", "incidenthub")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("REDIS_ADDR", "127.0.0.1:1")

	err = run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "init redis") {
		t.Fatalf("expected redis init error, got %v", err)
	}
}

func TestRunFailsOnElasticAfterRedisHandshake(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if chdirErr := os.Chdir("../.."); chdirErr != nil {
		t.Fatalf("chdir to backend root: %v", chdirErr)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	redisAddr := startFakeRedisServer(t)

	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("TRACE_ENABLED", "false")
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_PORT", "5432")
	t.Setenv("POSTGRES_USER", "incidenthub")
	t.Setenv("POSTGRES_PASSWORD", "incidenthub")
	t.Setenv("POSTGRES_DB", "incidenthub")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("REDIS_ADDR", redisAddr)
	t.Setenv("ELASTIC_ADDRESSES", "http://[::1")

	err = run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "init elastic") {
		t.Fatalf("expected elastic init error, got %v", err)
	}
}

func startFakeRedisServer(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake redis: %v", err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
	})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleFakeRedisConn(conn)
		}
	}()
	return ln.Addr().String()
}

func handleFakeRedisConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	for {
		cmd, err := readRESPCommand(reader)
		if err != nil {
			return
		}
		switch strings.ToUpper(strings.TrimSpace(cmd)) {
		case "HELLO":
			_, _ = conn.Write([]byte("%7\r\n+server\r\n$5\r\nredis\r\n+version\r\n$5\r\n7.2.0\r\n+proto\r\n:3\r\n+id\r\n:1\r\n+mode\r\n$10\r\nstandalone\r\n+role\r\n$6\r\nmaster\r\n+modules\r\n*0\r\n"))
		case "PING":
			_, _ = conn.Write([]byte("+PONG\r\n"))
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
	}
}

func readRESPCommand(reader *bufio.Reader) (string, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	if prefix != '*' {
		return "", fmt.Errorf("unsupported resp prefix %q", prefix)
	}

	countLine, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	countRaw := strings.TrimSpace(countLine)
	count, err := strconv.Atoi(countRaw)
	if err != nil || count <= 0 {
		return "", fmt.Errorf("invalid resp array len %q", countRaw)
	}

	command := ""
	for i := 0; i < count; i++ {
		blobPrefix, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if blobPrefix != '$' {
			return "", fmt.Errorf("unsupported resp blob prefix %q", blobPrefix)
		}
		lenLine, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		argLenRaw := strings.TrimSpace(lenLine)
		argLen, err := strconv.Atoi(argLenRaw)
		if err != nil || argLen < 0 {
			return "", fmt.Errorf("invalid resp bulk len %q", argLenRaw)
		}
		payload := make([]byte, argLen+2)
		if _, err := reader.Read(payload); err != nil {
			return "", err
		}
		arg := string(payload[:argLen])
		if i == 0 {
			command = arg
		}
	}
	return command, nil
}
