// Ticket page concept mocks: icons, theme, tabs and FH-63's real content
// (from ~/reports/Kanban/Flashheart/tickets/FH-63-*). Mock only.

const ICONS = {
  board: "M4 4h4.5v16H4zM9.75 4h4.5v10h-4.5zM15.5 4H20v13h-4.5z",
  overview: "M4.5 4h15v5h-15zM4.5 12h15v3.5h-15zM4.5 19.5h9",
  lines: "M3 7h6l3 5 3-5h6M3 17h18",
  table: "M3.5 5h17v14h-17zM3.5 10h17M3.5 14.5h17M9 10v9",
  power: "M12 3v8M7 6.3a8 8 0 1 0 10 0",
  review: "M7 3.5h7l4.5 4.5v12.5H7zM14 3.5V8h4.5M10 13l2 2 3.5-4",
  next: "M5 12h12M13 7l5 5-5 5",
  chevronDown: "M6 9l6 6 6-6",
  branch: "M6 4v10m0 0a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Zm12-6a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Zm0 0c0 5-6 4-12 6",
  edit: "M4 20h4L19 9l-4-4L4 16v4ZM13.5 6.5l4 4",
  check: "M5 12.5l4.5 4.5L19 7.5",
  archive: "M3.5 5h17v4h-17zM5 9v10h14V9M10 13h4",
  attachment: "M16.5 7.5 9 15a2.1 2.1 0 0 0 3 3l7.5-7.5a4.2 4.2 0 0 0-6-6L6 12a6.4 6.4 0 0 0 9 9",
  criteria: "M4.5 4.5h15v15h-15zM8 12l3 3 5-6",
  external: "M14 4h6v6M20 4l-9 9M18 14v6H4V6h6",
  more: "M5 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0M11 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0M17 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0",
  plus: "M12 5v14M5 12h14",
  undo: "M9 7 4 12l5 5M4 12h11a5 5 0 0 1 0 10h-2",
  warning: "M12 4 21 19.5H3L12 4Zm0 6v4.5m0 2.6v.1",
  question: "M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6V14M12 17.5v.1M12 3.5a8.5 8.5 0 1 0 0 17 8.5 8.5 0 0 0 0-17Z",
  note: "M5 4.5h14v15H5zM8.5 9h7M8.5 12.5h7M8.5 16h4",
  link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
  bolt: "M13 2 4 14h7l-1 8 9-12h-7l1-8Z",
  close: "M6 6l12 12M18 6 6 18",
  ring: "M12 4.5a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15Z",
  file: "M7 3.5h7l4.5 4.5v12.5H7zM14 3.5V8h4.5",
  image: "M4 5h16v14H4zM4 15l4.5-4.5 4 4 2.5-2.5L20 17M15 9.5h.01",
};

const svg = (name, cls = "") =>
  `<svg class="i ${cls}" viewBox="0 0 24 24" aria-hidden="true"><path d="${ICONS[name]}"/></svg>`;

function hydrateIcons(root = document) {
  root.querySelectorAll("i[data-i]").forEach((el) => {
    el.outerHTML = svg(el.dataset.i, el.className);
  });
}

// ---- FH-63 content -------------------------------------------------------

const A = "assets/20261011T0854-overview-pipeline-";
const SHOTS = [
  { src: A + "1280-light.jpg", title: "All projects, 1280, light", cap: "All projects at 1280 px, light theme." },
  { src: A + "1280-dark.jpg", title: "All projects, 1280, dark", cap: "All projects at 1280 px, dark theme." },
  { src: A + "1920-light.jpg", title: "All projects, 1920, light", cap: "1920 px, light: the largest count size." },
  { src: A + "1920-dark.jpg", title: "All projects, 1920, dark", cap: "1920 px, dark." },
  { src: A + "390-light.jpg", title: "Phone, light", cap: "390 px, light: one row, icons hidden, words wrap, no page scroll.", phone: true },
  { src: A + "390-dark.jpg", title: "Phone, dark", cap: "390 px, dark.", phone: true },
];

