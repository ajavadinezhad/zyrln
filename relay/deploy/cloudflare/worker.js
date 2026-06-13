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
// Number of warm Durable Objects to spread tunnel sessions across. A DO is
// single-threaded, so when many sessions hash to the same bucket they serialize
// behind each other's CPU work (base64). A larger pool means fewer collisions
// and more parallelism for a browser that opens dozens of sockets at once. It
// does NOT prevent per-DO eviction (see read pump below) — it only addresses
// throughput/latency under concurrency.
const TUNNEL_POOL_SIZE = 128;

// Max bytes buffered per session by the background read pump before it pauses
// reading from the socket (TCP backpressure). Bounds DO memory if the client
// stops draining via rx.
const RX_BUFFER_CAP = 1024 * 1024;

// Max bytes returned in a single rx response. Bounds base64 CPU/response size;
// the client keeps polling for the rest. Mirrors the VPS 128KB read granularity
// loosely while letting us coalesce several small chunks into one response.
const MAX_RX_RESP_BYTES = 256 * 1024;

/** Serializes async work (one writer at a time per lock, like the VPS writeMu). */
class AsyncMutex {
  constructor() {
    this.tail = Promise.resolve();
  }

  runExclusive(fn) {
    const run = this.tail.then(fn, fn);
    this.tail = run.then(
      () => undefined,
      () => undefined,
    );
    return run;
  }
}

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

  // Spread sessions across a small fixed pool of Durable Objects (bucket by
  // session id) instead of one global instance. This keeps concurrency — each
  // bucket is its own isolate, ~POOL_SIZE running in parallel — while keeping
  // the DOs warm: a fixed set of names gets reused, so sessions do NOT pay the
  // ~0.5s DO cold-start/placement cost on every open (one DO per session does).
  // Ops in a batch always share one session id (client sets ops[i].id), so a
  // session's ops always land on the same bucket and its socket stays put.
  const raw = await request.text();
  const sessionId = tunnelSessionKey(raw);
  const doName = tunnelPoolName(sessionId);
  const id = env.TUNNEL_HUB.idFromName(doName);
  const forwarded = new Request(request.url, {
    method: "POST",
    headers: request.headers,
    body: raw,
  });
  return env.TUNNEL_HUB.get(id).fetch(forwarded);
}

