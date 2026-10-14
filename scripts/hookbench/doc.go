// Command hookbench measures `flashheart hook <agent>` latency (HOOK-1: p95
// under 50 ms on a warm cache). It runs a built binary against a scratch
// root, home and git checkout with the agent's recorded payloads
// (testdata/hooks/<agent>, -agent) and prints percentiles. Scenarios
// (-scenario): a session's mix of hooks, tool results alone (the throttled
// path), session starts (the recovery note), turn ends that handoff
// enforcement blocks (each after an untimed prompt) and new unclaimed
// sessions' turn ends the claim reminder holds (D50; after an untimed
// prompt and edit). -seed-events first
// seeds the project's log with that many events a day across the run
// window: sessions in three worktrees doing what the protocol records, a
// third of them claiming tickets. It is a developer tool run by
// scripts/hook-bench.sh, not part of CI.
package main
