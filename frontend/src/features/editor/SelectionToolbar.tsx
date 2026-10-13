import type { StateCommand } from "@codemirror/state";
import type { EditorView } from "@codemirror/view";
import {
  type KeyboardEvent,
  type ReactNode,
  type RefObject,
  useState,
} from "react";

import { Icon } from "../../components/Icon";
import { mac } from "../editing/InPlace";
import {
  insertLink,
  toggleBold,
  toggleCode,
  toggleItalic,
  toggleLines,
} from "./commands";
import { formatKeys } from "./setup";

// SelectionToolbar formats the selected text (FH-16): a toolbar that
// floats over the selection. Tab from the text or Alt+F10 reaches it; the
// arrow keys move along it; Escape returns to the text with the selection
// kept. A pointer press keeps focus in the text.

// shortcutOf spells a CodeMirror key ("Mod-b") for people and for
// aria-keyshortcuts.
function shortcutOf(key: string) {
  const letter = key.slice(-1).toUpperCase();
  return {
    shown: mac ? `⌘${letter}` : `Ctrl+${letter}`,
    aria: `${mac ? "Meta" : "Control"}+${letter}`,
  };
}

interface Action {
  name: string;
  key?: string;
  icon: ReactNode;
  run: StateCommand;
}

const actions: Action[] = [
  {
    name: "Bold",
    key: formatKeys.bold,
    icon: <span className="text-sm font-bold">B</span>,
    run: toggleBold,
  },
  {
    name: "Italic",
    key: formatKeys.italic,
    icon: <span className="text-sm font-semibold italic">I</span>,
    run: toggleItalic,
  },
  {
    name: "Code",
    key: formatKeys.code,
    icon: <Icon name="code" size={15} />,
    run: toggleCode,
  },
  {
    name: "Link",
    key: formatKeys.link,
    icon: <Icon name="link" size={15} />,
    run: insertLink,
  },
  {
    name: "Bulleted list",
    icon: <Icon name="list" size={15} />,
    run: toggleLines("bullet"),
  },
  {
    name: "Task list",
    icon: <Icon name="criteria" size={15} />,
    run: toggleLines("task"),
  },
];

export function SelectionToolbar({
  view,
  position,
  toolbar,
}: {
  view: EditorView;
  // position is where the toolbar's bottom left sits, in the editor
  // frame's coordinates.
  position: { top: number; left: number };
  toolbar: RefObject<HTMLDivElement | null>;
}) {
  // current is the button in the tab order (a roving tab stop).
  const [current, setCurrent] = useState(0);
  const buttons = () =>
    [...(toolbar.current?.querySelectorAll("button") ?? [])] as HTMLElement[];
  const move = (to: number) => {
    const next = (to + actions.length) % actions.length;
    setCurrent(next);
    buttons()[next]?.focus();
  };
  const onKeyDown = (event: KeyboardEvent) => {
    const keys: Record<string, () => void> = {
      ArrowRight: () => move(current + 1),
      ArrowLeft: () => move(current - 1),
      Home: () => move(0),
      End: () => move(actions.length - 1),
      Escape: () => view.focus(),
    };
    const handle = keys[event.key];
    if (!handle) return;
    // Escape returns to the text; it must not also cancel the edit.
    event.preventDefault();
    event.stopPropagation();
    handle();
  };
  return (
    <div
      ref={toolbar}
      role="toolbar"
      aria-label="Format"
      aria-orientation="horizontal"
      onKeyDown={onKeyDown}
      onMouseDown={(event) => event.preventDefault()}
      style={{ top: position.top, left: position.left }}
      className="absolute z-30 flex -translate-y-full items-center gap-0.5 rounded-control bg-popover p-1 shadow-popover"
    >
      {actions.map((action, index) => {
        const keys = action.key ? shortcutOf(action.key) : undefined;
        return (
          <button
            key={action.name}
            type="button"
            aria-label={action.name}
            aria-keyshortcuts={keys?.aria}
            title={keys ? `${action.name} (${keys.shown})` : action.name}
            tabIndex={index === current ? 0 : -1}
            onFocus={() => setCurrent(index)}
            onClick={() => {
              action.run(view);
              view.focus();
            }}
            className="grid size-8 place-items-center rounded-inner text-ink transition-colors hover:bg-well"
          >
            {action.icon}
          </button>
        );
      })}
    </div>
  );
}
