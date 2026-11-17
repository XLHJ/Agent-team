// Electron main: spawns the Go Hub, generates the one-shot panel token and
// hands the endpoint to the renderer via preload (Phase-3 3.1).
const { app, BrowserWindow } = require("electron");
const { spawn } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");

let hub = null;
let win = null;

function hubBinary() {
  const override = process.env.ACP_HUB_BIN;
  if (override) return override;
  const name = process.platform === "win32" ? "hub.exe" : "hub";
  return path.join(process.resourcesPath ?? path.join(__dirname, "../../hub"), name);
}

function readToken(stateDir) {
  try {
    return fs.readFileSync(path.join(stateDir, "hub.token"), "utf8").trim();
  } catch {
    return "";
  }
}

app.whenReady().then(() => {
  const stateDir = path.join(app.getPath("userData"), "hub-state");
  fs.mkdirSync(stateDir, { recursive: true });
  const token = crypto.randomBytes(16).toString("hex");
  hub = spawn(hubBinary(), [
    "-addr", "127.0.0.1:0", // placeholder; real port picked below
    "-token", token,
    "-state-dir", stateDir,
  ], { stdio: ["ignore", "pipe", "pipe"] });
  hub.stdout.on("data", (d) => {
    const m = /listening on 127\.0\.0\.1:(\d+)/.exec(String(d));
    if (!m) return;
    const port = Number(m[1]);
    win?.webContents.send("hub-ready", {
      url: `ws://127.0.0.1:${port}/panel?token=${token}`,
      httpBase: `http://127.0.0.1:${port}`,
    });
  });
  hub.on("exit", () => { hub = null; });
  app.on("before-quit", () => hub?.kill());

  win = new BrowserWindow({
    width: 1280,
    height: 800,
    webPreferences: { preload: path.join(__dirname, "preload.cjs") },
  });
  if (process.env.ACP_DEV_URL) {
    win.loadURL(process.env.ACP_DEV_URL); // vite dev server
  } else {
    // Compiled panel build lives in the repo-root dist/panel/ folder
    // (all compiled artifacts are consolidated there).
    win.loadFile(path.join(__dirname, "../../dist/panel/index.html"));
  }
});

app.on("window-all-closed", () => app.quit());
