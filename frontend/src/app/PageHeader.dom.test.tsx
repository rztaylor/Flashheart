import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageHeader } from "./PageHeader";

// The page header names its scope (ui-layout.md §1): the key badge or the
// board icon beside the name, which is the page's h1; a project's image
// (FH-151) takes the badge and the key moves beside the name.
describe("PageHeader", () => {
  it("is the key badge and the project's name as the heading", () => {
    render(
      <PageHeader projectKey="FH" title="Flashheart" summary="5 tickets" />,
    );
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("Flashheart");
    expect(heading.getAttribute("title")).toBe("Flashheart");
    const badge = heading.previousElementSibling;
    expect(badge?.getAttribute("aria-hidden")).toBe("true");
    expect(badge?.textContent).toBe("FH");
    expect(document.querySelector("[data-project-key]")).toBeNull();
    expect(screen.getByText("5 tickets")).toBeTruthy();
  });

  it("shows the board icon with no key, in the all scope", () => {
    render(<PageHeader title="All projects" />);
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("All projects");
    expect(heading.previousElementSibling?.querySelector("svg")).not.toBeNull();
  });

  it("shows the project's image on the badge and keeps the key beside the name", () => {
    render(
      <PageHeader
        projectKey="FH"
        projectImage="/api/projects/flashheart/image?v=abc"
        title="Flashheart"
      />,
    );
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("Flashheart");
    const img = heading.previousElementSibling?.querySelector("img");
    expect(img?.getAttribute("src")).toBe(
      "/api/projects/flashheart/image?v=abc",
    );
    expect(img?.getAttribute("alt")).toBe("");
    expect(document.querySelector("[data-project-key]")?.textContent).toBe(
      "FH",
    );
    expect(screen.queryByRole("img")).toBeNull();
  });

  it("takes a control in the badge's place", () => {
    render(
      <PageHeader
        projectKey="FH"
        title="Flashheart"
        badge={<button type="button">Project image: FH</button>}
      />,
    );
    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("Flashheart");
    expect(heading.previousElementSibling).toBe(
      screen.getByRole("button", { name: "Project image: FH" }),
    );
  });
});
