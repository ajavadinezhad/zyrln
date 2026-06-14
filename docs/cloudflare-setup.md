# Cloudflare Worker Setup

Free exit alternative to a VPS. One Worker handles **HTTP relay** (desktop) and **TCP tunnel** (Android). Deploy with Wrangler only — it uploads the code and registers the `TUNNEL_HUB` Durable Object binding Android needs.

## Two keys

| | Key 1 — client | Key 2 — exit |
|---|----------------|--------------|
| **Path** | App → Apps Script | Apps Script → Worker |
| **In the app** | Yes (`auth-key`) | No |
| **Apps Script** | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| **Cloudflare** | — | `ZYRLN_RELAY_KEY` in `wrangler.toml` |

Key 2 is the **same key VPS uses** (`ZYRLN_RELAY_KEY`). Same value in Code.gs and wrangler.

## Deploy

```bash
cd relay/deploy/cloudflare
npm install -g wrangler   # once
wrangler login            # once
```

Edit before deploy:

1. **`worker.js`** — `WORKER_HOST = "your-worker.your-subdomain.workers.dev"` (hostname only, no `https://`)
2. **`wrangler.toml`** — `name` and `ZYRLN_RELAY_KEY` (key 2)
3. **`Code.gs`** — `AUTH_KEY` (key 1), `EXIT_RELAY_URL`, `EXIT_RELAY_KEY` (key 2, same value)

```toml
# wrangler.toml
[vars]
ZYRLN_RELAY_KEY = "your-exit-key"
```

```js
// Code.gs
const AUTH_KEY        = "your-app-key";
const EXIT_RELAY_URL  = "https://your-worker.workers.dev";  // no /relay
const EXIT_TUNNEL_URL = "";
const EXIT_RELAY_KEY  = "your-exit-key";
```

```bash
wrangler deploy
```

Confirm output includes `env.TUNNEL_HUB (TunnelHub)` and your `*.workers.dev` URL.

## Verify

```bash
curl -s -X POST "https://your-worker.workers.dev/tunnel" \
  -H "Content-Type: application/json" \
  -H "X-Relay-Key: your-exit-key" \
  -d '{"op":"open","id":"test-1","target":"149.154.167.92:443"}'
```

Expect `{"ok":true}`. If you see `missing TUNNEL_HUB binding`, run `wrangler deploy` again.

## Worker vs VPS

| | Worker | VPS |
|---|---|---|
| Cost | Free tier | ~$5/mo |
| Android tunnel / Telegram | Yes | Yes |
| Desktop HTTP relay | Yes | Yes |
| Key 2 name | `ZYRLN_RELAY_KEY` | `ZYRLN_RELAY_KEY` |
| Sites behind Cloudflare (X, Discord, ChatGPT…) | **No** — see below | Yes |
| Long-lived connection stability | Good, not perfect — see below | Best |

**Recommendation: use a VPS as the primary exit and the Worker as a free fallback.**

## Cloudflare-fronted destinations don't work

A Worker can't reach sites behind Cloudflare — the runtime blocks outbound TCP to Cloudflare IPs (close code `4001`). Platform limit, no Worker-side fix.

Major broken services: **X / Twitter, Discord, ChatGPT, LinkedIn** (plus most ad/CDN domains). Use a VPS exit for these. We intend to work toward routing such hosts to a non-Cloudflare exit.

Files: [`worker.js`](../relay/deploy/cloudflare/worker.js), [`wrangler.toml`](../relay/deploy/cloudflare/wrangler.toml).
