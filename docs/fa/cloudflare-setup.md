# راه‌اندازی Cloudflare Worker

خروجی رایگان به‌جای VPS. یک Worker هم **رله HTTP** (دسکتاپ) هم **تونل TCP** (اندروید) را دارد. فقط با Wrangler دیپلوی کن — binding مورد نیاز اندروید (`TUNNEL_HUB`) خودکار ثبت می‌شود.

## دیپلوی

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
cd relay/deploy/cloudflare
npm install -g wrangler   # یک بار
wrangler login            # یک بار
```

</div>

قبل از دیپلوی:

1. **`worker.js`** — `WORKER_HOST = "your-worker.subdomain.workers.dev"` (بدون `https://`)
2. **`wrangler.toml`** — `name` و `ZYRLN_RELAY_KEY` (یک رشته تصادفی)
3. **`Code.gs`** — همان مقدار به‌عنوان `EXIT_RELAY_KEY`

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
wrangler deploy
```

</div>

باید `env.TUNNEL_HUB (TunnelHub)` و آدرس `*.workers.dev` را ببینی.

## Apps Script

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```js
const AUTH_KEY        = "your-key-matching-zyrln-auth-key";
const EXIT_RELAY_URL  = "https://your-worker.workers.dev";  // بدون /relay
const EXIT_TUNNEL_URL = "";   // خالی — Apps Script خودش /tunnel می‌سازد
const EXIT_RELAY_KEY  = "your-exit-key";   // همان ZYRLN_RELAY_KEY
```

</div>

## کلید خروجی (همان VPS)

| کجا | تنظیم |
|-----|--------|
| Apps Script | `EXIT_RELAY_KEY` |
| Cloudflare `wrangler.toml` | `ZYRLN_RELAY_KEY` |
| VPS | `ZYRLN_RELAY_KEY` |

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```toml
[vars]
ZYRLN_RELAY_KEY = "your-exit-key"
```

```js
const EXIT_RELAY_KEY = "your-exit-key";
```

</div>

<div dir="rtl">

Apps Script: **Deploy → Manage deployments → New version**. Worker: `wrangler deploy`.

</div>

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
curl -s -X POST "https://your-worker.workers.dev/tunnel" \
  -H "Content-Type: application/json" \
  -H "X-Relay-Key: your-exit-key" \
  -d '{"op":"open","id":"test-1","target":"149.154.167.92:443"}'
```

</div>

<div dir="rtl">

پاسخ: `{"ok":true}`.

</div>

فایل‌ها: [`worker.js`](../../relay/deploy/cloudflare/worker.js)، [`wrangler.toml`](../../relay/deploy/cloudflare/wrangler.toml).
