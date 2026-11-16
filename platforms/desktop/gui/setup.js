"use strict";
const $ = (id) => document.getElementById(id);
function applyTheme() {
    const light = localStorage.getItem("zyrln_theme") === "light";
    document.body.classList.toggle("light-mode", light);
    document.body.classList.toggle("dark-mode", !light);
}
applyTheme();
window.addEventListener("storage", event => { if (event.key === "zyrln_theme" || event.key === null) applyTheme(); });

const fa = localStorage.getItem("zyrln_lang") === "fa";
const tr = (en, persian) => fa ? persian : en;
if (fa) {
    document.documentElement.lang = "fa";
    document.documentElement.dir = "rtl";
    document.body.classList.add("rtl");
    const labels = {
        back: "بازگشت به زیرلن", title: "ساخت اولین کانفیگ",
        intro: "محل اجرای رله را انتخاب کنید. هر دو گزینه به حساب گوگل نیاز دارند. تا ذخیره کانفیگ این صفحه را باز نگه دارید.",
        choose: "۱. انتخاب نوع رله", "choose-worker": "کلادفلر", "choose-vps": "سرور مجازی دارم",
        "worker-title": "راه‌اندازی کلادفلر", "host-label": "نام میزبان Worker",
        "worker-requires": 'به حساب کلادفلر و <a href="https://nodejs.org/" target="_blank" rel="noopener noreferrer">Node.js</a> به همراه npm نیاز دارید.',
        "vps-title": "استفاده از سرور مجازی", "existing-label": "رله زیرلن از قبل روی سرورم نصب است",
        "vps-url-label": "آدرس رله روی سرور",
        "vps-url-help": 'آدرس عمومی رله را که به <code dir="ltr">/relay</code> ختم می‌شود وارد کنید. برای نصب جدید، اگر این کادر خالی بماند از آدرس SSH و پورت <code dir="ltr">8787</code> استفاده می‌شود. بهتر است از HTTPS استفاده کنید؛ در HTTP کلید خروجی رمزگذاری نمی‌شود.',
        "vps-ssh-label": "نام کاربری و آدرس SSH سرور", "vps-key-label": "کلید خروجی فعلی سرور",
        "vps-prepare": "ادامه",
        "vps-download": 'فایل <a href="https://github.com/ajavadinezhad/zyrln/releases" target="_blank" rel="noopener noreferrer">ZIP مخصوص سرور</a> را دانلود و استخراج کنید. در همان پوشه ترمینال Bash را باز کنید. در ویندوز از WSL یا Git Bash استفاده کنید.',
        "vps-run": "دستور زیر را در ترمینال اجرا کنید. وقتی کلید خواست، کلید خروجی را کپی و وارد کنید. کاربر SSH باید دسترسی root یا sudo داشته باشد.",
        "vps-port": 'پورت TCP شماره <code dir="ltr">8787</code> را در فایروال ارائه‌دهنده باز کنید. اگر HTTPS دارید، مسیرهای رله و تونل را از همان پراکسی عبور دهید.',
        "copy-vps-command": "کپی دستور نصب", "vps-exit-key": "کپی کلید خروجی", "vps-ready": "رله نصب شد",
        "host-help": 'زیردامنه Workers را در <a href="https://dash.cloudflare.com/" target="_blank" rel="noopener noreferrer">داشبورد کلادفلر</a> پیدا کنید و به جای <code dir="ltr">your-subdomain</code> بنویسید. اگر قبلاً از این نام Worker استفاده کرده‌اید، نام دیگری انتخاب کنید.',
        prepare: "دانلود فایل‌های Worker", "deploy-help": "فایل را از حالت فشرده خارج کنید و در همان پوشه ترمینال باز کنید. دستورها را به‌ترتیب اجرا کنید. وقتی دستور دوم کلید خواست، کلید خروجی را کپی و در ترمینال جای‌گذاری کنید. اگر خواست Worker جدید بسازید، تأیید کنید.",
        "exit-key": "کپی کلید خروجی", "worker-ready": "Worker منتشر شد",
        "worker-help": "آدرس واردشده و اتصال TUNNEL_HUB باید در خروجی Wrangler دیده شوند.",
        "google-title": "۲. اتصال گوگل",
        "google-open": 'در <a href="https://script.google.com/" target="_blank" rel="noopener noreferrer">گوگل اپس اسکریپت</a> پروژه جدید بسازید.',
        "google-paste": "اسکریپت آماده را کپی و جایگزین تمام متن ویرایشگر گوگل کنید. نیازی به ویرایش نیست.",
        "google-deploy": 'مسیر <b dir="ltr">Deploy → New deployment → Web app</b> را باز کنید. گزینه‌های <b dir="ltr">Execute as: Me</b> و <b dir="ltr">Who has access: Anyone</b> را انتخاب و پروژه را منتشر کنید.',
        "google-link": 'آدرس وب‌اپ را که به <code dir="ltr">/exec</code> ختم می‌شود، در مرحله بعد وارد کنید.',
        script: "کپی اسکریپت آماده", "verify-title": "۳. آزمایش و اشتراک‌گذاری",
        "endpoint-label": "آدرس وب‌اپ گوگل", test: "آزمایش اتصال",
        success: "درخواست وب موفق بود و تونل اندروید باز شد. کانفیگ را کپی کنید و فقط به افراد مورد اعتماد بدهید.",
        copy: "کپی کانفیگ", save: "ذخیره در پروفایل‌های دسکتاپ", "import-help": "در اندروید، متن کانفیگ را کپی کنید، زیرلن را باز کنید و وارد کردن از کلیپ‌بورد و سپس اتصال را بزنید."
    };
    for (const [id, label] of Object.entries(labels)) $(id).innerHTML = label;
}

