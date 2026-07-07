package metis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Region is a Metis region as seen by a customer: the active-region catalogue
// fields plus Enabled — whether the account is already provisioned into it.
type Region struct {
	ID           int64  `json:"id"`
	UUID         string `json:"uuid"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	ResidencyTag string `json:"residency_tag"`
	GatewayHost  string `json:"gateway_host"`
	// Enabled reports whether the account is currently provisioned into the
	// region. Enable it with EnableRegion, disable it with DisableRegion.
	Enabled bool `json:"enabled"`
}

// RegionPlacement is the account's placement in a region, returned by
// EnableRegion: the join status plus the resolved region details.
type RegionPlacement struct {
	Status       string `json:"status"`
	RegionID     int64  `json:"region_id"`
	RegionUUID   string `json:"region_uuid"`
	RegionSlug   string `json:"region_slug"`
	RegionName   string `json:"region_name"`
	ResidencyTag string `json:"residency_tag"`
	GatewayHost  string `json:"gateway_host"`
}

// ListRegions lists the active region catalogue, marking each region with
// whether the account is already enabled (provisioned) in it.
func (c *Client) ListRegions(ctx context.Context) ([]Region, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/regions", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(msg))
	}

	var regions []Region
	if err := json.NewDecoder(resp.Body).Decode(&regions); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return regions, nil
}

// EnableRegion enables a region for the account, identified by its slug, and
// returns the resulting placement. It is idempotent: enabling a region the
// account is already provisioned into returns the existing placement rather than
// an error. Enabling a fresh region kicks off a backfill of the account's
// existing documents into that region's replica.
func (c *Client) EnableRegion(ctx context.Context, slug string) (*RegionPlacement, error) {
	endpoint := c.baseURL + "/api/regions/" + url.PathEscape(slug) + "/enable"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 201 Created on a fresh enable, 200 OK when already provisioned.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(msg))
	}

	var placement RegionPlacement
	if err := json.NewDecoder(resp.Body).Decode(&placement); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &placement, nil
}

// DisableRegion disables a region for the account, identified by its slug: it
// removes the account's placement and signals the region's gateway to purge the
// account's replica. It returns nil on success and is idempotent — disabling a
// region the account is not enabled in is a no-op success.
func (c *Client) DisableRegion(ctx context.Context, slug string) error {
	endpoint := c.baseURL + "/api/regions/" + url.PathEscape(slug) + "/disable"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(msg))
	}
	return nil
}
