// Package metis is a Go client for the Metis multi-region RAG platform.
//
// Metis is a regionally distributed retrieval-augmented generation service:
// customers add documents to a central control plane, which chunks and embeds
// them and replicates the resulting vectors into each region the account is
// provisioned for. Agents running in a region query that region's gateway
// directly for low-latency semantic search. This package is the server-side
// half of that story — it is intended to run on a customer's own backend,
// authenticated with the customer's API key.
//
// It covers what a customer server needs to do:
//
//   - Manage documents on the control plane (add / list / get / delete).
//   - Mint short-lived search tokens scoped to an account + region, to hand to
//     an in-region agent.
//   - Run semantic search against a regional gateway with a search token. The
//     gateway is search-only by design; document management is a control-plane
//     concern (there is no document get/list on the gateway).
//
// The package depends only on the standard library so it can be vendored or
// imported into external applications without pulling in the rest of Metis.
//
// # Authentication
//
// Two distinct credentials are in play; do not confuse them:
//
//   - The API key identifies the customer to the control plane. It is
//     long-lived and secret; keep it server-side. The Client sends it as an
//     "Authorization: ApiKey <key>" header.
//   - The search token is a short-lived JWT scoped to one account + region. It
//     is minted from the control plane, handed to an in-region agent, and
//     presented to the regional gateway as an "Authorization: Bearer <token>"
//     header. The gateway verifies it offline.
package metis

import (
	"net/http"
	"strings"
	"time"
)

// defaultTimeout is the request timeout applied by New. Callers needing a
// different timeout (or transport, proxy, etc.) should use NewWithHTTPClient.
const defaultTimeout = 30 * time.Second

// Client talks to the Metis control plane on behalf of a single customer
// account. It is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New creates a Client with sensible defaults.
//
// baseURL is the root URL of the Metis control plane (e.g.
// "https://metis.example.com"); a trailing slash is trimmed. apiKey is the
// customer's API key, sent as an "Authorization: ApiKey <key>" header on
// control-plane requests.
func New(baseURL, apiKey string) *Client {
	return NewWithHTTPClient(baseURL, apiKey, &http.Client{Timeout: defaultTimeout})
}

// NewWithHTTPClient is like New but uses the supplied *http.Client, letting the
// caller control timeouts, transport, proxies, and TLS configuration. The
// client must not be nil.
func NewWithHTTPClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  httpClient,
	}
}
