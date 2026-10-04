import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./app/App";
import { applyTheme } from "./app/theme";
import "./styles/index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("Flashheart root element is missing");
}

applyTheme("system");
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
