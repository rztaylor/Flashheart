import { useEffect, useState } from "react";

import type { AuthenticatedFetch } from "../api/client";
import { fetchInfo, type ServerInfo } from "../api/info";
import { messageOf } from "../model/errors";

export type ServerInfoState =
  | { status: "loading" }
  | { status: "ready"; info: ServerInfo }
  | { status: "error"; message: string };

export function useServerInfo(
  fetcher: AuthenticatedFetch,
  ready: boolean,
): ServerInfoState {
  const [state, setState] = useState<ServerInfoState>({ status: "loading" });
  useEffect(() => {
    if (!ready) return;
    const controller = new AbortController();
    fetchInfo(fetcher, controller.signal)
      .then((info) => setState({ status: "ready", info }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        setState({
          status: "error",
          message: messageOf(error, "Server info is unavailable"),
        });
      });
    return () => controller.abort();
  }, [fetcher, ready]);
  return state;
}
