package config

import (
	"strings"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://app@localhost/tenant_service")
	t.Setenv("OIDC_ISSUER", "http://localhost:8180/realms/people")
	t.Setenv("OIDC_JWKS_URL", "http://keycloak:8080/realms/people/protocol/openid-connect/certs")
	t.Setenv("KAFKA_BROKERS", " kafka:9092 , ,kafka-2:9092")
}

func TestLoad_DefaultsAndParsing(t *testing.T) {
	setRequired(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.KafkaBrokers) != 2 || c.KafkaBrokers[1] != "kafka-2:9092" {
		t.Fatalf("brokers = %v", c.KafkaBrokers)
	}
	if c.OutboxInterval != time.Second || c.OutboxBatchSize != 100 || c.HTTPAddr != ":8080" {
		t.Fatalf("padrões inesperados: %+v", c)
	}
	if c.AssetsPublicBaseURL != "https://people-platform-tenant-assets.s3.us-east-1.amazonaws.com" {
		t.Fatalf("URL pública padrão = %s", c.AssetsPublicBaseURL)
	}
}

func TestLoad_FailsFastOnMissingOrInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("OIDC_ISSUER", "")
	t.Setenv("OIDC_JWKS_URL", "")
	t.Setenv("KAFKA_BROKERS", "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "KAFKA_BROKERS") {
		t.Fatalf("esperava erro listando as variáveis ausentes, obteve %v", err)
	}

	setRequired(t)
	t.Setenv("OUTBOX_BATCH_SIZE", "0")
	if _, err := Load(); err == nil {
		t.Fatal("OUTBOX_BATCH_SIZE=0 deveria falhar")
	}
	t.Setenv("OUTBOX_BATCH_SIZE", "")
	t.Setenv("OUTBOX_POLL_INTERVAL", "rápido")
	if _, err := Load(); err == nil {
		t.Fatal("intervalo inválido deveria falhar")
	}
}
