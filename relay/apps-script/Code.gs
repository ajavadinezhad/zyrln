const AUTH_KEY = "CHANGE_ME_TO_A_LONG_RANDOM_SECRET";
const EXIT_RELAY_URL = "https://CHANGE_ME_EXIT_RELAY_URL/relay";
const EXIT_RELAY_KEY = "";

const SKIP_HEADERS = {
  host: true,
  connection: true,
  "content-length": true,
  "transfer-encoding": true,
  "proxy-connection": true,
  "proxy-authorization": true,
  "x-exit-relay-url": true
};

function doPost(e) {
  try {
    const dynamicExitUrl = e?.parameter?.exitUrl || e?.headers?.["X-Exit-Relay-Url"] || EXIT_RELAY_URL;
    
    if (e && e.headers && e.headers["X-Exit-Relay-Url"]) {
      delete e.headers["X-Exit-Relay-Url"];
    }

    const req = JSON.parse(e.postData.contents);
    if (req.k !== AUTH_KEY) {
      return json_({ e: "unauthorized" });
    }
    if (Array.isArray(req.q)) {
      return doBatch_(req.q);
    }
    return doSingle_(req, dynamicExitUrl);
  } catch (err) {
    return json_({ e: String(err) });
  }
}

function doSingle_(req, exitRelayUrl) {
  if (!isValidRelayRequest_(req)) {
    return json_({ e: "bad url" });
  }

  const resp = UrlFetchApp.fetch(exitRelayUrl, {
    method: "post",
    contentType: "application/json",
    payload: JSON.stringify(buildWorkerPayload_(req)),
    muteHttpExceptions: true,
    followRedirects: true,
    headers: exitRelayHeaders_(),
  });

  try {
    return json_(JSON.parse(resp.getContentText()));
  } catch (err) {
    return json_({ e: "invalid worker response", raw: resp.getContentText() });
  }
}

function doBatch_(items) {
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

  return json_({ q: results });
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

function json_(obj) {
  return ContentService
    .createTextOutput(JSON.stringify(obj))
    .setMimeType(ContentService.MimeType.JSON);
}