// tunnelPoolName maps a session id to one of TUNNEL_POOL_SIZE warm Durable
// Object names (FNV-1a hash). Sessions without an id (e.g. ping) fall back to
// the shared instance.
function tunnelPoolName(sessionId) {
  if (!sessionId) {
    return TUNNEL_HUB_NAME;
  }
  let h = 2166136261 >>> 0;
  for (let i = 0; i < sessionId.length; i++) {
    h ^= sessionId.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return "pool-" + (h % TUNNEL_POOL_SIZE);
}

// tunnelSessionKey extracts the session id from a tunnel request body without
// re-serializing it. Returns "" for ops without a session (e.g. ping) or on
// parse failure, so those fall back to the shared instance and old clients keep
// working unchanged. The DO re-parses the raw body authoritatively.
function tunnelSessionKey(raw) {
  if (!raw || raw.length > MAX_BODY_BYTES) {
    return "";
  }
  try {
    const body = JSON.parse(raw);
    if (Array.isArray(body.ops)) {
      for (const op of body.ops) {
        if (op && typeof op.id === "string" && op.id.trim() !== "") {
          return op.id.trim();
        }
      }
      return "";
    }
    if (typeof body.id === "string" && body.id.trim() !== "") {
      return body.id.trim();
    }
  } catch {
    // fall through to shared instance
  }
  return "";
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
      const sess = {
        socket,
        reader,
        writer,
        writeMu: new AsyncMutex(),
        rxChunks: [], // queued Uint8Array chunks read from the socket
        rxBytes: 0, // total bytes currently buffered in rxChunks
        eof: false, // remote closed cleanly (reader returned done)
        readErr: null, // fatal read error from the pump
        rxWaiter: null, // { resolve } for an rx op waiting on new data
        drainWaiter: null, // { resolve } to resume a paused pump after draining
        closed: false,
        target: String(target).trim(),
        lastSeen: Date.now(),
      };
      this.sessions.set(id, sess);
      // Continuously read from the socket in the background. A pending
      // reader.read() is in-flight I/O that helps keep this DO resident (fewer
      // idle evictions), buffers inbound data so rx returns immediately, and
      // surfaces remote EOF/errors instead of silently hanging.
      this.startReadPump(sess);
      await this.scheduleCleanup();
      return { ok: true };
    } catch (err) {
      return { e: String(err) };
    }
  }

  // startReadPump drains the socket into sess.rxChunks until EOF/error/close,
  // applying backpressure when the buffer fills so a slow client can't grow DO
  // memory without bound.
  startReadPump(sess) {
    const pump = async () => {
      try {
        for (;;) {
          if (sess.closed) {
            return;
          }
          while (sess.rxBytes >= RX_BUFFER_CAP && !sess.closed) {
            await new Promise((resolve) => {
              sess.drainWaiter = { resolve };
            });
          }
          if (sess.closed) {
            return;
          }
          const { value, done } = await sess.reader.read();
          if (done) {
            sess.eof = true;
            this.wakeRX(sess);
            return;
          }
          if (value && value.byteLength > 0) {
            sess.rxChunks.push(value);
            sess.rxBytes += value.byteLength;
            this.wakeRX(sess);
          }
        }
      } catch (err) {
        sess.readErr = err;
        this.wakeRX(sess);
      }
    };
    // Unawaited on purpose: the loop owns the reader for the session lifetime.
    sess.pump = pump();
  }

  // wakeRX resolves a pending rx waiter (if any) so it re-checks the buffer.
  wakeRX(sess) {
    if (sess.rxWaiter) {
      const w = sess.rxWaiter;
      sess.rxWaiter = null;
      w.resolve();
    }
  }

  // drainRX pulls up to MAX_RX_RESP_BYTES out of the buffer, coalescing queued
  // chunks and splitting the last one if needed. Returns null when empty.
  drainRX(sess) {
    if (sess.rxChunks.length === 0) {
      return null;
    }
    const out = [];
    let total = 0;
    while (sess.rxChunks.length > 0 && total < MAX_RX_RESP_BYTES) {
      const chunk = sess.rxChunks[0];
      const room = MAX_RX_RESP_BYTES - total;
      if (chunk.byteLength <= room) {
        out.push(chunk);
        total += chunk.byteLength;
        sess.rxChunks.shift();
      } else {
        out.push(chunk.subarray(0, room));
        sess.rxChunks[0] = chunk.subarray(room);
        total += room;
        break;
      }
    }
    sess.rxBytes -= total;
    if (sess.drainWaiter && sess.rxBytes < RX_BUFFER_CAP) {
      const d = sess.drainWaiter;
      sess.drainWaiter = null;
      d.resolve();
    }
    if (out.length === 1) {
      return out[0];
    }
    const merged = new Uint8Array(total);
    let offset = 0;
    for (const chunk of out) {
      merged.set(chunk, offset);
      offset += chunk.byteLength;
    }
    return merged;
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
      return await sess.writeMu.runExclusive(async () => {
        await sess.writer.write(data);
        sess.lastSeen = Date.now();
        return { ok: true };
      });
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
    sess.lastSeen = Date.now();

    // Fast path: data already buffered by the pump.
    let chunk = this.drainRX(sess);
    if (chunk) {
      return { ok: true, data: bytesToBase64(chunk) };
    }
    const drained = this.rxTerminalResp(sess);
    if (drained) {
      await this.closeSession(id);
      return drained;
    }

    // Wait up to waitMs for the pump to deliver data, EOF, or an error. A
    // single rx runs per session at a time (client serializes), so one waiter
    // slot is sufficient.
    const waitMs = clampRXWait(waitMS);
    await Promise.race([
      new Promise((resolve) => {
        sess.rxWaiter = { resolve };
      }),
      delay(waitMs),
    ]);
    sess.rxWaiter = null;
    sess.lastSeen = Date.now();

    chunk = this.drainRX(sess);
    if (chunk) {
      return { ok: true, data: bytesToBase64(chunk) };
    }
    const terminal = this.rxTerminalResp(sess);
    if (terminal) {
      await this.closeSession(id);
      return terminal;
    }
    return { ok: true };
  }

  // rxTerminalResp returns an error response when the session has drained and
  // the remote end has closed or errored, matching the VPS which surfaces a
  // remote close as an rx error. Returns null while the session is still live.
  rxTerminalResp(sess) {
    if (sess.rxChunks.length > 0) {
      return null;
    }
    if (sess.readErr) {
      return { e: String(sess.readErr) };
    }
    if (sess.eof) {
      return { e: "eof" };
    }
    return null;
  }

  async closeSession(id) {
    const sess = this.sessions.get(id);
    if (!sess) {
      return;
    }
    this.sessions.delete(id);
    sess.closed = true;
    // Wake the pump (if paused on backpressure) and any rx waiter so they
    // observe sess.closed and unwind instead of hanging.
    if (sess.drainWaiter) {
      const d = sess.drainWaiter;
      sess.drainWaiter = null;
      d.resolve();
    }
    this.wakeRX(sess);
    // Cancelling the reader unblocks the pump's pending read(); the writer is
    // closed under writeMu to avoid racing an in-flight tx write.
    try {
      await sess.reader.cancel();
    } catch {
      // ignore
    }
    await sess.writeMu.runExclusive(async () => {
      try {
        await sess.writer.close();
      } catch {
        // ignore
      }
    });
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
