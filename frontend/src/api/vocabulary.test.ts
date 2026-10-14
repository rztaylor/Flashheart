import { describe, expect, it } from "vitest";

import vocabulary from "../../../testdata/vocabulary.json";
import { AGES } from "../model/paint";
import { PRIORITIES, TICKET_TYPES } from "../model/tickets";
import { COLUMNS, TICKET_ID } from "./board";
import { ERROR_CODES } from "./client";
import {
  COLOUR_BYS,
  DENSITIES,
  EDITOR_MODES,
  HIDEABLE_COLUMNS,
  SAVED_STATES,
  SAVED_VIEWS,
  THEMES,
} from "./preferences";
import { QUESTION_KINDS, RUN_STATES } from "./runs";

// The browser's copies of the server's vocabularies (FH-97, D79), matched
// to testdata/vocabulary.json, which internal/api's TestVocabulary writes
// from the Go lists.

const sources = import.meta.glob<string>("../**/*.{ts,tsx}", {
  query: "?raw",
  import: "default",
  eager: true,
});

describe("vocabularies", () => {
  it("lists ticket types, priorities and columns as the board does", () => {
    expect([...TICKET_TYPES]).toEqual(vocabulary.ticketTypes);
    expect([...PRIORITIES]).toEqual(vocabulary.priorities);
    expect(COLUMNS.map((column) => column.id)).toEqual(vocabulary.columns);
  });

  it("knows every run state and question kind", () => {
    // The browser orders run states by how much they need you.
    expect([...RUN_STATES].sort()).toEqual([...vocabulary.runStates].sort());
    expect(QUESTION_KINDS).toEqual(vocabulary.questionKinds);
  });

  it("offers the preference values the server accepts", () => {
    const { preferences } = vocabulary;
    expect(THEMES).toEqual(preferences.theme);
    expect(DENSITIES).toEqual(preferences.density);
    expect(COLOUR_BYS).toEqual(preferences.colourBy);
    expect(EDITOR_MODES).toEqual(preferences.editor);
    expect(HIDEABLE_COLUMNS).toEqual(preferences.hiddenColumns);
    expect(SAVED_VIEWS).toEqual(preferences.view);
    expect(SAVED_STATES).toEqual(preferences.state);
    expect(AGES.map(([id]) => id)).toEqual(preferences.age);
  });

  it("reads ticket ids as the board does", () => {
    for (const [id, valid] of Object.entries(vocabulary.ticketIds))
      expect(TICKET_ID.test(id), id).toBe(valid);
  });

  it("branches only on error codes the server writes", () => {
    for (const code of ERROR_CODES)
      expect(vocabulary.errorCodes).toContain(code);
    // Every code a source compares (`code === "conflict"`), leaving out
    // type checks (`typeof error.code === "string"`).
    const compared = new Set<string>();
    for (const [path, source] of Object.entries(sources)) {
      if (path.includes(".test.")) continue;
      for (const line of source.split("\n")) {
        if (line.includes("typeof")) continue;
        for (const match of line.matchAll(/\bcode\s*[!=]==\s*"([a-z_]+)"/g))
          compared.add(match[1] ?? "");
      }
    }
    expect(compared.size).toBeGreaterThan(0);
    for (const code of compared)
      expect(ERROR_CODES as readonly string[], code).toContain(code);
  });
});
