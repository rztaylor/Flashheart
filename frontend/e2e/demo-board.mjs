// Generates a synthetic, realistic board root for visual checks: a
// 50-ticket project with four workstreams, a smaller project, and the sample
// board's edge cases. All content is invented for testing.
import { cp, mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { deflateSync } from "node:zlib";

const sample = resolve(import.meta.dirname, "../../testdata/boards/sample");

// syntheticScreenshot draws a plain test-report image (header band, passing
// rows) so the Review tab has a realistic attachment. It is labelled synthetic.
function syntheticScreenshot(width = 1200, height = 700) {
  const rows = Buffer.alloc((width * 3 + 1) * height);
  const set = (x, y, [r, g, b]) => {
    const offset = y * (width * 3 + 1) + 1 + x * 3;
    rows[offset] = r;
    rows[offset + 1] = g;
    rows[offset + 2] = b;
  };
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      let colour = [250, 250, 250];
      if (y < 64) colour = [17, 17, 17];
      else if (y > 96 && (y - 96) % 56 < 36 && x > 48 && x < width - 48) {
        const row = Math.floor((y - 96) / 56);
        if (x < 84) colour = [0, 132, 61];
        else if (x < 120) colour = [250, 250, 250];
        else
          colour =
            x < 120 + 420 + ((row * 97) % 380)
              ? [222, 224, 228]
              : [250, 250, 250];
      }
      set(x, y, colour);
    }
  }
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  const crc = (buffer) => {
    let c = 0xffffffff;
    for (const byte of buffer) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type, data) => {
    const length = Buffer.alloc(4);
    length.writeUInt32BE(data.length);
    const body = Buffer.concat([Buffer.from(type), data]);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(body));
    return Buffer.concat([length, body, sum]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8;
  header[9] = 2;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", header),
    chunk("IDAT", deflateSync(rows)),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

// [slug suffix, title, type, priority, status, workstream, depends-on,
// depends-on-workstreams, tags]. Ids follow row order.
const flashheart = [
  [
    "go-module",
    "Go module and CLI skeleton",
    "infra",
    "high",
    "done",
    "foundation",
  ],
  [
    "singleserve-shell",
    "Singleserve shell with lifecycle UI",
    "feature",
    "high",
    "done",
    "foundation",
  ],
  [
    "check-script",
    "Strict validation script",
    "infra",
    "medium",
    "done",
    "foundation",
  ],
  [
    "detached-serve",
    "Detach serve from the terminal",
    "feature",
    "high",
    "done",
    "foundation",
  ],
  [
    "serve-log",
    "Log background diagnostics to serve.log",
    "feature",
    "medium",
    "done",
    "foundation",
  ],
  [
    "markdown-parser",
    "Frontmatter and section parser",
    "feature",
    "high",
    "done",
    "board-core",
  ],
  [
    "blocking-rules",
    "Blocking rules with reasons",
    "feature",
    "high",
    "done",
    "board-core",
  ],
  [
    "store-read",
    "Store read side on os.Root",
    "feature",
    "high",
    "review",
    "board-core",
  ],
  [
    "board-index",
    "Revisioned board index",
    "feature",
    "high",
    "review",
    "board-core",
  ],
  [
    "read-api",
    "Read API for projects, boards and tickets",
    "feature",
    "high",
    "in-progress",
    "board-core",
  ],
  [
    "transit-ui",
    "Transit map board UI",
    "feature",
    "high",
    "in-progress",
    "board-core",
  ],
  [
    "workstream-diagram",
    "Workstreams as transit lines",
    "feature",
    "medium",
    "up-next",
    "board-core",
  ],
  [
    "table-view",
    "Sortable table view",
    "feature",
    "medium",
    "up-next",
    "board-core",
  ],
  [
    "locked-writes",
    "Locked atomic ticket writes",
    "feature",
    "high",
    "up-next",
    "board-editing",
    ["store-read"],
  ],
  [
    "drag-and-drop",
    "Drag cards between columns",
    "feature",
    "high",
    "backlog",
    "board-editing",
  ],
  [
    "conflict-dialog",
    "Show save conflicts side by side",
    "feature",
    "medium",
    "backlog",
    "board-editing",
  ],
  [
    "new-ticket",
    "New ticket from the board",
    "feature",
    "medium",
    "backlog",
    "board-editing",
  ],
  [
    "live-updates",
    "Live updates by long-polling",
    "feature",
    "high",
    "backlog",
    "board-editing",
  ],
  [
    "event-log",
    "Append-only event log with rotation",
    "feature",
    "high",
    "backlog",
    "agent-runs",
    [],
    ["board-editing"],
  ],
  [
    "run-state",
    "Run state derivation table",
    "feature",
    "high",
    "backlog",
    "agent-runs",
  ],
  [
    "claude-hooks",
    "Claude Code hook adapter",
    "feature",
    "high",
    "backlog",
    "agent-runs",
  ],
  [
    "agents-view",
    "Agents view with run lanes",
    "feature",
    "medium",
    "backlog",
    "agent-runs",
  ],
  [
    "recovery-note",
    "Recovery note on session start",
    "feature",
    "high",
    "backlog",
    "agent-runs",
  ],
  [
    "column-overflow",
    "Long titles push the column wider than its track",
    "bug",
    "medium",
    "up-next",
    "",
    ["transit-ui"],
  ],
  [
    "focus-ring-dark",
    "Focus ring invisible on the dark signage band",
    "bug",
    "high",
    "in-progress",
    "",
  ],
  [
    "slow-index-nfs",
    "Index rebuild slow on network home folders",
    "bug",
    "low",
    "backlog",
    "",
  ],
  [
    "stale-done-sort",
    "Done column sorts by name after reload",
    "bug",
    "medium",
    "done",
    "",
  ],
  [
    "hook-latency",
    "Measure hook latency on a warm cache",
    "test",
    "medium",
    "backlog",
    "",
  ],
  [
    "fixture-generator",
    "Generated 5,000-ticket fixture",
    "test",
    "medium",
    "done",
    "",
  ],
  [
    "playwright-lifecycle",
    "Playwright lifecycle suite",
    "test",
    "high",
    "done",
    "",
  ],
  [
    "axe-checks",
    "axe-core checks in both themes",
    "test",
    "medium",
    "review",
    "",
  ],
  ["release-notes", "Release notes template", "docs", "low", "backlog", ""],
  ["user-guide", "First-run user guide", "docs", "medium", "backlog", ""],
  [
    "protocol-skill",
    "Protocol skill text",
    "docs",
    "high",
    "backlog",
    "",
    ["recovery-note"],
  ],
  [
    "board-format-doc",
    "Document workstream blocking",
    "docs",
    "medium",
    "done",
    "",
  ],
  [
    "split-api",
    "Split API handlers by resource",
    "refactor",
    "low",
    "backlog",
    "",
  ],
  [
    "rename-store-errors",
    "Name store errors consistently",
    "refactor",
    "low",
    "done",
    "",
  ],
  [
    "windows-support",
    "Decide on Windows support",
    "spike",
    "low",
    "backlog",
    "",
    [],
    [],
    ["later-possibility"],
  ],
  [
    "session-logs",
    "Passive session-log reading",
    "spike",
    "low",
    "backlog",
    "",
    [],
    [],
    ["later-possibility"],
  ],
  [
    "git-history",
    "Board history from git",
    "spike",
    "low",
    "backlog",
    "",
    [],
    [],
    ["later-possibility"],
  ],
  ["notarise", "Notarise macOS builds", "infra", "medium", "backlog", ""],
  ["homebrew-tap", "Homebrew tap", "infra", "low", "backlog", "", ["notarise"]],
  [
    "ci-workflow",
    "CI workflow once the remote exists",
    "infra",
    "medium",
    "backlog",
    "",
  ],
  [
    "dependabot",
    "Dependabot for Go and npm",
    "infra",
    "low",
    "backlog",
    "",
    ["ci-workflow"],
  ],
  [
    "mcp-server",
    "MCP server with board_context",
    "feature",
    "high",
    "backlog",
    "",
    ["run-state"],
  ],
  [
    "claims",
    "Claims with leases",
    "feature",
    "high",
    "backlog",
    "",
    ["mcp-server"],
  ],
  [
    "checkpoint-tool",
    "Checkpoint tool rewrites the handoff",
    "feature",
    "high",
    "backlog",
    "",
    ["claims"],
  ],
  [
    "ask-human",
    "ask_human questions and answers",
    "feature",
    "medium",
    "backlog",
    "",
    ["mcp-server"],
  ],
  [
    "setup-claude",
    "setup claude shows a diff",
    "feature",
    "medium",
    "backlog",
    "",
  ],
  [
    "handoff-enforcement",
    "Opt-in handoff enforcement at Stop",
    "feature",
    "low",
    "backlog",
    "",
    ["checkpoint-tool"],
  ],
];

const workstreams = {
  flashheart: [
    ["foundation", "Foundation", "2026-09-20"],
    ["board-core", "Board core", "2026-10-01"],
    ["board-editing", "Board editing", "2026-10-03", ["board-core"]],
    ["agent-runs", "Agent runs", "2026-10-04", ["board-editing"]],
  ],
  ngplus: [
    ["ofqual-layer", "Ofqual regulatory layer", "2026-09-12"],
    ["study-ui", "Study UI", "2026-09-25"],
  ],
};

const ngplus = [
  [
    "board-label-claims",
    "Board label claims a known specification",
    "bug",
    "high",
    "in-progress",
    "ofqual-layer",
  ],
  [
    "assessment-objectives",
    "Ofqual assessment objectives",
    "feature",
    "high",
    "up-next",
    "ofqual-layer",
  ],
  [
    "grade-boundaries",
    "Grade boundaries per series",
    "feature",
    "medium",
    "backlog",
    "ofqual-layer",
  ],
  [
    "study-page",
    "Study page layout",
    "feature",
    "medium",
    "review",
    "study-ui",
  ],
  [
    "flashcards",
    "Flashcards from past papers",
    "feature",
    "medium",
    "backlog",
    "study-ui",
  ],
  ["timer", "Exam timer", "feature", "low", "done", "study-ui"],
  ["seed-data", "Seed data for three subjects", "infra", "medium", "done", ""],
  [
    "spec-links",
    "Link answers to specification points",
    "feature",
    "low",
    "backlog",
    "",
    ["assessment-objectives"],
  ],
];

const keys = { flashheart: "FH", ngplus: "NG" };

// ticket renders one row as a format v2 ticket. Dependencies name row
// suffixes and resolve to ids.
function ticket(
  project,
  ids,
  [
    suffix,
    title,
    type,
    priority,
    status,
    workstream,
    deps = [],
    depWs = [],
    tags = [],
  ],
  index,
) {
  const id = ids.get(suffix);
  const created = `2026-09-${String(10 + Math.floor(index / 3)).padStart(2, "0")}`;
  const lines = [
    "---",
    `id: ${id}`,
    `status: ${status}`,
    `type: ${type}`,
    `project: ${project}`,
    `created: ${created}`,
    `priority: ${priority}`,
    "session: demo",
    "git-ref: 0000000",
    `branch: ${status === "in-progress" ? `feature/${suffix}` : ""}`,
    `workstream: ${workstream}`,
    `depends-on: [${deps.map((dep) => ids.get(dep)).join(", ")}]`,
    `depends-on-workstreams: [${depWs.join(", ")}]`,
    `tags: [${tags.join(", ")}]`,
    "---",
    "",
    `# ${title}`,
    "",
    "## Description",
    "",
    `${title}. Synthetic demo ticket generated for visual checks; it describes plausible work on ${project}.`,
    "",
    "## Acceptance Criteria",
    "",
    "- [x] Failing test written first",
    `- [${status === "done" || status === "review" ? "x" : " "}] Behaviour implemented`,
    `- [${status === "done" ? "x" : " "}] Docs and changelog updated`,
    "",
    "## Notes",
    "",
    "None.",
    "",
  ];
  if (status === "in-progress") {
    lines.push(
      "## Handoff",
      "",
      "_Updated 2026-10-04 13:20 UTC by claude:3f2a9c1e (run)._",
      "",
      "**Done**",
      "- Wrote the failing test",
      "**Next**",
      `- Finish ${title.toLowerCase()}`,
      "- Run scripts/check.sh",
      "**Files**",
      "- internal/example/example.go",
      "**Open questions**",
      "- None",
      "",
    );
  }
  return {
    id,
    folder: `${id}-${suffix}`,
    workstream,
    content: lines.join("\n"),
  };
}

async function writeProject(root, project, display, rows) {
  const key = keys[project];
  const ids = new Map(
    rows.map(([suffix], index) => [suffix, `${key}-${index + 1}`]),
  );
  const tickets = rows.map((row, index) => ticket(project, ids, row, index));
  for (const item of tickets) {
    const dir = join(root, project, "tickets", item.folder);
    await mkdir(dir, { recursive: true });
    await writeFile(join(dir, `${item.folder}.md`), item.content);
  }
  await writeFile(
    join(root, project, "project.yaml"),
    `key: ${key}\nnext_id: ${rows.length + 1}\nname: ${display}\nrepos:\n  - /Users/example/src/${project}\n`,
  );
  for (const [slug, title, created, deps = []] of workstreams[project] ?? []) {
    const members = tickets
      .filter((item) => item.workstream === slug)
      .map((item) => item.id);
    const body = [
      "---",
      `slug: ${slug}`,
      "status: active",
      "priority: high",
      `created: ${created}`,
      "tickets:",
      ...members.map((member) => `  - ${member}`),
      `depends-on-workstreams: [${deps.join(", ")}]`,
      "tags: []",
      "---",
      "",
      `# ${title}`,
      "",
      "## Goal",
      "",
      `Deliver ${title.toLowerCase()}.`,
      "",
    ].join("\n");
    await mkdir(join(root, project, "workstreams"), { recursive: true });
    await writeFile(join(root, project, "workstreams", `${slug}.md`), body);
  }
  return new Map(tickets.map((item) => [item.id, item.folder]));
}

export async function writeDemoBoard(root) {
  await cp(sample, root, { recursive: true });
  const folders = await writeProject(
    root,
    "flashheart",
    "Flashheart",
    flashheart,
  );
  await writeProject(root, "ngplus", "NG+", ngplus);
  // A review with a screenshot for the Review tab.
  const reviewDir = join(root, "flashheart", "tickets", folders.get("FH-8"));
  await mkdir(join(reviewDir, "files"), { recursive: true });
  await writeFile(
    join(reviewDir, "files", "20261004T1412-store-tests.png"),
    syntheticScreenshot(),
  );
  await writeFile(
    join(reviewDir, "files", "index.yaml"),
    "- file: 20261004T1412-store-tests.png\n  caption: Store tests passing (synthetic)\n  kind: screenshot\n  run: claude:3f2a9c1e\n  added: 2026-10-04T14:12:09Z\n",
  );
  await writeFile(
    join(reviewDir, "review.md"),
    [
      "# Review: Store read side on os.Root",
      "",
      `**Work Item:** [FH-8](${folders.get("FH-8")}.md)`,
      "",
      "## Summary",
      "",
      "Reads every project through an `os.Root`, so symlinks cannot escape the board root. See the [os.Root documentation](https://pkg.go.dev/os#Root).",
      "",
      "## How to Verify",
      "",
      "1. Run `scripts/check.sh`.",
      "2. Symlink a ticket to a file outside the root and open the board: it shows as needs repair.",
      "",
      "![Store tests passing](files/20261004T1412-store-tests.png)",
      "",
      "## Risks / Things to Watch",
      "",
      "- Network home folders are slower to index (FH-26).",
      "",
    ].join("\n"),
  );
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const target = process.argv[2];
  if (!target) {
    console.error("usage: node demo-board.mjs <empty directory>");
    process.exit(2);
  }
  await writeDemoBoard(resolve(target));
}
