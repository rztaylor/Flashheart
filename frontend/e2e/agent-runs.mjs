// Drives the real `flashheart hook claude` command to put live agent runs on
// a sandbox board: the same path Claude Code takes. All sessions, repositories
// and task lists are synthetic test data. The task-list tools (TodoWrite,
// TaskCreate, TaskUpdate) are ordinary tool uses: hooks record no plans
// (D51).
import { execFileSync, spawn } from "node:child_process";
import { mkdir, readFile, realpath, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { executable } from "./support.mjs";

// IDLE_SESSION waits for a prompt in the gamma repository.
export const IDLE_SESSION = "e2d8f6a4-9b1c-4c3e-b5a7-1f0e9d8c7b6a";
// ASKING_SESSION waits on a permission in the alpha repository (AL-3).
export const ASKING_SESSION = "3f2a9c1e-7b44-4d0e-9a51-6c8e2f1d0b7a";

async function repo(home, name, branch, parent = "src") {
  const dir = join(home, parent, name);
  await mkdir(join(dir, ".git"), { recursive: true });
  await writeFile(join(dir, ".git", "HEAD"), `ref: refs/heads/${branch}\n`);
  // Hooks name repositories by real path (macOS temp folders sit behind a
  // symlink), so the sandbox does too.
  return realpath(dir);
}

function hook(root, event, payload, agent = "claude") {
  execFileSync(executable, ["hook", agent, event, "--root", root], {
    input: JSON.stringify({ hook_event_name: event, ...payload }),
    env: { ...process.env, PATH: "/nonexistent" },
  });
}

// CODEX_PAIR is two Codex session ids from the same minute: time-ordered
// UUIDs (version 7) that differ only at the end, as Codex's do (FH-122).
export const CODEX_PAIR = [
  "019a7c00-0000-7000-8000-000000000001",
  "019a7c00-0000-7000-8000-0000000000ff",
];

// seedCodexPair records the CODEX_PAIR sessions through the real
// `flashheart hook codex`, both working in a checkout of the flashheart
// project on a branch no ticket is on.
export async function seedCodexPair(home, root) {
  const cwd = await repo(home, "flashheart", "feature/codex-pair", "codex");
  const projectFile = join(root, "flashheart", "project.yaml");
  const project = await readFile(projectFile, "utf8");
  await writeFile(
    projectFile,
    project.replace("repos:\n", `repos:\n  - ${cwd}\n`),
  );
  for (const [index, id] of CODEX_PAIR.entries()) {
    const send = (event, extra = {}) =>
      hook(root, event, { session_id: id, cwd, ...extra }, "codex");
    send("SessionStart", { source: "startup" });
    send("UserPromptSubmit", {
      turn_id: `019a7c00-0000-7000-8000-00000000010${index}`,
      prompt: "synthetic",
    });
  }
  return cwd;
}

function session(root, id, cwd) {
  const send = (event, extra = {}) =>
    hook(root, event, { session_id: id, cwd, ...extra });
  const tool = (name, input = {}, extra = {}) =>
    send("PostToolUse", {
      tool_name: name,
      tool_input: input,
      tool_response: {},
      ...extra,
    });
  return { send, tool };
}

// seedRuns records four sessions: one in alpha waiting on a permission (its
// branch links it to AL-3) with three subagents, one in beta working with an Explore subagent,
// one in a new repository waiting for a prompt, and one in alpha that ended
// after edits without a checkpoint.
export async function seedRuns(home, root) {
  const alpha = await repo(home, "alpha", "feature/card-panel");
  // The sample alpha project records its repository; claim this checkout as
  // a second path of it so the hook files alpha's runs under alpha (PRJ-3).
  const projectFile = join(root, "alpha", "project.yaml");
  const project = await readFile(projectFile, "utf8");
  await writeFile(
    projectFile,
    project.replace("repos:\n", `repos:\n  - ${alpha}\n`),
  );
  const beta = await repo(home, "beta", "main");
  const gamma = await repo(home, "gamma", "spike/offline-sync");

  const ended = session(root, "9d01b2aa-5c1e-4f7a-8b3d-2e6f0a9c4d11", alpha);
  ended.send("SessionStart", { source: "startup" });
  ended.send("UserPromptSubmit", { prompt: "synthetic" });
  ended.tool("Edit", { file_path: join(alpha, "src/panel/Tabs.tsx") });
  ended.tool("Edit", { file_path: join(alpha, "src/panel/ReviewTab.tsx") });
  ended.send("Stop");
  ended.send("SessionEnd", { reason: "prompt_input_exit" });

  const asking = session(root, ASKING_SESSION, alpha);
  asking.send("SessionStart", { source: "resume" });
  asking.send("UserPromptSubmit", { prompt: "synthetic" });
  asking.tool("TodoWrite", {
    todos: [
      { content: "Ticket tab", status: "completed" },
      { content: "Review tab", status: "completed" },
      { content: "Runs tab timeline", status: "in_progress" },
      { content: "Keyboard checks", status: "pending" },
      { content: "Screenshots for review", status: "pending" },
    ],
  });
  asking.tool("Read", { file_path: join(alpha, "src/panel/CardPanel.tsx") });
  asking.tool("Read", { file_path: join(alpha, "src/panel/Tabs.tsx") });
  asking.tool("Edit", { file_path: join(alpha, "src/panel/RunsTab.tsx") });
  // It orchestrates three subagents: one finished, two still working, one
  // of them keeping its own task list (agent-protocol §10).
  const sub = (id, type) => ({ agent_id: id, agent_type: type });
  asking.send("SubagentStart", sub("c1a7e3b9", "Explore"));
  asking.tool("Grep", {}, sub("c1a7e3b9", "Explore"));
  asking.send("SubagentStop", sub("c1a7e3b9", "Explore"));
  asking.send("SubagentStart", sub("c2b8f4ca", "general-purpose"));
  asking.tool(
    "TodoWrite",
    {
      todos: [
        { content: "Read the tab", status: "completed" },
        { content: "Write the timeline test", status: "in_progress" },
      ],
    },
    sub("c2b8f4ca", "general-purpose"),
  );
  asking.tool(
    "Edit",
    { file_path: join(alpha, "src/panel/RunsTab.test.tsx") },
    sub("c2b8f4ca", "general-purpose"),
  );
  asking.send("SubagentStart", sub("c3c9a5db", "Plan"));
  asking.tool("Read", {}, sub("c3c9a5db", "Plan"));
  asking.send("PermissionRequest", {
    tool_name: "Bash",
    tool_input: { command: "synthetic" },
  });

  const working = session(root, "b7c4e9f2-1a3d-4e5f-8a9b-0c1d2e3f4a5b", beta);
  working.send("SessionStart", { source: "startup" });
  working.send("UserPromptSubmit", { prompt: "synthetic" });
  working.tool(
    "TaskCreate",
    { subject: "Map the hello flow" },
    { tool_response: { task: { id: "1" } } },
  );
  working.tool(
    "TaskCreate",
    { subject: "Write the greeting test" },
    { tool_response: { task: { id: "2" } } },
  );
  working.tool("TaskUpdate", { taskId: "1", status: "in_progress" });
  working.send("SubagentStart", {
    agent_id: "a5e1c0d7",
    agent_type: "Explore",
  });
  working.tool("Grep", {}, { agent_id: "a5e1c0d7", agent_type: "Explore" });
  working.tool("Read", {}, { agent_id: "a5e1c0d7", agent_type: "Explore" });

  const idle = session(root, IDLE_SESSION, gamma);
  idle.send("SessionStart", { source: "startup" });
  idle.send("UserPromptSubmit", { prompt: "synthetic" });
  idle.tool("Read", {});
  idle.send("Stop");
  return { alpha, beta, gamma };
}

// seedClaimedRun records a session in the demo board's flashheart project
// that claims ticket through the real MCP server and runs three subagents,
// one of them finished: an In progress row on the Overview (FH-51). It
// returns the session's run and working directory, for its MCP calls.
export async function seedClaimedRun(home, root, ticket) {
  const cwd = await repo(home, "flashheart", "feature/locks");
  const projectFile = join(root, "flashheart", "project.yaml");
  const project = await readFile(projectFile, "utf8");
  await writeFile(
    projectFile,
    project.replace("repos:\n", `repos:\n  - ${cwd}\n`),
  );
  const id = "aa11bb22-3c4d-4e5f-8a9b-0c1d2e3f4a5b";
  const claimed = session(root, id, cwd);
  claimed.send("SessionStart", { source: "startup" });
  claimed.send("UserPromptSubmit", { prompt: "synthetic" });
  await callTool(root, cwd, "claim", { ticket, run: `claude:${id}` });
  for (const [agent, type] of [
    ["d1e2f3a4", "Explore"],
    ["d2e3f4a5", "Explore"],
    ["d3e4f5a6", "general-purpose"],
  ]) {
    const sub = { agent_id: agent, agent_type: type };
    claimed.send("SubagentStart", sub);
    claimed.tool("Read", {}, sub);
  }
  claimed.send("SubagentStop", { agent_id: "d1e2f3a4", agent_type: "Explore" });
  claimed.tool("Read", {});
  return { run: `claude:${id}`, cwd };
}

// seedBranchRun records a session in a second checkout of project (a
// repository is named by its folder, PRJ-2) on branch, so the run links by
// branch to the ticket on it. The session edits each of edits, paths
// relative to the checkout.
export async function seedBranchRun(home, root, project, branch, edits = []) {
  const cwd = await repo(home, project, branch, "branch");
  const projectFile = join(root, project, "project.yaml");
  const config = await readFile(projectFile, "utf8");
  await writeFile(
    projectFile,
    config.replace("repos:\n", `repos:\n  - ${cwd}\n`),
  );
  const working = session(root, "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9", cwd);
  working.send("SessionStart", { source: "startup" });
  working.send("UserPromptSubmit", { prompt: "synthetic" });
  working.tool("Read", {});
  for (const path of edits)
    working.tool("Edit", { file_path: join(cwd, path) });
  return cwd;
}

// TASK_NOTIFICATION is the prompt Claude Code submits when a background
// command finishes: not the user.
export const TASK_NOTIFICATION =
  "<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>\n</task-notification>";

// promptHook submits a prompt for a session and returns the hook's output,
// which carries answers waiting for it (HOOK-5).
export function promptHook(root, sessionID, cwd, prompt = "synthetic") {
  return execFileSync(
    executable,
    ["hook", "claude", "UserPromptSubmit", "--root", root],
    {
      input: JSON.stringify({
        hook_event_name: "UserPromptSubmit",
        session_id: sessionID,
        cwd,
        prompt,
      }),
      env: { ...process.env, PATH: "/nonexistent" },
    },
  ).toString();
}

// grantPermission records a session's waiting tool running once the user
// allowed it in the session: it no longer needs the user.
export function grantPermission(root, sessionID, cwd, tool = "Bash") {
  session(root, sessionID, cwd).tool(tool, { command: "synthetic" });
}

// askPermission records a session asking permission for a tool again.
export function askPermission(root, sessionID, cwd, tool = "Bash") {
  session(root, sessionID, cwd).send("PermissionRequest", {
    tool_name: tool,
    tool_input: { command: "synthetic" },
  });
}

// endSession records a session ending, as Claude Code's SessionEnd hook
// does when the user closes it.
export function endSession(root, sessionID, cwd) {
  session(root, sessionID, cwd).send("SessionEnd", { reason: "other" });
}

// startSubagent records a subagent of a session starting and using a tool.
export function startSubagent(root, sessionID, cwd, agentID, type) {
  const sub = { agent_id: agentID, agent_type: type };
  const run = session(root, sessionID, cwd);
  run.send("SubagentStart", sub);
  run.tool("Read", {}, sub);
}

// callTool calls one tool of the real `flashheart mcp` server, as Claude
// Code would from a session working in cwd, and returns its text.
export function callTool(root, cwd, name, args) {
  return new Promise((resolvePromise, reject) => {
    const child = spawn(executable, ["mcp", "--root", root], {
      env: { ...process.env, PATH: "/nonexistent", CLAUDE_PROJECT_DIR: cwd },
      stdio: ["pipe", "pipe", "inherit"],
    });
    let buffer = "";
    const timer = setTimeout(() => {
      child.kill();
      reject(new Error(`flashheart mcp did not answer ${name}`));
    }, 10_000);
    child.stdout.on("data", (chunk) => {
      buffer += chunk;
      for (const line of buffer.split("\n").slice(0, -1)) {
        const message = JSON.parse(line);
        if (message.id !== 2) continue;
        clearTimeout(timer);
        child.stdin.end();
        const text = message.result.content.map((c) => c.text).join("");
        if (message.result.isError) reject(new Error(text));
        else resolvePromise(text);
      }
      buffer = buffer.slice(buffer.lastIndexOf("\n") + 1);
    });
    const send = (message) =>
      child.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", ...message })}\n`);
    send({
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2025-06-18",
        capabilities: {},
        clientInfo: { name: "e2e", version: "1" },
      },
    });
    send({ method: "notifications/initialized" });
    send({ id: 2, method: "tools/call", params: { name, arguments: args } });
  });
}

