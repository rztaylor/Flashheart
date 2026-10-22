import { describe, expect, it } from "vitest";

import {
  isArchiveCheck,
  isArchivedProjects,
  isArchiveList,
  isDeletePlan,
  isProjectDeletePlan,
} from "./archive";
import {
  isAllBoard,
  isBoard,
  isProjects,
  isTicketResponse,
  isWorkstreamResponse,
  isWorkstreams,
} from "./board";
import { isErrorBody } from "./client";
import {
  isBlockedBody,
  isChanges,
  isChangesRequested,
  isCommentDeleted,
  isCommentResponse,
  isConflictBody,
  isCreated,
  isCreatedWorkstream,
  isEditedWorkstream,
  isKeyTakenBody,
  isSaved,
} from "./edit";
import { isUploaded } from "./files";
import { isServerInfo } from "./info";
import { isPreferences } from "./preferences";
import { isRunResponse, isRuns } from "./runs";

// The API contract (FH-97): internal/api's TestAPIContract writes every
// endpoint's response to testdata/api/<name>.json, and each one must pass
// the guard the browser reads it with. Every field the server sends must
// be checked: removing it, or giving it another JSON type, must fail the
// guard, unless it is listed as optional (the server leaves it out when
// empty, `omitempty`). A field dropped or renamed in Go therefore fails
// here once the fixtures are rewritten.

interface Fixture {
  route: string;
  request: string;
  status: number;
  body: unknown;
}

const files: Record<string, Fixture> = import.meta.glob(
  "../../../testdata/api/*.json",
  { import: "default", eager: true },
);

const fixtures = Object.entries(files).map(([path, fixture]) => ({
  name: path.slice(path.lastIndexOf("/") + 1, -".json".length),
  fixture,
}));

type Guard = (value: unknown) => boolean;

// Optional paths, from the body's root: "[]" is any array element, "*"
// any one key (a map's) and a leading "**." any path before the rest.
const reasonOptional = [
  "**.blockedBy[].ticket",
  "**.blockedBy[].workstream",
  "**.blockedBy[].column",
  "**.blockedBy[].pending",
  "**.blockedBy[].via",
];
const questionOptional = [
  "**.questions[].ticket",
  "**.questions[].options",
  "**.questions[].answer",
  "**.questions[].answeredBy",
  "**.questions[].answeredAt",
  "**.questions[].delivered",
  "**.questions[].answeredInSession",
  "**.questions[].sessionEnded",
  "**.questions[].mcpOnly",
];
const runOptional = [
  ...questionOptional,
  "**.timeline",
  "**.timeline[].tool",
  "**.timeline[].path",
  "**.timeline[].failed",
  "**.timeline[].tools",
  "**.timeline[].failures",
  "**.timeline[].detail",
  "**.timeline[].ticket",
  "**.timeline[].branch",
];
const cardOptional = [
  ...reasonOptional,
  "cards[].searchText",
  "cards[].live",
  "cards[].sessions",
  "ticket.searchText",
  "ticket.live",
  "ticket.sessions",
];
const commentOptional = ["sentTo", "sentToShort", "warnings"];

const contract: Record<string, { guard: Guard; optional?: string[] }> = {
  info: { guard: isServerInfo },
  projects: { guard: isProjects, optional: ["projects[].image"] },
  "projects-with-image": {
    guard: isProjects,
    optional: ["projects[].image"],
  },
  board: { guard: isBoard, optional: cardOptional },
  "all-board": { guard: isAllBoard, optional: cardOptional },
  runs: { guard: isRuns, optional: runOptional },
  run: { guard: isRunResponse, optional: runOptional },
  changes: { guard: isChanges },
  workstreams: { guard: isWorkstreams, optional: reasonOptional },
  workstream: { guard: isWorkstreamResponse, optional: reasonOptional },
  preferences: {
    guard: isPreferences,
    optional: ["scopes.*", "expandedWorkstreams.*"],
  },
  "preferences-saved": { guard: (value) => value === null },
  "comment-added": { guard: isCommentResponse, optional: commentOptional },
  "comment-sent": { guard: isCommentResponse, optional: commentOptional },
  "comment-edited": { guard: isCommentResponse, optional: commentOptional },
  "request-resolved": { guard: isCommentResponse, optional: commentOptional },
  "changes-requested": { guard: isChangesRequested },
  "comment-deleted": { guard: isCommentDeleted },
  moved: { guard: isSaved },
  patched: { guard: isSaved },
  "section-saved": { guard: isSaved },
  "raw-saved": { guard: isSaved },
  "criterion-set": { guard: isSaved },
  "review-step-set": { guard: isSaved },
  archived: { guard: isSaved },
  unarchived: { guard: isSaved },
  uploaded: { guard: isUploaded },
  "ticket-created": { guard: isCreated },
  "workstream-created": { guard: isCreatedWorkstream },
  "workstream-edited": { guard: isEditedWorkstream },
  "workstream-named": { guard: isEditedWorkstream },
  "workstream-ordered": { guard: isSaved },
  "workstream-section-saved": { guard: isSaved },
  "key-set": { guard: isSaved },
  "project-image-set": { guard: isSaved },
  "project-image-cleared": { guard: isSaved },
  "error-not-an-image": { guard: isErrorBody },
  answered: { guard: isSaved },
  ticket: {
    guard: isTicketResponse,
    // Only a question's history carries it (FH-190).
    optional: [
      ...cardOptional,
      ...runOptional,
      "ticket.history[].changes[].question",
    ],
  },
  "ticket-edited": {
    guard: isTicketResponse,
    optional: [...cardOptional, ...runOptional],
  },
  "ticket-in-review": {
    guard: isTicketResponse,
    optional: [...cardOptional, ...runOptional],
  },
  archive: { guard: isArchiveList },
  "delete-plan": {
    guard: isDeletePlan,
    optional: ["tickets[].slug", "workstreams[].project", "workstreams[].id"],
  },
  "archived-deleted": { guard: isSaved },
  "archive-check": { guard: isArchiveCheck, optional: ["counts.*"] },
  "project-archived": { guard: isSaved },
  "archived-projects": { guard: isArchivedProjects },
  "project-delete-plan": { guard: isProjectDeletePlan },
  "project-restored": { guard: isSaved },
  "project-deleted": { guard: isSaved },
  "error-not-found": { guard: isErrorBody },
  "error-conflict": { guard: isConflictBody },
  "error-confirm-blocked": { guard: isBlockedBody },
  "error-key-taken": { guard: isKeyTakenBody },
  "error-title-taken": { guard: isErrorBody },
  "error-name-exists": { guard: isErrorBody },
};

