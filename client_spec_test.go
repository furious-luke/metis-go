package metis

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These specs describe the observable behaviour of the Metis client library
// from a customer server's point of view. They are human-owned contracts: they
// say what the client does, not how it is built. See client_arrange_test.go and
// client_actor_test.go for the supporting harness.

func TestSpec_AddDocument_ReturnsIdentityAndStatus(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	doc := server.MustAddDocument("First", "hello world", nil)
	assert.Equal(t, int64(7), doc.ID)
	assert.Equal(t, "doc-1", doc.UUID)
	assert.Equal(t, "pending", doc.Status)
	assert.False(t, doc.Unchanged)
}

func TestSpec_AddDocument_AuthenticatesWithAPIKey(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustAddDocument("First", "hello world", nil)
	_, _, _, auth := server.LastControlPlaneRequest()
	assert.Equal(t, "ApiKey "+defaultAPIKey, auth)
}

func TestSpec_AddDocument_ForwardsKeyEncodingAndTags(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustAddDocument("First", "hello world", &AddDocumentOptions{
		Key:      "k1",
		Encoding: "utf-8",
		Tags:     []string{"alpha", "beta"},
	})
	method, target, body, _ := server.LastControlPlaneRequest()
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/api/documents", target)
	assert.Contains(t, body, `"key":"k1"`)
	assert.Contains(t, body, `"encoding":"utf-8"`)
	assert.Contains(t, body, `"tags":["alpha","beta"]`)
}

