# Set up a Cloudflare exit

[Back to relay setup](relay-setup.md) · [فارسی](fa/cloudflare-setup.md)

This is step 2 of the relay setup guide. You need a Cloudflare account, Node.js/npm, and a terminal. Use your **exit key** here; the app key belongs in Apps Script and Zyrln.

**Before choosing Cloudflare:** its outbound TCP tunnel cannot connect to Cloudflare IP ranges. Affected destinations may fail on Android even when other sites work. Desktop HTTP relay uses a different path; this restriction does not establish that every desktop request to a Cloudflare-hosted site fails. A VPS avoids this particular TCP restriction. Account limits still apply.

## 1. Get the deployment files

Download and extract the repository's **Source code** archive from the [release page](https://github.com/ajavadinezhad/zyrln/releases), or clone the repository. Open a terminal in `relay/deploy/cloudflare` inside that folder.

Both `worker.js` and `wrangler.toml` must be present. Use Wrangler for deployment: it creates the Durable Object binding needed by Android. Pasting just the JavaScript into the dashboard does not configure that binding.

## 2. Choose the Worker name and hostname

1. In `wrangler.toml`, change `name = "mine"` to your chosen Worker name, for example `zyrln-exit`.
2. Find your account's Workers subdomain in the Cloudflare dashboard. Combine it with that name: `zyrln-exit.YOUR_SUBDOMAIN.workers.dev`.
3. In `worker.js`, replace `CHANGE_ME_WORKER_HOST` with that hostname, without `https://` or a path. This setting prevents the relay from fetching itself.

Keep the existing `TUNNEL_HUB` binding and migration entries in `wrangler.toml`.

## 3. Sign in, store the exit key, and deploy

Run these from the same folder:

```bash
npx wrangler login
npx wrangler secret put ZYRLN_RELAY_KEY
npx wrangler deploy
```

When prompted for the secret, paste your **exit key**. If asked to create the Worker before storing its secret, agree to create it, then continue with deployment. Do not put your real key into a committed `wrangler.toml`.

The deployment output should show the Worker URL and the `TUNNEL_HUB` binding. If the URL differs from the hostname you entered in `WORKER_HOST`, correct that value and deploy again.

## 4. Continue with Google Apps Script

Keep these two values:

| Value | Example |
|---|---|
| Worker base URL | `https://zyrln-exit.YOUR_SUBDOMAIN.workers.dev` |
| Exit key | The secret you just stored |

Return to [step 3: publish the Google bridge](relay-setup.md#3-publish-the-google-apps-script-bridge). Use the Worker base URL without `/relay`, leave `EXIT_TUNNEL_URL` empty, and set `EXIT_RELAY_KEY` to the same exit key.

## If deployment or connection fails

- **Wrangler cannot run:** check that Node.js/npm are installed and that the terminal is in the deployment folder.
- **Account, plan, or quota error:** resolve the specific message in your Cloudflare account; deployment is not guaranteed for every account.
- **Missing `TUNNEL_HUB` binding:** deploy from the folder containing the unchanged binding/migration sections with `npx wrangler deploy`.
- **Unauthorized:** check that Apps Script's `EXIT_RELAY_KEY` matches the Worker secret. The app itself uses the separate app key.
- **Only certain destinations fail:** try a VPS profile; repeated retries cannot remove Cloudflare's TCP destination restriction.

Sources in this repository: [worker.js](../relay/deploy/cloudflare/worker.js) and [wrangler.toml](../relay/deploy/cloudflare/wrangler.toml).

Cloudflare reference: [Worker secrets](https://developers.cloudflare.com/workers/configuration/secrets/).
