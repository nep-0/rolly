const { app, BrowserWindow, Menu, dialog, shell } = require("electron");
const { spawn } = require("child_process");
const fs = require("fs");
const net = require("net");
const path = require("path");

let backend = null;
let window = null;
let backendUrl = null;
let quitting = false;

function configPath() {
  return path.join(app.getPath("userData"), "config.json");
}

function readConfig() {
  try {
    return JSON.parse(fs.readFileSync(configPath(), "utf8"));
  } catch (err) {
    return {};
  }
}

function writeConfig(config) {
  fs.mkdirSync(app.getPath("userData"), { recursive: true });
  fs.writeFileSync(configPath(), JSON.stringify(config, null, 2));
}

function appDataDir() {
  if (process.env.ROLLY_APP_DATA_DIR) return process.env.ROLLY_APP_DATA_DIR;
  return readConfig().dataDir || path.join(app.getPath("userData"), "rolly-data");
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
  backendUrl = url;
  return url;
}

function stopBackend() {
  return new Promise((resolve) => {
    if (!backend || backend.killed) {
      resolve();
      return;
    }
    const child = backend;
    const done = () => resolve();
    child.once("exit", done);
    child.kill();
    setTimeout(done, 3000);
  });
}

async function restartBackend() {
  await stopBackend();
  const url = await startBackend();
  if (window) {
    await window.loadURL(url);
  }
}

async function chooseAppDataDir() {
  const result = await dialog.showOpenDialog(window, {
    title: "Choose Rolly app data directory",
    defaultPath: appDataDir(),
    properties: ["openDirectory", "createDirectory"],
  });
  if (result.canceled || !result.filePaths[0]) return;

  const dataDir = result.filePaths[0];
  writeConfig({ ...readConfig(), dataDir });
  await restartBackend();
}

async function openAppDataDir() {
  await shell.openPath(appDataDir());
}

function createMenu() {
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    {
      label: "Rolly",
      submenu: [
        {
          label: "Set App Data Directory...",
          click: () => {
            chooseAppDataDir().catch((err) => {
              console.error(err);
              dialog.showErrorBox("Could not change app data directory", err.message);
            });
          },
        },
        {
          label: "Open App Data Directory",
          click: () => {
            openAppDataDir().catch((err) => {
              console.error(err);
              dialog.showErrorBox("Could not open app data directory", err.message);
            });
          },
        },
        { type: "separator" },
        { role: "quit" },
      ],
    },
  ]));
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
  quitting = true;
  if (backend && !backend.killed) {
    backend.kill();
  }
});

app.whenReady().then(async () => {
  createMenu();
  const url = await startBackend();
  await createWindow(url);
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});
