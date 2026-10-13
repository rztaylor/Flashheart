import { type EditorView, WidgetType } from "@codemirror/view";

import { toggleTaskAt } from "./commands";

// The live preview's widgets: what stands in for markdown syntax on the
// lines not being edited. Each has a name the preview tests read.

// CheckboxWidget is a task's [ ] or [x]: a real checkbox, named by its
// task, that ticks the text it stands for (CARD-3).
export class CheckboxWidget extends WidgetType {
  constructor(
    readonly checked: boolean,
    readonly label: string,
  ) {
    super();
  }
  get name() {
    return `checkbox(${this.checked ? "done" : "open"})`;
  }
  eq(other: CheckboxWidget) {
    return other.checked === this.checked && other.label === this.label;
  }
  toDOM(view: EditorView) {
    const box = document.createElement("input");
    box.type = "checkbox";
    box.className = "fh-task";
    box.checked = this.checked;
    box.setAttribute("aria-label", this.label || "Task");
    // Tab leaves the editor rather than stopping at every task; from the
    // keyboard, Alt+Enter ticks the task on the cursor's line.
    box.tabIndex = -1;
    // The click ticks; it must not move the selection first.
    box.addEventListener("mousedown", (event) => event.preventDefault());
    box.addEventListener("click", (event) => {
      event.preventDefault();
      const change = toggleTaskAt(view.state, view.posAtDOM(box));
      if (change) view.dispatch(change);
    });
    return box;
  }
  ignoreEvent() {
    return true;
  }
}

// BulletWidget is a bullet list's -, * or +, drawn as the view's bullet.
export class BulletWidget extends WidgetType {
  readonly name = "bullet";
  eq() {
    return true;
  }
  toDOM() {
    const bullet = document.createElement("span");
    bullet.className = "fh-bullet";
    bullet.textContent = "•";
    return bullet;
  }
}

// RuleWidget is a thematic break (---), drawn as a rule.
export class RuleWidget extends WidgetType {
  readonly name = "rule";
  eq() {
    return true;
  }
  toDOM() {
    const rule = document.createElement("span");
    rule.className = "fh-rule";
    rule.setAttribute("role", "separator");
    return rule;
  }
}

// ImageWidget is an image from the ticket's own files, loaded from the
// attachment endpoint as the view loads it; nothing else is ever fetched.
export class ImageWidget extends WidgetType {
  constructor(
    readonly href: string,
    readonly alt: string,
  ) {
    super();
  }
  get name() {
    return `image(${this.href})`;
  }
  eq(other: ImageWidget) {
    return other.href === this.href && other.alt === this.alt;
  }
  toDOM() {
    const image = document.createElement("img");
    image.className = "fh-image";
    image.src = this.href;
    image.alt = this.alt;
    image.draggable = false;
    return image;
  }
}

// UnavailableImageWidget is an image the board does not show (a remote
// one, or a path outside the ticket's files): its words, as in the view.
export class UnavailableImageWidget extends WidgetType {
  constructor(readonly alt: string) {
    super();
  }
  get name() {
    return `image-unavailable(${this.alt})`;
  }
  eq(other: UnavailableImageWidget) {
    return other.alt === this.alt;
  }
  toDOM() {
    const note = document.createElement("span");
    note.className = "fh-image-note";
    note.textContent = `[image: ${this.alt || "unavailable"}]`;
    return note;
  }
}
