// Zyrln Cloudflare exit: HTTP relay (/relay) + raw TCP tunnel (/tunnel via Durable Object).
//
// Deploy: cd relay/deploy/cloudflare && wrangler login && wrangler deploy
// See docs/cloudflare-setup.md — set WORKER_HOST and wrangler.toml name first.

import { connect } from "cloudflare:sockets";
import { DurableObject } from "cloudflare:workers";

// Worker hostname only (no https://). Used to block relay self-fetch loops.
const WORKER_HOST = "CHANGE_ME_WORKER_HOST";

const MAX_BODY_BYTES = 32 * 1024 * 1024;
const MAX_RX_WAIT_MS = 3000;
const SESSION_IDLE_MS = 2 * 60 * 1000;
const TUNNEL_HUB_NAME = "zyrln-tunnel-hub";

export default {
  async fetch(request, env) {
    try {
      const url = new URL(request.url);
      if (isTunnelPath(url.pathname)) {
        return forwardTunnel(request, env);
      }
      return handleRelay(request, env);
    } catch (err) {
      return json({ e: String(err) }, 500);
    }
  },
};

function exitRelayKey(env) {
  return String(env.ZYRLN_RELAY_KEY || env.RELAY_KEY || "").trim();
}

function requireExitRelayKey(request, env) {
  const key = exitRelayKey(env);
  if (!key) {
    return null;
  }
  if (request.headers.get("X-Relay-Key") !== key) {
    return { ok: false, e: "unauthorized" };
  }
  return null;
}

function isTunnelPath(pathname) {
  return pathname === "/tunnel" || pathname.endsWith("/tunnel");
}

async function forwardTunnel(request, env) {
  const authErr = requireExitRelayKey(request, env);
  if (authErr) {
    return json(authErr, 401);
  }
  if (!env.TUNNEL_HUB) {
    return json({ ok: false, e: "tunnel not configured (missing TUNNEL_HUB binding)" }, 503);
  }
  const id = env.TUNNEL_HUB.idFromName(TUNNEL_HUB_NAME);
  return env.TUNNEL_HUB.get(id).fetch(request);
}

async function handleRelay(request, env) {
  const authErr = requireExitRelayKey(request, env);
  if (authErr) {
    return json(authErr, 401);
  }
  if (request.headers.get("x-relay-hop") === "1") {
    return json({ e: "loop detected" }, 508);
  }

  const req = await request.json();
  if (!req.u) {
    return json({ e: "missing url" }, 400);
  }

  const targetURL = new URL(req.u);
  if (isSelfFetch(targetURL.hostname)) {
    return json({ e: "self-fetch blocked" }, 400);
  }

  const headers = new Headers();
  if (req.h && typeof req.h === "object") {
    for (const [key, value] of Object.entries(req.h)) {
      headers.set(key, value);
    }
  }
  headers.set("x-relay-hop", "1");

  const options = {
    method: (req.m || "GET").toUpperCase(),
    headers,
    redirect: req.r === false ? "manual" : "follow",
  };

  if (req.b) {
    options.body = Uint8Array.from(atob(req.b), (char) => char.charCodeAt(0));
  }

  const resp = await fetch(targetURL.toString(), options);
  const buffer = await resp.arrayBuffer();
  const bytes = new Uint8Array(buffer);

  return json({
    s: resp.status,
    h: headersToObject(resp.headers),
    b: bytesToBase64(bytes),
  });
}

/** @extends {DurableObject} */
export class TunnelHub extends DurableObject {
  constructor(ctx, env) {
    super(ctx, env);
    /** @type {Map<string, Session>} */
    this.sessions = new Map();
  }

  async fetch(request) {
    if (request.method !== "POST") {
      return tunnelResp({ e: "POST required" }, 405);
    }

    const authErr = requireExitRelayKey(request, this.env);
    if (authErr) {
      return tunnelResp(authErr, 401);
    }

    const raw = await request.text();
    if (raw.length > MAX_BODY_BYTES) {
      return tunnelResp({ e: "body too large" }, 413);
    }

    let body;
    try {
      body = JSON.parse(raw);
    } catch {
      return tunnelResp({ e: "bad json" }, 400);
    }

    if (Array.isArray(body.ops) && body.ops.length > 0) {
      const results = [];
      for (const op of body.ops) {
        const resp = await this.handleOp(op);
        results.push(resp);
        if (!resp.ok && resp.e) {
          break;
        }
      }
      const status = results.some((r) => !r.ok && r.e) ? 502 : 200;
      return tunnelResp({ results }, status);
    }

    const resp = await this.handleOp(body);
    const status = resp.ok || !resp.e ? 200 : 502;
    return tunnelResp(resp, status);
  }

  async handleOp(req) {
    const op = String(req.op || "").toLowerCase();
    const id = String(req.id || "").trim();

    switch (op) {
      case "open":
        return this.opOpen(id, req.target);
      case "tx":
        return this.opTX(id, req.data);
      case "rx":
        return this.opRX(id, req.wait_ms);
      case "close":
        await this.closeSession(id);
        return { ok: true };
      default:
        return { e: "bad request" };
    }
  }

