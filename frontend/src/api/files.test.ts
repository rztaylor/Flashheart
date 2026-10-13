import { describe, expect, it } from "vitest";

import { ApiError } from "./client";
import { uploadFile } from "./files";

function respond(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fetcher = async (input: RequestInfo | URL, init?: RequestInit) => {
    calls.push({ url: String(input), init });
    return new Response(JSON.stringify(body), { status });
  };
  return { fetcher, calls };
}

describe("uploadFile", () => {
  it("sends the bytes as the body and the name and kind in the query", async () => {
    const { fetcher, calls } = respond(201, {
      file: "20261013T1432-screenshot.png",
      kind: "screenshot",
      image: true,
      reused: false,
      revision: 9,
    });
    const blob = new Blob(["png"]);
    const uploaded = await uploadFile(
      fetcher,
      "FH-18",
      blob,
      "screenshot.png",
      "screenshot",
    );
    expect(uploaded).toEqual({
      file: "20261013T1432-screenshot.png",
      kind: "screenshot",
      image: true,
      reused: false,
    });
    expect(calls[0]?.url).toBe(
      "/api/tickets/FH-18/files?name=screenshot.png&kind=screenshot",
    );
    expect(calls[0]?.init?.method).toBe("POST");
    expect(calls[0]?.init?.body).toBe(blob);
  });

  it("throws the server's words for a refusal", async () => {
    const { fetcher } = respond(415, {
      error: {
        code: "type_not_allowed",
        message: "drawing.png was not attached because it is an SVG image.",
      },
    });
    const refused = await uploadFile(
      fetcher,
      "FH-18",
      new Blob(["<svg/>"]),
      "drawing.png",
    ).catch((error: unknown) => error);
    expect(refused).toBeInstanceOf(ApiError);
    expect((refused as ApiError).code).toBe("type_not_allowed");
    expect((refused as ApiError).message).toContain("SVG");
  });
});
