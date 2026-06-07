# زیرلن (Zyrln)

[English](README.md)

ابزاری برای دور زدن فیلترینگ اینترنت در ایران. ترافیک شما را از زیرساخت گوگل عبور می‌دهد — بدون اثر انگشت VPN، بدون آی‌پی بلاک‌شده، بدون سرور اختصاصی قابل فیلتر.

---

## فهرست مطالب

- [چطور کار می‌کند](#چطور-کار-میکند)
- [فقط می‌خوام سرویس‌های گوگل باز بشه (جیمیل، درایو، مپس)](#فقط-میخوام-سرویسهای-گوگل-باز-بشه-جیمیل-درایو-مپس)
- [می‌خوام همه چیز باز بشه](#میخوام-همه-چیز-باز-بشه)
  - [چی نیاز دارم](#چی-نیاز-دارم)
  - [مرحله ۱ — اجرای برنامه دسکتاپ](#مرحله-۱--اجرای-برنامه-دسکتاپ)
  - [مرحله ۲ — راه‌اندازی رله خروجی](#مرحله-۲--راهاندازی-رله-خروجی)
  - [مرحله ۳ — راه‌اندازی Apps Script](#مرحله-۳--راهاندازی-apps-script)
  - [مرحله ۴ — اتصال](#مرحله-۴--اتصال)
  - [مرحله ۵ — راه‌اندازی اندروید](#مرحله-۵--راهاندازی-اندروید)
- [ساخت از سورس](#ساخت-از-سورس)
- [مشکلات رایج](#مشکلات-رایج)
- [نکات امنیتی](#نکات-امنیتی)

---

## چطور کار می‌کند

سیستم فیلترینگ ایران (SNDPI) ترافیک را بررسی می‌کند تا سایت‌های فیلترشده را شناسایی و بلاک کند. زیرلن به دو روش این سیستم را دور می‌زند:

**برای سرویس‌های گوگل (جیمیل، درایو، مپس و...):**
ترافیک مستقیم به گوگل فرستاده می‌شود، اما دست‌دهی TLS به قطعات کوچک تقسیم می‌شود. سیستم فیلترینگ نمی‌تواند این قطعات را به‌موقع بازسازی کند تا آدرس سایت را بخواند، پس اتصال را رد می‌کند. نیازی به هیچ سروری نیست.

**برای بقیه سایت‌ها (اینستاگرام، توییتر و...):**
ترافیک از طریق Google Apps Script هدایت می‌شود — یک سرویس رایگان گوگل. از نظر سیستم فیلترینگ، این ترافیک عادی گوگل به نظر می‌رسد. Apps Script آن را به رله خروجی شما (VPS یا Cloudflare) می‌فرستد که سایت واقعی را باز می‌کند.

---

## فقط می‌خوام سرویس‌های گوگل باز بشه (جیمیل، درایو، مپس)

**نیازی به سرور نیست. هیچ راه‌اندازی‌ای لازم نیست. فقط دانلود کن و فعال کن.**

1. برنامه را از صفحه [Releases](../../releases) دانلود کن
2. اجرا کن — رابط گرافیکی خودبه‌خود در مرورگر باز می‌شود
   - **ویندوز:** روی فایل `.exe` دوبار کلیک کن — GUI خودکار باز می‌شود
   - **لینوکس / macOS:** از ترمینال با فلگ `-gui` اجرا کن:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
# Linux
./zyrln-VERSION-linux-amd64 -gui
# macOS Apple Silicon
./zyrln-VERSION-darwin-arm64 -gui
# macOS Intel
./zyrln-VERSION-darwin-amd64 -gui
```

</div>

3. روی دکمه **⚡ برق** در نوار بالا کلیک کن تا حالت مستقیم فعال شود (سبز می‌شود)
4. در مرورگرت پروکسی HTTP را روی `127.0.0.1:8085` تنظیم کن

همین. بسیاری از سرویس‌های گوگل می‌توانند وقتی مسیر شبکه اجازه بدهد از مسیر مستقیم و سریع‌تر استفاده کنند.

> حالت مستقیم برای سرویس‌های گوگلی کار می‌کند که SNI-filtered هستند اما IP-blocked نیستند — جیمیل، درایو، مپس، گوگل داکس و مشابه. پخش ویدیو در یوتیوب و دانلود از پلی استور از طریق رله می‌رود. رفتار فیلترینگ بسته به ISP، شهر، اپراتور و زمان تغییر می‌کند.

---

## می‌خوام همه چیز باز بشه

برای دسترسی به اینستاگرام، توییتر، تلگرام و سایت‌های غیر گوگل باید زنجیره رله راه‌اندازی کنی. حدود ۱۵ دقیقه طول می‌کشد.

### چی نیاز دارم

| | چی | هزینه |
|---|---|---|
| ✅ ضروری | اکانت گوگل | رایگان |
| ✅ ضروری | کلید امنیتی (خودت می‌سازی) | رایگان |
| ☁️ یکی از این دو | VPS با آی‌پی ثابت | ~۵ دلار در ماه |
| ☁️ یا این | Cloudflare Worker | رایگان — [راه‌اندازی](docs/fa/cloudflare-setup.md) |

### مرحله ۱ — اجرای برنامه دسکتاپ

1. باینری سیستم‌عاملت را از [Releases](../../releases) دانلود کن
2. اجرا کن — رابط گرافیکی خودبه‌خود در مرورگر باز می‌شود
   - **ویندوز:** روی فایل `.exe` دوبار کلیک کن — GUI خودکار باز می‌شود
   - **لینوکس / macOS:** از ترمینال با فلگ `-gui` اجرا کن:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
# Linux
./zyrln-VERSION-linux-amd64 -gui
# macOS Apple Silicon
./zyrln-VERSION-darwin-arm64 -gui
# macOS Intel
./zyrln-VERSION-darwin-amd64 -gui
```

</div>

3. در بخش **Security** گواهینامه CA بساز و نصب کن (برای سایت‌های HTTPS لازم است)
4. در بخش **Settings** روی **Generate Key** کلیک کن و کلید را کپی کن — در مراحل بعد به آن نیاز داری

**تنظیم مرورگر:**

| مرورگر | کجا تنظیم شود |
|---|---|
| Chrome / Edge | Settings → System → Open proxy settings → Manual proxy → `127.0.0.1:8085` |
| Firefox | Settings → Network → Manual proxy → HTTP `127.0.0.1` port `8085` |
| کل سیستم | SOCKS5 با آدرس `127.0.0.1` پورت `1080` در تنظیمات شبکه سیستم‌عامل |

**نصب گواهینامه CA** (برای سایت‌های HTTPS ضروری است):

- **Chrome/Edge**: Settings → Privacy → Security → Manage certificates → Authorities → Import فایل `zyrln-ca.pem`
- **Firefox**: Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import

### مرحله ۲ — راه‌اندازی رله خروجی

این گره خروجی است که سایت‌های واقعی را باز می‌کند. یکی را انتخاب کن:

#### گزینه الف — Cloudflare Worker (پیشنهادی، رایگان)

[راه‌اندازی Worker](docs/fa/cloudflare-setup.md) — دیپلوی با Wrangler از `relay/deploy/cloudflare/`.

#### گزینه ب — VPS

VPS لینوکس (amd64 یا arm64)، IP عمومی، پورت **۸۷۸۷** باز، SSH با `user@host` و `sudo`. روی لپ‌تاپ فقط `ssh`/`scp` (بدون Go).

1. **`zyrln-VERSION-vps.zip`** را از [Releases](../../releases) بگیر و از حالت فشرده خارج کن.
2. در همان پوشه: `./install-vps-relay.sh user@IP_VPS` — `ZYRLN_RELAY_KEY=secret` یا `ZYRLN_RELAY_KEY=auto` (همان `EXIT_RELAY_KEY` در Apps Script).
3. در `Code.gs`: `EXIT_RELAY_URL = "http://IP_VPS:8787/relay"` و `EXIT_RELAY_KEY` مطابق VPS.

تست: `curl -s http://IP_VPS:8787/healthz` باید `ok` چاپ کند.

### مرحله ۳ — راه‌اندازی Apps Script

این درب ورودی است. روی سرورهای گوگل اجرا می‌شود و ترافیک تو را دریافت می‌کند.

#### دو کلید

دقیقاً **دو کلید** داری. اپ فقط کلید اول را می‌خواهد.

| | کلید ۱ — کلاینت | کلید ۲ — exit |
|---|----------------|---------------|
| **مسیر** | اپ → Apps Script | Apps Script → VPS یا Cloudflare |
| **در اپ** | بله (`auth-key`) | خیر |
| **Apps Script** | `AUTH_KEY` | `EXIT_RELAY_KEY` |
| **Exit** | — | `ZYRLN_RELAY_KEY` |
| **روی سیم** | JSON `"k"` | هدر `X-Relay-Key` |

کلید ۲ روی VPS و Cloudflare **همان نام** دارد (`ZYRLN_RELAY_KEY`). همان مقدار در `EXIT_RELAY_KEY` (Code.gs) و `ZYRLN_RELAY_KEY` (exit).

1. به [script.google.com](https://script.google.com) برو → **پروژه جدید**
2. کد پیش‌فرض را پاک کن و محتوای فایل [`relay/deploy/apps-script/Code.gs`](relay/deploy/apps-script/Code.gs) را جای‌گذاری کن
3. ثابت‌های بالای فایل را ویرایش کن — **Cloudflare** یا **VPS**، نه هر دو:

**Cloudflare Worker**:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```js
const AUTH_KEY        = "your-key-from-step-1";
const EXIT_RELAY_URL  = "https://your-worker.your-subdomain.workers.dev";  // بدون /relay
const EXIT_TUNNEL_URL = "";   // خالی — Apps Script خودش /tunnel می‌سازد
const EXIT_RELAY_KEY  = "your-exit-key";   // همان ZYRLN_RELAY_KEY در wrangler.toml
```

</div>

**VPS**:

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```js
const AUTH_KEY        = "your-key-from-step-1";
const EXIT_RELAY_URL  = "http://YOUR_VPS_IP:8787/relay";
const EXIT_TUNNEL_URL = "http://YOUR_VPS_IP:8787/tunnel";
const EXIT_RELAY_KEY  = "your-exit-key";   // همان ZYRLN_RELAY_KEY روی VPS
```

</div>

<div dir="rtl">

جزئیات دیپلوی Wrangler: [راه‌اندازی Cloudflare Worker](docs/fa/cloudflare-setup.md).

</div>

4. روی **Deploy → New deployment** کلیک کن
   - Type: **Web app**
   - Execute as: **Me**
   - Who has access: **Anyone**
5. روی **Deploy** کلیک کن و لینک را کپی کن — چیزی شبیه:
   `https://script.google.com/macros/s/AKfycb.../exec`

> هر اکانت گوگل ۲۰,۰۰۰ درخواست در روز دارد. می‌توانی چندین لینک از اکانت‌های مختلف را با ویرگول جدا کنی. زیرلن همه را موازی امتحان می‌کند.

### مرحله ۴ — اتصال

1. در برنامه روی **+** کلیک کن تا پروفایل جدید بسازی
2. آدرس Apps Script و کلید امنیتی را وارد کن
3. **Save** → **Connect**

### مرحله ۵ — راه‌اندازی اندروید

خلاصه:
1. APK را از [Releases](../../releases) نصب کن
2. در برنامه دسکتاپ روی دکمه **export** کلیک کن → JSON را کپی کن
3. در اپ اندروید: **Import Config from Clipboard** را بزن
4. روی کانفیگ بزن، **Connect** را بزن و اجازه VPN بده

---

## ساخت از سورس

نیاز به Go نسخه ۱.۲۵ به بالا دارد.

<div dir="ltr" align="left" style="direction: ltr; text-align: left;">

```bash
# باینری دسکتاپ
make desktop

# باینری‌های انتشار دسکتاپ برای لینوکس، ویندوز و مک
make desktop-release

# یا ساخت یک پلتفرم
make desktop-linux
make desktop-windows
make desktop-macos

# APK اندروید (نیاز به Android SDK و NDK)
# یک بار اجرا کن — کلید امضا می‌سازد
make keystore
# APK نهایی می‌سازد
make android

# اجرای پروکسی از سورس
make proxy

# تست
make test

# zip نصب VPS (اسکریپت + باینری amd64/arm64)
make vps-relay-bundle   # → dist/zyrln-VERSION-vps.zip
```

</div>

`make desktop` یک باینری محلی `./zyrln` برای همین سیستم می‌سازد. `make desktop-release` باینری‌های مخصوص هر پلتفرم را داخل `dist/` می‌سازد.

---

## مشکلات رایج

**هیچ سایتی از طریق پروکسی باز نمی‌شود**
- مطمئن شو برنامه در حال اجراست (نقطه سبز در GUI)
- تنظیمات پروکسی مرورگر را بررسی کن: `127.0.0.1:8085`
- از ابزار Diagnostics (دکمه پلی در بخش Tools) استفاده کن

**سایت‌های HTTPS خطای SSL می‌دهند (فقط دسکتاپ)**
- گواهینامه CA نصب نشده یا مورد اعتماد نیست
- دسکتاپ: فایل `certs/zyrln-ca.pem` را دوباره در مرورگر Import کن

**سهمیه Apps Script تمام شده**
- از چند اکانت گوگل استفاده کن و لینک‌ها را با ویرگول جدا کن

**یوتیوب باز می‌شود ولی اینستاگرام نه**
- اینستاگرام علاوه بر SNI، آی‌پی‌هایش هم بلاک است — به زنجیره رله کامل نیاز دارد
- مطمئن شو VPS یا Cloudflare در حال اجراست


---

## نکات امنیتی

- هر کاربر باید Apps Script و کلید اختصاصی خودش را داشته باشد
- فایل `config.env`، پوشه `certs/` و هیچ کلید امنیتی‌ای را کامیت نکن
- گوگل و ارائه‌دهنده VPS/Cloudflare می‌توانند متادیتای ترافیک را ببینند، اما محتوا را نه
- اگر کلیدت جایی لیک شد، یک کلید جدید بساز و در همه جا جایگزین کن
- کلید خصوصی CA (`certs/zyrln-ca-key.pem`) را با کسی به اشتراک نگذار

---

## لایسنس

MIT — فایل [LICENSE](LICENSE) را ببین.
