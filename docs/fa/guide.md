# راهنمای راه‌اندازی زیرلن

[English](../../README.md)

ابزار دور زدن فیلترینگ اینترنت در ایران. ترافیک از زیرساخت گوگل عبور می‌کند — بدون اثر انگشت VPN، بدون IP بلاک‌شده، بدون سرور اختصاصی قابل فیلتر.

دانلود: [GitHub Releases](https://github.com/ajavadinezhad/zyrln/releases)

---

## چطور کار می‌کند

سیستم فیلترینگ ایران (SNDPI) ترافیک را بررسی می‌کند. زیرلن به دو روش دور می‌زند:

**سرویس‌های گوگل (جیمیل، درایو، مپس و…):**
ترافیک مستقیم به گوگل می‌رود، اما دست‌دهی TLS به قطعات کوچک تقسیم می‌شود. سانسور نمی‌تواند به‌موقع SNI را بخواند، پس اتصال رد می‌شود. سرور لازم نیست.

**بقیه سایت‌ها (اینستاگرام، توییتر و…):**
ترافیک از Google Apps Script عبور می‌کند. از نظر سانسور شبیه ترافیک عادی گوگل است. Apps Script آن را به رله خروجی (VPS یا Cloudflare) می‌فرستد که به سایت واقعی وصل می‌شود.

همه ترافیک رله از کلاینت **باید** از Apps Script عبور کند (هرگز اپ را مستقیم به URL مربوط به VPS/Worker وصل نکن).

---

## فقط سرویس‌های گوگل (جیمیل، درایو، مپس)

**بدون سرور. بدون راه‌اندازی رله.**

1. از [Releases](https://github.com/ajavadinezhad/zyrln/releases) برای پلتفرم خودت دانلود کن
2. اجرا کن — GUI در مرورگر باز می‌شود
   - **ویندوز:** روی `.exe` دوبار کلیک
   - **لینوکس / macOS:** با `-gui` اجرا کن:
     ```bash
     ./zyrln-VERSION-linux-amd64 -gui          # Linux
     ./zyrln-VERSION-darwin-arm64 -gui         # macOS Apple Silicon
     ./zyrln-VERSION-darwin-amd64 -gui         # macOS Intel
     ```
3. دکمه **برق** در نوار بالا → حالت مستقیم (Direct Mode)
4. پروکسی HTTP مرورگر: `127.0.0.1:8085`

حالت مستقیم برای سرویس‌های گوگلی که SNI-filtered هستند اما IP-blocked نیستند کار می‌کند. پخش یوتیوب و دانلود پلی‌استور معمولاً به رله کامل نیاز دارند. بسته به ISP و شهر فرق می‌کند.

---

## دسترسی کامل (اینستاگرام، توییتر، تلگرام، …)

راه‌اندازی زنجیره رله حدود ۱۵ دقیقه.

### چی نیاز دارم

| | چی | هزینه |
|---|---|---|
| ضروری | اکانت گوگل | رایگان |
| ضروری | کلید امنیتی (خودت می‌سازی) | رایگان |
| یکی از دو | VPS با IP عمومی | ~۵ دلار/ماه |
| یا | Cloudflare Worker | رایگان — [راه‌اندازی](cloudflare-setup.md) |

### مرحله ۱ — برنامه دسکتاپ

1. از [Releases](https://github.com/ajavadinezhad/zyrln/releases) دانلود کن
2. اجرا — GUI در مرورگر (ویندوز: `.exe`؛ لینوکس/macOS: `-gui` مثل بالا)
3. **Security** → ساخت و نصب گواهی CA (**فقط دسکتاپ** — برای HTTPS در مسیر رله با MITM)
4. **Settings** → **Generate Key** → کلید را برای Apps Script کپی کن

**پروکسی مرورگر:**

| مرورگر | تنظیم |
|---|---|
| Chrome / Edge | Manual proxy → `127.0.0.1:8085` |
| Firefox | Manual HTTP proxy → `127.0.0.1` پورت `8085` |
| کل سیستم | SOCKS5 → `127.0.0.1:1080` |

**گواهی CA (فقط رله دسکتاپ):**

- Chrome/Edge: Settings → Privacy → Security → Manage certificates → Authorities → Import `zyrln-ca.pem`
- Firefox: Settings → Privacy & Security → Certificates → Authorities → Import

### مرحله ۲ — رله خروجی

exit سایت‌های واقعی را باز می‌کند. یکی را انتخاب کن:

#### گزینه الف — Cloudflare Worker (رایگان)

[راه‌اندازی Cloudflare Worker](cloudflare-setup.md) — دیپلوی از `relay/deploy/cloudflare/`.

#### گزینه ب — VPS

VPS لینوکس (amd64 یا arm64)، IP عمومی، پورت **8787** باز، SSH با `user@host` و `sudo`.

1. **`zyrln-VERSION-vps.zip`** را از [Releases](https://github.com/ajavadinezhad/zyrln/releases) بگیر و unzip کن
2. `./install-vps-relay.sh user@YOUR_VPS_IP` را اجرا کن  
   `ZYRLN_RELAY_KEY=secret` یا `ZYRLN_RELAY_KEY=auto` — همان مقدار `EXIT_RELAY_KEY` در Apps Script
3. در `Code.gs`: `EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay"` و `EXIT_RELAY_KEY` مطابق VPS

تست: `curl -s http://YOUR_VPS_IP:8787/healthz` → `ok`

### مرحله ۳ — Apps Script

درب ورودی روی سرورهای گوگل.

#### دو کلید

اپ فقط کلید ۱ را می‌خواهد.

| | کلید ۱ — کلاینت | کلید ۲ — exit |
|---|---|---|
| مسیر | اپ → Apps Script | Apps Script → VPS یا Cloudflare |
| در اپ | بله (`auth-key`) | خیر |
| Apps Script | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| Exit | — | `ZYRLN_RELAY_KEY` |
| روی سیم | JSON `"k"` | هدر `X-Relay-Key` |

کلید ۲ روی VPS و Cloudflare **همان نام** دارد (`ZYRLN_RELAY_KEY`). همان مقدار در `EXIT_RELAY_KEY` (Code.gs) و `ZYRLN_RELAY_KEY` (exit). اگر exit بدون کلید است، `EXIT_RELAY_KEY` را در Code.gs خالی بگذار.

```
App ──key 1──► Apps Script ──key 2──► Cloudflare or VPS
```

1. [script.google.com](https://script.google.com) → **New project**
2. محتوای [`relay/deploy/apps-script/Code.gs`](../../relay/deploy/apps-script/Code.gs) را paste کن
3. ثابت‌ها را ویرایش کن — **Cloudflare** یا **VPS**، نه هر دو:

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
5. URL را کپی کن: `https://script.google.com/macros/s/AKfycb.../exec`

هر اکانت گوگل حدود ۲۰٬۰۰۰ درخواست رله در روز دارد. برای resilience چند deployment (اکانت‌های مختلف) را با ویرگول در اپ اضافه کن.

### مرحله ۴ — اتصال (دسکتاپ)

1. **+** → پروفایل جدید
2. URL Apps Script و کلید را paste کن
3. **Save** → **Connect**

### مرحله ۵ — اندروید

1. APK را از [Releases](https://github.com/ajavadinezhad/zyrln/releases) نصب کن
2. افزودن کانفیگ:
   - **Import Config from Clipboard** (JSON از دسکتاپ **Tools → Mobile Export**، یا share روی گوشی دیگر)، یا
   - **+** و وارد کردن دستی URL + کلید
3. روی کانفیگ بزن → **Connect** → اجازه VPN
4. اختیاری: **Split tunnel** — فقط اپ‌های انتخاب‌شده از زیرلن رد شوند (برای تغییر VPN را قطع کن)
5. اشتراک کانفیگ: **share** روی هر ردیف → JSON در کلیپبورد

---

## ساخت از سورس

Go 1.25+ لازم است.

```bash
make desktop              # ./zyrln محلی
make desktop-release      # dist/ برای لینوکس، ویندوز، macOS
make desktop-linux|windows|macos
make keystore             # یک بار — کلید امضای اندروید
make android              # APK release امضاشده
make proxy                # پروکسی از سورس (نیاز به CA)
make test
make vps-relay-bundle     # → dist/zyrln-VERSION-vps.zip
```

ساختار پروژه و نکات توسعه: [contributing.md](../contributing.md) (English).

---

## مشکلات رایج

**هیچ سایتی از پروکسی باز نمی‌شود**

- پروکسی روشن است؟ (نشانگر سبز در GUI)
- پروکسی مرورگر: `127.0.0.1:8085`
- Diagnostics (دکمه play در Tools)

**خطای SSL در HTTPS (فقط دسکتاپ)**

- CA نصب یا trusted نیست — دوباره `certs/zyrln-ca.pem` را import کن

**سهمیه Apps Script تمام شد**

- deployment از اکانت‌های دیگر؛ URLها را با ویرگول در اپ بگذار

**یوتیوب باز می‌شود، اینستاگرام نه**

- اینستاگرام IP-blocked است — زنجیره رله کامل لازم است
- VPS/Cloudflare exit در حال اجراست؟

**X / Discord / ChatGPT روی exit Cloudflare Worker**

- Worker به سایت‌های پشت Cloudflare نمی‌رسد — VPS لازم است. [cloudflare-setup.md](cloudflare-setup.md).

---

## امنیت

- Apps Script و کلید اختصاصی خودت را deploy کن
- `config.env`، `certs/` و کلیدها را commit نکن
- گوگل و exit provider متادیتا (زمان، حجم) را می‌بینند، نه محتوای رمزشده روی مسیر تونل
- در صورت leak کلید را عوض کن
- `certs/zyrln-ca-key.pem` فقط روی دستگاه خودت (دسکتاپ)

---

## اعتبار

Domain-fronting: [denuitt1/mhr-cfw](https://github.com/denuitt1/mhr-cfw). TLS fragmentation: [GFW-knocker](https://github.com/GFW-knocker).

License: MIT — [LICENSE](../../LICENSE).
