const { app, BrowserWindow } = require("electron");
const { spawn } = require("child_process");
const fs = require("fs");
const net = require("net");
const path = require("path");

let backend = null;
let window = null;

function appDataDir() {
  return path.join(app.getPath("userData"), "rolly-data");
}

function backendPath() {
  if (process.env.ROLLY_BACKEND_PATH) return process.env.ROLLY_BACKEND_PATH;
  if (app.isPackaged) return path.join(process.resourcesPath, "backend", "rolly.exe");
  const local = path.join(__dirname, "dist", "backend", "rolly.exe");
  if (fs.existsSync(local)) return local;
  return path.join(process.resourcesPath, "backend", "rolly.exe");
}

function frontendDir() {
  if (process.env.ROLLY_FRONTEND_DIR) return process.env.ROLLY_FRONTEND_DIR;
  if (app.isPackaged) return path.join(process.resourcesPath, "frontend");
  const local = path.join(__dirname, "..", "frontend");
  if (fs.existsSync(local)) return local;
  return path.join(process.resourcesPath, "frontend");
}

function waitForHealth(url, timeoutMs = 15000) {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const tick = async () => {
      try {
        const res = await fetch(url);
        if (res.ok) {
          resolve();
          return;
        }
      } catch (err) {
        // retry
      }
      if (Date.now() - started > timeoutMs) {
        reject(new Error("backend did not start"));
        return;
      }
      setTimeout(tick, 250);
    };
    tick();
  });
}

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
  });
}

async function startBackend() {
  const dataDir = appDataDir();
  const uploads = path.join(dataDir, "uploads");
  const exportsDir = path.join(dataDir, "exports");
  const dbPath = path.join(dataDir, "rolly.db");
  fs.mkdirSync(dataDir, { recursive: true });
  fs.mkdirSync(uploads, { recursive: true });
  fs.mkdirSync(exportsDir, { recursive: true });

  const port = await freePort();
  const addr = `127.0.0.1:${port}`;
  const url = `http://${addr}/`;

  backend = spawn(backendPath(), [
    "-addr", addr,
    "-db", dbPath,
    "-uploads", uploads,
    "-exports", exportsDir,
    "-frontend", frontendDir(),
  ], {
    cwd: dataDir,
    env: {
      ...process.env,
      ROLLY_DB: dbPath,
      ROLLY_UPLOAD_DIR: uploads,
      ROLLY_EXPORT_DIR: exportsDir,
      ROLLY_FRONTEND_DIR: frontendDir(),
    },
    stdio: "inherit",
    windowsHide: true,
  });

  backend.on("exit", (code) => {
    if (code && code !== 0) {
      console.error(`backend exited with code ${code}`);
    }
    backend = null;
  });

  await waitForHealth(`${url}healthz`);
  return url;
}

async function createWindow(url) {
  window = new BrowserWindow({
    width: 1400,
    height: 1000,
    webPreferences: {
      contextIsolation: true,
    },
  });
  await window.loadURL(url);
  window.on("closed", () => {
    window = null;
  });
}

app.on("before-quit", () => {
  if (backend && !backend.killed) {
    backend.kill();
  }
});

app.whenReady().then(async () => {
  const url = await startBackend();
  await createWindow(url);
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
