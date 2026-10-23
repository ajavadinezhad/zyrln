# Create your own relay

[Back to app setup](../README.md) · [فارسی](fa/relay-setup.md)

Use this guide only if you do not already have a working config. You need a Google account and one exit: a Cloudflare Worker or a Linux VPS. A desktop Zyrln installation is not required to create an Android config.

The order is: **choose exit → deploy exit → publish Google bridge → create app config**.

## 1. Prepare two separate keys

Generate two long random secrets with a password manager. Call them **app key** and **exit key**; keep both private.

| Value | Where it goes |
|---|---|
| App key | App's key field and `AUTH_KEY` in Apps Script |
| Exit key | `EXIT_RELAY_KEY` in Apps Script and `ZYRLN_RELAY_KEY` on the exit |

The app never needs the exit key. You will also collect an **exit URL** in step 2 and an **Apps Script URL** in step 3. They are different addresses.

## 2. Deploy one exit

### Cloudflare Worker

Follow [Cloudflare setup](cloudflare-setup.md), then return to step 3 with your Worker URL and exit key.

Cloudflare's TCP tunnel cannot connect to destinations on Cloudflare IP ranges. Use a VPS if you need those destinations. Availability and usage limits depend on your Cloudflare account.

### Linux VPS

You need an amd64 or arm64 Linux VPS, SSH access, and root or sudo privileges. Run these steps from a machine with a Bash shell, SSH, and SCP; Windows users can use WSL or Git Bash.

1. Download `zyrln-VERSION-vps.zip` from [Releases](https://github.com/ajavadinezhad/zyrln/releases) and extract it.
2. Open a terminal in the extracted folder. Replace both placeholders and install:

   ```bash
   chmod +x install-vps-relay.sh
   ZYRLN_RELAY_KEY='YOUR_EXIT_KEY' ./install-vps-relay.sh user@YOUR_VPS_IP
   ```

3. Ensure inbound TCP port `8787` is allowed in the VPS provider's firewall. The installer also attempts to configure UFW when available.
4. Check the server:

   ```bash
   curl http://YOUR_VPS_IP:8787/healthz
   ```

   Expected response: `ok`. This confirms the service is reachable, not that the full Google relay chain works.

Keep your exit key. Your relay URL is `http://YOUR_VPS_IP:8787/relay` and tunnel URL is `http://YOUR_VPS_IP:8787/tunnel`.

These example URLs use HTTP. If you use them, the Google-to-VPS leg, including its exit key, is not protected by TLS. Use an HTTPS reverse proxy for transport protection on that leg and set both URLs accordingly.

## 3. Publish the Google Apps Script bridge

1. Open [Google Apps Script](https://script.google.com) and create a **New project**.
2. Replace the editor contents with the complete [Code.gs](../relay/deploy/apps-script/Code.gs) file.
3. Replace the four constants at the top using **one** of the following examples. Keep the rest of the script.

**For Cloudflare:**

```js
const AUTH_KEY = "YOUR_APP_KEY";
const EXIT_RELAY_URL = "https://YOUR_WORKER.YOUR_SUBDOMAIN.workers.dev";
const EXIT_TUNNEL_URL = "";
const EXIT_RELAY_KEY = "YOUR_EXIT_KEY";
```

Use the Worker base URL, without `/relay`. An empty tunnel URL lets the script derive `/tunnel`.

**For a VPS:**

```js
const AUTH_KEY = "YOUR_APP_KEY";
const EXIT_RELAY_URL = "http://YOUR_VPS_IP:8787/relay";
const EXIT_TUNNEL_URL = "http://YOUR_VPS_IP:8787/tunnel";
const EXIT_RELAY_KEY = "YOUR_EXIT_KEY";
```

4. Choose **Deploy → New deployment → Web app**. Set **Execute as: Me**, and **Who has access: Anyone**. Review and complete Google's authorization prompts for your own project. If your account does not offer public access, this deployment cannot be used with these instructions; check with your account administrator.
5. Deploy and copy the web app URL ending in `/exec`.

**After editing a published script:** use **Deploy → Manage deployments → Edit (pencil) → New version → Deploy**. Saving in the editor alone does not update the deployed version.

## 4. Create your app config

Fill in this template with the **Apps Script URL** and **app key**:

```json
{"url":"https://script.google.com/macros/s/YOUR_DEPLOYMENT_ID/exec","key":"YOUR_APP_KEY"}
```

Copy the full JSON and follow [Android connection steps](../README.md#android), or enter the two values manually on desktop. Do not put the Worker/VPS URL or exit key in the app config.

## Check before sharing

1. Check that Apps Script access is **Anyone** and your latest version is deployed.
2. Check the app key matches `AUTH_KEY`.
3. Check the exit key matches on **both** Apps Script and the Worker/VPS.
4. Connect Zyrln and load a website. Ping alone is not a browsing test.
5. Share only the app config privately, not your exit key or Google account credentials.

Google fetch quotas are shared by scripts owned by the same account. More deployments on one account do not create extra account quota. Quotas and service availability can limit access even when your setup is correct.
