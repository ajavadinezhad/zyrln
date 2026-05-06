# Zyrln

A domain-fronting relay that routes traffic through Google infrastructure to bypass DPI-based censorship.

## How It Works

```
your device
  → local proxy (Go)
  → Google-fronted Apps Script   ← looks like Google traffic to DPI
  → VPS exit relay
  → target site
```

TLS connections go to Google's IP ranges. The encrypted `Host` header targets your Apps Script deployment. From a DPI perspective the traffic is indistinguishable from normal Google traffic.

## Features

- **Undetectable by DPI** — all traffic exits from Google's IP ranges and is indistinguishable from normal Google traffic. There is no VPN fingerprint, no unusual port, and no dedicated server IP to block.
- **Request coalescing** — concurrent browser requests are batched into a single Apps Script call. A page load that fires 30 requests uses 1–3 Apps Script executions instead of 30, dramatically extending daily quota.
- **In-proxy response cache** — static assets (JS, CSS, fonts, images) are served from memory on repeat visits. Cached responses skip the relay entirely, making subsequent page loads significantly faster.
- **Multi-URL quota failover** — configure multiple Apps Script deployments across different Google accounts. The relay sticks to the first URL until quota runs out, then switches transparently with no reconnection or dropped requests.
- **Full HTTPS support** — the proxy performs local TLS termination so blocked HTTPS sites work transparently. No plaintext data leaves the device.
- **Android VPN — no root** — one tap routes all browser traffic through the relay at the system level. No per-app configuration, no ADB, no root required.
- **Multiple saved configs** — save as many relay configs as you want on Android and switch between them with a single tap. Useful for managing multiple Apps Script deployments or sharing configs between users.
- **Reachability probe tool** — the desktop CLI can test which endpoints are reachable from your network before setting anything up. Covers baseline connectivity, Google APIs, domain-fronting, and the full relay chain so you know exactly what works and what doesn't.

## Before You Start

**The pre-built binaries and APK do not work out of the box.** Zyrln is a client — it has no built-in relay. You must set up your own relay chain before anything works:

| What you need | Why |
|---|---|
| Google account | To deploy the Apps Script relay (free) |
| VPS with a public IP | To run the exit relay, or use a Cloudflare Worker instead |
| Auth key | A shared secret that ties all components together |

Generate your auth key once and keep it — you will use it in every component:

```bash
openssl rand -base64 32
# example: swrkwbMS1X666fjzReip+PbodKcPyDK7Xbk5gRSgRUE=
```

---

## Setup

### 1. Deploy the Apps Script relay

This is the front door — it receives your traffic and forwards it to your VPS.

