package deploy

import _ "embed"

//go:embed apps-script/Code.gs
var AppsScript string

//go:embed cloudflare/worker.js
var Worker string

//go:embed cloudflare/wrangler.toml
var Wrangler string
