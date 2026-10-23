<div dir="rtl">

# راه‌اندازی خروجی <span dir="ltr">Cloudflare</span>

[بازگشت به ساخت رله](relay-setup.md) · [<span dir="ltr">English</span>](../cloudflare-setup.md)

این صفحه مرحله ۲ راهنمای ساخت رله است. حساب <span dir="ltr">Cloudflare</span>، برنامه <span dir="ltr">Node.js/npm</span> و ترمینال لازم دارید. اینجا از **کلید خروجی** استفاده می‌کنید؛ کلید برنامه برای <span dir="ltr">Apps Script</span> و زیرلن است.

**قبل از انتخاب:** تونل <span dir="ltr">TCP</span> کلادفلر به محدوده <span dir="ltr">IP</span> خود <span dir="ltr">Cloudflare</span> وصل نمی‌شود. ممکن است بعضی سایت‌ها روی اندروید باز نشوند، حتی وقتی سایت‌های دیگر کار می‌کنند. رله <span dir="ltr">HTTP</span> دسکتاپ مسیر متفاوتی دارد و این محدودیت به معنی شکست همه درخواست‌های دسکتاپ به سایت‌های کلادفلری نیست. <span dir="ltr">VPS</span> این محدودیت مشخص <span dir="ltr">TCP</span> را ندارد. محدودیت مصرف حساب همچنان اعمال می‌شود.

## ۱. فایل‌های استقرار را بگیرید

فایل **<span dir="ltr">Source code</span>** مخزن را از [صفحه انتشار](https://github.com/ajavadinezhad/zyrln/releases) بگیرید و استخراج کنید، یا مخزن را <span dir="ltr">clone</span> کنید. ترمینال را در پوشه <code dir="ltr">relay/deploy/cloudflare</code> باز کنید.

باید هر دو فایل <code dir="ltr">worker.js</code> و <code dir="ltr">wrangler.toml</code> وجود داشته باشند. برای دیپلوی از <span dir="ltr">Wrangler</span> استفاده کنید تا اتصال <span dir="ltr">Durable Object</span> موردنیاز اندروید ساخته شود. چسباندن <span dir="ltr">JavaScript</span> به‌تنهایی در داشبورد این اتصال را تنظیم نمی‌کند.

## ۲. نام و آدرس <span dir="ltr">Worker</span> را مشخص کنید

1. در <code dir="ltr">wrangler.toml</code> مقدار <code dir="ltr">name = "mine"</code> را به نام دلخواه، مثلاً <code dir="ltr">zyrln-exit</code>، تغییر دهید.
2. زیردامنه <span dir="ltr">Workers</span> حساب خود را در داشبورد <span dir="ltr">Cloudflare</span> پیدا کنید. همراه با نام انتخابی، آدرس می‌شود <code dir="ltr">zyrln-exit.YOUR_SUBDOMAIN.workers.dev</code>.
3. در <code dir="ltr">worker.js</code> مقدار <code dir="ltr">CHANGE_ME_WORKER_HOST</code> را با همین نام میزبان، بدون <code dir="ltr">https://</code> یا مسیر، جایگزین کنید. این تنظیم جلوی درخواست رله به خودش را می‌گیرد.

بخش‌های موجود مربوط به <code dir="ltr">TUNNEL_HUB</code> و <span dir="ltr">migration</span> را در <code dir="ltr">wrangler.toml</code> نگه دارید.

## ۳. وارد حساب شوید، کلید خروجی را ذخیره و دیپلوی کنید

در همان پوشه اجرا کنید:

<div dir="ltr">

```bash
npx wrangler login
npx wrangler secret put ZYRLN_RELAY_KEY
npx wrangler deploy
```

</div>

وقتی مقدار <span dir="ltr">secret</span> را خواست، **کلید خروجی** را وارد کنید. اگر قبل از ذخیره کلید پرسید <span dir="ltr">Worker</span> را بسازد، ساخت آن را تأیید و بعد دیپلوی را ادامه دهید. کلید واقعی را در فایل <code dir="ltr">wrangler.toml</code> که در گیت ثبت می‌شود نگذارید.

در خروجی دیپلوی باید آدرس <span dir="ltr">Worker</span> و اتصال <code dir="ltr">TUNNEL_HUB</code> دیده شود. اگر آدرس با مقداری که در <code dir="ltr">WORKER_HOST</code> نوشته‌اید فرق دارد، مقدار را اصلاح و دوباره دیپلوی کنید.

## ۴. به راه‌اندازی <span dir="ltr">Google Apps Script</span> برگردید

این دو مقدار را نگه دارید:

| مقدار | نمونه |
|---|---|
| آدرس پایه <span dir="ltr">Worker</span> | <code dir="ltr">https://zyrln-exit.YOUR_SUBDOMAIN.workers.dev</code> |
| کلید خروجی | همان <span dir="ltr">secret</span> که ذخیره کردید |

به [راهنمای ساخت رله، مرحله ۳](relay-setup.md) برگردید. آدرس پایه <span dir="ltr">Worker</span> را بدون <code dir="ltr">/relay</code> وارد کنید، <code dir="ltr">EXIT_TUNNEL_URL</code> را خالی بگذارید و <code dir="ltr">EXIT_RELAY_KEY</code> را برابر همان کلید خروجی تنظیم کنید.

## اگر دیپلوی یا اتصال انجام نشد

- **<span dir="ltr">Wrangler</span> اجرا نمی‌شود:** نصب بودن <span dir="ltr">Node.js/npm</span> و پوشه فعلی ترمینال را بررسی کنید.
- **خطای حساب، پلن یا سهمیه:** پیام مشخص را در حساب <span dir="ltr">Cloudflare</span> بررسی کنید؛ دیپلوی برای همه حساب‌ها تضمین‌شده نیست.
- **خطای اتصال <code dir="ltr">TUNNEL_HUB</code>:** از پوشه‌ای که تنظیمات <span dir="ltr">binding</span> و <span dir="ltr">migration</span> را دارد، دوباره <code dir="ltr">npx wrangler deploy</code> را اجرا کنید.
- **خطای <span dir="ltr">Unauthorized</span>:** مقدار <code dir="ltr">EXIT_RELAY_KEY</code> در <span dir="ltr">Apps Script</span> باید با <span dir="ltr">secret</span> روی <span dir="ltr">Worker</span> یکی باشد. خود برنامه از کلید برنامه استفاده می‌کند.
- **فقط بعضی مقصدها باز نمی‌شوند:** پروفایل <span dir="ltr">VPS</span> را امتحان کنید؛ تلاش دوباره محدودیت مقصد <span dir="ltr">TCP</span> کلادفلر را برطرف نمی‌کند.

فایل‌های مخزن: [<span dir="ltr">worker.js</span>](../../relay/deploy/cloudflare/worker.js) و [<span dir="ltr">wrangler.toml</span>](../../relay/deploy/cloudflare/wrangler.toml).

مرجع کلادفلر: [ذخیره <span dir="ltr">secret</span> در <span dir="ltr">Worker</span>](https://developers.cloudflare.com/workers/configuration/secrets/).

</div>