const CRITERIA = [
  "The Overview headline shows five stages in board order, Backlog, Up next, In progress, Ready to review and Done, each with the column's full count for the project shown, or summed across projects for All projects",
  "Stages are drawn as interlocking chevron cards that point left to right, in both themes, with WCAG AA text contrast",
  "Each stage is a link to the Board for that scope, with an accessible name naming the stage and its count",
  "Narrow widths keep the pipeline readable without horizontal page scroll",
  "The since-your-last-change label and tiles, GET /api/metrics and the backend change counting used only by them are removed, with their tests",
  "SPEC VIEW-3, ui-layout.md, DESIGN.md, the Overview BOUNDARY.md and the changelog describe the pipeline",
];

const STEPS = [
  "Open Overview with All projects selected. Compare the five counts with the Board's column counts.",
  "Select Flashheart. Ready to review should match the Ready for your review section's count.",
  "Hover a stage, then Tab through the stages with the keyboard.",
  "Click any stage.",
  "Switch to the dark theme and narrow the window to phone width.",
];

const RISKS = [
  "Done leaves out Rejected tickets (your decision), so it can be lower than the Board's Done column count.",
  "Backlog and Up next share the neutral state colour, as the State role defines.",
  "At phone width “Ready to review” wraps to three short lines.",
  "The focus ring is a drop-shadow filter. It renders in Chromium (checked) and is standard CSS, but it was not checked in Safari or Firefox.",
];

const KEY_FILES = [
  ["frontend/src/features/overview/PipelineHeadline.tsx", "New: the chevron row, links, accessible names"],
  ["frontend/src/model/overview.ts", "<code>pipelineStages</code> (per project or summed); headline label/tiles removed"],
  ["frontend/src/styles/index.css, tokens.css", "<code>chevron</code>, <code>chevron-start</code>, <code>chevron-ring</code> utilities; <code>--fh-shadow-chevron</code> in both themes"],
  ["internal/api/board.go", "Project summaries carry a <code>rejected</code> count, subtracted from Done"],
  ["frontend/src/components/Pill.tsx", "State tone class and icon maps exported for reuse"],
  ["frontend/src/app/Shell.tsx, OverviewView.tsx", "Board link passed in; metrics fetch removed"],
  ["internal/api/metrics.go, internal/index/metrics.go, …", "FH-52's endpoint, counting and change list removed; FH-49's LatestHuman kept"],
  ["frontend/e2e/runs.spec.mjs", "FH-52 test replaced by the FH-63 pipeline test"],
  ["docs/SPEC.md, ui-layout.md, DESIGN.md, …", "Describe the pipeline"],
];

const TESTS = [
  "<code>model/overview.test.ts</code> <code>pipelineStages</code>: order, names and tones; Done without Rejected; summed across projects; an unknown project counts zeros. Written first and failing.",
  "<code>internal/api</code> <code>TestProjectsCountRejectedTickets</code>: a rejected ticket counts in Done and in <code>rejected</code>. Written first and failing.",
  "<code>PipelineHeadline.test.tsx</code>: a named Pipeline list of five links with accessible names and Board hrefs; text order; stage and tone markers. Written first and failing.",
  "e2e <em>the Overview's pipeline counts every column of the scope (FH-63)</em>: counts update live; clip-path present; axe-core passes; at 390 px one row with no page scroll in both themes.",
  "<code>scripts/check.sh</code> passes; <code>scripts/e2e.sh</code> passes (55/55) on the final branch.",
];

