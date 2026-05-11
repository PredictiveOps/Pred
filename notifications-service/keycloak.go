package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type KeycloakClient struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

type Recipient struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"` // required for type="email"
}

type kcUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func NewKeycloakClient(baseURL, realm, clientID, clientSecret string) *KeycloakClient {
	return &KeycloakClient{
		baseURL:      strings.TrimRight(baseURL, "/"),
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

func (k *KeycloakClient) token(ctx context.Context) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if time.Now().Before(k.tokenExpiry) {
		return k.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {k.clientID},
		"client_secret": {k.clientSecret},
	}

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", k.baseURL, k.realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	k.accessToken = result.AccessToken
	// Subtract 10 s to avoid using a token right as it expires.
	k.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-10) * time.Second)
	return k.accessToken, nil
}

// GetTenantRecipients returns all Keycloak users whose tenant_id attribute
// matches tenantID, mapped to Recipient values.
func (k *KeycloakClient) GetTenantRecipients(ctx context.Context, tenantID string) ([]Recipient, error) {
	tok, err := k.token(ctx)
	if err != nil {
		return nil, fmt.Errorf("keycloak token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/admin/realms/%s/users?q=tenant_id:%s&max=1000",
		k.baseURL, k.realm, url.QueryEscape(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("users request failed (%d): %s", resp.StatusCode, body)
	}

	var users []kcUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	recipients := make([]Recipient, 0, len(users))
	for _, u := range users {
		recipients = append(recipients, Recipient{
			UserID: u.ID,
			Email:  u.Email,
		})
	}
	return recipients, nil
}
