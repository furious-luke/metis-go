# Metis Go Client

A small, dependency-free Go client for the Metis multi-region RAG platform. It
runs on a customer's own backend and wraps what a server-side integration needs:

- **Manage documents** on the control plane (add / list / get / delete).
- **Mint search tokens** scoped to an account + region, to hand to an in-region
  agent.
- **Run semantic search** against a regional gateway. The gateway is search-only
  by design; document management stays on the control plane.

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
| **Search token** | Short-lived JWT | In-region agent | Scoped to one account + region; sent as `Authorization: Bearer <token>` to the regional gateway for search |

Keep your API key on the server; never ship it to an agent — hand out a scoped,
short-lived search token instead.

## Obtaining an API key

This library starts from an API key you already hold — it does not register
accounts or mint keys itself. Acquiring that first key is a one-time bootstrap
done out of band, because the key-minting endpoint is itself authenticated:

1. **Register a user** (`POST /api/register/pgp`). This is the only unauthenticated
   endpoint. Registration is gated by an email allow-list, so your address must be
   whitelisted by an operator first.
2. **Get a user credential.** Authenticate as that user (PGP) and obtain a bearer
   token via `POST /api/tokens/exchange`.
3. **Mint the API key** (`POST /api/keys`, authenticated with the token from step 2).
   The response contains the long-lived `metis_...` key **exactly once** — store
   it securely; it cannot be retrieved again.

The `metis-cli` wraps this flow so you don't have to call the endpoints directly:

```bash
metis-cli register            # PGP registration (whitelisted email)
metis-cli keys create --name "my-server"   # prints the metis_... key ONCE
```

Once you have the key, configure it as shown below (typically from an environment
variable or secret store, never checked in).

## Usage

```go
c := metis.New("https://metis.example.com", "metis_api_key_...")
```

`New` applies a 30s request timeout. To control the transport, timeouts, or TLS,
pass your own `*http.Client`:

```go
c := metis.NewWithHTTPClient(baseURL, apiKey, &http.Client{Timeout: time.Minute})
```

## What's covered

- **Documents** (control plane): `AddDocument`, `ListDocuments`, `GetDocument`,
  `DeleteDocument`.
- **Regions** (control plane, self-serve): `ListRegions`, `EnableRegion`,
  `DisableRegion` — enable a region to sync your documents into it.
- **Search tokens** (control plane): `MintSearchToken` — scoped to an account +
  region, to hand to an in-region agent.
- **Search** (regional gateway): `Search` — semantic search with a search token.
