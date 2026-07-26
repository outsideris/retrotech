// Electron main process for the RetroTech episode editor.
//
// Mirrors the architecture of blog.outsider.ne.kr's writing tool: Electron is a
// thin shell that
//   - decides which RetroTech repo folder to use (env → saved setting → native
//     folder picker), persisting the choice,
//   - spawns the Go backend (cmd/app) pointed at that folder and reads the
//     loopback port it prints,
//   - loads http://127.0.0.1:<port>/_write/ in a BrowserWindow,
//   - installs a standard menu (so copy/paste/undo + IME work — it's Chromium),
//   - tears the server down on quit.
// All episode logic lives in the Go server; this file owns only the window,
// the folder choice and the lifecycle.

const { app, BrowserWindow, dialog, Menu, nativeImage, shell } = require("electron");
const { spawn } = require("node:child_process");
const readline = require("node:readline");
const path = require("node:path");
const fs = require("node:fs");

// Show "RetroTech Editor" (not "Electron") in the menu bar and dialogs. Must be
// set before app is ready; electron-builder's productName sets the bundle name.
app.setName("RetroTech Editor");

// desktop/ sits at the repo root; its parent is the project root used for the
// `go build`/dev binary lookup.
const PROJECT_ROOT = path.resolve(__dirname, "..");
const CONFIG_PATH = path.join(app.getPath("userData"), "config.json");

let serverProc = null;
let mainWindow = null;

// Single instance: a second launch focuses the existing window instead of
// spawning a second server.
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
} else {
  app.on("second-instance", () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    }
  });
}

function loadConfig() {
  try {
    return JSON.parse(fs.readFileSync(CONFIG_PATH, "utf8"));
  } catch {
    return {};
  }
}

function saveConfig(cfg) {
  try {
    fs.mkdirSync(path.dirname(CONFIG_PATH), { recursive: true });
    fs.writeFileSync(CONFIG_PATH, JSON.stringify(cfg, null, 2));
  } catch (e) {
    console.error("save config:", e);
  }
}

// A folder is the RetroTech repo if it has content/episodes — the same check
// the Go server's editor.New makes.
function isValidRepo(dir) {
  return (
    typeof dir === "string" &&
    dir.length > 0 &&
    fs.existsSync(path.join(dir, "content", "episodes"))
  );
}

async function chooseRepoFolder() {
  const res = await dialog.showOpenDialog({
    properties: ["openDirectory"],
    message: "RetroTech 저장소 폴더를 선택하세요 (content/episodes 포함)",
  });
  if (res.canceled || res.filePaths.length === 0) return null;
  const dir = res.filePaths[0];
  if (!isValidRepo(dir)) {
    await dialog.showMessageBox({
      type: "warning",
      message: "선택한 폴더는 RetroTech 저장소가 아닙니다",
      detail: "content/episodes 가 있는 폴더를 선택하세요.",
    });
    return null;
  }
  return dir;
}

// Priority: EDITOR_REPO env (dev override) → saved config → folder picker. The
// chosen folder is persisted for next launch.
async function resolveRepoDir() {
  if (isValidRepo(process.env.EDITOR_REPO)) return process.env.EDITOR_REPO;
  const cfg = loadConfig();
  if (isValidRepo(cfg.repoDir)) return cfg.repoDir;
  const dir = await chooseRepoFolder();
  if (!dir) return null;
  saveConfig({ ...cfg, repoDir: dir });
  return dir;
}

