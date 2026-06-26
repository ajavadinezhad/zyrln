# Contributing

User setup: [README.md](../README.md) · [فارسی](fa/guide.md). Relay deploy: [cloudflare-setup.md](cloudflare-setup.md).

## Project structure

```
zyrln/
├── platforms/
│   ├── desktop/        # CLI + browser GUI (main package)
│   │   ├── main.go
│   │   └── gui/
│   └── mobile/         # gomobile bindings for Android
│       └── mobile.go   # StartTunnel, StartDirect, AttachTUN, Stop, Ping, …
│
├── relay/
│   ├── core/           # Stable API re-export (desktop + mobile import this)
│   ├── route/          # Google direct, domestic bypass, TLS fragmentation
│   ├── appscript/      # Domain-fronted HTTP relay + Coalescer
│   ├── mitm/           # Local HTTP CONNECT + SOCKS5 MITM (desktop)
│   ├── tunnel/         # Raw TCP-over-HTTP tunnel client (Android relay path)
│   ├── tun/            # TUN IPv4 forwarder (Android VPN fd)
│   ├── exit/           # VPS / self-hosted exit relay binary
│   ├── conn/, log/, netdial/
│   └── deploy/
│       ├── apps-script/Code.gs
│       └── cloudflare/worker.js
│
├── android/
│   └── app/src/main/java/com/zyrln/relay/
│       ├── MainActivity.kt
│       ├── RelayVpnService.kt
│       ├── SplitTunnelAppsActivity.kt
│       └── ConfigUtils.kt
│
├── docs/
├── Makefile
└── go.mod
```

## Relay packages

| Package | Used by | Role |
|---------|---------|------|
| `route` | `tunnel`, `mitm`, `tun` | Per-host routing before relay/tunnel |
| `appscript` | `mitm`, `tunnel` | Outbound HTTP to Apps Script (domain-fronted) |
| `mitm` | Desktop | Browser proxy + local CA MITM |
| `tunnel` | Android | CONNECT proxy over Apps Script `/tunnel` |
| `tun` | Android | VPN TUN → local proxy (with `AttachTUN`) |
| `exit` | VPS deploy | `/relay` + `/tunnel` exit |

When changing relay JSON or `Code.gs`, update **`deploy/apps-script/Code.gs`** and **`appscript`** together. Tunnel fields (`t`, `tb`) must not alter legacy relay parsing (`u`, `q`, `gz`).

## Routing (by host)

Evaluated in order:

1. **Domestic** — `.ir` or bundled list → plain TCP (no Apps Script, no MITM).
2. **Direct** — when enabled, Google domains → TLS ClientHello fragmentation.
3. **Relay** — everything else → Apps Script (desktop: MITM + HTTP relay; Android: TCP tunnel).

**Android:** VPN + TUN forwarder + local CONNECT proxy. End-to-end TLS through tunnel; no CA install.

**Desktop:** HTTP CONNECT / SOCKS. Foreign HTTPS on the relay path needs **CA installed** (MITM decrypt → Apps Script relay).

## Key concepts

**`relay/core`** re-exports the stable API. Implementation lives in subpackages:

- `relay/appscript/relay.go` — domain-fronted relay requests (`buildRelayPayload`, `RelayRequestMulti`).
- `relay/mitm/proxy.go` — HTTP/SOCKS proxy; HTTPS uses CONNECT + MITM when relaying.
- `relay/mitm/cert.go` — local CA and per-host leaf certs.
- `relay/route/direct.go`, `fragment.go` — Google detection and TLS fragmentation.

**`platforms/mobile`** exposes a flat string-based API because gomobile only supports primitives at the boundary. Errors return as strings, not Go `error` values.

## Running tests

```bash
go test ./relay/... ./platforms/desktop/...
```

Or everything:

```bash
go test ./...
```

## Building

```bash
make desktop          # local ./zyrln
make desktop-release  # dist/ binaries for Linux, Windows, macOS
make android          # signed release APK (requires keystore)
```

First-time gomobile:

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
export ANDROID_HOME=~/Android/Sdk
```

## Adding a probe

Probes live in `platforms/desktop/main.go` → `defaultProbes()`:

```go
{
    ID:          "unique-id",
    Name:        "Human readable name",
    Category:    "baseline",
    Method:      http.MethodGet,
    URL:         "https://example.com/",
    Expectation: "what a passing result means",
}
```

## Changing the relay protocol

Payload format: `relay/appscript/relay.go` (`BuildRelayPayload`) must match `relay/deploy/apps-script/Code.gs`.

Response shape (`workerResponse` in `relay.go`):

```go
type workerResponse struct {
    Status  int               `json:"s"`
    Headers map[string]string `json:"h"`
    Body    string            `json:"b"` // base64
    Error   string            `json:"e"`
}
```

## Secrets

Never commit `config.env`, `certs/`, or files containing `AUTH_KEY` / relay keys. Generate the client auth key in the desktop GUI: **Settings → Generate Key**.
