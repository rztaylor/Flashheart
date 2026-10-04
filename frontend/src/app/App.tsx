import { useSingleserve } from "../lifecycle/useSingleserve";
import { Shell } from "./Shell";
import { StoppedScreen } from "./StoppedScreen";
import { useTheme } from "./theme";
import { useServerInfo } from "./useServerInfo";

export function App() {
  const lifecycle = useSingleserve();
  const info = useServerInfo(lifecycle.fetch, lifecycle.ready);
  useTheme(info.status === "ready" ? info.info.theme : "system");

  const { phase, failures, detail } = lifecycle.state;
  if (phase === "stopped" || phase === "lost" || phase === "failed") {
    return <StoppedScreen phase={phase} failures={failures} detail={detail} />;
  }
  return <Shell lifecycle={lifecycle} info={info} />;
}
