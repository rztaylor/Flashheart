import { spawn } from "node:child_process";
import { cp, mkdir, mkdtemp, rm } from "node:fs/promises";
import { connect } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

export const projectRoot = resolve(import.meta.dirname, "../..");
export const executable = resolve(projectRoot, "build/flashheart");
export const screenshotDir = resolve(
  projectRoot,
  ".cache/playwright-screenshots",
);
const sampleBoard = resolve(projectRoot, "testdata/boards/sample");
const manualURLPattern = /Open this URL within two minutes: (http:\/\/\S+)/g;

// makeSandbox copies the sample board into a temporary HOME so tests never
// touch the real board root or agent configuration.
export async function makeSandbox() {
  const home = await mkdtemp(join(tmpdir(), "flashheart-e2e-"));
  const root = join(home, "board");
  await cp(sampleBoard, root, { recursive: true });
  await mkdir(screenshotDir, { recursive: true });
  return {
    home,
    root,
    cleanup: () => rm(home, { recursive: true, force: true }),
  };
}

// launch starts flashheart with an empty PATH so the browser opener fails and
// the manual URL path is exercised; Playwright then opens that URL.
export function launch(sandbox, args) {
  const env = {
    ...process.env,
    HOME: sandbox.home,
    XDG_CONFIG_HOME: join(sandbox.home, ".config"),
    PATH: "/nonexistent",
  };
  delete env.FLASHHEART_ROOT;
  const child = spawn(executable, [...args, "--root", sandbox.root], {
    cwd: sandbox.home,
    env,
    stdio: ["ignore", "pipe", "pipe"],
  });
  const output = { stdout: "", stderr: "" };
  child.stdout.setEncoding("utf8");
  child.stderr.setEncoding("utf8");
  child.stdout.on("data", (chunk) => {
    output.stdout += chunk;
  });
  child.stderr.on("data", (chunk) => {
    output.stderr += chunk;
  });
  return { child, output };
}

export function manualURLs(stderr) {
  return [...stderr.matchAll(manualURLPattern)].map((match) => match[1]);
}

export function waitForManualURL(child, output, timeoutMs = 15_000) {
  return new Promise((resolveURL, reject) => {
    const timeout = setTimeout(
      () => reject(new Error(`no manual URL; stderr: ${output.stderr}`)),
      timeoutMs,
    );
    const check = () => {
      const [url] = manualURLs(output.stderr);
      if (url) {
        clearTimeout(timeout);
        child.stderr.off("data", check);
        resolveURL(url);
      }
    };
    child.stderr.on("data", check);
    child.once("exit", () => setTimeout(check, 0));
    check();
  });
}

export function waitForExit(child, timeoutMs = 20_000) {
  if (child.exitCode !== null || child.signalCode !== null) {
    return Promise.resolve({ code: child.exitCode, signal: child.signalCode });
  }
  return new Promise((resolveExit, reject) => {
    const timeout = setTimeout(
      () => reject(new Error("flashheart did not exit")),
      timeoutMs,
    );
    child.once("exit", (code, signal) => {
      clearTimeout(timeout);
      resolveExit({ code, signal });
    });
  });
}

export async function stopIfRunning(child) {
  if (child.exitCode === null && child.signalCode === null) {
    child.kill("SIGTERM");
    await waitForExit(child).catch(() => undefined);
  }
}

// listening reports whether anything accepts connections on 127.0.0.1:port.
export function listening(port) {
  return new Promise((resolveListening) => {
    const socket = connect({ host: "127.0.0.1", port });
    socket.once("connect", () => {
      socket.destroy();
      resolveListening(true);
    });
    socket.once("error", () => resolveListening(false));
  });
}

export async function waitUntilNotListening(port, timeoutMs = 30_000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!(await listening(port))) return true;
    await new Promise((r) => setTimeout(r, 250));
  }
  return false;
}

// killDetached stops a detached server left behind by a failed test.
export async function killDetached(port) {
  if (!port || !(await listening(port))) return;
  const { execFileSync } = await import("node:child_process");
  const { existsSync } = await import("node:fs");
  const lsof = ["/usr/sbin/lsof", "/usr/bin/lsof"].find((path) =>
    existsSync(path),
  );
  if (!lsof) return;
  try {
    const pids = execFileSync(lsof, ["-ti", `tcp:${port}`, "-sTCP:LISTEN"], {
      encoding: "utf8",
    })
      .split("\n")
      .filter(Boolean);
    for (const pid of pids) process.kill(Number(pid), "SIGTERM");
  } catch {
    // Nothing is listening any more.
  }
}

// filterButton is a toolbar filter's button (FH-39), whose name is the
// filter's and, while it applies, what it holds ("Type: 2 chosen").
export function filterButton(page, name) {
  return page
    .getByRole("button", { name: new RegExp(`^${name}(:|$)`) })
    .and(page.locator("[aria-expanded]"));
}

// filterMenu opens a toolbar filter (FH-39), named "Type", "Priority",
// "Workstream" or "State", and returns its open panel.
export async function filterMenu(page, name) {
  const button = filterButton(page, name);
  await button.click();
  return page.locator(`[id="${await button.getAttribute("aria-controls")}"]`);
}

// viewOption picks an option in the Board's View options menu ("Colour by"
// or "Density") and closes the menu.
export async function viewOption(page, group, option) {
  await page.getByRole("button", { name: "View options" }).click();
  await page
    .getByRole("group", { name: group })
    .getByText(option, { exact: true })
    .click();
  await page.keyboard.press("Escape");
}
