import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Testing Library unmounts after each test only when the runner exposes
// globals, which Vitest does not; every interaction test shares this.
afterEach(() => {
  cleanup();
});
