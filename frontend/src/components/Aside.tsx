import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useState,
} from "react";

import {
  nextRemark,
  PLACEMENTS,
  type Placement,
  winningAside,
} from "../model/remarks";

// Marginal remarks (FH-22): at most one is visible per screen. Every Aside
// registers with the provider, and only the highest-priority one renders.

interface Registry {
  register(key: string, priority: number): () => void;
  winner?: string;
}

const AsideContext = createContext<Registry | null>(null);

export function AsideProvider({ children }: { children: ReactNode }) {
  const [registered, setRegistered] = useState<
    { key: string; priority: number }[]
  >([]);
  const register = useCallback((key: string, priority: number) => {
    setRegistered((current) => [...current, { key, priority }]);
    return () =>
      setRegistered((current) => current.filter((item) => item.key !== key));
  }, []);
  const winner = winningAside(registered);
  const value = useMemo(() => ({ register, winner }), [register, winner]);
  return (
    <AsideContext.Provider value={value}>{children}</AsideContext.Provider>
  );
}

// Aside is one dry remark in the margin of a real state, chosen once from
// its placement's lines and kept while it stays on screen. text overrides
// the choice (the toast picks its remark when the move succeeds). It is
// quiet secondary text, never inside a control or an instruction; hidden
// ones are not announced.
export function Aside({
  placement,
  text,
  className = "",
  hidden,
}: {
  placement: Placement;
  text?: string;
  className?: string;
  // hidden keeps it out of the accessibility tree (inside live regions).
  hidden?: boolean;
}) {
  const registry = useContext(AsideContext);
  const register = registry?.register;
  const key = useId();
  const [remark] = useState(() => text ?? nextRemark(placement).text);
  useEffect(
    () => register?.(key, PLACEMENTS[placement].priority),
    [register, key, placement],
  );
  if (registry && registry.winner !== key) return null;
  return (
    <p
      data-aside=""
      aria-hidden={hidden ? true : undefined}
      className={`text-xs text-ink-faint ${className}`}
    >
      {remark}
    </p>
  );
}
