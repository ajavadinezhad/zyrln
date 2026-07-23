# Zyrln

A domain-fronting relay that routes your traffic through Google infrastructure to bypass DPI-based (deep packet inspection) censorship.

## The Big Picture (Read This First)

Zyrln works by hiding your traffic *inside what looks like a normal connection to Google*. There are three pieces, and each one gets **its own secret key**. Getting these three pieces and two keys straight is the whole trick to setting this up — everything else is just following steps.

```
Your Device → Zyrln Client → Google Apps Script → your exit relay → Internet
```

**Why it works:** your device's TLS connections go to Google's own IP ranges. Only the encrypted `Host` header, invisible to DPI, actually points at your Apps Script deployment. From a censor's point of view, this traffic is indistinguishable from someone using Google Docs.

**Setup order:** set up the client first (so you have both keys in hand), then the exit relay (which needs one of those keys), then Apps Script last (which needs the URL the exit relay gives you, plus both keys). Doing it in this order means you never have to backtrack and re-paste a value you didn't have yet.

### The two pieces you deploy

| Piece | What it does | Where it lives |
|---|---|---|
| **Exit relay** | The "back door." Receives forwarded requests and actually fetches the target website. | Your own VPS, **or** a free Cloudflare Worker |
| **Apps Script relay** | The "front door." Receives your traffic disguised as Google traffic, forwards it to your exit relay. | Google's servers (free, via script.google.com) |

### The two keys you generate

This is the part the original instructions blur together. There are **two separate, independent secrets**:

| Key | Purpose | Used by |
|---|---|---|
| **`AUTH_KEY`** (client key) | Proves *your device* is allowed to use *your* Apps Script relay. Stops randoms from finding your Apps Script URL and using it as an open proxy. | Set once in Apps Script; set once in your client config |
| **`EXIT_RELAY_KEY`** (relay key) | Proves *your Apps Script* is allowed to use *your* VPS exit relay. **Only needed if you're running your own VPS** — skip it if you use Cloudflare. | Set once in Apps Script; set once in your VPS relay's config |

Visually They are used like this:
```
Your Device → Zyrln Client --[AUTH_KEY]-→ Google Apps Script --[EXIT_RELAY_KEY]-→ your exit relay → Internet
```
Both are just random strings you generate yourself — nobody hands them to you, and you never share them.

---

## Prerequisites

