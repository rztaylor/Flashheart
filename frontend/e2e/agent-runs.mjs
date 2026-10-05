// Drives the real `flashheart hook claude` command to put live agent runs on
// a sandbox board: the same path Claude Code takes. All sessions, repositories
// and plans are synthetic test data.
import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { executable } from "./support.mjs";

async function repo(home, name, branch) {
  const dir = join(home, "src", name);
  await mkdir(join(dir, ".git"), { recursive: true });
  await writeFile(join(dir, ".git", "HEAD"), `ref: refs/heads/${branch}\n`);
  return dir;
}

function hook(root, event, payload) {
  execFileSync(executable, ["hook", "claude", event, "--root", root], {
    input: JSON.stringify({ hook_event_name: event, ...payload }),
    env: { ...process.env, PATH: "/nonexistent" },
  });
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
// branch links it to AL-3), one in beta working with an Explore subagent,
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

  const asking = session(root, "3f2a9c1e-7b44-4d0e-9a51-6c8e2f1d0b7a", alpha);
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

  const idle = session(root, "e2d8f6a4-9b1c-4c3e-b5a7-1f0e9d8c7b6a", gamma);
  idle.send("SessionStart", { source: "startup" });
  idle.send("UserPromptSubmit", { prompt: "synthetic" });
  idle.tool("Read", {});
  idle.send("Stop");
}
