package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const issuer = "http://keycloak.local/realms/people"

type keyServer struct {
	key *rsa.PrivateKey
	srv *httptest.Server
}

func newKeyServer(t *testing.T) *keyServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ks := &keyServer{key: key}
	ks.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(ks.srv.Close)
	return ks
}

func (ks *keyServer) sign(t *testing.T, kid string, mutate func(jwt.MapClaims)) string {
	t.Helper()
	c := jwt.MapClaims{
		"iss": issuer, "sub": "user-1", "aud": "people-api",
		"exp":          time.Now().Add(5 * time.Minute).Unix(),
		"realm_access": map[string]any{"roles": []string{"tenant-admin"}},
	}
	if mutate != nil {
		mutate(c)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(ks.key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerify_ValidTokenWithTenantClaim(t *testing.T) {
	ks := newKeyServer(t)
	v := NewVerifier(issuer, "people-api", ks.srv.URL, ks.srv.Client())
	tenant := uuid.New()
	p, err := v.Verify(context.Background(), ks.sign(t, "k1", func(c jwt.MapClaims) { c["tenant_id"] = tenant.String() }))
	if err != nil {
		t.Fatal(err)
	}
	if p.TenantID != tenant || !p.HasRole("tenant-admin") || p.Subject != "user-1" {
		t.Fatalf("principal inesperado: %+v", p)
	}
}

func TestVerify_Rejections(t *testing.T) {
	ks := newKeyServer(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	cases := map[string]func(t *testing.T) string{
		"expirado": func(t *testing.T) string {
			return ks.sign(t, "k1", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() })
		},
		"emissor errado": func(t *testing.T) string {
			return ks.sign(t, "k1", func(c jwt.MapClaims) { c["iss"] = "http://outro/realms/x" })
		},
		"audiência errada":   func(t *testing.T) string { return ks.sign(t, "k1", func(c jwt.MapClaims) { c["aud"] = "outra-api" }) },
		"kid desconhecido":   func(t *testing.T) string { return ks.sign(t, "k9", nil) },
		"tenant_id inválido": func(t *testing.T) string { return ks.sign(t, "k1", func(c jwt.MapClaims) { c["tenant_id"] = "abc" }) },
		"assinado por outra chave": func(t *testing.T) string {
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": issuer, "aud": "people-api", "exp": time.Now().Add(time.Minute).Unix()})
			tok.Header["kid"] = "k1"
			s, _ := tok.SignedString(other)
			return s
		},
		"algoritmo none": func(t *testing.T) string {
			tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"iss": issuer, "aud": "people-api", "exp": time.Now().Add(time.Minute).Unix()})
			s, _ := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
			return s
		},
		"lixo": func(*testing.T) string { return "nao.e.jwt" },
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewVerifier(issuer, "people-api", ks.srv.URL, ks.srv.Client())
			if _, err := v.Verify(context.Background(), mk(t)); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("esperava ErrUnauthenticated, obteve %v", err)
			}
		})
	}
}
