/* global chrome */

const DEFAULT_BRIDGE_URL = "ws://127.0.0.1:17333/ws";

let bridgeUrl = DEFAULT_BRIDGE_URL;
let token = "";

let socket = null;
let connected = false;
let lastError = "";
let reconnectAttempts = 0;
let reconnectTimer = null;

function setBadge(state) {
  const text = state === "connected" ? "ON" : "OFF";
  const color = state === "connected" ? "#16a34a" : "#9ca3af";
  chrome.action.setBadgeText({ text });
  chrome.action.setBadgeBackgroundColor({ color });
}

async function loadConfig() {
  const items = await chrome.storage.local.get(["bridgeUrl", "token"]);
  if (typeof items.bridgeUrl === "string" && items.bridgeUrl.trim() !== "") {
    bridgeUrl = items.bridgeUrl.trim();
  } else {
    bridgeUrl = DEFAULT_BRIDGE_URL;
  }
  if (typeof items.token === "string") {
    token = items.token.trim();
  } else {
    token = "";
  }
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

function scheduleReconnect() {
  clearReconnectTimer();
  reconnectAttempts += 1;
  const delayMs = Math.min(30000, 500 * Math.pow(1.6, reconnectAttempts));
  reconnectTimer = setTimeout(() => {
    void connect();
  }, delayMs);
}

function disconnect() {
  clearReconnectTimer();
  if (socket) {
    try {
      socket.close();
    } catch (_) {
      // ignore
    }
  }
  socket = null;
  connected = false;
  setBadge("disconnected");
}

async function connect() {
  await loadConfig();

  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return;
  }

  disconnect();

  try {
    socket = new WebSocket(bridgeUrl);
  } catch (e) {
    lastError = String(e && e.message ? e.message : e);
    scheduleReconnect();
    return;
  }

  socket.onopen = () => {
    reconnectAttempts = 0;
    lastError = "";
    const hello = {
      type: "hello",
      token: token || "",
      client: "chrome_extension",
      version: 1
    };
    socket.send(JSON.stringify(hello));
  };

  socket.onmessage = (event) => {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch (_) {
      return;
    }

    if (msg && msg.type === "welcome") {
      connected = true;
      setBadge("connected");
      return;
    }

    void handleRPC(msg);
  };

  socket.onerror = () => {
    // Let onclose handle reconnect scheduling.
  };

  socket.onclose = (evt) => {
    connected = false;
    setBadge("disconnected");
    lastError = evt && typeof evt.reason === "string" && evt.reason ? evt.reason : "disconnected";
    scheduleReconnect();
  };
}

function sendRPCResponse(id, result, error) {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const resp = { jsonrpc: "2.0", id };
  if (error) {
    resp.error = error;
  } else {
    resp.result = result;
  }
  socket.send(JSON.stringify(resp));
}

async function handleRPC(msg) {
  if (!msg || msg.jsonrpc !== "2.0" || !msg.id || !msg.method) {
    return;
  }

  try {
    const result = await dispatchMethod(msg.method, msg.params || {});
    sendRPCResponse(msg.id, result, null);
  } catch (e) {
    const errMsg = String(e && e.message ? e.message : e);
    sendRPCResponse(msg.id, null, { code: -32000, message: errMsg });
  }
}

async function dispatchMethod(method, params) {
  switch (method) {
    case "bridge.ping":
      return { ok: true, ts: Date.now() };

    case "tabs.list": {
      const tabs = await chrome.tabs.query({});
      return tabs
        .filter((t) => typeof t.id === "number" && typeof t.windowId === "number")
        .map((t) => ({
          tabId: t.id,
          windowId: t.windowId,
          url: t.url || "",
          title: t.title || "",
          active: !!t.active
        }));
    }

    case "cookies.getAll": {
      const details = {};
      if (params && typeof params.domain === "string" && params.domain.trim() !== "") {
        details.domain = params.domain.trim();
      }
      if (params && typeof params.url === "string" && params.url.trim() !== "") {
        details.url = params.url.trim();
      }
      if (params && typeof params.name === "string" && params.name.trim() !== "") {
        details.name = params.name.trim();
      }
      const cookies = await chrome.cookies.getAll(details);
      return cookies.map((c) => ({
        name: c.name,
        value: c.value,
        domain: c.domain,
        path: c.path,
        expirationDate: c.expirationDate,
        secure: !!c.secure,
        httpOnly: !!c.httpOnly,
        sameSite: c.sameSite,
        hostOnly: !!c.hostOnly,
        session: !!c.session,
        storeId: c.storeId
      }));
    }

    case "cookies.toHeader": {
      if (!params || typeof params.domain !== "string" || params.domain.trim() === "") {
        throw new Error("domain is required");
      }
      const domain = params.domain.trim();
      const cookies = await chrome.cookies.getAll({ domain });
      cookies.sort((a, b) => {
        const n = String(a.name || "").localeCompare(String(b.name || ""));
        if (n !== 0) return n;
        const d = String(a.domain || "").localeCompare(String(b.domain || ""));
        if (d !== 0) return d;
        return String(a.path || "").localeCompare(String(b.path || ""));
      });
      return cookies.map((c) => `${c.name}=${c.value}`).join("; ");
    }

    case "storage.getLocal": {
      const tabId = params && typeof params.tabId === "number" ? params.tabId : 0;
      const keys = params && Array.isArray(params.keys) ? params.keys : [];
      if (!tabId || tabId <= 0) {
        throw new Error("tabId is required");
      }
      const stringKeys = keys.filter((k) => typeof k === "string" && k.trim() !== "").map((k) => k.trim());
      if (stringKeys.length === 0) {
        throw new Error("keys is required");
      }

      const results = await chrome.scripting.executeScript({
        target: { tabId },
        args: [stringKeys],
        func: (readKeys) => {
          const out = {};
          for (const key of readKeys) {
            try {
              out[key] = window.localStorage.getItem(key);
            } catch (_) {
              out[key] = null;
            }
          }
          return out;
        }
      });

      if (!results || results.length === 0) {
        return {};
      }
      return results[0] && typeof results[0].result === "object" ? results[0].result : {};
    }

    default:
      throw new Error(`method not found: ${method}`);
  }
}

chrome.runtime.onInstalled.addListener(() => {
  setBadge("disconnected");
  void connect();
});

chrome.runtime.onStartup.addListener(() => {
  setBadge("disconnected");
  void connect();
});

chrome.storage.onChanged.addListener((changes, areaName) => {
  if (areaName !== "local") return;
  if (changes.bridgeUrl || changes.token) {
    void connect();
  }
});

chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || typeof msg.type !== "string") {
    sendResponse({ ok: false, error: "invalid message" });
    return;
  }

  if (msg.type === "getStatus") {
    sendResponse({
      ok: true,
      connected,
      bridgeUrl,
      tokenSet: !!token,
      lastError
    });
    return;
  }

  if (msg.type === "reconnect") {
    void connect().then(
      () => sendResponse({ ok: true }),
      (e) => sendResponse({ ok: false, error: String(e && e.message ? e.message : e) })
    );
    return true;
  }

  sendResponse({ ok: false, error: "unknown message type" });
});

// Kick off initial connection.
setBadge("disconnected");
void connect();

