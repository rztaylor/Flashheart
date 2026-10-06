// Command hookbench measures `flashheart hook claude` latency (HOOK-1: p95
// under 50 ms on a warm cache). It runs a built binary against a scratch
// root and git checkout with recorded payloads and prints percentiles; it
// is a developer tool run by scripts/hook-bench.sh, not part of CI.
package main