// Notes and the question, oldest first, as the ticket records them.
const ACTIVITY = [
  { kind: "note", who: "claude:bf8c1187", when: "11 Oct", text: "All six criteria met. Evidence: PipelineHeadline and pipelineStages unit tests; the rewritten e2e test in <code>frontend/e2e/runs.spec.mjs</code>; <code>scripts/check.sh</code> and <code>scripts/e2e.sh</code> (55/55) pass. Committed 644c0d0 on feature/overview-pipeline and pushed; PR not opened yet." },
  { kind: "question", who: "claude:bf8c1187", when: "11 Oct", text: "FH-63 is built, tested and pushed, and the review has screenshots. Shall I run the pre-PR review, open the PR, and move FH-63 to Ready to review? Also: Done currently counts Rejected tickets. Keep that, or count accepted work only?", answer: "Done excludes rejected. Run the /pre-pr-review and then open the PR.", answerBy: "Robert Taylor" },
  { kind: "note", who: "claude:bf8c1187", when: "11 Oct", text: "The user decided that Done excludes Rejected tickets. Project summaries now carry <code>rejected</code>, and pipelineStages subtracts it from Done (d75681b). Pre-PR review: the styles boundary now names the chevron utilities (76c8420)." },
  { kind: "handoff", who: "claude:bf8c1187", when: "11 Oct · 11:29", text: "Handoff updated. Next: human verifies per the review and merges." },
];

const C = {
  description: `
    <p>The Overview's headline metrics (<a href="#">FH-52</a>) count agent changes since the user's last change on the board. They confused more than they helped: “To review 0” sat above “Ready for your review 4”, because the tile counted arrivals since 09:24 while the section lists what is waiting now.</p>
    <p>Replace them with the project's pipeline: Backlog › Up next › In progress › Ready to review › Done, one stage per board column with its full count, using the Board's own column names. Drawn as interlocking chevron cards that read as movement from left to right, and visually strong enough to be the page's headline. For All projects the counts are summed across projects. Each stage links to the Board. Counts come from the project summaries the shell already loads (<code>ProjectSummary.counts</code>), so no new API; <code>GET /api/metrics</code> and the change counting behind it are removed.</p>
    <p>Done is all-time (the whole Done column, including rejected tickets; archived tickets stay on the Archive link). The other stages are a snapshot of now, so the strip can never disagree with the sections below it.</p>`,
  testPlan: `<p>Red first: a model test for <code>pipelineStages(projects, project)</code> (order, names, per-project and summed counts, missing project gives zeros) and a component test for <code>PipelineHeadline</code> (five links in order with names like “Ready to review: 4 tickets”, hrefs to the Board). Update the Overview e2e in <code>frontend/e2e/runs.spec.mjs</code> to assert the five stages instead of the old tiles, and take screenshots at 1280 and 1920 in light and dark plus a narrow width; axe-core passes. Remove the Go tests for /api/metrics and the change counting together with the code.</p>`,
  summary: `<p>The Overview now opens with the scope's pipeline: Backlog, Up next, In progress, Ready to review and Done as one row of interlocking chevron cards pointing left to right. Each card shows the column's whole count over its word, in its status pill colours, and links to the Board. Done counts accepted work only: Rejected tickets are left out. FH-52's <em>Since your last change</em> tiles are gone, along with <code>GET /api/metrics</code> and the change counting behind them.</p>`,
  prereq: `<p>Build and install this branch (<code>scripts/build.sh</code>, then replace <code>~/bin/flashheart</code> via a temp file and <code>mv</code>), and open the board.</p>`,
  expected: `<ul>
      <li>Counts match the Board. Done is the Done column minus its Rejected tickets, and archived tickets are not counted.</li>
      <li>Hovering lifts a stage 2px. The focus ring follows the chevron shape, not a rectangle.</li>
      <li>Clicking a stage opens the Board for the same scope.</li>
      <li>Both themes read clearly. At phone width the five stages stay on one row, with no horizontal scroll.</li>
    </ul>`,
};

const checkItem = (text, on) =>
  `<li><span class="box ${on ? "on" : ""}">${on ? svg("check") : ""}</span><span>${text}</span></li>`;
const criteriaList = (compact = false) =>
  `<ul class="checklist ${compact ? "compact" : ""}">${CRITERIA.map((c) => checkItem(c, true)).join("")}</ul>`;
const stepsList = (done = 0) =>
  `<ul class="checklist">${STEPS.map((s, i) => checkItem(s, i < done)).join("")}</ul>`;
