import { afterEach, describe, expect, it, vi } from "vitest";

import { guarded, guardLeaving, guardUnloading, leave } from "./leaveGuard";

let release: (() => void) | undefined;
afterEach(() => release?.());

const unload = () => {
  const event = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(event);
  return event.defaultPrevented;
};

describe("the leave guard (FH-66)", () => {
  it("leaves at once when nothing is unsaved", () => {
    const go = vi.fn();
    leave(go);
    expect(go).toHaveBeenCalledOnce();
    expect(guarded()).toBe(false);
    expect(unload()).toBe(false);
  });

  it("asks the guard's owner, who decides when to go", () => {
    const asks: (() => void)[] = [];
    release = guardLeaving((proceed) => asks.push(proceed));
    const go = vi.fn();
    leave(go);
    expect(go).not.toHaveBeenCalled();
    expect(guarded()).toBe(true);
    asks[0]?.();
    expect(go).toHaveBeenCalledOnce();
  });

  it("holds the browser's prompt on reload until released", () => {
    release = guardLeaving(() => undefined);
    expect(unload()).toBe(true);
    release();
    expect(unload()).toBe(false);
    expect(guarded()).toBe(false);
  });

  it("is released only by its own holder", () => {
    const first = guardLeaving(() => undefined);
    release = guardLeaving(() => undefined);
    first();
    expect(guarded()).toBe(true);
  });
});

describe("a save in flight (FH-156)", () => {
  it("keeps the browser's prompt on reload without asking on leaving the page", () => {
    const saving = guardUnloading();
    const go = vi.fn();
    leave(go);
    expect(go).toHaveBeenCalledOnce();
    expect(guarded()).toBe(false);
    expect(unload()).toBe(true);
    saving();
    expect(unload()).toBe(false);
  });

  it("keeps the prompt while a guard or another save still holds it", () => {
    const first = guardUnloading();
    const second = guardUnloading();
    const guard = guardLeaving(() => undefined);
    first();
    first();
    guard();
    expect(unload()).toBe(true);
    second();
    expect(unload()).toBe(false);
  });
});
