const AUTH_KEY = "CHANGE_ME_TO_A_LONG_RANDOM_SECRET";
const EXIT_RELAY_URL = "https://CHANGE_ME_EXIT_RELAY_URL"; // Cloudflare: worker URL, no /relay. VPS: http://IP:8787/relay
const EXIT_TUNNEL_URL = ""; // Cloudflare: leave empty (derives /tunnel). VPS: http://YOUR_VPS_IP:8787/tunnel
const EXIT_RELAY_KEY = ""; // same value as ZYRLN_RELAY_KEY on VPS or Cloudflare Worker

const SKIP_HEADERS = {
  host: true,
  connection: true,
  "content-length": true,
  "transfer-encoding": true,
  "proxy-connection": true,
  "proxy-authorization": true,
};

function doPost(e) {
  try {
    if (!e || !e.postData || !e.postData.contents) {
      return json_({ e: "bad request" });
    }
    const req = JSON.parse(e.postData.contents);
    if (req.k !== AUTH_KEY) {
      return json_({ e: "unauthorized" });
    }
    const compress = !!req.gz;
    if (req.t && typeof req.t === "object" && !Array.isArray(req.t) && req.t.op === "ping") {
      return json_({ ok: true });
    }
    if (req.t && typeof req.t === "object" && !Array.isArray(req.t)) {
      return doTunnel_(req);
    }
    if (Array.isArray(req.tb) && req.tb.length > 0) {
      return doTunnel_(req);
    }
    if (Array.isArray(req.q)) {
      return doBatch_(req.q, compress);
    }
    return doSingle_(req, compress);
  } catch (err) {
    return json_({ e: String(err) });
  }
}

function tunnelURL_() {
  if (EXIT_TUNNEL_URL) {
    return EXIT_TUNNEL_URL;
  }
  const relay = (EXIT_RELAY_URL || "").replace(/\/+$/, "");
  if (/\/relay$/i.test(relay)) {
    return relay.replace(/\/relay$/i, "/tunnel");
  }
  if (relay) {
    return relay + "/tunnel";
  }
  return "/tunnel";
}

function doTunnel_(req) {
  const payload = Array.isArray(req.tb) ? JSON.stringify({ ops: req.tb }) : JSON.stringify(req.t);
  const resp = UrlFetchApp.fetch(tunnelURL_(), {
    method: "post",
    contentType: "application/json",
    payload: payload,
    muteHttpExceptions: true,
    followRedirects: true,
    headers: exitRelayHeaders_(),
  });

  try {
    return json_(JSON.parse(resp.getContentText()));
  } catch (err) {
    return json_({ ok: false, e: "invalid tunnel response", raw: resp.getContentText() });
  }
}

function doSingle_(req, compress) {
  if (!isValidRelayRequest_(req)) {
    return json_({ e: "bad url" }, compress);
  }

  const resp = UrlFetchApp.fetch(EXIT_RELAY_URL, {
    method: "post",
    contentType: "application/json",
    payload: JSON.stringify(buildWorkerPayload_(req)),
    muteHttpExceptions: true,
    followRedirects: true,
    headers: exitRelayHeaders_(),
  });

  try {
    return json_(JSON.parse(resp.getContentText()), compress);
  } catch (err) {
    return json_({ e: "invalid worker response", raw: resp.getContentText() }, compress);
  }
}

function doBatch_(items, compress) {
  const fetches = [];
  const errors = {};

  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (!isValidRelayRequest_(item)) {
      errors[i] = "bad url";
      continue;
    }
    fetches.push({
      index: i,
      request: {
        url: EXIT_RELAY_URL,
        method: "post",
        contentType: "application/json",
        payload: JSON.stringify(buildWorkerPayload_(item)),
        muteHttpExceptions: true,
        followRedirects: true,
        headers: exitRelayHeaders_(),
      },
    });
  }

  const responses = fetches.length ? UrlFetchApp.fetchAll(fetches.map((x) => x.request)) : [];
  const results = [];
  let responseIndex = 0;

  for (let i = 0; i < items.length; i++) {
    if (Object.prototype.hasOwnProperty.call(errors, i)) {
      results.push({ e: errors[i] });
      continue;
    }

    const resp = responses[responseIndex++];
    try {
      results.push(JSON.parse(resp.getContentText()));
    } catch (err) {
      results.push({ e: "invalid worker response", raw: resp.getContentText() });
    }
  }

  return json_({ q: results }, compress);
}

function exitRelayHeaders_() {
  if (!EXIT_RELAY_KEY) {
    return {};
  }
  return { "X-Relay-Key": EXIT_RELAY_KEY };
}

function isValidRelayRequest_(req) {
  return !!req.u && typeof req.u === "string" && !!req.u.match(/^https?:\/\//i);
}

function buildWorkerPayload_(req) {
  const headers = {};
  if (req.h && typeof req.h === "object") {
    for (const key in req.h) {
      if (Object.prototype.hasOwnProperty.call(req.h, key) && !SKIP_HEADERS[key.toLowerCase()]) {
        headers[key] = req.h[key];
      }
    }
  }

  return {
    u: req.u,
    m: (req.m || "GET").toUpperCase(),
    h: headers,
    b: req.b || null,
    ct: req.ct || null,
    r: req.r !== false,
  };
}

function doGet() {
  return HtmlService.createHtmlOutput("Relay Active");
}

function json_(obj, compress) {
  const text = JSON.stringify(obj);
  if (compress) {
    try {
      const blob = Utilities.newBlob(text, 'application/json');
      const gz = Utilities.gzip(blob);
      const b64 = Utilities.base64Encode(gz.getBytes());
      return ContentService
        .createTextOutput(JSON.stringify({ z: b64 }))
        .setMimeType(ContentService.MimeType.JSON);
    } catch (_) {
      // fall through to uncompressed
    }
  }
  return ContentService
    .createTextOutput(text)
    .setMimeType(ContentService.MimeType.JSON);
}
