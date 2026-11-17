const { contextBridge, ipcRenderer } = require("electron");

contextBridge.exposeInMainWorld("__ACP_HUB_BRIDGE__", {
  onHubReady: (cb) => ipcRenderer.on("hub-ready", (_e, info) => cb(info)),
});
