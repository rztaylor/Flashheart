import { existsSync } from "node:fs";
import { cp } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

import {
  expectConnected,
  expectNoAxeViolations,
  killDetached,
  launch,
  listening,
  makeSandbox,
  manualURLs,
  projectRoot,
  shot,
  stopIfRunning,
  waitForExit,
  waitForManualURL,
  waitUntilNotListening,
} from "./support.mjs";

const denial = {
  code: "save_in_progress",
  message: "A change is still being saved. Quit again when it finishes.",
};

function originPort(page) {
  return Number(new URL(page.url()).port);
}

// Every request the page makes must stay on the launch's loopback origin.
function recordForeignRequests(page) {
  const foreign = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (!/^ss-[0-9a-f]+\.localhost$/.test(url.hostname)) {
      foreign.push(request.url());
    }
  });
  return foreign;
}

async function expectRunning(page) {
  await expect(
    page.getByRole("navigation", { name: "Projects" }),
  ).toBeVisible();
  await expectConnected(page);
}

test("detached launch returns the terminal; reload, new tab and last-tab close follow the browser", async ({
  page,
  context,
}) => {
  const sandbox = await makeSandbox();
  const { child, output } = launch(sandbox, []);
  let port;
  try {
    // LIFE-4: the launcher exits once the server is up, printing the manual
    // URL exactly once because the browser could not be opened.
    const exit = await waitForExit(child, 20_000);
    expect(exit, output.stderr).toEqual({ code: 0, signal: null });
    expect(output.stdout).toBe("");
    const urls = manualURLs(output.stderr);
    expect(urls, output.stderr).toHaveLength(1);
    expect(output.stderr).toMatch(/^Could not open a browser: .+\n/);

    const foreign = recordForeignRequests(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto(urls[0], { waitUntil: "domcontentloaded" });
    port = originPort(page);
    await expectRunning(page);
    await expect(page.getByText(sandbox.root, { exact: true })).toBeVisible();
    await expect(page.getByText("Agent protocol")).toBeVisible();
    await expect(page.getByText(/Reconnecting/)).toHaveCount(0);
    await expect(page).toHaveURL(
      (url) => url.pathname === "/" && url.hash === "",
    );

    const storage = await page.evaluate(() => ({
      local: Object.keys(localStorage),
      session: Object.keys(sessionStorage),
      cookie: document.cookie,
    }));
    expect(storage).toEqual({ local: [], session: [], cookie: "" });

    // Manual health check reports through the live region.
    await page
      .getByRole("button", { name: "Backend connected. Check connection" })
      .click();
    await expect(page.getByText(/^Responding · /)).toBeVisible();

    // Reload keeps the session.
    await page.reload({ waitUntil: "domcontentloaded" });
    await expectRunning(page);

    // A clean new tab on the same origin connects without a new link.
    const second = await context.newPage();
    await second.goto(new URL("/", page.url()).href, {
      waitUntil: "domcontentloaded",
    });
    await expectRunning(second);

    // Closing one of two tabs leaves the server running.
    await page.close({ runBeforeUnload: true });
    await second.waitForTimeout(8_000);
    expect(await listening(port)).toBe(true);
    await second
      .getByRole("button", { name: "Backend connected. Check connection" })
      .click();
    await expect(second.getByText(/^Responding · /)).toBeVisible();

    // Closing the last tab stops the detached server.
    await second.close({ runBeforeUnload: true });
    expect(await waitUntilNotListening(port, 30_000)).toBe(true);
    expect(foreign).toEqual([]);
    // A clean run logs nothing, so serve.log is never created.
    expect(existsSync(join(sandbox.root, ".flashheart", "serve.log"))).toBe(
      false,
    );
  } finally {
    await stopIfRunning(child);
    await killDetached(port);
    await sandbox.cleanup();
  }
});

test("quit is refused while the guard denies it, then stops the server", async ({
  page,
}) => {
  const sandbox = await makeSandbox();
  const { child, output } = launch(sandbox, ["serve", "--foreground"]);
  try {
    const url = await waitForManualURL(child, output);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto(url, { waitUntil: "domcontentloaded" });
    await expectRunning(page);

    // Nothing can hold a save open yet (foundation has no writes), so the
    // denial response is injected; internal/app tests prove the real guard.
    await page.route("**/_singleserve/shutdown", (route) =>
      route.fulfill({ status: 409, json: { error: denial } }),
    );
    await page.getByRole("button", { name: "Quit" }).click();
    const alert = page.getByRole("alert");
    await expect(alert).toContainText("Flashheart did not quit.");
    await expect(alert).toContainText(denial.message);
    await expectRunning(page);
    await expect(page.getByRole("button", { name: "Quit" })).toBeEnabled();
    await shot(page, "quit-denied-1280-light", { keepFocus: true });
    await page.getByRole("button", { name: "Dismiss" }).click();
    await expect(page.getByRole("alert")).toHaveCount(0);
    expect(child.exitCode).toBeNull();

    await page.unroute("**/_singleserve/shutdown");
    const exit = waitForExit(child);
    await page.getByRole("button", { name: "Quit" }).click();
    await expect(
      page.getByRole("heading", { name: "Flashheart has stopped" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Close tab" })).toBeVisible();
    await expect(page.getByText(/close it yourself/)).toBeVisible();
    // A clean stop earns one parting remark.
    await expect(page.locator("[data-aside]")).toHaveCount(1);
    expect(await exit, output.stderr).toEqual({ code: 0, signal: null });
    await expectNoAxeViolations(page, "stopped-1280-light");
    await shot(page, "stopped-1280-light", { keepFocus: true });
  } finally {
    await stopIfRunning(child);
    await sandbox.cleanup();
  }
});

test("a project in board format v1 shows the migrate command", async ({
  page,
}) => {
  const sandbox = await makeSandbox();
  await cp(
    join(projectRoot, "testdata/boards/sample-v1/alpha"),
    join(sandbox.root, "legacy"),
    { recursive: true },
  );
  const { child, output } = launch(sandbox, ["serve", "--foreground"]);
  try {
    const url = await waitForManualURL(child, output);
    await page.goto(url, { waitUntil: "domcontentloaded" });
    await expectRunning(page);
    const notice = page.getByRole("region", { name: "Older board format" });
    await expect(notice).toContainText("legacy");
    await expect(notice).toContainText(
      `flashheart migrate --root ${sandbox.root}`,
    );
    await expectNoAxeViolations(page, "migrate notice");
  } finally {
    await stopIfRunning(child);
    await sandbox.cleanup();
  }
});

test("backend loss enters the terminal state", async ({ page }) => {
  test.setTimeout(90_000);
  const sandbox = await makeSandbox();
  const { child, output } = launch(sandbox, ["serve", "--foreground"]);
  try {
    const url = await waitForManualURL(child, output);
    await page.goto(url, { waitUntil: "domcontentloaded" });
    await expectRunning(page);

    child.kill("SIGKILL");
    await waitForExit(child);
    await expect(
      page.getByRole("heading", { name: "Lost connection to Flashheart" }),
    ).toBeVisible({ timeout: 45_000 });
    await expect(page.getByText(/missed heartbeats/)).toBeVisible();
    // Losing the backend is no moment for a joke.
    await expect(page.locator("[data-aside]")).toHaveCount(0);
    await expect(page.getByText("flashheart", { exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "Quit" })).toHaveCount(0);
  } finally {
    await stopIfRunning(child);
    await sandbox.cleanup();
  }
});

test("shell is keyboard reachable, accessible and renders in both themes", async ({
  page,
}) => {
  const sandbox = await makeSandbox();
  const { child, output } = launch(sandbox, ["serve", "--foreground"]);
  try {
    const url = await waitForManualURL(child, output);
    await page.emulateMedia({ colorScheme: "light", reducedMotion: "reduce" });
    await page.goto(url, { waitUntil: "domcontentloaded" });
    await expectRunning(page);
    await expect(page.getByText(sandbox.root, { exact: true })).toBeVisible();

    // The first stop is the first view tab, Overview (FH-61).
    await page.keyboard.press("Tab");
    await expect(page.getByRole("link", { name: "Overview" })).toBeFocused();
    await page.getByRole("button", { name: "Quit" }).focus();
    await expect(page.getByRole("button", { name: "Quit" })).toBeFocused();
    for (const theme of ["light", "dark"]) {
      await page.emulateMedia({ colorScheme: theme });
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      for (const width of [1280, 1920]) {
        await page.setViewportSize({
          width,
          height: width === 1280 ? 800 : 1080,
        });
        await expectNoAxeViolations(page, `${theme} ${width}`);
        // The resting state, without the keyboard focus ring.
        await shot(page, `lifecycle-${width}-${theme}`);
      }
    }
    const exit = waitForExit(child);
    await page.getByRole("button", { name: "Quit" }).click();
    await expect(
      page.getByRole("heading", { name: "Flashheart has stopped" }),
    ).toBeVisible();
    await expectNoAxeViolations(page, "stopped-1920-dark");
    await shot(page, "stopped-1920-dark", { keepFocus: true });
    expect((await exit).code).toBe(0);
  } finally {
    await stopIfRunning(child);
    await sandbox.cleanup();
  }
});
