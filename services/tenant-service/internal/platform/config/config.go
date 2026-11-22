// Package config lê a configuração do serviço a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne a configuração do tenant-service.
type Config struct {
	HTTPAddr        string
	DatabaseURL     string // usuário da aplicação, sujeito a RLS
	KafkaBrokers    []string
	OIDCIssuer      string
	OIDCAudience    string
	OIDCJWKSURL     string
	OutboxInterval  time.Duration
	OutboxBatchSize int
	S3Bucket        string
	// S3PresignEndpoint é o endpoint que entra na URL pré-assinada, ou seja, o que o navegador
	// alcança: vazio na AWS real; http://localhost:4566 com LocalStack.
	S3PresignEndpoint   string
	AssetsPublicBaseURL string
	AWSRegion           string
	LogoUploadTTL       time.Duration

	// Provisionador de identidade (Organization no Keycloak). Desligado quando KeycloakURL está vazia.
	KeycloakURL              string
	KeycloakRealm            string
	KeycloakClientID         string
	KeycloakClientSecret     string
	TenantEventsTopic        string
	ProvisionerConsumerGroup string
}

// ProvisioningEnabled informa se o serviço consome TenantCriado para criar a Organization no Keycloak.
func (c Config) ProvisioningEnabled() bool { return c.KeycloakURL != "" }

// Load lê e valida a configuração. Falha cedo se faltar algo obrigatório.
func Load() (Config, error) {
	c := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		KafkaBrokers:      splitNonEmpty(env("KAFKA_BROKERS", "")),
		OIDCIssuer:        os.Getenv("OIDC_ISSUER"),
		OIDCAudience:      env("OIDC_AUDIENCE", "people-api"),
		OIDCJWKSURL:       os.Getenv("OIDC_JWKS_URL"),
		OutboxBatchSize:   100,
		S3Bucket:          env("S3_ASSETS_BUCKET", "people-platform-tenant-assets"),
		S3PresignEndpoint: os.Getenv("S3_PRESIGN_ENDPOINT"),
		AWSRegion:         env("AWS_REGION", "us-east-1"),

		KeycloakURL:              os.Getenv("KEYCLOAK_URL"),
		KeycloakRealm:            env("KEYCLOAK_REALM", "people"),
		KeycloakClientID:         env("KEYCLOAK_PROVISIONER_CLIENT_ID", "people-provisioner"),
		KeycloakClientSecret:     os.Getenv("KEYCLOAK_PROVISIONER_CLIENT_SECRET"),
		TenantEventsTopic:        env("TENANT_EVENTS_TOPIC", "people.tenant.events.v1"),
		ProvisionerConsumerGroup: env("KAFKA_PROVISIONER_GROUP", "tenant-service-keycloak-provisioner"),
	}
	c.AssetsPublicBaseURL = env("ASSETS_PUBLIC_BASE_URL", fmt.Sprintf("https://%s.s3.%s.amazonaws.com", c.S3Bucket, c.AWSRegion))
	var err error
	if c.OutboxInterval, err = time.ParseDuration(env("OUTBOX_POLL_INTERVAL", "1s")); err != nil {
		return Config{}, fmt.Errorf("OUTBOX_POLL_INTERVAL: %w", err)
	}
	if c.LogoUploadTTL, err = time.ParseDuration(env("LOGO_UPLOAD_TTL", "10m")); err != nil {
		return Config{}, fmt.Errorf("LOGO_UPLOAD_TTL: %w", err)
	}
	if v := os.Getenv("OUTBOX_BATCH_SIZE"); v != "" {
		if c.OutboxBatchSize, err = strconv.Atoi(v); err != nil || c.OutboxBatchSize <= 0 {
			return Config{}, fmt.Errorf("OUTBOX_BATCH_SIZE inválido: %q", v)
		}
	}
	var missing []string
	for name, v := range map[string]string{"DATABASE_URL": c.DatabaseURL, "OIDC_ISSUER": c.OIDCIssuer, "OIDC_JWKS_URL": c.OIDCJWKSURL} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(c.KafkaBrokers) == 0 {
		missing = append(missing, "KAFKA_BROKERS")
	}
	if c.ProvisioningEnabled() && c.KeycloakClientSecret == "" {
		missing = append(missing, "KEYCLOAK_PROVISIONER_CLIENT_SECRET (obrigatória com KEYCLOAK_URL)")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("variáveis obrigatórias ausentes: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
