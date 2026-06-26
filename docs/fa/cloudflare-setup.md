# راه‌اندازی Cloudflare Worker

[English](../cloudflare-setup.md)

خروجی رایگان به‌جای VPS. یک Worker هم **رله HTTP** (دسکتاپ) هم **تونل TCP** (اندروید) را دارد. فقط با Wrangler دیپلوی کن — کد را آپلود می‌کند و binding `TUNNEL_HUB` Durable Object که اندروید نیاز دارد را ثبت می‌کند.

## دو کلید

| | کلید ۱ — کلاینت | کلید ۲ — exit |
|---|---|---|
| **مسیر** | اپ → Apps Script | Apps Script → Worker |
| **در اپ** | بله (`auth-key`) | خیر |
| **Apps Script** | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| **Cloudflare** | — | `ZYRLN_RELAY_KEY` در `wrangler.toml` |

کلید ۲ **همان کلید VPS** است (`ZYRLN_RELAY_KEY`). همان مقدار در Code.gs و wrangler.

## دیپلوی

```bash
cd relay/deploy/cloudflare
npm install -g wrangler   # یک بار
wrangler login            # یک بار
```

قبل از دیپلوی:

1. **`worker.js`** — `WORKER_HOST = "your-worker.your-subdomain.workers.dev"` (فقط hostname، بدون `https://`)
2. **`wrangler.toml`** — `name` و `ZYRLN_RELAY_KEY` (کلید ۲)
3. **`Code.gs`** — `AUTH_KEY` (کلید ۱)، `EXIT_RELAY_URL`، `EXIT_RELAY_KEY` (کلید ۲، همان مقدار)

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

خروجی باید شامل `env.TUNNEL_HUB (TunnelHub)` و URL `*.workers.dev` باشد.

## تأیید

```bash
curl -s -X POST "https://your-worker.workers.dev/tunnel" \
  -H "Content-Type: application/json" \
  -H "X-Relay-Key: your-exit-key" \
  -d '{"op":"open","id":"test-1","target":"149.154.167.92:443"}'
```

انتظار: `{"ok":true}`. اگر `missing TUNNEL_HUB binding` دیدی، دوباره `wrangler deploy` بزن.

## Worker در برابر VPS

| | Worker | VPS |
|---|---|---|
| هزینه | Free tier | ~۵ دلار/ماه |
| تونل اندروید / تلگرام | بله | بله |
| رله HTTP دسکتاپ | بله | بله |
| نام کلید ۲ | `ZYRLN_RELAY_KEY` | `ZYRLN_RELAY_KEY` |
| سایت‌های پشت Cloudflare (X، Discord، ChatGPT…) | **خیر** — پایین را ببین | بله |
| پایداری اتصال طولانی | خوب، نه عالی — پایین | بهترین |

**پیشنهاد: VPS به‌عنوان exit اصلی، Worker به‌عنوان fallback رایگان.**

## مقصدهای پشت Cloudflare کار نمی‌کنند

Worker نمی‌تواند به سایت‌های پشت Cloudflare برسد — runtime اتصال TCP خروجی به IPهای Cloudflare را بلاک می‌کند (close code `4001`). محدودیت پلتفرم؛ fix سمت Worker وجود ندارد.

سرویس‌های مهم مشکل‌دار: **X / Twitter، Discord، ChatGPT، LinkedIn** (و بیشتر دامنه‌های ad/CDN). برای این‌ها exit VPS استفاده کن.

فایل‌ها: [`worker.js`](../../relay/deploy/cloudflare/worker.js)، [`wrangler.toml`](../../relay/deploy/cloudflare/wrangler.toml).
