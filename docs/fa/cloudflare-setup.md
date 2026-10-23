# راه‌اندازی خروجی Cloudflare

[بازگشت به ساخت رله](relay-setup.md) · [English](../cloudflare-setup.md)

این صفحه مرحله ۲ راهنمای ساخت رله است. حساب Cloudflare، برنامه Node.js/npm و ترمینال لازم دارید. اینجا از **کلید خروجی** استفاده می‌کنید؛ کلید برنامه برای Apps Script و زیرلن است.

**قبل از انتخاب:** تونل TCP کلادفلر به محدوده IP خود Cloudflare وصل نمی‌شود. ممکن است بعضی سایت‌ها روی اندروید باز نشوند، حتی وقتی سایت‌های دیگر کار می‌کنند. رله HTTP دسکتاپ مسیر متفاوتی دارد و این محدودیت به معنی شکست همه درخواست‌های دسکتاپ به سایت‌های کلادفلری نیست. VPS این محدودیت مشخص TCP را ندارد. محدودیت مصرف حساب همچنان اعمال می‌شود.

## ۱. فایل‌های استقرار را بگیرید

فایل **Source code** مخزن را از [صفحه انتشار](https://github.com/ajavadinezhad/zyrln/releases) بگیرید و استخراج کنید، یا مخزن را clone کنید. ترمینال را در پوشه `relay/deploy/cloudflare` باز کنید.

باید هر دو فایل `worker.js` و `wrangler.toml` وجود داشته باشند. برای دیپلوی از Wrangler استفاده کنید تا اتصال Durable Object موردنیاز اندروید ساخته شود. چسباندن JavaScript به‌تنهایی در داشبورد این اتصال را تنظیم نمی‌کند.

## ۲. نام و آدرس Worker را مشخص کنید

1. در `wrangler.toml` مقدار `name = "mine"` را به نام دلخواه، مثلاً `zyrln-exit`، تغییر دهید.
2. زیردامنه Workers حساب خود را در داشبورد Cloudflare پیدا کنید. همراه با نام انتخابی، آدرس می‌شود `zyrln-exit.YOUR_SUBDOMAIN.workers.dev`.
3. در `worker.js` مقدار `CHANGE_ME_WORKER_HOST` را با همین نام میزبان، بدون `https://` یا مسیر، جایگزین کنید. این تنظیم جلوی درخواست رله به خودش را می‌گیرد.

بخش‌های موجود مربوط به `TUNNEL_HUB` و migration را در `wrangler.toml` نگه دارید.

## ۳. وارد حساب شوید، کلید خروجی را ذخیره و دیپلوی کنید

در همان پوشه اجرا کنید:

```bash
npx wrangler login
npx wrangler secret put ZYRLN_RELAY_KEY
npx wrangler deploy
```

وقتی مقدار secret را خواست، **کلید خروجی** را وارد کنید. اگر قبل از ذخیره کلید پرسید Worker را بسازد، ساخت آن را تأیید و بعد دیپلوی را ادامه دهید. کلید واقعی را در فایل `wrangler.toml` که در گیت ثبت می‌شود نگذارید.

در خروجی دیپلوی باید آدرس Worker و اتصال `TUNNEL_HUB` دیده شود. اگر آدرس با مقداری که در `WORKER_HOST` نوشته‌اید فرق دارد، مقدار را اصلاح و دوباره دیپلوی کنید.

## ۴. به راه‌اندازی Google Apps Script برگردید

این دو مقدار را نگه دارید:

| مقدار | نمونه |
|---|---|
| آدرس پایه Worker | `https://zyrln-exit.YOUR_SUBDOMAIN.workers.dev` |
| کلید خروجی | همان secret که ذخیره کردید |

به [راهنمای ساخت رله، مرحله ۳](relay-setup.md) برگردید. آدرس پایه Worker را بدون `/relay` وارد کنید، `EXIT_TUNNEL_URL` را خالی بگذارید و `EXIT_RELAY_KEY` را برابر همان کلید خروجی تنظیم کنید.

## اگر دیپلوی یا اتصال انجام نشد

- **Wrangler اجرا نمی‌شود:** نصب بودن Node.js/npm و پوشه فعلی ترمینال را بررسی کنید.
- **خطای حساب، پلن یا سهمیه:** پیام مشخص را در حساب Cloudflare بررسی کنید؛ دیپلوی برای همه حساب‌ها تضمین‌شده نیست.
- **خطای اتصال `TUNNEL_HUB`:** از پوشه‌ای که تنظیمات binding و migration را دارد، دوباره `npx wrangler deploy` را اجرا کنید.
- **خطای Unauthorized:** مقدار `EXIT_RELAY_KEY` در Apps Script باید با secret روی Worker یکی باشد. خود برنامه از کلید برنامه استفاده می‌کند.
- **فقط بعضی مقصدها باز نمی‌شوند:** پروفایل VPS را امتحان کنید؛ تلاش دوباره محدودیت مقصد TCP کلادفلر را برطرف نمی‌کند.

فایل‌های مخزن: [worker.js](../../relay/deploy/cloudflare/worker.js) و [wrangler.toml](../../relay/deploy/cloudflare/wrangler.toml).

مرجع کلادفلر: [ذخیره secret در Worker](https://developers.cloudflare.com/workers/configuration/secrets/).