// connectMCP opens one session with the real `flashheart mcp` server from a
// client that names itself as client does, working in cwd, with no hooks:
// call runs a tool and returns its text; close ends the session.
export async function connectMCP(root, cwd, client) {
  const child = spawn(executable, ["mcp", "--root", root], {
    env: { ...process.env, PATH: "/nonexistent", CLAUDE_PROJECT_DIR: cwd },
    stdio: ["pipe", "pipe", "inherit"],
  });
  const waiting = new Map();
  let buffer = "";
  let next = 1;
  child.stdout.on("data", (chunk) => {
    buffer += chunk;
    for (const line of buffer.split("\n").slice(0, -1)) {
      const message = JSON.parse(line);
      waiting.get(message.id)?.(message);
      waiting.delete(message.id);
    }
    buffer = buffer.slice(buffer.lastIndexOf("\n") + 1);
  });
  const request = (method, params) =>
    new Promise((resolvePromise, reject) => {
      const id = next++;
      const timer = setTimeout(
        () => reject(new Error(`flashheart mcp did not answer ${method}`)),
        10_000,
      );
      waiting.set(id, (message) => {
        clearTimeout(timer);
        resolvePromise(message);
      });
      child.stdin.write(
        `${JSON.stringify({ jsonrpc: "2.0", id, method, params })}\n`,
      );
    });
  await request("initialize", {
    protocolVersion: "2025-06-18",
    capabilities: {},
    clientInfo: { name: client, version: "1" },
  });
  child.stdin.write(
    `${JSON.stringify({ jsonrpc: "2.0", method: "notifications/initialized" })}\n`,
  );
  return {
    async call(name, args) {
      const message = await request("tools/call", { name, arguments: args });
      const text = message.result.content.map((c) => c.text).join("");
      if (message.result.isError) throw new Error(text);
      return text;
    },
    close() {
      return new Promise((resolvePromise) => {
        child.on("exit", resolvePromise);
        child.stdin.end();
      });
    },
  };
}
