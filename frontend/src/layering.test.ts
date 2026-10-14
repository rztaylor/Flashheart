import { describe, expect, it } from "vitest";

// The layering rules in .agents/facts/frontend-ui.md, checked over the
// source rather than left to review (FH-106): the shared layers never
// import a feature or the app shell, the model never imports components,
// and a feature never imports the app shell. Features may use each other.

const sources = import.meta.glob<string>("./**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
});

const shared = [
  "styles",
  "model",
  "components",
  "api",
  "state",
  "lifecycle",
  "test",
];

// forbidden says which layers a layer may not import, and why.
const forbidden: Record<string, Record<string, string>> = {
  features: { app: "a feature never imports the app shell" },
};
for (const layer of shared) {
  forbidden[layer] = {
    features: "shared layers never import features",
    app: "shared layers never import the app shell",
  };
}
const model = forbidden.model;
if (model) model.components = "the model is plain functions and types";

// specifiers lists every module a source names: static and type imports,
// re-exports, side-effect imports and dynamic imports.
function specifiers(source: string): string[] {
  const found: string[] = [];
  for (const match of source.matchAll(
    /(?:^|[^\w.$])(?:from|import)\s*\(?\s*["']([^"'\n]+)["']/g,
  )) {
    found.push(match[1] ?? "");
  }
  return found;
}

// resolve turns a relative specifier in file (./a/b.ts) into a
// src-relative path, or undefined for a package.
function resolve(file: string, specifier: string): string | undefined {
  if (!specifier.startsWith(".")) return undefined;
  const parts = file.split("/").slice(0, -1);
  for (const part of specifier.split("/")) {
    if (part === "..") parts.pop();
    else if (part !== ".") parts.push(part);
  }
  return parts.join("/");
}

// layerOf names a src-relative path's layer: its top-level folder, or
// none for a file at the top (main.tsx, the type declarations).
function layerOf(path: string): string | undefined {
  const parts = path.split("/");
  return parts.length > 2 ? parts[1] : undefined;
}

function violations(files: Record<string, string>): string[] {
  const found: string[] = [];
  for (const [file, source] of Object.entries(files)) {
    const from = layerOf(file);
    const rules = from ? forbidden[from] : undefined;
    if (!rules) continue;
    for (const specifier of specifiers(source)) {
      const target = resolve(file, specifier);
      const to = target ? layerOf(target) : undefined;
      const why = to ? rules[to] : undefined;
      if (why) found.push(`${file} imports ${specifier}: ${why}`);
    }
  }
  return found.sort();
}

describe("frontend layering", () => {
  it("reads the source tree", () => {
    expect(Object.keys(sources).length).toBeGreaterThan(100);
    expect(Object.keys(sources)).toContain("./app/route.ts");
  });

  it("finds every kind of import", () => {
    const source = [
      'import a from "./a";',
      'import type { B } from "../b";',
      "import {",
      "  c,",
      '} from "./c";',
      'export { d } from "./d";',
      'import "./e.css";',
      'const f = await import("./f");',
      'const g = "not from a module";',
    ].join("\n");
    expect(specifiers(source)).toEqual([
      "./a",
      "../b",
      "./c",
      "./d",
      "./e.css",
      "./f",
    ]);
  });

  it("reports an import against the rules", () => {
    expect(
      violations({
        "./features/projects/Rail.tsx":
          'import type { S } from "../../app/route";',
        "./features/board/Board.tsx":
          'import { F } from "../filters/FilterBar";',
        "./components/Pill.tsx": 'import { B } from "../features/board/Board";',
        "./model/paint.ts": 'import { P } from "../components/Pill";',
        "./app/Shell.tsx": 'import { B } from "../features/board/Board";',
      }),
    ).toEqual([
      "./components/Pill.tsx imports ../features/board/Board: shared layers never import features",
      "./features/projects/Rail.tsx imports ../../app/route: a feature never imports the app shell",
      "./model/paint.ts imports ../components/Pill: the model is plain functions and types",
    ]);
  });

  it("holds for the whole app", () => {
    expect(violations(sources)).toEqual([]);
  });
});
