// Package keycloak implementa provisioning.IdentityProvisioner com a Admin REST API do Keycloak 26 (Organizations).
// Autentica com client credentials de um client de serviço (grant_type=client_credentials).
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/JohnnyMatteus/people-platform/services/tenant-service/internal/provisioning"
)

// TenantIDAttribute é o atributo da Organization que guarda o UUID do tenant; a busca por ele é o que dá idempotência
// (os realms de dev já trazem a Organization com este atributo).
const TenantIDAttribute = "tenant_id"

type Client struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	http         *http.Client
	now          func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

var _ provisioning.IdentityProvisioner = (*Client)(nil)

func New(baseURL, realm, clientID, clientSecret string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), realm: realm, clientID: clientID, clientSecret: clientSecret,
		http: httpClient, now: time.Now,
	}
}

type organization struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name"`
	Alias      string              `json:"alias"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}

// EnsureOrganization procura a Organization pelo atributo tenant_id e só cria se não existir. Se a criação colidir
// com outra instância fazendo o mesmo, busca de novo antes de desistir.
func (c *Client) EnsureOrganization(ctx context.Context, org provisioning.Organization) error {
	found, err := c.exists(ctx, org)
	if err != nil || found {
		return err
	}
	body := organization{Name: org.Name, Alias: org.Slug, Enabled: true, Attributes: map[string][]string{TenantIDAttribute: {org.TenantID.String()}}}
	status, detail, err := c.do(ctx, http.MethodPost, "/organizations", nil, body, nil)
	if err != nil {
		return err
	}
	switch {
	case status == http.StatusCreated:
		return nil
	case status == http.StatusBadRequest || status == http.StatusConflict:
		// Pode ser a corrida com outra instância (a Organization já existe) ou dado recusado (alias ocupado por outro tenant).
		if found, err := c.exists(ctx, org); err != nil || found {
			return err
		}
		return fmt.Errorf("%w: keycloak recusou a Organization (%d): %s", provisioning.ErrPermanent, status, detail)
	default:
		return fmt.Errorf("keycloak respondeu %d ao criar a Organization: %s", status, detail)
	}
}

func (c *Client) exists(ctx context.Context, org provisioning.Organization) (bool, error) {
	q := url.Values{"q": {TenantIDAttribute + ":" + org.TenantID.String()}, "max": {"1"}}
	var found []organization
	status, detail, err := c.do(ctx, http.MethodGet, "/organizations", q, nil, &found)
	if err != nil {
		return false, err
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("keycloak respondeu %d ao buscar a Organization: %s", status, detail)
	}
	return len(found) > 0, nil
}

// do chama a Admin API com o token de serviço e decodifica a resposta 200 em out. Um 401 renova o token uma vez.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) (int, string, error) {
	for attempt := 0; ; attempt++ {
		token, err := c.accessToken(ctx, attempt > 0)
		if err != nil {
			return 0, "", err
		}
		status, detail, err := c.request(ctx, method, path, query, body, out, token)
		if err != nil || status != http.StatusUnauthorized || attempt > 0 {
			return status, detail, err
		}
	}
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, body, out any, token string) (int, string, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, "", err
		}
		reader = bytes.NewReader(raw)
	}
	u := c.baseURL + "/admin/realms/" + url.PathEscape(c.realm) + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("chamar o Keycloak: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusOK && out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, "", fmt.Errorf("resposta do Keycloak ilegível: %w", err)
		}
	}
	return resp.StatusCode, truncate(string(raw), 300), nil
}

func (c *Client) accessToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && c.now().Before(c.expires) {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.clientID}, "client_secret": {c.clientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/realms/"+url.PathEscape(c.realm)+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("obter token do Keycloak: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak respondeu %d ao emitir o token de serviço: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("token do Keycloak ilegível")
	}
	c.token = tok.AccessToken
	// Renova um pouco antes de vencer para não usar um token no limite.
	c.expires = c.now().Add(time.Duration(tok.ExpiresIn)*time.Second - 10*time.Second)
	return c.token, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
