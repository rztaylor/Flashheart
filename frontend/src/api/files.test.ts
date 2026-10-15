import { describe, expect, it } from "vitest";

import { ApiError } from "./client";
import { clearProjectImage, setProjectImage, uploadFile } from "./files";

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

// A project's image (FH-151) goes up as the body of a PUT and comes off
// with a DELETE; a refusal carries the server's words.
describe("setProjectImage and clearProjectImage", () => {
  const saved = { hash: "", revision: 4, warnings: [] };

  it("puts the file's bytes as the project's image", async () => {
    const { fetcher, calls } = respond(200, saved);
    const blob = new Blob(["png"]);
    await setProjectImage(fetcher, "my project", blob);
    expect(calls[0]?.url).toBe("/api/projects/my%20project/image");
    expect(calls[0]?.init?.method).toBe("PUT");
    expect(calls[0]?.init?.body).toBe(blob);
  });

  it("deletes the project's image", async () => {
    const { fetcher, calls } = respond(200, saved);
    await clearProjectImage(fetcher, "alpha");
    expect(calls[0]?.url).toBe("/api/projects/alpha/image");
    expect(calls[0]?.init?.method).toBe("DELETE");
  });

  it("throws the server's words for a refusal", async () => {
    const { fetcher } = respond(415, {
      error: {
        code: "type_not_allowed",
        message:
          "The image was not saved because it is not a JPEG or PNG image. Choose a JPEG or PNG image.",
      },
    });
    const refused = await setProjectImage(
      fetcher,
      "alpha",
      new Blob(["words"]),
    ).catch((error: unknown) => error);
    expect(refused).toBeInstanceOf(ApiError);
    expect((refused as ApiError).code).toBe("type_not_allowed");
    expect((refused as ApiError).message).toContain("not a JPEG or PNG");
  });
});