let backend = "cloudflare";
let setup = null;
let verified = null;
const key = () => Array.from(crypto.getRandomValues(new Uint8Array(32)), b => b.toString(16).padStart(2, "0")).join("");
const say = (message) => { $("manual-copy").hidden = true; $("manual-copy").value = ""; $("status").textContent = message; };
async function copy(value) {
    try {
        await navigator.clipboard.writeText(value);
        say(tr("Copied.", "کپی شد."));
    } catch {
        say(tr("Select and copy the text below.", "متن زیر را انتخاب و کپی کنید."));
        $("manual-copy").hidden = false;
        $("manual-copy").value = value;
        $("manual-copy").select();
    }
}
const post = (path, data) => fetch(path, {
    method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(data)
});

function resetSetup() {
    setup = null;
    verified = null;
    for (const id of ["deploy", "vps-install", "google", "verify", "result"]) $(id).hidden = true;
    say("");
}
for (const choice of document.querySelectorAll('input[name="backend"]')) {
    choice.addEventListener("change", () => {
        backend = choice.value;
        $("worker").hidden = backend !== "cloudflare";
        $("vps").hidden = backend !== "vps";
        resetSetup();
    });
}
$("host").addEventListener("input", resetSetup);
for (const id of ["vps-url", "vps-ssh", "vps-existing-key"]) $(id).addEventListener("input", resetSetup);
$("existing-relay").addEventListener("change", () => {
    const existing = $("existing-relay").checked;
    $("vps-new-fields").hidden = existing;
    $("vps-ssh").disabled = existing;
    $("vps-existing-fields").hidden = !existing;
    $("vps-existing-key").disabled = !existing;
    resetSetup();
});

function openGoogle() {
    if (!setup) return;
    $("google").hidden = $("verify").hidden = false;
    $("google").scrollIntoView();
}
function relayURL(value) {
    const url = new URL(value);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password ||
        !url.hostname || url.href.includes("?") || url.href.includes("#") || url.pathname !== "/relay") throw Error();
    return url.href;
}

