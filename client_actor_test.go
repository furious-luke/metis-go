package metis

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const defaultAPIKey = "metis_api_key_test"

// CustomerServerActor drives a Client against a fake control plane and regional
// gateway, standing in for a customer's own backend. The captured requests on
// the embedded fakes let specs assert on what the client actually sent.
type CustomerServerActor struct {
	t            *testing.T
	client       *Client
	controlPlane *fakeControlPlane
	gateway      *fakeGateway
	gatewayURL   string
}

// --- documents --------------------------------------------------------------

// MustAddDocument ingests a document and fails the test on error.
func (a *CustomerServerActor) MustAddDocument(title, content string, opts *AddDocumentOptions) *Document {
	a.t.Helper()
	doc, err := a.client.AddDocument(context.Background(), title, content, opts)
	require.NoError(a.t, err)
	return doc
}

// AddDocument ingests a document and returns the error for the spec to assert on.
func (a *CustomerServerActor) AddDocument(title, content string, opts *AddDocumentOptions) (*Document, error) {
	return a.client.AddDocument(context.Background(), title, content, opts)
}

// MustListDocuments lists documents and fails the test on error.
func (a *CustomerServerActor) MustListDocuments(opts *ListDocumentsOptions) []DocumentSummary {
	a.t.Helper()
	docs, err := a.client.ListDocuments(context.Background(), opts)
	require.NoError(a.t, err)
	return docs
}

// MustGetDocument fetches a document by uuid and fails the test on error.
func (a *CustomerServerActor) MustGetDocument(uuid string) *DocumentDetail {
	a.t.Helper()
	doc, err := a.client.GetDocument(context.Background(), uuid)
	require.NoError(a.t, err)
	return doc
}

// GetDocument fetches a document and returns the error for the spec to assert on.
func (a *CustomerServerActor) GetDocument(uuid string) (*DocumentDetail, error) {
	return a.client.GetDocument(context.Background(), uuid)
}

// MustDeleteDocument deletes a document by uuid and fails the test on error.
func (a *CustomerServerActor) MustDeleteDocument(uuid string) {
	a.t.Helper()
	require.NoError(a.t, a.client.DeleteDocument(context.Background(), uuid))
}

// DeleteDocument deletes a document and returns the error for the spec to assert.
func (a *CustomerServerActor) DeleteDocument(uuid string) error {
	return a.client.DeleteDocument(context.Background(), uuid)
}

// --- regions ----------------------------------------------------------------

// MustListRegions lists regions and fails the test on error.
func (a *CustomerServerActor) MustListRegions() []Region {
	a.t.Helper()
	regions, err := a.client.ListRegions(context.Background())
	require.NoError(a.t, err)
	return regions
}

// MustEnableRegion enables a region and fails the test on error.
func (a *CustomerServerActor) MustEnableRegion(slug string) *RegionPlacement {
	a.t.Helper()
	placement, err := a.client.EnableRegion(context.Background(), slug)
	require.NoError(a.t, err)
	return placement
}

// MustDisableRegion disables a region and fails the test on error.
func (a *CustomerServerActor) MustDisableRegion(slug string) {
	a.t.Helper()
	require.NoError(a.t, a.client.DisableRegion(context.Background(), slug))
}

// --- search tokens ----------------------------------------------------------

// MustMintSearchToken mints a search token and fails the test on error.
func (a *CustomerServerActor) MustMintSearchToken(region string) *SearchToken {
	a.t.Helper()
	token, err := a.client.MintSearchToken(context.Background(), region)
	require.NoError(a.t, err)
	return token
}

// --- gateway search ---------------------------------------------------------

// MustSearch runs a gateway search and fails the test on error.
func (a *CustomerServerActor) MustSearch(searchToken, query string, opts *SearchOptions) []SearchResult {
	a.t.Helper()
	results, err := a.client.Search(context.Background(), a.gatewayURL, searchToken, query, opts)
	require.NoError(a.t, err)
	return results
}

// Search runs a gateway search and returns the error for the spec to assert on.
func (a *CustomerServerActor) Search(searchToken, query string, opts *SearchOptions) ([]SearchResult, error) {
	return a.client.Search(context.Background(), a.gatewayURL, searchToken, query, opts)
}

// --- captured requests + response overrides ---------------------------------

// LastControlPlaneRequest returns the method, path+query, raw body, and
// Authorization header the control plane last received.
func (a *CustomerServerActor) LastControlPlaneRequest() (method, target, body, authHeader string) {
	return a.controlPlane.lastMethod, a.controlPlane.lastTarget, a.controlPlane.lastBody, a.controlPlane.lastAuth
}