// Spawn the Go server and resolve once it prints "EDITOR_PORT <n>". Uses a
// pre-built binary (npm run build:server → desktop/server-bin, or the packaged
// editor-server) so it starts instantly and needs no Go toolchain at runtime.
function startServer(repoDir) {
  return new Promise((resolve, reject) => {
    const bin = app.isPackaged
      ? path.join(process.resourcesPath, "editor-server")
      : path.join(__dirname, "server-bin");
    const proc = spawn(bin, ["-repo", repoDir], {
      cwd: app.isPackaged ? process.resourcesPath : PROJECT_ROOT,
      env: process.env,
    });
    const rl = readline.createInterface({ input: proc.stdout });
    let settled = false;
    rl.on("line", (line) => {
      const m = line.match(/^EDITOR_PORT (\d+)/);
      if (m && !settled) {
        settled = true;
        resolve({ proc, port: Number(m[1]) });
      }
    });
    proc.stderr.on("data", (d) => process.stderr.write(`[server] ${d}`));
    proc.on("exit", (code) => {
      if (!settled) {
        settled = true;
        reject(new Error(`server exited before listening (code ${code})`));
      }
    });
  });
}

async function launch() {
  const repoDir = await resolveRepoDir();
  if (!repoDir) {
    app.quit();
    return;
  }

  let port;
  try {
    const started = await startServer(repoDir);
    serverProc = started.proc;
    port = started.port;
  } catch (e) {
    await dialog.showMessageBox({
      type: "error",
      message: "에디터 서버를 시작하지 못했습니다",
      detail: String(e),
    });
    app.quit();
    return;
  }

  const saved = loadConfig().windowBounds || {};
  mainWindow = new BrowserWindow({
    width: saved.width || 1280,
    height: saved.height || 860,
    ...(Number.isInteger(saved.x) && Number.isInteger(saved.y) ? { x: saved.x, y: saved.y } : {}),
    title: "RetroTech — 에피소드 관리",
    icon: path.join(__dirname, "icons", "icon.png"),
  });
  mainWindow.loadURL(`http://127.0.0.1:${port}/_write/`);

  // Route every window.open / target=_blank / cmd-click to the system browser
  // instead of a nested app window (every outbound link is meant to leave the
  // editor — e.g. the preview's reference links).
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: "deny" };
  });

  // Persist size + position for next launch ("close" fires while the window is
  // still alive; "closed" is too late for getBounds).
  mainWindow.on("close", () => {
    if (mainWindow && !mainWindow.isDestroyed()) {
      const cfg = loadConfig();
      saveConfig({ ...cfg, windowBounds: mainWindow.getBounds() });
    }
  });
  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

// Pick a new repo folder, persist it, and relaunch pointed at it. Triggered from
// the "작업 폴더 변경…" menu item.
async function changeRepoFolder() {
  const dir = await chooseRepoFolder();
  if (!dir) return;
  const cfg = loadConfig();
  saveConfig({ ...cfg, repoDir: dir });
  app.relaunch();
  app.exit(0);
}

// Standard menu. editMenu gives Cmd+C/V/X/A/Z + IME for free; the File menu adds
// the folder switcher.
function buildMenu() {
  const isMac = process.platform === "darwin";
  const template = [
    ...(isMac
      ? [
          {
            label: "RetroTech Editor",
            submenu: [
              { role: "about" },
              { type: "separator" },
              { role: "hide" },
              { role: "hideOthers" },
              { role: "unhide" },
              { type: "separator" },
              { role: "quit" },
            ],
          },
        ]
      : []),
    {
      label: "파일",
      submenu: [
        { label: "작업 폴더 변경…", click: () => changeRepoFolder() },
        { type: "separator" },
        isMac ? { role: "close" } : { role: "quit" },
      ],
    },
    { role: "editMenu" },
    { role: "viewMenu" },
    { role: "windowMenu" },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

app.whenReady().then(() => {
  if (!gotTheLock) return;
  if (process.platform === "darwin" && app.dock) {
    try {
      app.dock.setIcon(nativeImage.createFromPath(path.join(__dirname, "icons", "icon.png")));
    } catch (e) {
      console.error("dock icon:", e);
    }
  }
  buildMenu();
  launch();
});

app.on("window-all-closed", () => {
  app.quit();
});

app.on("quit", () => {
  if (serverProc) {
    serverProc.kill();
    serverProc = null;
  }
});