- A Google account (for Apps Script — free)
- **Either** a VPS (a few dollars/month) **or** a free Cloudflare account for the exit relay
- **No compiling needed.** Precompiled binaries for the client and the VPS exit relay are on the [Releases page](https://github.com/ajavadinezhad/zyrln/releases) for Windows, macOS, and Linux — just download the one matching your OS. You only need the Go toolchain if you want to build from source instead.

---

## Step 1 — Set Up the Client and Generate Your Keys

1. Download the client binary for your OS from the [Releases page](https://github.com/ajavadinezhad/zyrln/releases) and place it somewhere convenient.
2. Generate your **client key** (`AUTH_KEY`). You can use either the client itself or `openssl` — both produce an equally valid random key, pick whichever's handy:

   Using the client's built-in generator:
   ```
   ./zyrln -gen-key
   # example (Windows): .\zyrln-2.0-pre-5-windows-amd64.exe -gen-key
   # prints something like: OBGdqrZVgSd4GEvTzyeHn2Jf2kERBEcXkyWQq/DoPug=
   ```
   Or using OpenSSL:
   ```bash
   openssl rand -base64 32
   ```
   
   If you are on Android, you can also use [generate-random.org](https://generate-random.org/base64-string) website. It generates keys via javascript engine on your browser. choose `--byte-count` to `32`.
   
   Copy the output somewhere safe — this is your `AUTH_KEY`.

3. If you're going the **VPS route** for your exit relay (skip this if you'll use Cloudflare), generate a **second, different** key the same way — this will be your `EXIT_RELAY_KEY`.

You now have everything you need to configure the other two pieces. Don't fill in `config.env` yet — you're missing the Apps Script URL until Step 3. Just keep both keys handy.

---

## Step 2 — Deploy the Exit Relay (the "back door")

Pick **one** of these two options. Cloudflare is simpler and free; a VPS gives you more control over who can see your traffic metadata.

### Option A: Cloudflare Worker (simpler, free)

1. Go to [dash.cloudflare.com](https://dash.cloudflare.com) → **Workers & Pages** → **Create application** → **Worker**
2. Replace the default code with the contents of `relay/cloudflare/worker.js` from this repo
3. Click **Deploy**
4. Copy the Worker URL, e.g. `https://your-worker.your-subdomain.workers.dev` — you'll need this in Step 3.

**Trade-offs:** free tier gives you 100,000 requests/day, which is plenty for personal use, but Cloudflare itself can see your traffic's destination metadata. If that matters to you, use the VPS option instead. Cloudflare Workers don't use `EXIT_RELAY_KEY` at all — leave that blank later.

### Option B: Your own VPS (more control)

1. Download the precompiled exit-relay binary for Linux from the [Releases page](https://github.com/ajavadinezhad/zyrln/releases) — no need to build anything.
2. Copy it to your server:
   ```bash
   scp zyrln-relay-linux-amd64 root@YOUR_VPS:/usr/local/bin/zyrln-relay
   chmod +x /usr/local/bin/zyrln-relay
   ```
3. On the server, create `/etc/systemd/system/zyrln-relay.service`:
   ```ini
   [Unit]
   Description=Zyrln Exit Relay
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=simple
   EnvironmentFile=/etc/zyrln-relay.env
   ExecStart=/usr/local/bin/zyrln-relay
   Restart=always
   RestartSec=3

   [Install]
   WantedBy=multi-user.target
   ```
4. Create `/etc/zyrln-relay.env`:
   ```
   ZYRLN_RELAY_LISTEN=0.0.0.0:8787
   ZYRLN_RELAY_KEY=your-relay-key-from-step-1
   ```
   This `ZYRLN_RELAY_KEY` **must exactly match** the `EXIT_RELAY_KEY` you'll paste into Apps Script in Step 3. If they don't match, every request from Apps Script to your VPS fails with a `401`.
5. Enable and start the service:
   ```bash
   systemctl daemon-reload
   systemctl enable --now zyrln-relay
   ```
6. Open the firewall port (skip if your provider manages firewall rules via a dashboard):
   ```bash
   ufw allow 8787/tcp
   ```
7. Test it directly, before involving Apps Script at all — both checks should succeed:
   ```bash
   # basic health check
   curl -s http://YOUR_VPS_IP:8787/healthz

   # full relay round-trip
   curl -X POST http://YOUR_VPS_IP:8787/relay \
     -H "Content-Type: application/json" \
     -H "X-Relay-Key: your-relay-key-from-step-1" \
     -d '{"u":"https://www.gstatic.com/generate_204","m":"GET","h":{},"r":true}'
   # expect: {"s":204,...}
   ```
8. Note your VPS's URL (`http://YOUR_VPS_IP:8787/relay`) — you'll need it in Step 3.

---

## Step 3 — Deploy the Apps Script Relay (the "front door")

This is the piece that makes your traffic look like Google traffic. By now you have both keys and your exit relay's URL, so this is the last piece of the puzzle.

1. Go to [script.google.com](https://script.google.com) → **New project**
2. Delete the placeholder code and paste in the contents of `relay/apps-script/Code.gs` from this repo
3. Near the top of the file, fill in these three constants:

```js
const AUTH_KEY = ""; // your AUTH_KEY from step 1.
const EXIT_RELAY_URL = ""; // For Cloudflare Exit Relay: worker URL, without /relay. For VPS Exit Relay: http://YOUR_VPS_IP:8787/relay
const EXIT_TUNNEL_URL = ""; // For Cloudflare  Exit Relay: leave empty (it derives /tunnel automatically). For VPS  Exit Relay: http://YOUR_VPS_IP:8787/tunnel
const EXIT_RELAY_KEY = ""; // your EXIT_RELAY_KEY from Step 1
```

4. Click **Deploy → New deployment**
   - Type: **Web app**
   - Execute as: **Me**
   - Who has access: **Anyone**
5. Copy the deployment URL it gives you — it looks like:
   ```
   https://script.google.com/macros/s/AKfycb.../exec
   ```
   You'll need this URL to finish configuring your client next.

**Whenever you change the constants in this file later, you must redeploy:** Deploy → Manage deployments → pencil icon → New version → Deploy.

---

## Step 4a - Finish Configuring Windows/Linux Client

1. In the project root, create a file called `config.env` (already gitignored — never commit this):
   ```
   fronted-appscript-url = https://script.google.com/macros/s/YOUR_ID/exec
   auth-key              = YOUR_AUTH_KEY_FROM_STEP_1
   listen                = 127.0.0.1:8085
   ```
2. Generate a local certificate authority (one-time):
   ```bash
   ./zyrln -init-ca
   ```
3. Trust that certificate in your browser, so it can decrypt-and-relay HTTPS sites transparently:
   - **Chrome/Edge:** Settings → Privacy → Security → Manage certificates → Authorities → Import → select `certs/zyrln-ca.pem`
   - **Firefox:** Settings → Privacy & Security → View Certificates → Authorities → Import → select `certs/zyrln-ca.pem`
4. Start the proxy:
   ```bash
   ./zyrln
   ```
5. Point your browser's HTTP and HTTPS proxy settings at `127.0.0.1:8085`.

### Test it

```bash
./zyrln -test
```

You should see `relay fetch ok` and `status: 204`. If you don't, check in this order:
1. Is the Apps Script deployment live and set to "Anyone" access?
2. Does `EXIT_RELAY_KEY` in Apps Script exactly match your VPS's `ZYRLN_RELAY_KEY` (VPS route only)?
3. Is the VPS relay service running (`systemctl status zyrln-relay`) and the firewall port open — recheck with `curl -s http://YOUR_VPS_IP:8787/healthz`?
4. Did you redeploy Apps Script after your last constant change?

If that all checks out, open a browser with the proxy configured and visit a normally-blocked site — it should load like any other page.

---

## Step 4b - Finish Configuring Android Client

The Android app runs the whole relay chain on your phone directly in Tunnel Mode — no certificate installation is needed. Unlike the windows or linux client, the app has **no fields to manually type in a deployment URL or key** — everything is imported as a single JSON blob.

### Install

Download the APK from the [Releases page](https://github.com/ajavadinezhad/zyrln/releases), copy it to your phone, and open it.

### First run

1. **Build your config JSON.** The app needs a single JSON object with your client key and one or more Apps Script deployment URLs (comma-separated in one string, no spaces, if you have more than one — handy if you've deployed several Apps Script copies as fallbacks):
   ```json
   {
     "key": "YOUR_AUTH_KEY_FROM_STEP_1",
     "url": "https://script.google.com/macros/s/DEPLOYMENT_ID_1/exec,https://script.google.com/macros/s/DEPLOYMENT_ID_2/exec"
   }
   ```
   A single-deployment config just has one URL in the `url` field. If you already have set up  desktop client, you can also export this via GUI instead of typing it by hand:

3. **Import it** — copy that JSON to your phone's clipboard, then in the app tap **Import Config from Clipboard**. It's saved to your list of configs automatically (duplicates are skipped), and you can import several and switch between them.

4. **Connect** — tap a config in the list, allow the VPN permission when prompted. Green dot = connected. Tap again to disconnect.

5. **Test** — open Chrome and visit a normally-blocked site. If you get SSL errors, the CA certificate isn't trusted yet — repeat step 1.

---

## Security Notes

- Generate your own keys — don't reuse an example or someone else's
- On windows/Linux Clients, The local CA private key (`certs/zyrln-ca-key.pem`) must never be shared — anyone with it could intercept your HTTPS traffic
- Google and your VPS/Cloudflare provider can still see connection metadata (timing, volume) even though they can't read the actual content

## Credits

The domain-fronting technique this project builds on — routing traffic through Google Apps Script with a Cloudflare Worker as the exit relay — originates from [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw). This project extends that idea with a self-hosted VPS exit relay option, a full Go rewrite, an Android VPN app, and HTTPS MITM proxy support.
