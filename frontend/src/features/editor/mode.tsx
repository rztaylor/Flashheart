import { createContext, useContext, useState } from "react";

import type { EditorMode } from "../../api/preferences";

// The editor's view, formatted or markdown, is a UI preference saved
// through the backend (CFG-2): the shell provides it from the saved
// preferences. Without a provider (a test, a page that saves nothing) the
// choice lasts as long as the editor.

export interface EditorModeChoice {
  mode: EditorMode;
  setMode(mode: EditorMode): void;
}

const ModeContext = createContext<EditorModeChoice | null>(null);

export const EditorModeProvider = ModeContext.Provider;

export function useEditorMode(): EditorModeChoice {
  const provided = useContext(ModeContext);
  const [mode, setMode] = useState<EditorMode>("formatted");
  return provided ?? { mode, setMode };
}
