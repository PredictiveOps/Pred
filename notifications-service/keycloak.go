package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2/clientcredentials"
)

type KeycloakClient struct {
	baseURL string
	realm   string
	http    *http.Client
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
	base := strings.TrimRight(baseURL, "/")
	cfg := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", base, realm),
	}
	return &KeycloakClient{
		baseURL: base,
		realm:   realm,
		http:    cfg.Client(context.Background()),
	}
}

// GetTenantRecipients returns all Keycloak users whose tenant_id attribute
// matches tenantID, mapped to Recipient values.
func (k *KeycloakClient) GetTenantRecipients(ctx context.Context, tenantID string) ([]Recipient, error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/users?q=tenant_id:%s&max=1000",
		k.baseURL, k.realm, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := k.http.Do(req)
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
