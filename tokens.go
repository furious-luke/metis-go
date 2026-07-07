package metis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SearchToken is a minted search token: the signed token plus its scope and
// expiry. The token is a short-lived JWT scoped to the account and Region; hand
// it to an in-region agent to present to the regional gateway's Search endpoint.
type SearchToken struct {
	// Token is the signed HS256 search token. Present it as an
	// "Authorization: Bearer <token>" header to a regional gateway.
	Token string `json:"token"`
	// Region is the slug of the region the token is scoped to.
	Region string `json:"region"`
	// ExpiresAt is the token expiry as an RFC 3339 timestamp (UTC).
	ExpiresAt string `json:"expires_at"`
}

// mintSearchTokenBody mirrors the control plane's mintSearchTokenRequest shape.
type mintSearchTokenBody struct {
	Region string `json:"region"`
}

// MintSearchToken mints a short-lived search token scoped to the account and the
// named region. The account must already be provisioned into the region (enable
// it with EnableRegion first). The returned token is verified offline by the
// regional gateway, so no server state is created.
func (c *Client) MintSearchToken(ctx context.Context, region string) (*SearchToken, error) {
	payload, err := json.Marshal(mintSearchTokenBody{Region: region})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/search-tokens", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(msg))
	}

	var token SearchToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &token, nil
}
