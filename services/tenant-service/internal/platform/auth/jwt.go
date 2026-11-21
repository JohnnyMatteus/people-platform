// Package auth valida tokens JWT emitidos pelo Keycloak e extrai o principal.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/app"
)

// ErrUnauthenticated indica token ausente, inválido ou expirado.
var ErrUnauthenticated = errors.New("não autenticado")

// Verifier valida tokens RS256 contra o JWKS do Keycloak.
type Verifier struct {
	issuer   string
	audience string
	keys     *jwksCache
}

// NewVerifier cria o verificador. O JWKS é buscado sob demanda e em cache.
func NewVerifier(issuer, audience, jwksURL string, client *http.Client) *Verifier {
	return &Verifier{issuer: issuer, audience: audience, keys: &jwksCache{url: jwksURL, client: client, keys: map[string]*rsa.PublicKey{}}}
}

type claims struct {
	jwt.RegisteredClaims
	TenantID    string `json:"tenant_id"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Verify valida assinatura, emissor, audiência e expiração, e devolve o principal.
func (v *Verifier) Verify(ctx context.Context, raw string) (app.Principal, error) {
	var c claims
	opts := []jwt.ParserOption{jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithExpirationRequired(), jwt.WithLeeway(30 * time.Second)}
	if v.audience != "" {
		opts = append(opts, jwt.WithAudience(v.audience))
	}
	_, err := jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.keys.get(ctx, kid)
	}, opts...)
	if err != nil {
		return app.Principal{}, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	p := app.Principal{Subject: c.Subject, Roles: c.RealmAccess.Roles}
	if c.TenantID != "" {
		id, err := uuid.Parse(c.TenantID)
		if err != nil {
			return app.Principal{}, fmt.Errorf("%w: claim tenant_id inválida", ErrUnauthenticated)
		}
		p.TenantID = id
	}
	return p, nil
}

// jwksCache guarda as chaves públicas por kid e rebusca quando aparece um kid novo
// (rotação de chave), no máximo uma vez a cada minRefresh para não amplificar ataques.
type jwksCache struct {
	url       string
	client    *http.Client
	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

const minRefresh = 30 * time.Second

func (c *jwksCache) get(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	if time.Since(c.fetchedAt) < minRefresh {
		return nil, fmt.Errorf("chave %q desconhecida", kid)
	}
	if err := c.refresh(ctx); err != nil {
		return nil, err
	}
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("chave %q desconhecida", kid)
}

func (c *jwksCache) refresh(ctx context.Context) error {
	c.fetchedAt = time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("buscar JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("buscar JWKS: status %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kid, Kty, Use, N, E string
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("decodificar JWKS: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	c.keys = keys
	return nil
}
