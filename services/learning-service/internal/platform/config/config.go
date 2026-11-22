// Package config lê a configuração do serviço a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr      string
	DatabaseURL   string // usuário da aplicação, sujeito a RLS
	KafkaBrokers  []string
	RecruitTopic  string
	PrivacyTopic  string
	ConsumerGroup string
	OIDCIssuer    string
	OIDCAudience  string
	OIDCJWKSURL   string
}

// Load lê e valida a configuração. Falha cedo se faltar algo obrigatório.
func Load() (Config, error) {
	c := Config{
		HTTPAddr:      env("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		KafkaBrokers:  splitNonEmpty(os.Getenv("KAFKA_BROKERS")),
		RecruitTopic:  env("RECRUITMENT_EVENTS_TOPIC", "people.recruitment.events.v1"),
		PrivacyTopic:  env("PRIVACY_EVENTS_TOPIC", "people.privacy.events.v1"),
		ConsumerGroup: env("KAFKA_CONSUMER_GROUP", "learning-service"),
		OIDCIssuer:    os.Getenv("OIDC_ISSUER"),
		OIDCAudience:  env("OIDC_AUDIENCE", "people-api"),
		OIDCJWKSURL:   os.Getenv("OIDC_JWKS_URL"),
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