const shotsGrid = (list = SHOTS) =>
  `<div class="shots">${list
    .map((s) => `<figure class="shot ${s.phone ? "phone" : ""}"><img src="${s.src}" alt="${s.title}"><figcaption><b>${s.title}</b>${s.cap}</figcaption></figure>`)
    .join("")}</div>`;
const keyFilesTable = () =>
  `<table class="files"><thead><tr><th>File</th><th>What changed</th></tr></thead><tbody>${KEY_FILES.map(
    ([f, w]) => `<tr><td class="mono">${f}</td><td>${w}</td></tr>`,
  ).join("")}</tbody></table>`;

function editBtn(label) {
  return `<button class="icon-btn edit" aria-label="Edit ${label}" title="Edit (or double-click the section)">${svg("edit")}</button>`;
}

// Fill [data-c] slots from the content above.
function fill(root = document) {
  const map = {
    description: () => C.description,
    testPlan: () => C.testPlan,
    summary: () => C.summary,
    prereq: () => C.prereq,
    expected: () => C.expected,
    criteria: () => criteriaList(false),
    criteriaCompact: () => criteriaList(true),
    steps: () => stepsList(0),
    shots: () => shotsGrid(),
    keyFiles: () => keyFilesTable(),
    tests: () => `<ul class="prose-list">${TESTS.map((t) => `<li>${t}</li>`).join("")}</ul>`,
    risks: () => `<ul class="prose-list">${RISKS.map((t) => `<li>${t}</li>`).join("")}</ul>`,
  };
  root.querySelectorAll("[data-c]").forEach((el) => {
    const f = map[el.dataset.c];
    if (f) el.innerHTML = f();
  });
  root.querySelectorAll("[data-edit]").forEach((el) => {
    el.insertAdjacentHTML("beforeend", editBtn(el.dataset.edit));
  });
}

// Tabs: [role=tablist] buttons with aria-controls.
function wireTabs(root = document) {
  root.querySelectorAll("[role=tablist]").forEach((list) => {
    list.addEventListener("click", (e) => {
      const b = e.target.closest("[role=tab]");
      if (!b) return;
      const ids = (t) => (t.dataset.panels || t.getAttribute("aria-controls")).split(" ");
      const tabs = [...list.querySelectorAll("[role=tab]")];
      tabs.forEach((t) => {
        t.setAttribute("aria-selected", t === b);
        ids(t).forEach((id) => document.getElementById(id)?.setAttribute("hidden", ""));
      });
      ids(b).forEach((id) => document.getElementById(id)?.removeAttribute("hidden"));
    });
  });
}

function wireTheme() {
  const params = new URLSearchParams(location.search);
  const set = (t) => {
    document.documentElement.dataset.theme = t;
    document.querySelectorAll("[data-theme-btn]").forEach((b) => b.setAttribute("aria-pressed", b.dataset.themeBtn === t));
  };
  set(params.get("theme") || "light");
  document.querySelectorAll("[data-theme-btn]").forEach((b) => b.addEventListener("click", () => set(b.dataset.themeBtn)));
  const tab = params.get("tab");
  if (tab) document.querySelector(`[role=tab][data-name="${tab}"]`)?.click();
}

const BAND = `
  <header class="band">
    <a class="wordmark" href="b-outline.html">${svg("bolt")}Flashheart</a>
    <nav aria-label="Views">
      <a href="#">${svg("overview")}Overview</a>
      <a href="#">${svg("lines")}Workstreams</a>
      <a href="#">${svg("table")}Table</a>
      <a href="#">${svg("board")}Board</a>
    </nav>
    <span class="spacer"></span>
    <span class="mock-switch" role="group" aria-label="Mock theme">
      <button data-theme-btn="light">Light</button><button data-theme-btn="dark">Dark</button>
    </span>
    <span class="dot" title="Connected"></span>
    <button class="quit">${svg("power")}Quit</button>
  </header>`;

document.addEventListener("DOMContentLoaded", () => {
  document.body.insertAdjacentHTML("afterbegin", BAND);
  fill();
  hydrateIcons();
  wireTabs();
  wireTheme();
  window.dispatchEvent(new Event("mock-ready"));
});
