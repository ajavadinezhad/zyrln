# Cloudflare Worker exit

**[Setup guide](../../docs/cloudflare-setup.md)** · [فارسی](../../docs/fa/cloudflare-setup.md)

```bash
cd relay/deploy/cloudflare && wrangler login && wrangler deploy
```

Before deploy: set `WORKER_HOST` in `worker.js`, `name` and `ZYRLN_RELAY_KEY` in `wrangler.toml`, then match `EXIT_RELAY_URL` / `EXIT_RELAY_KEY` in `Code.gs`.
