import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { PullRequestLabel } from "./PullRequestLabel";

const render = (pr: string) =>
  renderToStaticMarkup(<PullRequestLabel pr={pr} />);

describe("PullRequestLabel (FH-114)", () => {
  it("links an https pull request in a new tab, named by its number", () => {
    const html = render("https://github.com/o/r/pull/7");
    expect(html).toMatch(
      /<a [^>]*href="https:\/\/github\.com\/o\/r\/pull\/7"[^>]*target="_blank"[^>]*rel="noopener noreferrer"/,
    );
    expect(html).toContain("PR #7");
  });

  it("links any other http(s) address by its text", () => {
    const html = render("http://git.local/o/r/merge_requests/12");
    expect(html).toContain('href="http://git.local/o/r/merge_requests/12"');
    expect(html).toContain("git.local/o/r/merge_requests/12");
  });

  it.each([
    "javascript:alert(1)",
    "data:text/html,<b>x</b>",
    "#42",
    "//evil.example/pull/1",
  ])("shows %s as plain text, not a link", (pr) => {
    const html = render(pr);
    expect(html).not.toContain("<a");
    expect(html).not.toContain("<b>");
    expect(html).toContain("Pull request");
  });

  it("renders nothing without a pull request", () => {
    expect(render("")).toBe("");
  });
});
