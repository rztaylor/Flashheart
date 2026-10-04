import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { connect, type SingleserveSession } from "singleserve-client";

import type { AuthenticatedFetch } from "../api/client";
import {
  initialLifecycleState,
  isTerminal,
  type LifecycleState,
  lifecycleReducer,
} from "./state";

export interface SingleserveLifecycle {
  state: LifecycleState;
  ready: boolean;
  checking: boolean;
  fetch: AuthenticatedFetch;
  checkHealth(): Promise<void>;
  quit(): Promise<void>;
  dismissDenial(): void;
}

// useSingleserve owns the page's one Singleserve session. Call it once, from
// the app root.
export function useSingleserve(): SingleserveLifecycle {
  const [state, dispatch] = useReducer(lifecycleReducer, initialLifecycleState);
  const [ready, setReady] = useState(false);
  const [checking, setChecking] = useState(false);
  const sessionRef = useRef<SingleserveSession | undefined>(undefined);
  const pending = useRef(new Set<AbortController>());

  useEffect(() => {
    let disposed = false;
    void connect({
      onHeartbeat(result) {
        if (!disposed) {
          dispatch({
            type: "heartbeat",
            ok: result.ok,
            failures: result.failures,
            checkedAt: result.checkedAt,
          });
        }
      },
      onServerUnavailable({ failures }) {
        if (!disposed) dispatch({ type: "server-unavailable", failures });
      },
    })
      .then((session) => {
        if (disposed) {
          session.stop();
          return;
        }
        sessionRef.current = session;
        setReady(true);
        dispatch({ type: "connected" });
      })
      .catch((error: unknown) => {
        if (!disposed) {
          dispatch({
            type: "connect-failed",
            message:
              error instanceof Error
                ? error.message
                : "The local session could not be established",
          });
        }
      });
    return () => {
      disposed = true;
      abortPending(pending.current);
      sessionRef.current?.stop();
      sessionRef.current = undefined;
    };
  }, []);

  // Terminal states stop the session and try to close the tab; browsers often
  // refuse, so the stopped screen explains how to close it manually.
  const terminal = isTerminal(state.phase);
  useEffect(() => {
    if (!terminal) return;
    abortPending(pending.current);
    sessionRef.current?.stop();
    sessionRef.current = undefined;
    setReady(false);
    if (state.phase !== "failed") window.close();
  }, [terminal, state.phase]);

  const authenticatedFetch = useCallback<AuthenticatedFetch>((input, init) => {
    const session = sessionRef.current;
    if (!session) {
      return Promise.reject(new Error("The local session is not ready"));
    }
    const controller = new AbortController();
    const callerSignal = init?.signal;
    const abort = () => controller.abort(callerSignal?.reason);
    if (callerSignal?.aborted) abort();
    else callerSignal?.addEventListener("abort", abort, { once: true });
    pending.current.add(controller);
    return session
      .fetch(input, { ...init, signal: controller.signal })
      .finally(() => {
        callerSignal?.removeEventListener("abort", abort);
        pending.current.delete(controller);
      });
  }, []);

  const checkHealth = useCallback(async () => {
    const session = sessionRef.current;
    if (!session) return;
    setChecking(true);
    try {
      const ok = await session.health();
      dispatch({ type: "health", ok, at: new Date() });
    } finally {
      setChecking(false);
    }
  }, []);

  const quit = useCallback(async () => {
    const session = sessionRef.current;
    if (!session) return;
    dispatch({ type: "quit-requested" });
    try {
      await session.requestShutdown();
      dispatch({ type: "quit-accepted" });
    } catch (error: unknown) {
      dispatch({
        type: "quit-denied",
        message:
          error instanceof Error ? error.message : "Flashheart could not quit",
      });
    }
  }, []);

  const dismissDenial = useCallback(
    () => dispatch({ type: "dismiss-denial" }),
    [],
  );

  return {
    state,
    ready,
    checking,
    fetch: authenticatedFetch,
    checkHealth,
    quit,
    dismissDenial,
  };
}

function abortPending(controllers: Set<AbortController>) {
  for (const controller of controllers) controller.abort();
  controllers.clear();
}
