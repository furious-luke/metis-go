package metis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SearchResult is one hit returned by Search: the chunk and enough document
// identity to resolve it, the rerank relevance score (higher is more relevant),
// and the underlying ANN cosine distance for observability.
type SearchResult struct {
	DocumentUUID string  `json:"document_uuid"`
	ChunkUUID    string  `json:"chunk_uuid"`
	ChunkIndex   int32   `json:"chunk_index"`
	Title        string  `json:"title"`
	Content      string  `json:"content"`
	Score        float64 `json:"score"`
	Distance     float32 `json:"distance"`
	// TagKeys holds the document's tag keys, populated only when SearchOptions.IncludeTags
	// is set. It is nil otherwise — the gateway does not query tags on the default path.
	TagKeys []string `json:"tag_keys,omitempty"`
}

// SearchOptions configures a Search request. A nil *SearchOptions applies no tag
// filter and uses the gateway's default result count.
type SearchOptions struct {
	// TagQuery, if set, is a boolean tag-filter expression (e.g. "(a AND b) OR c")
	// that constrains which documents are searched. Empty applies no tag filter.
	TagQuery string
	// K, if set, is the number of results to return. Zero (or negative) uses the
	// gateway default (10).
	K int
	// IncludeTags, if true, asks the gateway to populate each result's TagKeys with
	// the matching document's tag keys. It is off by default so the search path does
	// no extra work; it is mainly useful for inspecting a region's corpus.
	IncludeTags bool
}

// searchBody mirrors the gateway's searchRequest JSON shape.
type searchBody struct {
	Query       string `json:"query"`
	TagQuery    string `json:"tag_query,omitempty"`
	K           int    `json:"k,omitempty"`
	IncludeTags bool   `json:"include_tags,omitempty"`
}

// searchResponseBody mirrors the gateway's searchResponse JSON shape.
type searchResponseBody struct {
	Results []SearchResult `json:"results"`
}

// Search runs a semantic search against a regional gateway.
//
// gatewayURL is the base URL of the region's gateway; a trailing slash is
// trimmed. searchToken is the bearer token authorizing the search — mint one
// with MintSearchToken and relay it to the in-region caller. query is the search
// text; opts carries the optional tag-filter expression and result count. A nil
// opts applies no tag filter and uses the gateway's default result count.
//
// The gateway is search-only: it embeds the query in-region, retrieves an ANN
// candidate pool from the regional replica, reranks it, and returns the top-k
// hits ordered by relevance descending.
func (c *Client) Search(ctx context.Context, gatewayURL, searchToken, query string, opts *SearchOptions) ([]SearchResult, error) {
	body := searchBody{Query: query}
	if opts != nil {
		body.TagQuery = opts.TagQuery
		body.K = opts.K
		body.IncludeTags = opts.IncludeTags
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := strings.TrimRight(gatewayURL, "/") + "/api/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+searchToken)
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

	var out searchResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return out.Results, nil
}
