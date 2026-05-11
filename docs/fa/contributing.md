# راهنمای مشارکت

## ساختار پروژه

<div dir="ltr">

```
zyrln/
├── platforms/
│   ├── desktop/        # باینری دسکتاپ (main package)
│   │   ├── main.go     # فلگ‌های CLI، runner پروب، لانچر پروکسی
│   │   └── main_test.go
│   └── mobile/         # bindings های gomobile برای اندروید
│       └── mobile.go   # API خروجی: Start, Stop, IsRunning, LastError, GenerateCA
│
├── relay/
│   ├── core/           # منطق مشترک رله (هم دسکتاپ هم اندروید استفاده می‌کنند)
│   │   ├── relay.go    # RelayRequest، HTTP با domain-fronting، کدگذاری payload
│   │   ├── proxy.go    # StartProxy، هندلر HTTP+HTTPS MITM
│   │   ├── cert.go     # GenerateCA، LoadCA، CertForHost (گواهینامه موقت هر دامنه)
│   │   ├── direct.go   # حالت مستقیم: تشخیص دامنه‌های گوگل، dial با fragmentation
│   │   ├── fragment.go # تکه‌تکه کردن ClientHello TLS برای دور زدن SNI
│   │   └── *_test.go
│   ├── apps-script/
│   │   └── Code.gs     # رله Google Apps Script (روی سرورهای گوگل اجرا می‌شود)
│   ├── vps/
│   │   └── main.go     # باینری رله خروجی برای VPS
│   └── cloudflare/
│       └── worker.js   # رله خروجی جایگزین به عنوان Cloudflare Worker
│
├── android/            # پروژه Android Studio
│   └── app/src/main/java/com/zyrln/relay/
│       ├── MainActivity.kt      # رابط کاربری: اتصال/قطع، نصب CA، حالت مستقیم
│       └── RelayVpnService.kt   # VpnService: اجرای پروکسی Go، تنظیم پروکسی سیستم
│
├── docs/               # راهنماها
├── Makefile
└── go.mod
```

</div>

## مفاهیم کلیدی

**`relay/core`** قلب پروژه است. هم دسکتاپ و هم اندروید آن را import می‌کنند.

- `relay.go`: درخواست رله را از طریق domain-fronting می‌سازد و می‌فرستد. ترفند domain-fronting این است که `req.URL.Host` به دامنه جلویی (مثلاً `www.google.com`) تنظیم می‌شود تا TLS به IP‌های گوگل وصل شود، در حالی که `req.Host` آدرس واقعی Apps Script را داخل تونل TLS رمزنگاری‌شده حمل می‌کند.
- `proxy.go`: پروکسی HTTP که ترافیک مرورگر را می‌گیرد. درخواست‌های HTTP مستقیم رله می‌شوند؛ اتصال‌های HTTPS از تونل `CONNECT` با TLS termination محلی (MITM) استفاده می‌کنند.
- `cert.go`: یک CA محلی می‌سازد و گواهینامه‌های leaf به ازای هر hostname روی demand امضا می‌کند، با cache در حافظه.
- `direct.go`: حالت مستقیم برای سرویس‌های گوگل. دامنه‌های گوگل را تشخیص می‌دهد و اتصال مستقیم با fragmentation برقرار می‌کند — بدون MITM، بدون رله.
- `fragment.go`: ClientHello اول TLS را به ۸۷ قطعه تصادفی تقسیم می‌کند با ۵ms تأخیر بین هر قطعه. این کار سیستم SNDPI را از خواندن SNI باز می‌دارد.

**`platforms/mobile`** یک API flat مبتنی بر string ارائه می‌دهد (`Start`، `Stop` و غیره) چون gomobile فقط از انواع primitive در مرز پشتیبانی می‌کند. تمام خطاها به عنوان string برگردانده می‌شوند، نه مقادیر `error` در Go.

## اجرای تست‌ها

<div dir="ltr">

```bash
go test ./relay/core/... ./platforms/desktop/...
```

</div>

یا همه چیز با هم:

<div dir="ltr">

```bash
go test ./...
```

</div>

تست‌ها فقط از کتابخانه استاندارد استفاده می‌کنند، بدون framework تست خارجی.

## ساخت

<div dir="ltr">

```bash
make desktop          # ساخت باینری ./zyrln
make desktop-release  # ساخت باینری‌های دسکتاپ لینوکس، ویندوز و مک
make android          # ساخت APK نهایی امضاشده (نیاز به keystore)
```

</div>

راه‌اندازی اول gomobile:

<div dir="ltr">

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
export ANDROID_HOME=~/Android/Sdk
```

</div>

## افزودن Probe جدید

Probe‌ها در `platforms/desktop/main.go` در تابع `defaultProbes()` تعریف می‌شوند. هر probe یک struct `probe` است:

<div dir="ltr">

```go
{
    ID:          "unique-id",
    Name:        "نام قابل خواندن",
    Category:    "baseline",
    Method:      http.MethodGet,
    URL:         "https://example.com/",
    Expectation: "معنای موفقیت تست",
}
```

</div>

## تغییر پروتکل رله

فرمت payload رله در `relay/core/relay.go` (تابع `buildRelayPayload`) تعریف شده و باید با انتظارات `relay/apps-script/Code.gs` مطابقت داشته باشد. اگر یک طرف را تغییر دادی، طرف دیگر را هم به‌روز کن.

## اسرار و Gitignore

هرگز commit نکن:
- `config.env`: آدرس Apps Script و کلید امنیتی تو را دارد
- `certs/`: کلید خصوصی CA محلی تو را دارد
- هر فایلی که `AUTH_KEY` یا کلیدهای رله دارد

این موارد در `.gitignore` پوشش داده شده‌اند.