  async opOpen(id, target) {
    if (!id || !validTunnelTarget(target)) {
      return { e: "bad request" };
    }
    if (this.sessions.has(id)) {
      return { e: "session exists" };
    }

    const { host, port } = splitHostPort(target);
    if (isSelfFetch(host)) {
      return { e: "self-connect blocked" };
    }

    try {
      const socket = connect({ hostname: host, port });
      await socket.opened;
      const reader = socket.readable.getReader();
      const writer = socket.writable.getWriter();
      this.sessions.set(id, {
        socket,
        reader,
        writer,
        target: String(target).trim(),
        lastSeen: Date.now(),
      });
      await this.scheduleCleanup();
      return { ok: true };
    } catch (err) {
      return { e: String(err) };
    }
  }

  async opTX(id, dataB64) {
    const sess = this.sessions.get(id);
    if (!sess) {
      return { e: "unknown session" };
    }
    let data;
    try {
      data = base64ToBytes(dataB64 || "");
    } catch {
      return { e: "bad base64" };
    }
    try {
      await sess.writer.write(data);
      sess.lastSeen = Date.now();
      return { ok: true };
    } catch (err) {
      await this.closeSession(id);
      return { e: String(err) };
    }
  }

  async opRX(id, waitMS) {
    const sess = this.sessions.get(id);
    if (!sess) {
      return { e: "unknown session" };
    }

    const waitMs = clampRXWait(waitMS);
    try {
      const chunk = await readWithTimeout(sess, waitMs);
      sess.lastSeen = Date.now();
      if (!chunk || chunk.length === 0) {
        return { ok: true };
      }
      return { ok: true, data: bytesToBase64(chunk) };
    } catch (err) {
      await this.closeSession(id);
      return { e: String(err) };
    }
  }

  async closeSession(id) {
    const sess = this.sessions.get(id);
    if (!sess) {
      return;
    }
    this.sessions.delete(id);
    try {
      await sess.reader.cancel();
    } catch {
      // ignore
    }
    try {
      await sess.writer.close();
    } catch {
      // ignore
    }
    try {
      await sess.socket.close();
    } catch {
      // ignore
    }
  }

  async scheduleCleanup() {
    const existing = await this.ctx.storage.getAlarm();
    if (existing == null) {
      await this.ctx.storage.setAlarm(Date.now() + 60_000);
    }
  }

  async alarm() {
    const cutoff = Date.now() - SESSION_IDLE_MS;
    for (const [id, sess] of this.sessions.entries()) {
      if (sess.lastSeen < cutoff) {
        await this.closeSession(id);
      }
    }
    if (this.sessions.size > 0) {
      await this.ctx.storage.setAlarm(Date.now() + 60_000);
    }
  }
}

async function readWithTimeout(sess, waitMs) {
  const deadline = Date.now() + waitMs;
  while (Date.now() < deadline) {
    const remaining = deadline - Date.now();
    const result = await Promise.race([
      sess.reader.read(),
      delay(remaining).then(() => ({ timedOut: true })),
    ]);
    if (result.timedOut) {
      try {
        await sess.reader.cancel();
      } catch {
        // ignore
      }
      sess.reader = sess.socket.readable.getReader();
      return null;
    }
    if (result.done) {
      return null;
    }
    if (result.value && result.value.byteLength > 0) {
      return result.value;
    }
  }
  return null;
}

function validTunnelTarget(target) {
  const t = String(target || "").trim();
  const i = t.lastIndexOf(":");
  if (i <= 0 || i === t.length - 1) {
    return false;
  }
  const host = t.slice(0, i).replace(/^\[|\]$/g, "");
  const port = t.slice(i + 1);
  return host.length > 0 && /^\d+$/.test(port) && Number(port) > 0 && Number(port) <= 65535;
}

function splitHostPort(target) {
  const t = String(target).trim();
  const i = t.lastIndexOf(":");
  let host = t.slice(0, i);
  const port = Number(t.slice(i + 1));
  if (host.startsWith("[") && host.endsWith("]")) {
    host = host.slice(1, -1);
  }
  return { host, port };
}

function clampRXWait(waitMS) {
  let ms = Number(waitMS);
  if (!Number.isFinite(ms) || ms < 0) {
    ms = 0;
  }
  if (ms === 0) {
    ms = 1;
  }
  if (ms > MAX_RX_WAIT_MS) {
    ms = MAX_RX_WAIT_MS;
  }
  return ms;
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function headersToObject(headers) {
  const obj = {};
  headers.forEach((value, key) => {
    obj[key] = value;
  });
  return obj;
}

function isSelfFetch(hostname) {
  if (!WORKER_HOST || WORKER_HOST === "CHANGE_ME_WORKER_HOST") {
    return false;
  }
  return hostname === WORKER_HOST || hostname.endsWith("." + WORKER_HOST);
}

function bytesToBase64(bytes) {
  let binary = "";
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize));
  }
  return btoa(binary);
}

function base64ToBytes(b64) {
  const binary = atob(b64);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), {
    status,
    headers: { "content-type": "application/json" },
  });
}

function tunnelResp(obj, status = 200) {
  return json(obj, status);
}
