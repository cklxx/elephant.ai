/* global chrome */

const DEFAULT_BRIDGE_URL = "ws://127.0.0.1:17333/ws";

function $(id) {
  return document.getElementById(id);
}

async function loadForm() {
  const items = await chrome.storage.local.get(["bridgeUrl", "token"]);
  $("bridgeUrl").value = (items.bridgeUrl || DEFAULT_BRIDGE_URL).trim();
  $("token").value = (items.token || "").trim();
}

async function saveForm() {
  const bridgeUrl = $("bridgeUrl").value.trim() || DEFAULT_BRIDGE_URL;
  const token = $("token").value.trim();
  await chrome.storage.local.set({ bridgeUrl, token });
}

async function reconnect() {
  return await chrome.runtime.sendMessage({ type: "reconnect" });
}

async function renderStatus() {
  const resp = await chrome.runtime.sendMessage({ type: "getStatus" });
  if (!resp || !resp.ok) {
    $("status").textContent = `status: unknown\nerror: ${resp && resp.error ? resp.error : "no response"}`;
    return;
  }
  const lines = [
    `connected: ${resp.connected ? "true" : "false"}`,
    `bridgeUrl: ${resp.bridgeUrl || ""}`,
    `tokenSet: ${resp.tokenSet ? "true" : "false"}`,
    `lastError: ${resp.lastError || ""}`
  ];
  $("status").textContent = lines.join("\n");
}

document.addEventListener("DOMContentLoaded", () => {
  void loadForm().then(renderStatus);

  $("save").addEventListener("click", async () => {
    await saveForm();
    await reconnect();
    await renderStatus();
  });

  $("reconnect").addEventListener("click", async () => {
    await reconnect();
    await renderStatus();
  });

  setInterval(() => {
    void renderStatus();
  }, 1000);
});

