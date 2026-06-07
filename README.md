# Zyrln

[راهنمای فارسی](README_FA.md)

Bypass internet censorship in Iran. Routes your traffic through Google's infrastructure — no VPN fingerprint, no blocked IP, no dedicated server to block.

---

## Table of Contents

- [How It Works](#how-it-works)
- [I just want Google services (Gmail, Drive, Maps)](#i-just-want-google-services-gmail-drive-maps)
- [I want to access everything](#i-want-to-access-everything)
  - [What you need](#what-you-need)
  - [Step 1 — Run the desktop app](#step-1--run-the-desktop-app)
  - [Step 2 — Deploy the exit relay](#step-2--deploy-the-exit-relay)
  - [Step 3 — Deploy the Apps Script relay](#step-3--deploy-the-apps-script-relay)
  - [Step 4 — Connect](#step-4--connect)
  - [Step 5 — Set up Android](#step-5--set-up-android)
- [Building from Source](#building-from-source)
- [Troubleshooting](#troubleshooting)
- [Security Notes](#security-notes)
- [Credits](#credits)

---

## How It Works

Iran's censorship system (SNDPI) blocks sites by inspecting traffic. Zyrln defeats it two ways:

**For Google services (Gmail, Drive, Maps, etc.):**
Traffic is sent directly to Google but with the TLS handshake split into tiny fragments. The censor's system can't reassemble them fast enough to read the SNI, so it lets the connection through. No server needed.

**For everything else (Instagram, Twitter, etc.):**
Traffic is routed through Google Apps Script — a free Google service. From the censor's perspective it looks like normal Google traffic. Apps Script then forwards it to an exit relay (your VPS or Cloudflare) which fetches the real site.

---

## I just want Google services (Gmail, Drive, Maps)

**No server needed. No setup. Just download and enable.**

1. Download the app for your platform from the [Releases](../../releases) page
2. Run it — the GUI opens in your browser automatically
   - **Windows:** double-click the `.exe` — the GUI opens automatically
   - **Linux / macOS:** run from terminal with the `-gui` flag:
     ```bash
     # Linux
     ./zyrln-VERSION-linux-amd64 -gui
     # macOS Apple Silicon
     ./zyrln-VERSION-darwin-arm64 -gui
     # macOS Intel
     ./zyrln-VERSION-darwin-amd64 -gui
     ```
3. Click the **⚡ lightning bolt** button in the top bar to enable Direct Mode
4. Set your browser to use HTTP proxy `127.0.0.1:8085`

That's it. Many Google services can use the faster direct path when the local network allows it.

> Direct Mode works for Google services that are SNI-filtered but not IP-blocked — Gmail, Drive, Maps, Google Docs, and similar. YouTube video streaming and Play Store downloads go through the relay instead. Filtering behavior varies by ISP, city, carrier, and time.

---

## I want to access everything

To access Instagram, Twitter, Telegram, and other non-Google sites, you need to set up a relay chain. This takes about 15 minutes.

### What you need

| | What | Cost |
|---|---|---|
| ✅ Required | Google account | Free |
| ✅ Required | A shared auth key (you generate it) | Free |
| ☁️ Pick one | VPS with a public IP | ~$5/mo |
| ☁️ Or this | Cloudflare Worker | Free — [setup guide](docs/cloudflare-setup.md) |

### Step 1 — Run the desktop app

1. Download the binary for your OS from [Releases](../../releases)
2. Run it — the GUI opens automatically in your browser
   - **Windows:** double-click the `.exe` — the GUI opens automatically
   - **Linux / macOS:** run from terminal with the `-gui` flag:
     ```bash
     # Linux
     ./zyrln-VERSION-linux-amd64 -gui
     # macOS Apple Silicon
     ./zyrln-VERSION-darwin-arm64 -gui
     # macOS Intel
     ./zyrln-VERSION-darwin-amd64 -gui
     ```
3. Go to **Security** → generate and install the CA certificate (needed for HTTPS sites)
4. Go to **Settings** → click **Generate Key** and copy the auth key — you'll need it in the next steps

**Configure your browser:**

| Browser | Where to set it |
|---|---|
| Chrome / Edge | Settings → System → Open proxy settings → Manual proxy → `127.0.0.1:8085` |
| Firefox | Settings → Network → Manual proxy → HTTP `127.0.0.1` port `8085` |
| System-wide (all apps) | Use SOCKS5 `127.0.0.1:1080` in your OS network settings |

**Install the CA certificate** (required for HTTPS):

- **Chrome/Edge**: Settings → Privacy → Security → Manage certificates → Authorities → Import `zyrln-ca.pem`
- **Firefox**: Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import

### Step 2 — Deploy the exit relay

This is the exit node that fetches real websites. Pick one option:

#### Option A — Cloudflare Worker (recommended, free)

See [Cloudflare Worker setup](docs/cloudflare-setup.md) — deploy with Wrangler from `relay/deploy/cloudflare/`.

#### Option B — VPS

Linux VPS (amd64 or arm64), public IP, port **8787** open, SSH as `user@host` with `sudo`. On your laptop you only need `ssh`/`scp` (no Go).

1. Download **`zyrln-VERSION-vps.zip`** from [Releases](../../releases) and unzip it.
2. In the unzipped folder, run `./install-vps-relay.sh user@YOUR_VPS_IP` (e.g. `ubuntu@1.2.3.4`).  
   Shared secret: `ZYRLN_RELAY_KEY=secret` or `ZYRLN_RELAY_KEY=auto` — same value as `EXIT_RELAY_KEY` (key 2) in Apps Script.
3. In `Code.gs`: `EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay"` and `EXIT_RELAY_KEY` matching the VPS.

Check: `curl -s http://YOUR_VPS_IP:8787/healthz` should print `ok`.

### Step 3 — Deploy the Apps Script relay

This is the front door. It sits on Google's servers and receives your traffic.

#### Two keys

There are exactly **two keys**. The app only uses the first.

| | Key 1 — client | Key 2 — exit |
|---|----------------|--------------|
| **Path** | App → Apps Script | Apps Script → VPS or Cloudflare |
| **In the app** | Yes (`auth-key`) | No |
| **Apps Script** | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| **Exit (VPS or Cloudflare)** | — | `ZYRLN_RELAY_KEY` |
| **On the wire** | JSON `"k"` | HTTP header `X-Relay-Key` |

Key 2 uses the **same name** on VPS and Cloudflare Worker (`ZYRLN_RELAY_KEY`). Set the **same value** in `EXIT_RELAY_KEY` (Code.gs) and `ZYRLN_RELAY_KEY` (exit). If the exit has no key configured, leave `EXIT_RELAY_KEY` empty in Code.gs.

```
App ──key 1──► Apps Script ──key 2──► Cloudflare or VPS
```

1. Go to [script.google.com](https://script.google.com) → **New project**
2. Delete the default code and paste the contents of [`relay/deploy/apps-script/Code.gs`](relay/deploy/apps-script/Code.gs)
3. Edit the constants at the top — use the **Cloudflare** or **VPS** block, not both:

**Cloudflare Worker**:

```js
const AUTH_KEY        = "your-key-from-step-1";          // key 1 — goes in the app
const EXIT_RELAY_URL  = "https://your-worker.your-subdomain.workers.dev";  // no /relay
const EXIT_TUNNEL_URL = "";                             // Cloudflare: empty (auto /tunnel)
const EXIT_RELAY_KEY  = "your-exit-key";                 // key 2 — same as ZYRLN_RELAY_KEY in wrangler.toml
```

**VPS**:

```js
const AUTH_KEY        = "your-key-from-step-1";          // key 1 — goes in the app
const EXIT_RELAY_URL  = "http://YOUR_VPS_IP:8787/relay";
const EXIT_TUNNEL_URL = "http://YOUR_VPS_IP:8787/tunnel";
const EXIT_RELAY_KEY  = "your-exit-key";                 // key 2 — same as ZYRLN_RELAY_KEY on the VPS
```

See [Cloudflare Worker setup](docs/cloudflare-setup.md) for Wrangler deploy details.

4. Click **Deploy → New deployment**
   - Type: **Web app**
   - Execute as: **Me**
   - Who has access: **Anyone**
5. Click **Deploy** and copy the URL — it looks like:
   `https://script.google.com/macros/s/AKfycb.../exec`

> Each Google account gets 20,000 relay calls/day. Add multiple deployments (from different Google accounts) as a comma-separated list for resilience.

### Step 4 — Connect

1. In the app click **+** to add a new profile
2. Paste your Apps Script URL and auth key
3. Click **Save**, then click **Connect**

### Step 5 — Set up Android

Quick steps:
1. Install the APK from [Releases](../../releases)
2. In the desktop app: click the **export** button → copy the JSON
3. In the Android app: tap **Import Config from Clipboard**
4. Tap your config, then tap **Connect** and allow VPN permission

---

## Building from Source

Requires Go 1.25+.

```bash
# Desktop binary + GUI
make desktop

# Desktop release binaries for Linux, Windows, and macOS
make desktop-release

# Or build one platform
make desktop-linux
make desktop-windows
make desktop-macos

# Android APK (requires Android SDK + NDK)
make keystore       # run once — generates signing key
make android        # builds signed release APK

# Start the proxy from source
make proxy

# Run tests
make test

# VPS install zip (installer + linux amd64/arm64 binaries)
make vps-relay-bundle   # → dist/zyrln-VERSION-vps.zip
```

`make desktop` builds a local `./zyrln` binary for your current machine. `make desktop-release` writes platform-specific binaries into `dist/` using the release names shown above.

---

## Troubleshooting

**Nothing loads through the proxy**
- Check the proxy is running (green dot in the GUI)
- Confirm your browser proxy is set to `127.0.0.1:8085`
- Run the diagnostics tool (play button in the Tools section)

**HTTPS sites show SSL errors (desktop only)**
- The CA certificate is not installed or not trusted
- Desktop: re-import `certs/zyrln-ca.pem` in your browser

**Apps Script quota exceeded**
- Add more Apps Script deployments from different Google accounts
- Paste them comma-separated in the relay URL field

**YouTube works but Instagram doesn't**
- Instagram is IP-blocked, not just SNI-filtered — it needs the full relay chain
- Make sure your VPS/Cloudflare exit relay is running

---

## Security Notes

- Each user should deploy their own Apps Script and generate their own auth key
- Never commit `config.env`, `certs/`, or any file containing your auth key
- Google and your VPS/Cloudflare provider can see traffic metadata (timing, volume) but not content
- Rotate your auth key if it appears in logs or chat
- The local CA private key (`certs/zyrln-ca-key.pem`) must stay on your device

---

## Credits

Domain-fronting technique pioneered by [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw).

TLS fragmentation approach based on research by [GFW-knocker](https://github.com/GFW-knocker).

---

## License

MIT — see [LICENSE](LICENSE).