type Path = (string | number)[];

// Part is one field (key) or array item of a body, by its first path and
// its pattern ("[]" for any array element), so each is mutated once.
interface Part {
  path: Path;
  pattern: string;
  item: boolean;
}

function parts(value: unknown): Part[] {
  const found = new Map<string, Part>();
  const walk = (node: unknown, path: Path, pattern: string) => {
    if (Array.isArray(node)) {
      const next = `${pattern}[]`;
      node.forEach((item, index) => {
        if (!found.has(next))
          found.set(next, {
            path: [...path, index],
            pattern: next,
            item: true,
          });
        walk(item, [...path, index], next);
      });
      return;
    }
    if (typeof node !== "object" || node === null) return;
    for (const [key, item] of Object.entries(node)) {
      const next = pattern ? `${pattern}.${key}` : key;
      if (!found.has(next))
        found.set(next, { path: [...path, key], pattern: next, item: false });
      walk(item, [...path, key], next);
    }
  };
  walk(value, [], "");
  return [...found.values()];
}

// matcher turns an optional path into a test of a part's pattern.
function matcher(optional: string): RegExp {
  const body = optional
    .split(".")
    .map((segment) =>
      segment === "**"
        ? "(?:[^.]+\\.)*"
        : `${segment === "*" ? "[^.]+" : segment.replace(/[[\]]/g, "\\$&")}\\.`,
    )
    .join("");
  return new RegExp(`^${body.slice(0, -2)}$`);
}

function changed(
  value: unknown,
  path: Path,
  change: (
    parent: Record<string | number, unknown>,
    key: string | number,
  ) => void,
): unknown {
  const copy = structuredClone(value);
  let parent = copy as Record<string | number, unknown>;
  for (const key of path.slice(0, -1))
    parent = parent[key] as Record<string | number, unknown>;
  change(parent, path[path.length - 1] as string | number);
  return copy;
}

// otherType is a value of a different JSON type.
function otherType(value: unknown): unknown {
  if (typeof value === "string") return 0;
  if (typeof value === "number") return "0";
  if (typeof value === "boolean") return "true";
  if (Array.isArray(value)) return {};
  if (value === null) return 0;
  return [];
}

describe("API contract fixtures", () => {
  it("has a guard for every fixture and a fixture for every guard", () => {
    expect(fixtures.length).toBeGreaterThan(0);
    expect(fixtures.map(({ name }) => name).sort()).toEqual(
      Object.keys(contract).sort(),
    );
  });

  it("reads optional paths as intended", () => {
    expect(matcher("**.timeline").test("ticket.runs[].timeline")).toBe(true);
    expect(matcher("**.timeline").test("timeline")).toBe(true);
    expect(matcher("**.timeline").test("timelines")).toBe(false);
    expect(matcher("sentTo").test("comment.sentTo")).toBe(false);
    expect(matcher("scopes.*").test("scopes.alpha")).toBe(true);
    expect(matcher("scopes.*").test("scopes.alpha.view")).toBe(false);
  });

  describe.each(fixtures)("$name", ({ name, fixture }) => {
    const entry = contract[name];
    if (!entry) return;
    const { guard, optional = [] } = entry;
    const optionalPaths = optional.map(matcher);
    const isOptional = (pattern: string) =>
      optionalPaths.some((path) => path.test(pattern));

    it("passes its guard", () => {
      expect(guard(fixture.body)).toBe(true);
    });

    it.each(parts(fixture.body))(
      "checks $pattern",
      ({ path, pattern, item }) => {
        const retyped = changed(fixture.body, path, (parent, key) => {
          parent[key] = otherType(parent[key]);
        });
        expect(guard(retyped), `${pattern} is not type-checked`).toBe(false);
        if (item) return;
        const removed = changed(fixture.body, path, (parent, key) => {
          delete parent[key];
        });
        expect(
          guard(removed),
          `${pattern} may be missing; check it or list it as optional`,
        ).toBe(isOptional(pattern));
      },
    );
  });
});
