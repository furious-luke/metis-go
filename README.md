# Metis Go Client

A small, dependency-free Go client for the Metis multi-region RAG platform. It
runs on a customer's own backend and wraps what a server-side integration needs:

- **Manage documents** on the control plane (add / list / get / delete).
- **Mint search tokens** scoped to an account + region, to hand to an in-region
  agent.
- **Run semantic search** and fetch documents against a regional gateway.

The package imports only the Go standard library, so it can be vendored or
dropped into an external application without pulling in the rest of Metis.

## Installation

```bash
go get github.com/furious-luke/metis-go
```

```go
import metis "github.com/furious-luke/metis-go"
```

The module path is `github.com/furious-luke/metis-go`; the package it exports is
named `metis`, so call sites read `metis.New(...)`.

## Authentication

Two distinct credentials are in play; don't confuse them:

| Credential | Lifetime | Holder | Used for |
| --- | --- | --- | --- |
| **API key** | Long-lived, secret | Customer server (this library) | Managing documents; minting search tokens; sent as `Authorization: ApiKey <key>` |
| **Search token** | Short-lived JWT | In-region agent | Scoped to one account + region; sent as `Authorization: Bearer <token>` to the regional gateway for search/get |

Keep your API key on the server; never ship it to an agent — hand out a scoped,
short-lived search token instead.

## Usage

```go
c := metis.New("https://metis.example.com", "metis_api_key_...")
```

`New` applies a 30s request timeout. To control the transport, timeouts, or TLS,
pass your own `*http.Client`:

```go
c := metis.NewWithHTTPClient(baseURL, apiKey, &http.Client{Timeout: time.Minute})
```

## Status

This client is a skeleton. Document management, search-token minting, and the
regional search/get methods are added as the platform is fleshed out.
