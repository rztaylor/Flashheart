import { existsSync } from "node:fs";
import { join, resolve } from "node:path";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import {
  killDetached,
  launch,
  listening,
  makeSandbox,
  manualURLs,
  screenshotDir,
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
    page.getByRole("heading", { level: 1, name: "Flashheart is running" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Backend connected. Check connection" }),
  ).toBeVisible();
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
    await page.screenshot({
      path: resolve(screenshotDir, "quit-denied-1280-light.png"),
    });
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
    expect(await exit, output.stderr).toEqual({ code: 0, signal: null });
    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
    await page.screenshot({
      path: resolve(screenshotDir, "stopped-1280-light.png"),
    });
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

    await page.keyboard.press("Tab");
    await expect(
      page.getByRole("button", { name: "Backend connected. Check connection" }),
    ).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Quit" })).toBeFocused();
    // Screenshots show the resting state, not the keyboard focus ring.
    await page.evaluate(() => {
      if (document.activeElement instanceof HTMLElement)
        document.activeElement.blur();
    });

    for (const theme of ["light", "dark"]) {
      await page.emulateMedia({ colorScheme: theme });
      await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
      for (const width of [1280, 1920]) {
        await page.setViewportSize({
          width,
          height: width === 1280 ? 800 : 1080,
        });
        const results = await new AxeBuilder({ page }).analyze();
        expect(results.violations, `${theme} ${width}`).toEqual([]);
        await page.screenshot({
          path: resolve(screenshotDir, `shell-${width}-${theme}.png`),
        });
      }
    }
    const exit = waitForExit(child);
    await page.getByRole("button", { name: "Quit" }).click();
    await expect(
      page.getByRole("heading", { name: "Flashheart has stopped" }),
    ).toBeVisible();
    const results = await new AxeBuilder({ page }).analyze();
    expect(results.violations).toEqual([]);
    await page.screenshot({
      path: resolve(screenshotDir, "stopped-1920-dark.png"),
    });
    expect((await exit).code).toBe(0);
  } finally {
    await stopIfRunning(child);
    await sandbox.cleanup();
  }
});
