import { Aside } from "../components/Aside";
import { Button } from "../components/Button";

interface StoppedScreenProps {
  phase: "stopped" | "lost" | "failed";
  failures: number;
  detail?: string;
}

// StoppedScreen is the terminal state after Quit, backend loss or a failed
// connection. Browsers may refuse window.close(), so it always explains how
// to close the tab by hand.
export function StoppedScreen({ phase, failures, detail }: StoppedScreenProps) {
  const copy = describe(phase, failures, detail);
  return (
    <main className="grid min-h-full place-items-center p-6">
      <section
        aria-labelledby="stopped-heading"
        className="w-full max-w-lg rounded-panel border border-rule bg-card p-8"
      >
        <h1 id="stopped-heading" className="text-2xl display-cut">
          {copy.heading}
        </h1>
        <p className="mt-3 text-ink-muted">{copy.body}</p>
        {phase === "stopped" ? (
          <Aside placement="shutdown" className="mt-3" />
        ) : null}
        {copy.restart ? (
          <p className="mt-3 text-ink-muted">
            To start it again, run <code>flashheart</code> in a terminal.
          </p>
        ) : null}
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Button variant="primary" onClick={() => window.close()}>
            Close tab
          </Button>
          <span className="text-sm text-ink-muted">
            If the tab stays open, close it yourself (⌘W or Ctrl+W).
          </span>
        </div>
      </section>
    </main>
  );
}

function describe(
  phase: StoppedScreenProps["phase"],
  failures: number,
  detail?: string,
) {
  switch (phase) {
    case "stopped":
      return {
        heading: "Flashheart has stopped",
        body: "The local server has shut down. You can close this tab.",
        restart: false,
      };
    case "lost":
      return {
        heading: "Lost connection to Flashheart",
        body: `The local server stopped responding after ${failures} missed heartbeats, so this tab can no longer reach it.`,
        restart: true,
      };
    case "failed":
      return {
        heading: "Could not connect to Flashheart",
        body: `This tab could not start a private session${detail ? ` (${detail})` : ""}. Links to Flashheart work only once and only while it runs.`,
        restart: true,
      };
  }
}