// LastSearchRequest returns the path+query, raw body, and bearer token the
// gateway last saw.
func (a *CustomerServerActor) LastSearchRequest() (target, body, authHeader string) {
	return a.gateway.lastTarget, a.gateway.lastBody, a.gateway.lastAuth
}

// SetControlPlaneResponse overrides the control plane's reply to the next
// request, regardless of route.
func (a *CustomerServerActor) SetControlPlaneResponse(status int, body string) {
	a.controlPlane.override = &fakeResponse{status: status, body: body}
}

// SetSearchResponse overrides the gateway's reply to the next search request.
func (a *CustomerServerActor) SetSearchResponse(status int, body string) {
	a.gateway.status = status
	a.gateway.body = body
}

// --- fakes ------------------------------------------------------------------

type fakeResponse struct {
	status int
	body   string
}

// fakeControlPlane stands in for the Metis control-plane API. It routes on
// method+path to a sensible default response per endpoint, captures the last
// request, and honours a one-shot override set by SetControlPlaneResponse.
type fakeControlPlane struct {
	override *fakeResponse

	lastMethod string
	lastTarget string
	lastBody   string
	lastAuth   string
}

func newFakeControlPlane() *fakeControlPlane {
	return &fakeControlPlane{}
}

func (f *fakeControlPlane) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.lastMethod = r.Method
	f.lastTarget = r.URL.RequestURI()
	f.lastAuth = r.Header.Get("Authorization")
	raw, _ := io.ReadAll(r.Body)
	f.lastBody = string(raw)

	if f.override != nil {
		ov := f.override
		f.override = nil
		f.write(w, ov.status, ov.body)
		return
	}

	status, body := f.defaultResponse(r)
	f.write(w, status, body)
}

func (f *fakeControlPlane) write(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// defaultResponse returns the seeded happy-path reply for a route.
func (f *fakeControlPlane) defaultResponse(r *http.Request) (int, string) {
	path := r.URL.Path
	switch {
	case r.Method == http.MethodPost && path == "/api/documents":
		return http.StatusAccepted, `{"id":7,"uuid":"doc-1","status":"pending","unchanged":false}`
	case r.Method == http.MethodGet && path == "/api/documents":
		return http.StatusOK, `[{"uuid":"doc-1","key":"k1","title":"First","encoding":"utf-8",` +
			`"status":"ready","version":3,"created_at":"2026-07-01T10:00:00Z","updated_at":"2026-07-02T10:00:00Z"}]`
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/api/documents/"):
		return http.StatusOK, `{"uuid":"doc-1","key":"k1","title":"First","content":"hello world",` +
			`"encoding":"utf-8","status":"ready","version":3,"created_at":"2026-07-01T10:00:00Z",` +
			`"updated_at":"2026-07-02T10:00:00Z","tags":["alpha","beta"],` +
			`"chunks":[{"uuid":"chunk-1","index":0,"content":"hello world","token_count":2}]}`
	case r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/documents/"):
		return http.StatusNoContent, ""
	case r.Method == http.MethodGet && path == "/api/regions":
		return http.StatusOK, `[{"id":1,"uuid":"region-1","slug":"eu-west-1","name":"EU West",` +
			`"residency_tag":"eu","gateway_host":"gw.eu.example.com","enabled":true}]`
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/enable"):
		return http.StatusCreated, `{"status":"active","region_id":1,"region_uuid":"region-1",` +
			`"region_slug":"eu-west-1","region_name":"EU West","residency_tag":"eu",` +
			`"gateway_host":"gw.eu.example.com"}`
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/disable"):
		return http.StatusNoContent, ""
	case r.Method == http.MethodPost && path == "/api/search-tokens":
		return http.StatusOK, `{"token":"search-jwt","region":"eu-west-1","expires_at":"2026-07-07T13:00:00Z"}`
	default:
		return http.StatusNotFound, `{"error":"no fake route"}`
	}
}

// fakeGateway stands in for a regional gateway's POST /api/search endpoint. It
// captures the last request and serves a configurable response.
type fakeGateway struct {
	status int
	body   string

	lastTarget string
	lastBody   string
	lastAuth   string
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{
		status: http.StatusOK,
		body: `{"results":[{"document_uuid":"doc-1","chunk_uuid":"chunk-1","chunk_index":0,` +
			`"title":"First","content":"hello world","score":0.91,"distance":0.12}]}`,
	}
}

func (f *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.lastTarget = r.URL.RequestURI()
	f.lastAuth = r.Header.Get("Authorization")
	raw, _ := io.ReadAll(r.Body)
	f.lastBody = string(raw)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.status)
	_, _ = io.WriteString(w, f.body)
}