$("vps-form").addEventListener("submit", e => {
    e.preventDefault();
    if (!window.isSecureContext) {
        say(tr("Open Zyrln on this computer at its local address to create keys.", "برای ساخت کلید، زیرلن را با آدرس محلی همین کامپیوتر باز کنید."));
        return;
    }
    const existing = $("existing-relay").checked;
    const ssh = $("vps-ssh").value.trim();
    if (!existing && !/^[A-Za-z_][A-Za-z0-9_-]*@[A-Za-z0-9][A-Za-z0-9.-]*$/.test(ssh)) {
        say(tr("Enter your SSH login as user@server, without extra options.", "آدرس SSH را به شکل user@server بدون گزینه‌های اضافی وارد کنید."));
        return;
    }
    let exitURL;
    try {
        exitURL = relayURL($("vps-url").value.trim() || (existing ? "" : "http://" + ssh.split("@")[1] + ":8787/relay"));
    } catch {
        say(tr("Use a public HTTP or HTTPS address ending in /relay.", "آدرس عمومی HTTP یا HTTPS را با انتهای /relay وارد کنید."));
        return;
    }
    const existingKey = $("vps-existing-key").value;
    if (existing && !existingKey.trim()) {
        say(tr("Enter the exit key already set on your VPS.", "کلید خروجی فعلی سرور را وارد کنید."));
        return;
    }
    if (!setup) setup = {exitURL, appKey: key(), exitKey: existing ? existingKey : key(), backend: "vps"};
    if (existing) {
        openGoogle();
    } else {
        $("vps-command").textContent = [
            '(',
            '  read -rsp "Exit key: " zyrln_setup_key || exit 1',
            '  printf "\\n"',
            '  if [[ ! $zyrln_setup_key =~ ^[a-f0-9]{64}$ ]]; then',
            '    echo "Paste the 64-character exit key from the wizard." >&2',
            '    exit 1',
            '  fi',
            '  ZYRLN_RELAY_KEY="$zyrln_setup_key" bash ./install-vps-relay.sh ' + ssh,
            ')'
        ].join("\n");
        $("vps-install").hidden = false;
        $("vps-install").scrollIntoView({block: "nearest"});
    }
    say("");
});
$("copy-vps-command").onclick = () => copy($("vps-command").textContent);
$("vps-exit-key").onclick = () => { if (setup) copy(setup.exitKey); };
$("vps-ready").onclick = openGoogle;