func TestSpec_AddDocument_SurfacesServerError(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetControlPlaneResponse(http.StatusBadRequest, "content is required")
	_, err := server.AddDocument("First", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestSpec_ListDocuments_ReturnsSummaries(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	docs := server.MustListDocuments(nil)
	require.Len(t, docs, 1)
	assert.Equal(t, "doc-1", docs[0].UUID)
	assert.Equal(t, "First", docs[0].Title)
	assert.Equal(t, int64(3), docs[0].Version)
}

func TestSpec_ListDocuments_ForwardsLimit(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustListDocuments(&ListDocumentsOptions{Limit: 25})
	_, target, _, _ := server.LastControlPlaneRequest()
	assert.Contains(t, target, "limit=25")
}

func TestSpec_GetDocument_ReturnsContentChunksAndTags(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	doc := server.MustGetDocument("doc-1")
	assert.Equal(t, "doc-1", doc.UUID)
	assert.Equal(t, "hello world", doc.Content)
	assert.Equal(t, []string{"alpha", "beta"}, doc.Tags)
	require.Len(t, doc.Chunks, 1)
	assert.Equal(t, "chunk-1", doc.Chunks[0].UUID)
	assert.Equal(t, int32(2), doc.Chunks[0].TokenCount)
}

func TestSpec_GetDocument_SurfacesNotFound(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetControlPlaneResponse(http.StatusNotFound, "document not found")
	_, err := server.GetDocument("missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

func TestSpec_DeleteDocument_SucceedsOnNoContent(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustDeleteDocument("doc-1")
	method, target, _, auth := server.LastControlPlaneRequest()
	assert.Equal(t, http.MethodDelete, method)
	assert.Equal(t, "/api/documents/doc-1", target)
	assert.Equal(t, "ApiKey "+defaultAPIKey, auth)
}

func TestSpec_DeleteDocument_SurfacesServerError(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetControlPlaneResponse(http.StatusNotFound, "document not found")
	err := server.DeleteDocument("missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "document not found")
}

// A delete of a document the account no longer has (a 404) wraps the
// ErrDocumentNotFound sentinel so callers can treat "already gone" as success.
func TestSpec_DeleteDocument_NotFoundWrapsSentinel(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetControlPlaneResponse(http.StatusNotFound, "document not found")
	err := server.DeleteDocument("missing")
	assert.ErrorIs(t, err, ErrDocumentNotFound)
}

// Only a 404 maps to ErrDocumentNotFound; any other failing status is a plain
// error, so an idempotent-delete caller does not mistake it for "already gone".
func TestSpec_DeleteDocument_OtherFailuresDoNotWrapSentinel(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetControlPlaneResponse(http.StatusInternalServerError, "boom")
	err := server.DeleteDocument("doc-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDocumentNotFound)
}

func TestSpec_ListRegions_ReturnsCatalogueWithEnablement(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	regions := server.MustListRegions()
	require.Len(t, regions, 1)
	assert.Equal(t, "eu-west-1", regions[0].Slug)
	assert.Equal(t, "gw.eu.example.com", regions[0].GatewayHost)
	assert.True(t, regions[0].Enabled)
}

func TestSpec_ListRegions_AuthenticatesWithAPIKey(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustListRegions()
	_, _, _, auth := server.LastControlPlaneRequest()
	assert.Equal(t, "ApiKey "+defaultAPIKey, auth)
}

func TestSpec_EnableRegion_ReturnsPlacement(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	placement := server.MustEnableRegion("eu-west-1")
	assert.Equal(t, "active", placement.Status)
	assert.Equal(t, "eu-west-1", placement.RegionSlug)
	assert.Equal(t, "gw.eu.example.com", placement.GatewayHost)
	method, target, _, _ := server.LastControlPlaneRequest()
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/api/regions/eu-west-1/enable", target)
}

func TestSpec_EnableRegion_AcceptsAlreadyEnabled(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	// 200 OK is the already-provisioned reply; the client accepts it like a 201.
	server.SetControlPlaneResponse(http.StatusOK, `{"status":"active","region_id":1,"region_uuid":"region-1",`+
		`"region_slug":"eu-west-1","region_name":"EU West","residency_tag":"eu","gateway_host":"gw.eu.example.com"}`)
	placement := server.MustEnableRegion("eu-west-1")
	assert.Equal(t, "active", placement.Status)
}

func TestSpec_DisableRegion_SucceedsOnNoContent(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustDisableRegion("eu-west-1")
	method, target, _, auth := server.LastControlPlaneRequest()
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/api/regions/eu-west-1/disable", target)
	assert.Equal(t, "ApiKey "+defaultAPIKey, auth)
}

func TestSpec_MintSearchToken_ReturnsScopedToken(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	token := server.MustMintSearchToken("eu-west-1")
	assert.Equal(t, "search-jwt", token.Token)
	assert.Equal(t, "eu-west-1", token.Region)
	assert.Equal(t, "2026-07-07T13:00:00Z", token.ExpiresAt)
	_, target, body, _ := server.LastControlPlaneRequest()
	assert.Equal(t, "/api/search-tokens", target)
	assert.Contains(t, body, `"region":"eu-west-1"`)
}

func TestSpec_Search_ReturnsRankedResults(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	results := server.MustSearch("search-jwt", "hello", nil)
	require.Len(t, results, 1)
	assert.Equal(t, "doc-1", results[0].DocumentUUID)
	assert.Equal(t, "chunk-1", results[0].ChunkUUID)
	assert.Equal(t, 0.91, results[0].Score)
}

func TestSpec_Search_AuthenticatesWithBearerToken(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustSearch("search-jwt", "hello", nil)
	target, _, auth := server.LastSearchRequest()
	assert.Equal(t, "/api/search", target)
	assert.Equal(t, "Bearer search-jwt", auth)
}

func TestSpec_Search_ForwardsTagQueryAndK(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.MustSearch("search-jwt", "hello", &SearchOptions{TagQuery: "alpha AND beta", K: 5})
	_, body, _ := server.LastSearchRequest()
	assert.Contains(t, body, `"query":"hello"`)
	assert.Contains(t, body, `"tag_query":"alpha AND beta"`)
	assert.Contains(t, body, `"k":5`)
}

func TestSpec_Search_SurfacesServerError(t *testing.T) {
	a := newArranger(t)
	server := a.CustomerServer()
	server.SetSearchResponse(http.StatusUnauthorized, "invalid search token")
	_, err := server.Search("bad-token", "hello", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid search token")
}
