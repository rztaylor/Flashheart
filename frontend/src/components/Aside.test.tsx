import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { remarksFor } from "../model/remarks";
import { Aside, AsideProvider } from "./Aside";
import { Toast } from "./Toast";

describe("Aside", () => {
  it("shows one remark from its placement on its own", () => {
    const html = renderToStaticMarkup(<Aside placement="shutdown" />);
    const texts = remarksFor("shutdown").map((remark) => remark.text);
    expect(html).toContain("data-aside");
    expect(
      texts.some(
        (text) =>
          html.includes(text.replace(/'/g, "&#x27;")) || html.includes(text),
      ),
    ).toBe(true);
  });

  it("waits for the screen's registry before showing", () => {
    const html = renderToStaticMarkup(
      <AsideProvider>
        <Aside placement="search" />
        <Aside placement="review-empty" />
      </AsideProvider>,
    );
    expect(html).not.toContain("data-aside");
  });

  it("rides in a toast without being announced", () => {
    const html = renderToStaticMarkup(
      <Toast
        message={{
          id: 1,
          text: "Moved FH-3 to Done.",
          aside: { placement: "completion", text: "A remark" },
        }}
        onDismiss={() => undefined}
      />,
    );
    expect(html).toContain("A remark");
    expect(html).toMatch(/aria-hidden="true"[^>]*>A remark/);
  });
});
