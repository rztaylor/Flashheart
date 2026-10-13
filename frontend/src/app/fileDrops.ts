import { useEffect } from "react";

import { hasFiles } from "../components/FileDrop";

// useNoFileNavigation keeps a file dropped where nothing takes it from
// opening in the tab, which would leave the board and any unsaved edit
// (FH-18). Places that take files (the editor, Attachments, Files) handle
// their drops first; this only stops the browser's own.
export function useNoFileNavigation() {
  useEffect(() => {
    const stop = (event: DragEvent) => {
      if (event.defaultPrevented || !hasFiles(event.dataTransfer)) return;
      event.preventDefault();
      if (event.type === "dragover" && event.dataTransfer)
        event.dataTransfer.dropEffect = "none";
    };
    window.addEventListener("dragover", stop);
    window.addEventListener("drop", stop);
    return () => {
      window.removeEventListener("dragover", stop);
      window.removeEventListener("drop", stop);
    };
  }, []);
}
