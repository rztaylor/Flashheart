import { Button } from "../components/Button";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import { BackendStatus } from "./BackendStatus";
import type { ServerInfoState } from "./useServerInfo";

interface ShellProps {
  lifecycle: SingleserveLifecycle;
  info: ServerInfoState;
}

export function Shell({ lifecycle, info }: ShellProps) {
  const { state } = lifecycle;
  const stopping = state.phase === "stopping";
  return (
    <div className="flex min-h-full flex-col">
      <header className="flex items-center justify-between gap-4 border-b border-border bg-surface px-4 py-2">
        <span className="text-base font-semibold tracking-tight">
          Flashheart
        </span>
        <div className="flex items-center gap-3">
          <BackendStatus
            state={state}
            checking={lifecycle.checking}
            onCheck={() => void lifecycle.checkHealth()}
          />
          <Button
            onClick={() => void lifecycle.quit()}
            disabled={!lifecycle.ready || stopping}
          >
            {stopping ? "Quitting…" : "Quit"}
          </Button>
        </div>
      </header>
      {state.shutdownDenied ? (
        <div
          role="alert"
          className="flex items-center justify-between gap-4 border-b border-border bg-danger-surface px-4 py-2 text-sm"
        >
          <span>
            <strong className="font-semibold">Flashheart did not quit.</strong>{" "}
            {state.shutdownDenied}
          </span>
          <Button variant="quiet" onClick={lifecycle.dismissDenial}>
            Dismiss
          </Button>
        </div>
      ) : null}
      <main className="mx-auto w-full max-w-3xl flex-1 px-6 py-12">
        <h1 className="text-2xl font-semibold tracking-tight">
          Flashheart is running
        </h1>
        <p className="mt-2 text-text-muted">
          Board views arrive in the next milestone. Quit here, or close the last
          Flashheart tab, to stop the server.
        </p>
        <ServerDetails info={info} />
      </main>
    </div>
  );
}

function ServerDetails({ info }: { info: ServerInfoState }) {
  if (info.status === "loading") {
    return (
      <p className="mt-8 text-sm text-text-muted">Loading server details…</p>
    );
  }
  if (info.status === "error") {
    return (
      <p role="alert" className="mt-8 text-sm text-danger">
        Server details are unavailable: {info.message}
      </p>
    );
  }
  const rows: [string, string][] = [
    ["Board root", info.info.root],
    ["Version", `${info.info.version} (${info.info.commit})`],
    ["Agent protocol", String(info.info.protocolVersion)],
  ];
  return (
    <dl className="mt-8 grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 rounded-panel border border-border bg-surface p-5 text-sm">
      {rows.map(([term, value]) => (
        <div key={term} className="contents">
          <dt className="text-text-muted">{term}</dt>
          <dd className="font-mono break-all">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
