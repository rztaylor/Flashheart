import { describe, expect, it, vi } from "vitest";

import { ApiError } from "./client";
import { fetchInfo } from "./info";

const valid = {
  name: "Flashheart",
  version: "dev",
  commit: "unknown",
  buildDate: "unknown",
  protocolVersion: 1,
  root: "/Users/example/reports/Kanban",
  theme: "system",
  answers: true,
};

describe("fetchInfo", () => {
  it("requests /api/info through the authenticated fetch", async () => {
    const authenticatedFetch = vi.fn(async () => Response.json(valid));
    await expect(fetchInfo(authenticatedFetch)).resolves.toEqual(valid);
    expect(authenticatedFetch).toHaveBeenCalledWith("/api/info", {
      cache: "no-store",
      signal: undefined,
    });
  });

  it("rejects an unknown theme", async () => {
    const authenticatedFetch = vi.fn(async () =>
      Response.json({ ...valid, theme: "sepia" }),
    );
    await expect(fetchInfo(authenticatedFetch)).rejects.toThrow(
      "Server info response was invalid",
    );
  });

  it("rejects a missing field", async () => {
    const { root: _root, ...withoutRoot } = valid;
    const authenticatedFetch = vi.fn(async () => Response.json(withoutRoot));
    await expect(fetchInfo(authenticatedFetch)).rejects.toThrow(
      "Server info response was invalid",
    );
  });

  it("surfaces the backend's error message and code", async () => {
    const authenticatedFetch = vi.fn(async () =>
      Response.json(
        { error: { code: "not_found", message: "No such API endpoint" } },
        { status: 404 },
      ),
    );
    const error = await fetchInfo(authenticatedFetch).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      message: "No such API endpoint",
      status: 404,
      code: "not_found",
    });
  });

  it("describes a failure without a JSON body", async () => {
    const authenticatedFetch = vi.fn(
      async () => new Response("gateway", { status: 502 }),
    );
    await expect(fetchInfo(authenticatedFetch)).rejects.toThrow(
      "Request failed (502)",
    );
  });
});
