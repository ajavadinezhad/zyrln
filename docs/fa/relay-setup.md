<div dir="rtl">

# ساخت رله شخصی

[بازگشت به راهنمای برنامه](guide.md) · [<span dir="ltr">English</span>](../relay-setup.md)

اگر کانفیگ آماده دارید، به این مراحل نیازی ندارید. برای ساخت رله، یک حساب گوگل و یکی از این دو خروجی را لازم دارید: <span dir="ltr">Cloudflare Worker</span> یا <span dir="ltr">VPS</span> لینوکسی. برای ساخت کانفیگ اندروید لازم نیست برنامه دسکتاپ زیرلن را نصب کنید.

ترتیب کار: **انتخاب خروجی ← راه‌اندازی خروجی ← انتشار پل گوگل ← ساخت کانفیگ برنامه**.

## ۱. دو کلید جداگانه بسازید

با یک برنامه مدیریت رمز، دو رمز طولانی و تصادفی بسازید: **کلید برنامه** و **کلید خروجی**. هر دو را خصوصی نگه دارید.

| مقدار | محل استفاده |
|---|---|
| کلید برنامه | قسمت کلید در برنامه و <code dir="ltr">AUTH_KEY</code> در <span dir="ltr">Apps Script</span> |
| کلید خروجی | <code dir="ltr">EXIT_RELAY_KEY</code> در <span dir="ltr">Apps Script</span> و <code dir="ltr">ZYRLN_RELAY_KEY</code> روی خروجی |

برنامه به کلید خروجی نیاز ندارد. در مرحله ۲ **آدرس خروجی** و در مرحله ۳ **آدرس <span dir="ltr">Apps Script</span>** را می‌گیرید؛ این دو آدرس متفاوت‌اند.

## ۲. یک خروجی راه‌اندازی کنید

### <span dir="ltr">Cloudflare Worker</span>

[راهنمای <span dir="ltr">Cloudflare</span>](cloudflare-setup.md) را انجام دهید، سپس با آدرس <span dir="ltr">Worker</span> و کلید خروجی به مرحله ۳ برگردید.

تونل <span dir="ltr">TCP</span> کلادفلر به مقصدهایی روی محدوده <span dir="ltr">IP</span> خود <span dir="ltr">Cloudflare</span> وصل نمی‌شود. اگر این مقصدها را نیاز دارید، <span dir="ltr">VPS</span> انتخاب کنید. دسترسی و محدودیت مصرف به حساب کلادفلر شما بستگی دارد.

### <span dir="ltr">VPS</span> لینوکسی

سرور لینوکسی <span dir="ltr">amd64</span> یا <span dir="ltr">arm64</span>، دسترسی <span dir="ltr">SSH</span> و کاربر <span dir="ltr">root</span> یا دارای <span dir="ltr">sudo</span> لازم است. این مراحل را از دستگاهی با <span dir="ltr">Bash</span>، <span dir="ltr">SSH</span> و <span dir="ltr">SCP</span> انجام دهید؛ در ویندوز می‌توانید از <span dir="ltr">WSL</span> یا <span dir="ltr">Git Bash</span> استفاده کنید.