$("worker-form").addEventListener("submit", async e => {
    e.preventDefault();
    const host = $("host").value.trim().toLowerCase().replace(/^https:\/\//, "").replace(/\/$/, "");
    if (host.includes(".your-subdomain.")) {
        say(tr("Replace your-subdomain with your Cloudflare Workers subdomain.", "زیردامنه Workers حساب کلادفلر را جایگزین کنید."));
        return;
    }
    if (!window.isSecureContext) {
        say(tr("Open Zyrln on this computer at its local address to create keys.", "برای ساخت کلید، زیرلن را با آدرس محلی همین کامپیوتر باز کنید."));
        return;
    }
    $("prepare").disabled = true;
    $("host").readOnly = true;
    try {
        const response = await post("/api/setup/worker", {host});
        if (!response.ok) throw Error();
        const blob = await response.blob();
        if (backend !== "cloudflare") return;
        const asset = URL.createObjectURL(blob);
        const link = Object.assign(document.createElement("a"), {href: asset, download: "zyrln-worker.zip"});
        link.click();
        setTimeout(() => URL.revokeObjectURL(asset), 1000);
        if (!setup) setup = {exitURL: "https://" + host, appKey: key(), exitKey: key(), backend: "cloudflare"};
        $("deploy").hidden = false;
        say("");
    } catch {
        say(tr("Couldn’t prepare the download. Check the hostname, then try again.", "دانلود آماده نشد. نام میزبان را بررسی و دوباره امتحان کنید."));
    } finally {
        $("prepare").disabled = false;
        $("host").readOnly = false;
    }
});
$("exit-key").onclick = () => { if (setup) copy(setup.exitKey); };
$("worker-ready").onclick = openGoogle;
$("script").onclick = async () => {
    const active = setup;
    if (!active) return;
    try {
        const response = await fetch("/api/setup/script");
        if (!response.ok) throw Error();
        let script = await response.text();
        for (const [name, value] of Object.entries({
            AUTH_KEY: active.appKey, EXIT_RELAY_URL: active.exitURL, EXIT_RELAY_KEY: active.exitKey
        })) {
            const pattern = new RegExp("^const " + name + " = [^;]*;", "m");
            if (!pattern.test(script)) throw Error();
            script = script.replace(pattern, () => "const " + name + " = " + JSON.stringify(value) + ";");
        }
        if (setup === active) await copy(script);
    } catch {
        say(tr("Couldn’t prepare the Google script. Try again.", "آماده‌سازی اسکریپت گوگل ناموفق بود. دوباره امتحان کنید."));
    }
};
$("endpoint").addEventListener("input", () => { verified = null; $("result").hidden = true; say(""); });
$("test-form").addEventListener("submit", async e => {
    e.preventDefault();
    const active = setup;
    const url = $("endpoint").value.trim();
    verified = null;
    $("result").hidden = true;
    if (!active) return;
    if (!/^https:\/\/script\.google\.com\/macros\/s\/[A-Za-z0-9_-]{1,256}\/exec$/.test(url)) {
        say(tr("Use the Google web app URL ending in /exec.", "آدرس وب‌اپ گوگل با انتهای /exec را وارد کنید."));
        return;
    }
    $("test").disabled = true;
    say(tr("Checking the web request and Android tunnel…", "در حال بررسی درخواست وب و تونل اندروید…"));
    const current = () => active === setup && url === $("endpoint").value.trim();
    try {
        const response = await post("/api/setup/check", {url, key: active.appKey});
        if (!response.ok) throw Error();
        const result = await response.json();
        if (!current()) return;
        if (result.ok) {
            verified = {url, key: active.appKey};
            $("result").hidden = false;
            $("save").disabled = false;
            say("");
        } else {
            const messages = {
                quota: tr("Google reported a quota limit. Try again later.", "گوگل محدودیت سهمیه گزارش کرد. بعداً امتحان کنید."),
                keys: tr("A key was rejected. Check that the exit key matches your relay, then redeploy the prepared Google script.", "یکی از کلیدها پذیرفته نشد. مطمئن شوید کلید خروجی با تنظیمات رله یکی است و اسکریپت آماده گوگل را دوباره منتشر کنید."),
                google: tr("Check that the Google web app is published with access set to Anyone.", "بررسی کنید که وب‌اپ گوگل منتشر شده و دسترسی روی Anyone باشد."),
                tunnel: active.backend === "vps"
                    ? tr("The web request worked, but the Android tunnel didn’t open. Check that the VPS serves /tunnel at the same address.", "درخواست وب موفق بود اما تونل اندروید باز نشد. مسیر /tunnel را روی همان آدرس سرور بررسی کنید.")
                    : tr("The web request worked, but the Android tunnel didn’t open. Check the Worker deployment for TUNNEL_HUB.", "درخواست وب موفق بود، اما تونل اندروید باز نشد. اتصال TUNNEL_HUB را در خروجی انتشار Worker بررسی کنید.")
            };
            say(messages[result.issue] || (active.backend === "vps"
                ? tr("Check the relay URL and firewall, and make sure the VPS relay is running.", "آدرس رله و فایروال را بررسی کنید و مطمئن شوید رله روی سرور اجراست.")
                : tr("Check that the Worker is deployed at the hostname shown in step 1, then retry.", "بررسی کنید که Worker روی آدرس مرحله ۱ منتشر شده باشد و دوباره امتحان کنید.")));
        }
    } catch {
        if (current()) say(tr("The check couldn’t finish. Try again.", "آزمایش کامل نشد. دوباره امتحان کنید."));
    } finally {
        $("test").disabled = false;
    }
});
$("copy").onclick = () => { if (verified) copy(JSON.stringify(verified)); };

$("save").onclick = async () => {
    const config = verified;
    if (!config) return;
    $("save").disabled = true;
    try {
        const response = await post("/api/profiles", {config: {"fronted-appscript-url": config.url, "auth-key": config.key}});
        if (!response.ok) throw Error();
        say(tr("Saved. Go back to Zyrln and choose it in the profile list.", "ذخیره شد. به زیرلن برگردید و آن را از فهرست پروفایل‌ها انتخاب کنید."));
    } catch {
        $("save").disabled = false;
        say(tr("Couldn’t save the profile. You can still copy your config.", "پروفایل ذخیره نشد. همچنان می‌توانید کانفیگ را کپی کنید."));
    }
};
