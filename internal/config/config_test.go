package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDev(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppEnv != EnvDev || cfg.Port != "8081" {
		t.Fatalf("env=%s port=%s", cfg.AppEnv, cfg.Port)
	}
	if !strings.Contains(cfg.MySQLDSN, "/mdg_tempo?") {
		t.Fatalf("dsn=%s", cfg.MySQLDSN)
	}
	if cfg.JWKSURL == "" {
		t.Fatal("jwks url empty")
	}
	if cfg.TelegramBotUsername != "mitdobot" || cfg.TelegramBotName != "Mitdo" {
		t.Fatalf("bot %s %s", cfg.TelegramBotUsername, cfg.TelegramBotName)
	}
	if cfg.RemindEvery != time.Minute {
		t.Fatalf("interval %s", cfg.RemindEvery)
	}
	if cfg.TelegramBotToken == "" {
		t.Fatal("dev bot token empty")
	}
}

func TestLoadTestEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "5003" {
		t.Fatalf("port=%s", cfg.Port)
	}
	if !strings.Contains(cfg.MySQLDSN, "/mdg_tempo_test?") {
		t.Fatalf("dsn=%s", cfg.MySQLDSN)
	}
}

func TestMySQLHostEnvPlaceholder(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("MYSQL_HOST", "db.internal")
	t.Setenv("MYSQL_DATABASE", "custom_db")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.MySQLDSN, "db.internal") {
		t.Fatalf("dsn=%s", cfg.MySQLDSN)
	}
	if !strings.Contains(cfg.MySQLDSN, "/custom_db?") {
		t.Fatalf("dsn=%s", cfg.MySQLDSN)
	}
}

func TestJWKSURLEnvPlaceholder(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("JWT_JWKS_URL", "http://auth:8080/auth/.well-known/jwks.json")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWKSURL != "http://auth:8080/auth/.well-known/jwks.json" {
		t.Fatalf("jwks=%s", cfg.JWKSURL)
	}
}

func TestNormalizeEnv(t *testing.T) {
	if NormalizeEnv("") != EnvDev {
		t.Fatal("default")
	}
	if NormalizeEnv("development") != EnvDev {
		t.Fatal("development alias")
	}
	if NormalizeEnv("prd") != EnvPrd {
		t.Fatal("prd")
	}
	if NormalizeEnv("production") != EnvPrd {
		t.Fatal("production alias")
	}
}