1. فایل <code dir="ltr">zyrln-VERSION-vps.zip</code> را از [انتشارها](https://github.com/ajavadinezhad/zyrln/releases) بگیرید و از حالت فشرده خارج کنید.
2. در پوشه استخراج‌شده ترمینال باز کنید. کلید خروجی و مشخصات <span dir="ltr">SSH</span> سرورتان را جایگزین کنید:

   <div dir="ltr">

   ```bash
   chmod +x install-vps-relay.sh
   ZYRLN_RELAY_KEY='YOUR_EXIT_KEY' ./install-vps-relay.sh user@YOUR_VPS_IP
   ```

   </div>

3. پورت <span dir="ltr">TCP</span> شماره <code dir="ltr">8787</code> را در فایروال ارائه‌دهنده <span dir="ltr">VPS</span> باز کنید. نصب‌کننده در صورت وجود <span dir="ltr">UFW</span> تلاش می‌کند آن را هم تنظیم کند.
4. بررسی کنید:

   <div dir="ltr">

   ```bash
   curl http://YOUR_VPS_IP:8787/healthz
   ```

   </div>

   پاسخ مورد انتظار <code dir="ltr">ok</code> است. این فقط در دسترس بودن سرور را نشان می‌دهد، نه کار کردن کل زنجیره گوگل.

کلید خروجی را نگه دارید. آدرس رله <code dir="ltr">http://YOUR_VPS_IP:8787/relay</code> و آدرس تونل <code dir="ltr">http://YOUR_VPS_IP:8787/tunnel</code> است.

این آدرس‌های نمونه <span dir="ltr">HTTP</span> هستند و مسیر گوگل تا <span dir="ltr">VPS</span>، از جمله کلید خروجی، در این حالت <span dir="ltr">TLS</span> ندارد. برای محافظت این بخش از مسیر، یک <span dir="ltr">reverse proxy</span> با <span dir="ltr">HTTPS</span> راه‌اندازی کنید و هر دو آدرس را مطابق آن تغییر دهید.

## ۳. پل <span dir="ltr">Google Apps Script</span> را منتشر کنید

1. در [<span dir="ltr">Google Apps Script</span>](https://script.google.com) یک **<span dir="ltr">New project</span>** بسازید.
2. تمام محتوای [<span dir="ltr">Code.gs</span>](../../relay/deploy/apps-script/Code.gs) را جایگزین متن ویرایشگر کنید.
3. چهار مقدار ابتدای فایل را با **یکی** از نمونه‌های زیر تنظیم کنید. باقی فایل را نگه دارید.

**برای <span dir="ltr">Cloudflare</span>:**

<div dir="ltr">

```js
const AUTH_KEY = "YOUR_APP_KEY";
const EXIT_RELAY_URL = "https://YOUR_WORKER.YOUR_SUBDOMAIN.workers.dev";
const EXIT_TUNNEL_URL = "";
const EXIT_RELAY_KEY = "YOUR_EXIT_KEY";
```

</div>

آدرس پایه <span dir="ltr">Worker</span> را بدون <code dir="ltr">/relay</code> وارد کنید. خالی بودن آدرس تونل باعث می‌شود اسکریپت مسیر <code dir="ltr">/tunnel</code> را خودش بسازد.

**برای <span dir="ltr">VPS</span>:**

<div dir="ltr">

```js
const AUTH_KEY = "YOUR_APP_KEY";
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";
const EXIT_TUNNEL_URL = "http://YOUR_VPS_IP:8787/tunnel";
const EXIT_RELAY_KEY = "YOUR_EXIT_KEY";
```

</div>

4. مسیر **<span dir="ltr">Deploy → New deployment → Web app</span>** را باز کنید. **<span dir="ltr">Execute as: Me</span>** و **<span dir="ltr">Who has access: Anyone</span>** را انتخاب کنید. مجوزهای گوگل را برای پروژه خودتان بررسی و تأیید کنید. اگر حساب شما گزینه دسترسی عمومی ندارد، با این تنظیمات قابل استفاده نیست؛ با مدیر حسابتان بررسی کنید.
5. منتشر کنید و آدرس وب‌اپ با انتهای <code dir="ltr">/exec</code> را بردارید.

**بعد از تغییر اسکریپت منتشرشده:** به **<span dir="ltr">Deploy → Manage deployments → Edit</span> (مداد) → <span dir="ltr">New version → Deploy</span>** بروید. ذخیره متن در ویرایشگر به‌تنهایی نسخه منتشرشده را تغییر نمی‌دهد.

## ۴. کانفیگ برنامه را بسازید

در این قالب، **آدرس <span dir="ltr">Apps Script</span>** و **کلید برنامه** را جایگزین کنید:

<div dir="ltr">

```json
{"url":"https://script.google.com/macros/s/YOUR_DEPLOYMENT_ID/exec","key":"YOUR_APP_KEY"}
```

</div>

تمام <span dir="ltr">JSON</span> را کپی کنید و [مراحل اندروید](guide.md#اندروید) را انجام دهید؛ در دسکتاپ می‌توانید دو مقدار را دستی وارد کنید. آدرس <span dir="ltr">Worker</span> یا <span dir="ltr">VPS</span> و کلید خروجی را در کانفیگ برنامه نگذارید.

## بررسی قبل از اشتراک‌گذاری

1. دسترسی <span dir="ltr">Apps Script</span> روی **<span dir="ltr">Anyone</span>** باشد و آخرین نسخه را دیپلوی کرده باشید.
2. کلید برنامه با <code dir="ltr">AUTH_KEY</code> یکی باشد.
3. کلید خروجی در <span dir="ltr">Apps Script</span> و <span dir="ltr">Worker</span> یا <span dir="ltr">VPS</span> یکسان باشد.
4. زیرلن را وصل کنید و یک سایت باز کنید؛ <span dir="ltr">Ping</span> به‌تنهایی کافی نیست.
5. فقط کانفیگ برنامه را خصوصی به اشتراک بگذارید، نه کلید خروجی یا اطلاعات ورود حساب گوگل.

سهمیه درخواست‌های گوگل بین اسکریپت‌های متعلق به یک حساب مشترک است. ساخت دیپلوی بیشتر روی همان حساب سهمیه جدید نمی‌دهد. حتی با تنظیم درست، محدودیت سهمیه یا اختلال سرویس می‌تواند روی اتصال اثر بگذارد.

</div>
