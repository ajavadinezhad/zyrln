# Zyrln

[راهنمای فارسی](README_FA.md)

Bypass internet censorship in Iran. Routes your traffic through Google's infrastructure — no VPN fingerprint, no blocked IP, no dedicated server to block.

---

## Table of Contents

- [How It Works](#how-it-works)
- [I just want YouTube and Google](#i-just-want-youtube-and-google)
- [I want to access everything](#i-want-to-access-everything)
  - [What you need](#what-you-need)
  - [Step 1 — Generate an auth key](#step-1--generate-an-auth-key)
  - [Step 2 — Deploy the Apps Script relay](#step-2--deploy-the-apps-script-relay)
  - [Step 3 — Deploy the exit relay](#step-3--deploy-the-exit-relay)
  - [Step 4 — Run the desktop app](#step-4--run-the-desktop-app)
  - [Step 5 — Set up Android](#step-5--set-up-android)
- [Building from Source](#building-from-source)
- [Troubleshooting](#troubleshooting)
- [Security Notes](#security-notes)
- [Credits](#credits)

---

## How It Works

Iran's censorship system (SNDPI) blocks sites by inspecting traffic. Zyrln defeats it two ways:

**For Google services (YouTube, Gmail, Drive, etc.):**
Traffic is sent directly to Google but with the TLS handshake split into tiny fragments. The censor's system can't reassemble them fast enough to read the SNI, so it lets the connection through. No server needed.

**For everything else (Instagram, Twitter, etc.):**
Traffic is routed through Google Apps Script — a free Google service. From the censor's perspective it looks like normal Google traffic. Apps Script then forwards it to an exit relay (your VPS or Cloudflare) which fetches the real site.

```
YouTube/Google:   your device ──[fragmented TLS]──▶ Google (direct, full speed)

Everything else:  your device ──▶ Apps Script (Google) ──▶ your VPS ──▶ target site
                                   looks like Google traffic ↑
```

---

## I just want YouTube and Google

**No server needed. No setup. Just download and enable.**

1. Download the app for your platform from the [Releases](../../releases) page
2. Run it — the GUI opens in your browser automatically
3. Click the **⚡ lightning bolt** button in the top bar to enable Direct Mode
4. Set your browser to use HTTP proxy `127.0.0.1:8085`

That's it. YouTube, Gmail, Google Drive, Google Meet, and all Google services will work at full speed.

> Direct Mode works because Iran only blocks Google by SNI — they can't block the IPs without breaking half the Iranian internet. Zyrln fragments the TLS handshake so the SNI is never visible to the censor.

---

## I want to access everything

To access Instagram, Twitter, Telegram, and other non-Google sites, you need to set up a relay chain. This takes about 15 minutes.

### What you need

| | What | Cost |
|---|---|---|
| ✅ Required | Google account | Free |
| ✅ Required | A shared auth key (you generate it) | Free |
| ☁️ Pick one | VPS with a public IP | ~$5/mo |
| ☁️ Or this | Cloudflare account | Free tier is enough |

### Step 1 — Generate an auth key

Run this once on any computer. Save the output — you'll use it in every step.

```bash
zyrln -gen-key
```

Example output: `swrkwbMS1X666fjzReip+PbodKcPyDK7Xbk5gRSgRUE=`

### Step 2 — Deploy the Apps Script relay

This is the front door. It sits on Google's servers and receives your traffic.

1. Go to [script.google.com](https://script.google.com) → **New project**
2. Delete the default code and paste the contents of [`relay/apps-script/Code.gs`](relay/apps-script/Code.gs)
3. Edit the three lines at the top:

```js
const AUTH_KEY       = "your-key-from-step-1";
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";  // or your Cloudflare Worker URL
const EXIT_RELAY_KEY = "";
```

4. Click **Deploy → New deployment**
   - Type: **Web app**
   - Execute as: **Me**
   - Who has access: **Anyone**
5. Click **Deploy** and copy the URL — it looks like:
   `https://script.google.com/macros/s/AKfycb.../exec`

> Each Google account gets 20,000 relay calls/day. Add multiple deployments (from different Google accounts) as a comma-separated list for resilience.

### Step 3 — Deploy the exit relay

This is the exit node. It fetches real websites on behalf of Apps Script. Pick one option:

#### Option A — Cloudflare Worker (recommended, free)

1. Go to [dash.cloudflare.com](https://dash.cloudflare.com) → **Workers & Pages → Create**
2. Paste the contents of [`relay/cloudflare/worker.js`](relay/cloudflare/worker.js)
3. Click **Deploy** and copy the Worker URL:
   `https://your-worker.your-subdomain.workers.dev`
4. Go back to your Apps Script and update `EXIT_RELAY_URL`:
   ```js
   const EXIT_RELAY_URL = "https://your-worker.your-subdomain.workers.dev/relay";
   ```
5. Redeploy the Apps Script (Deploy → Manage deployments → New version)

#### Option B — VPS

See **[docs/vps-setup.md](docs/vps-setup.md)** for full instructions.

Short version — on your VPS:
```bash
# Build locally and copy to server
GOOS=linux GOARCH=amd64 go build -o zyrln-relay ./relay/vps/main.go
scp zyrln-relay root@YOUR_VPS:/usr/local/bin/

# On the server — create /etc/zyrln-relay.env:
ZYRLN_RELAY_LISTEN=0.0.0.0:8787
ZYRLN_RELAY_KEY=

# Then enable and start as a systemd service (see docs/vps-setup.md)
ufw allow 8787/tcp
```

### Step 4 — Run the desktop app

1. Download the binary for your OS from [Releases](../../releases), or build from source (see [Building from Source](#building-from-source))
2. Run it — the GUI opens automatically
3. Click **+** to add a new profile
4. Paste your Apps Script URL and auth key
5. Click **Save**, then click **Connect**
6. Go to **Security** → generate and install the CA certificate (needed for HTTPS sites)

**Configure your browser:**

| Browser | Where to set it |
|---|---|
| Chrome / Edge | Settings → System → Open proxy settings → Manual proxy → `127.0.0.1:8085` |
| Firefox | Settings → Network → Manual proxy → HTTP `127.0.0.1` port `8085` |
| System-wide (all apps) | Use SOCKS5 `127.0.0.1:1080` in your OS network settings |

**Install the CA certificate** (required for HTTPS):

- **Chrome/Edge**: Settings → Privacy → Security → Manage certificates → Authorities → Import `zyrln-ca.pem`
- **Firefox**: Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import

### Step 5 — Set up Android

See **[docs/android-setup.md](docs/android-setup.md)** for the full guide.

Quick steps:
1. Install the APK from [Releases](../../releases)
2. In the desktop app: click the **export** button → copy the JSON
3. In the Android app: tap **Import Config from Clipboard**
4. Tap **Install CA Certificate** and follow the prompts
5. Tap your config to connect

> ⚠️ Never copy the CA certificate file from your computer to your phone. Each device generates its own. Always use **Install CA Certificate** inside the Android app.

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
```

---

## Troubleshooting

**Nothing loads through the proxy**
- Check the proxy is running (green dot in the GUI)
- Confirm your browser proxy is set to `127.0.0.1:8085`
- Run the diagnostics tool (play button in the Tools section)

**HTTPS sites show SSL errors**
- The CA certificate is not installed or not trusted
- Desktop: re-import `certs/zyrln-ca.pem` in your browser
- Android: use **Install CA Certificate** in the app, not a manual file copy

**Apps Script quota exceeded**
- Add more Apps Script deployments from different Google accounts
- Paste them comma-separated in the relay URL field

**YouTube works but Instagram doesn't**
- Instagram is IP-blocked, not just SNI-filtered — it needs the full relay chain
- Make sure your VPS/Cloudflare exit relay is running

**Android: some apps don't work**
- Apps that hardcode their own certificates (banking apps, some social apps) ignore the system proxy and can't be intercepted without root

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
