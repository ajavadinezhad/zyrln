# Zyrln

[راهنمای فارسی](docs/fa/guide.md)

Censorship circumvention that routes traffic through Google's infrastructure — no typical VPN fingerprint, no dedicated exit IP that is easy to block on its own.

Downloads: [GitHub Releases](https://github.com/ajavadinezhad/zyrln/releases)

Also: [Cloudflare exit setup](docs/cloudflare-setup.md) · [Contributing](docs/contributing.md)

---

## How it works

Many networks block sites by inspecting TLS (DPI / SNI). Zyrln uses two paths:

**Google services (Gmail, Drive, Maps, etc.):**
Traffic goes straight to Google, but the TLS ClientHello is split into small fragments so middleboxes that rely on a clean SNI often fail to classify the connection. No server needed.

**Everything else (Instagram, X, Telegram, …):**

```
Your device → Zyrln → Google Apps Script → your exit relay → the real site
```

Your device's TLS connections go to Google's own IP ranges; only the encrypted `Host` header — invisible to DPI — actually points at your Apps Script deployment. From a censor's point of view, this traffic is indistinguishable from someone using Google Docs. Apps Script then forwards to your exit relay (VPS or Cloudflare), which opens the real destination.

Client relay/tunnel traffic **must** go through Apps Script (never point the app at the VPS/Worker URL). The exit is reached only from Google's side.

---

## Google only (Gmail, Drive, Maps)

**No server. No relay setup.**

1. Download for your platform from [Releases](https://github.com/ajavadinezhad/zyrln/releases)
2. Run the app — the GUI opens in your browser
   - **Windows:** double-click the `.exe`
   - **Linux / macOS:** run with `-gui`:
     ```bash
     ./zyrln-VERSION-linux-amd64 -gui          # Linux
     ./zyrln-VERSION-darwin-arm64 -gui         # macOS Apple Silicon
     ./zyrln-VERSION-darwin-amd64 -gui         # macOS Intel
     ```
3. Click the **lightning bolt** in the top bar to enable Direct Mode
4. Set your browser HTTP proxy to `127.0.0.1:8085`

Direct Mode works for Google services that are SNI-filtered but not IP-blocked. YouTube streaming and Play Store downloads typically need the full relay. Behavior varies by ISP and location.

---

## Full access (Instagram, Twitter, Telegram, …)

Relay chain setup takes about 15 minutes.

Follow the steps in order: the client generates your keys first, the exit relay needs one of them, and Apps Script needs the exit relay's URL plus both keys. Doing it in this order means you never have to backtrack and re-paste a value you didn't have yet.

### What you need

| | What | Cost |
|---|---|---|
| Required | Google account | Free |
| Required | Auth key (you generate it) | Free |
| Pick one | VPS with public IP | ~$5/mo |
| Or | Cloudflare Worker | Free — [setup guide](docs/cloudflare-setup.md) |

### Step 1 — Desktop app

1. Download from [Releases](https://github.com/ajavadinezhad/zyrln/releases)
2. Run — GUI opens in the browser (Windows: `.exe`; Linux/macOS: `-gui` as above)
3. **Security** → generate and install the CA certificate (**desktop only** — needed for HTTPS relay via MITM)
4. **Settings** → **Generate Key** → copy the auth key for Apps Script

**Browser proxy:**

| Browser | Setting |
|---|---|
| Chrome / Edge | Manual proxy → `127.0.0.1:8085` |
| Firefox | Manual HTTP proxy → `127.0.0.1` port `8085` |
| System-wide | SOCKS5 → `127.0.0.1:1080` |

**CA certificate (desktop relay only):**

- Chrome/Edge: Settings → Privacy → Security → Manage certificates → Authorities → Import `zyrln-ca.pem`
- Firefox: Settings → Privacy & Security → Certificates → Authorities → Import

### Step 2 — Exit relay

The exit fetches real websites. Pick one — Cloudflare is simpler and free; a VPS gives you more control over who can see your traffic's destination metadata.

#### Option A — Cloudflare Worker (free)

See [Cloudflare Worker setup](docs/cloudflare-setup.md) — deploy from `relay/deploy/cloudflare/`.

#### Option B — VPS

Linux VPS (amd64 or arm64), public IP, port **8787** open, SSH as `user@host` with `sudo`.

1. Download `zyrln-VERSION-vps.zip` from [Releases](https://github.com/ajavadinezhad/zyrln/releases) and unzip
2. Run `./install-vps-relay.sh user@YOUR_VPS_IP`  
   Set `ZYRLN_RELAY_KEY=secret` or `ZYRLN_RELAY_KEY=auto` — same value as `EXIT_RELAY_KEY` in Apps Script
3. In `Code.gs`: `EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay"` and matching `EXIT_RELAY_KEY`

Check: `curl -s http://YOUR_VPS_IP:8787/healthz` → `ok`

### Step 3 — Apps Script relay

Front door on Google's servers.

#### Two keys

Two separate, independent secrets — confusing them is the most common setup mistake. The app only uses key 1.

| | Key 1 — client | Key 2 — exit |
|---|---|---|
| Purpose | Proves *your device* may use *your* Apps Script relay | Proves *your Apps Script* may use *your* exit relay |
| Path | App → Apps Script | Apps Script → VPS or Cloudflare |
| In the app | Yes (`auth-key`) | No |
| Apps Script | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| Exit | — | `ZYRLN_RELAY_KEY` |
| On the wire | JSON `"k"` | Header `X-Relay-Key` |

Key 2 uses the same name on VPS and Cloudflare (`ZYRLN_RELAY_KEY`). Same value in `EXIT_RELAY_KEY` (Code.gs) and `ZYRLN_RELAY_KEY` (exit). If the exit has no key, leave `EXIT_RELAY_KEY` empty in Code.gs.

```
App ──key 1──► Apps Script ──key 2──► Cloudflare or VPS
```

1. [script.google.com](https://script.google.com) → **New project**
2. Paste [`relay/deploy/apps-script/Code.gs`](relay/deploy/apps-script/Code.gs)
3. Edit constants — **Cloudflare** or **VPS**, not both:

**Cloudflare:**

```js
const AUTH_KEY        = "your-key-from-step-1";
const EXIT_RELAY_URL  = "https://your-worker.your-subdomain.workers.dev";  // no /relay
const EXIT_TUNNEL_URL = "";
const EXIT_RELAY_KEY  = "your-exit-key";
```

**VPS:**

```js
const AUTH_KEY        = "your-key-from-step-1";
const EXIT_RELAY_URL  = "http://YOUR_VPS_IP:8787/relay";
const EXIT_TUNNEL_URL = "http://YOUR_VPS_IP:8787/tunnel";
const EXIT_RELAY_KEY  = "your-exit-key";
```

4. **Deploy → New deployment** → Web app → Execute as **Me** → Access **Anyone**
5. Copy the URL: `https://script.google.com/macros/s/AKfycb.../exec`

Each Google account gets ~20,000 relay calls/day. Add multiple deployments (different accounts) comma-separated in the app for resilience.

### Step 4 — Connect (desktop)

1. Click **+** to add a profile
2. Paste Apps Script URL and auth key
3. **Save** → **Connect**

### Step 5 — Android

1. Install APK from [Releases](https://github.com/ajavadinezhad/zyrln/releases)
2. Add config:
   - **Import Config from Clipboard** (JSON from desktop **Tools → Mobile Export**, or from another phone's share icon), or
   - **+** and enter URL + key manually
3. Tap the config → **Connect** → allow VPN permission
4. Optional: **Split tunnel** — route only selected apps through Zyrln (disconnect VPN to change settings)
5. Share a config: tap the **share icon** on a config row — JSON copies to clipboard

---

## Building from source

Requires Go 1.25+.

```bash
make desktop              # local ./zyrln
make desktop-release      # dist/ for Linux, Windows, macOS
make desktop-linux|windows|macos
make keystore             # once — Android signing key
make android              # signed release APK
make proxy                # run proxy from source (needs CA)
make test
make vps-relay-bundle     # → dist/zyrln-VERSION-vps.zip
```

See [docs/contributing.md](docs/contributing.md) for project layout and developer notes.

---

## Troubleshooting

**Relay chain not connecting (full access setup)**

Check in order:

1. Apps Script deployment live, with access set to "Anyone"?
2. `EXIT_RELAY_KEY` in Apps Script exactly matches `ZYRLN_RELAY_KEY` on your VPS? (Cloudflare doesn't use this key.)
3. VPS relay running and firewall port open? `curl -s http://YOUR_VPS_IP:8787/healthz` → `ok`
4. Redeployed Apps Script after your last constant change? (Deploy → Manage deployments → pencil icon → New version → Deploy)

**Nothing loads through the proxy**

- Proxy running? (green indicator in GUI)
- Browser proxy set to `127.0.0.1:8085`
- Run diagnostics (play button in Tools)

**HTTPS SSL errors (desktop only)**

- CA not installed or not trusted — re-import `certs/zyrln-ca.pem`

**Apps Script quota exceeded**

- Add deployments from other Google accounts; comma-separate URLs in the app

**YouTube works but Instagram doesn't**

- Instagram is IP-blocked — needs full relay chain
- Confirm VPS/Cloudflare exit is running

**X / Discord / ChatGPT fail on Cloudflare Worker exit**

- Worker can't reach other Cloudflare-hosted sites — use a VPS exit. See [docs/cloudflare-setup.md](docs/cloudflare-setup.md).

---

## Security

- Deploy your own Apps Script and generate your own auth key
- Never commit `config.env`, `certs/`, or keys
- Google and your exit provider see metadata (timing, volume), not encrypted payload content on the tunnel path
- Rotate keys if leaked
- Keep `certs/zyrln-ca-key.pem` on your device only (desktop)

---

## Credits

Domain-fronting: [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw). TLS fragmentation: [GFW-knocker](https://github.com/GFW-knocker).

License: MIT — [LICENSE](LICENSE).