1. Open [script.google.com](https://script.google.com) → New project
2. Paste the contents of `relay/apps-script/Code.gs`
3. Set the constants at the top:

```js
const AUTH_KEY       = "YOUR_KEY";           // from the openssl command above
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";
const EXIT_RELAY_KEY = "";                   // optional, leave empty for now
```

4. Click **Deploy → New deployment → Web app**
   - Execute as: **Me**
   - Who has access: **Anyone**
5. Copy the deployment URL — it looks like:
   `https://script.google.com/macros/s/AKfycb.../exec`

> **Quota:** each Google account gets 20,000 relay calls/day. You can add multiple Apps Script deployments (under different Google accounts) as a comma-separated list in your config. The relay automatically switches to the next URL when one runs out.

### 2. Deploy the exit relay (VPS or Cloudflare)

This is the exit node — it fetches the real target URLs on behalf of Apps Script. Pick one:

**Option A — VPS** (any Linux server with a public IP):
See **[docs/vps-setup.md](docs/vps-setup.md)** for build, deploy, systemd service, and firewall steps.

**Option B — Cloudflare Worker** (no VPS needed, free tier is enough):
Deploy `relay/cloudflare/worker.js` as a Worker. See **[docs/cloudflare-setup.md](docs/cloudflare-setup.md)**.

### 3. Set up the desktop proxy

**Prerequisites:** Go 1.25+, `make`. On Windows, use **Git Bash** — cmd.exe and PowerShell are not supported.

Create `config.env` (gitignored):

```
fronted-appscript-url = https://script.google.com/macros/s/YOUR_ID/exec
auth-key              = YOUR_KEY
listen                = 127.0.0.1:8085
```

Multiple Apps Script URLs (comma-separated, no spaces):

```
fronted-appscript-url = https://script.google.com/.../exec1,https://script.google.com/.../exec2
```

Build and generate the local CA once:

```bash
make desktop && ./zyrln -init-ca
```

Install `certs/zyrln-ca.pem` as a trusted CA in your browser:
- **Chrome/Edge**: Settings → Privacy → Security → Manage certificates → Authorities → Import
- **Firefox**: Settings → Privacy & Security → View Certificates → Authorities → Import

Start the proxy:

```bash
# Option 1: CLI mode
make proxy

# Option 2: Browser-based GUI (recommended)
make gui
# Opens at http://127.0.0.1:8086 with tabs for Control, Config, and Tools
```

Set your browser's HTTP and HTTPS proxy to `127.0.0.1:8085`.

### 4. Verify the full chain

```bash
make test
```

You should see `relay fetch ok` and `status: 204`. If not, check that your Apps Script deployment and VPS are running and the auth key matches in all three places.

---

## Android App

See **[docs/android-setup.md](docs/android-setup.md)** for the full build and install guide.

> The app requires steps 1–2 above to be completed first. The pre-built APK from the release page needs your own Apps Script URL and auth key — it has no built-in relay.

**Using the pre-built APK from the release:**
1. Install the APK on your phone
2. On desktop, run `./zyrln -export-config` → copy the JSON output
3. In the app: tap **Import Config from Clipboard** → tap the config to connect

**Building the APK yourself** (requires Android SDK + NDK):
1. `make keystore && make android`
2. Install the APK from `android/app/build/outputs/apk/release/`
3. Same steps 2–3 as above

---

## Build Reference

```bash
make desktop        # build desktop CLI binary
make proxy          # start desktop proxy (reads config.env)
make gui            # start browser-based GUI at http://127.0.0.1:8086
make test           # smoke test the full relay chain
make android        # build signed release APK (requires keystore + Android SDK)
make android-debug  # build debug APK (no keystore needed)
```

### GUI Mode

The GUI provides a browser-based interface with a modern tab layout:

- **Control tab**: Big circle button (VPN-style) to start/stop proxy, config validation before start, **live proxy logs**
- **Config tab**: Edit `config.env` (all flags as form fields)
- **Tools tab**: Generate CA, test relay, run probes, export config for Android

Features:
- Auto-opens browser when GUI starts (supports Linux/macOS/Windows)
- Config validation check before proxy start
- Real-time status updates and proxy log viewer

```bash
# Start GUI (opens at http://127.0.0.1:8086)
make gui
# Or with custom port:
go run ./platforms/desktop/ -gui -gui-listen=127.0.0.1:9090
```

---

## Components

| Component | Path | Role |
|---|---|---|
| Desktop proxy | `platforms/desktop/` | Local HTTPS MITM proxy + reachability probes |
| Relay core | `relay/core/` | Shared Go relay logic (desktop + Android) |
| Mobile bindings | `platforms/mobile/` | gomobile API for Android |
| Apps Script relay | `relay/apps-script/Code.gs` | Google-side relay (the front door) |
| VPS relay | `relay/vps/main.go` | Exit relay running on your server |
| Cloudflare Worker | `relay/cloudflare/worker.js` | Optional alternative exit relay |
| Android app | `android/` | Android VPN app |

---

## Limitations

- **Browser-based only**: this is an HTTP proxy, not a full VPN. Only browser traffic and apps that respect the system proxy are relayed. Apps like Instagram, WhatsApp, and Telegram bypass it entirely.
- **Apps Script quota**: each Google account gets 20,000 relay requests/day. Heavy sites can exhaust this quickly. Each user should deploy their own Apps Script.
- **Large downloads**: responses over ~12MB per request will be truncated (Apps Script response limit).

---

## Security Notes

- Each user should deploy their own Apps Script and generate their own auth key
- Never commit `config.env`, `certs/`, or any auth keys
- Rotate your auth key if it appeared in logs or chat
- The local CA private key (`certs/zyrln-ca-key.pem`) must not be shared
- Google and your VPS provider can see metadata (timing, volume) even though they cannot read content

---

## Credits

The domain-fronting technique used here was pioneered by [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw).

Developed with the assistance of [Claude](https://claude.ai) by Anthropic.

## License

MIT — see [LICENSE](LICENSE).
