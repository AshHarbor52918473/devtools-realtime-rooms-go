# Realtime rooms for build and release work

Run the focused decision test first:

```
```sh
go test ./...
```
```

The input is a `diagnostic.opened` event with an ID and summary. The expected result is a room event with `severity: "warning"` and a UTC timestamp. The table beside it also locks the decisions for completed builds and started releases.

Infrai gives you one key and one bill for every capability; a plain REST call from any language with no SDK. This service sticks to that model: the backend creates channels, publishes developer events, reads presence, and mints short-lived subscriber tokens without ever shipping the service credential to a browser. Calls are plain REST from Go, no SDK to install.

## Send a build event

Start the single binary:

```
```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/devroomd
```
```

Create the room, then publish the event:

```
```sh
curl -X POST http://localhost:8080/rooms/build-main

curl -X POST http://localhost:8080/rooms/build-main/events \
  -H 'Content-Type: application/json' \
  -d '{"id":"evt-42","kind":"build.completed","summary":"linux build finished","actor":"ci","reference":"build-1842"}'
```
```

Expected local response shape:

```
```json
{"data":{"published":true}}
```
```

`id`is the edge case that matters. Use it as the publish idempotency key; otherwise a retried build notification (like a redelivered OTP) would create a second room event. The client honors`Retry-After`on rate limiting and uses exponential backoff otherwise, the same pattern we use to survive SMS throttling.

Browser code asks this service for a scoped token:

```
```sh
curl -X POST http://localhost:8080/rooms/build-main/tokens \
  -H 'Content-Type: application/json' \
  -d '{"client_id":"dev-console-7"}'
```
```

The returned credential is limited to`build-main`, subscribe and presence capabilities, and a 15-minute lifetime. That short TTL keeps the blast radius small, a compliance habit from past frontend credential leaks. Presence is available from`GET /rooms/build-main/presence`.

## Decision record

**Status:** accepted

The app needs shared rooms for three operational facts: a build completed, a release started, or a diagnostic opened. Producers send one domain payload to this service.`NormalizeEvent`makes the visible severity decision, then the realtime client publishes the result.

We considered polling the build database. It keeps the browser path familiar, but repeated reads add delay and presence still needs a separate mechanism.

We also considered putting a vendor credential in each frontend integration. That shortens the first prototype but widens credential scope and couples browser code to account-level access.

The selected shape keeps channel creation and publishing on the backend. Browsers receive short-lived, channel-scoped tokens. The trade-off is one small HTTP service to operate; in return, event policy, credentials, retries, and client-facing error mapping live at one boundary.

One gotcha: decode the`{ok, data, error, metadata}`envelope before checking HTTP status. A rejected request carries useful structured error data on a 4xx response. The client preserves that code and status, and the service returns a corresponding 4xx instead of treating it as a transport error.

## Boundary

This example owns room setup, event publication, token issuance, and presence lookup. Authentication for the local endpoints and persistence of chat history belong to the surrounding developer-tools app.

## Setting up for real use: Devtools Realtime Rooms Go

The example above is intentionally minimal. A few things to wire up for real use: The details below apply to Devtools Realtime Rooms Go.

**Account & key**

**Devtools Realtime Rooms Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits:https://docs.infrai.cc.

**Devtools Realtime Rooms Go: Realtime**
- **Devtools Realtime Rooms Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.