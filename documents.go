package metis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Document is the result of ingesting a document via AddDocument. It carries
// enough for the caller to poll the document's embedding status by UUID.
type Document struct {
	// ID is the control plane's internal numeric document id.
	ID int64 `json:"id"`
	// UUID is the stable external identifier ingest returns and replication keys
	// on. Use it with GetDocument and DeleteDocument.
	UUID string `json:"uuid"`
	// Status is the document's embedding status (e.g. "pending").
	Status string `json:"status"`
	// Unchanged reports whether an upsert was a no-op — the content and metadata
	// were identical to what was already stored, so nothing was re-embedded or
	// replicated. It is only ever true on an upsert (a key that matched a live
	// document); a fresh create leaves it false.
	Unchanged bool `json:"unchanged"`
}

// DocumentSummary is the light per-document view returned by ListDocuments.
// Content is deliberately omitted so list responses stay small; use GetDocument
// to fetch a document's content and chunks.
type DocumentSummary struct {
	UUID      string `json:"uuid"`
	Key       string `json:"key,omitempty"`
	Title     string `json:"title"`
	Encoding  string `json:"encoding"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// DocumentDetail is the full per-document view returned by GetDocument: the
// document (including its content) plus its chunks and tag keys.
type DocumentDetail struct {
	UUID      string          `json:"uuid"`
	Key       string          `json:"key,omitempty"`
	Title     string          `json:"title"`
	Content   string          `json:"content"`
	Encoding  string          `json:"encoding"`
	Status    string          `json:"status"`
	Version   int64           `json:"version"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	Tags      []string        `json:"tags"`
	Chunks    []DocumentChunk `json:"chunks"`
}

// DocumentChunk is a chunk as returned in a DocumentDetail: its identity, order,
// and text. The embedding vector is deliberately excluded — it is large and not
// useful to the customer.
type DocumentChunk struct {
	UUID       string `json:"uuid"`
	Index      int32  `json:"index"`
	Content    string `json:"content"`
	TokenCount int32  `json:"token_count"`
}

// AddDocumentOptions configures an AddDocument request. All fields are optional;
// a nil *AddDocumentOptions ingests with no key, default encoding, and no tags.
type AddDocumentOptions struct {
	// Key, if set, is the caller's stable identifier for the document. When it
	// names an existing live document for the account, ingest upserts that
	// document (content-hash gated); otherwise a new document is created.
	Key string
	// Encoding, if set, is the document's content encoding. Empty uses the server
	// default (and, on an upsert, preserves the stored encoding).
	Encoding string
	// Tags is the authoritative set of tag keys to attach. On an upsert it
	// replaces the document's tag membership, so it reflects exactly the keys
	// supplied here.
	Tags []string
}

// addDocumentBody mirrors the control plane's createDocumentRequest JSON shape.
type addDocumentBody struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Encoding string   `json:"encoding,omitempty"`
	Key      string   `json:"key,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// AddDocument ingests a document on the control plane and returns its identity
// and embedding status. title and content are required; opts carries the
// optional key, encoding, and tags. A nil opts ingests with defaults.
//
// The control plane chunks and embeds the document asynchronously, so a
// successful call reports the document as pending rather than searchable. When
// opts.Key names an existing live document the ingest upserts it; an upsert whose
// content and metadata are unchanged returns with Unchanged set true.
func (c *Client) AddDocument(ctx context.Context, title, content string, opts *AddDocumentOptions) (*Document, error) {
	body := addDocumentBody{Title: title, Content: content}
	if opts != nil {
		body.Key = opts.Key
		body.Encoding = opts.Encoding
		body.Tags = opts.Tags
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/documents", bytes.NewReader(payload))
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

	// The server returns 202 Accepted on ingest (async embed) and 200 OK on an
	// unchanged upsert; accept both, plus 201 for forward compatibility.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(msg))
	}

	var doc Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &doc, nil
}

// ListDocumentsOptions configures a ListDocuments request. A nil
// *ListDocumentsOptions uses the server default limit.
type ListDocumentsOptions struct {
	// Limit caps the number of documents returned. Zero (or negative) uses the
	// server default (100).
	Limit int
}

// ListDocuments lists the account's live documents (soft-deleted excluded),
// newest first. A nil opts uses the server default limit.
func (c *Client) ListDocuments(ctx context.Context, opts *ListDocumentsOptions) ([]DocumentSummary, error) {
	endpoint := c.baseURL + "/api/documents"
	if opts != nil && opts.Limit > 0 {
		endpoint += "?" + url.Values{"limit": {strconv.Itoa(opts.Limit)}}.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var docs []DocumentSummary
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return docs, nil
}

// GetDocument returns a single document by its UUID — the stable external
// identifier AddDocument returns — scoped to the account. The response carries
// the full document plus its chunks and tag keys (no embeddings).
func (c *Client) GetDocument(ctx context.Context, uuid string) (*DocumentDetail, error) {
	endpoint := c.baseURL + "/api/documents/" + url.PathEscape(uuid)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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

	var detail DocumentDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &detail, nil
}

// DeleteDocument soft-deletes a document by its UUID (scoped to the account) and
// fans a delete out to the account's regions so the replica is dropped. It
// returns nil on success. A miss (already deleted / not owned) surfaces as an
// error carrying the server's 404.
func (c *Client) DeleteDocument(ctx context.Context, uuid string) error {
	endpoint := c.baseURL + "/api/documents/" + url.PathEscape(uuid)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
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
