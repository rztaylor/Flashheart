// htmlToMarkdown turns HTML copied from a web page into markdown for the
// editor (FH-142, D66): headings (demoted to levels 3 and up, so none
// starts a new section, D62), paragraphs, emphasis, strikethrough, code,
// links, lists (nested, numbered, tasks), quotes, tables and rules.
//
// It is safe by construction:
// - The HTML is parsed with DOMParser into a document with no browsing
//   context, which runs no script and loads no image, style or frame; the
//   converter only reads it. Inline styles and handlers are renamed first
//   (quiet), so the page's CSP has none to refuse and report.
// - Only http, https and mailto links, and relative files/ links to the
//   ticket's own files, survive; any other link keeps its words alone.
// - A remote image becomes a link to it; any other image (data:, blob:)
//   keeps only its alt text; a files/ image stays an image.
// - Nothing but markdown comes out: an element it does not know gives its
//   text, scripts, styles, frames, media, forms, icons and hidden parts
//   give nothing, and text that would read as a tag is escaped.
//
// A copy with nothing markdown would keep (plain paragraphs, a code
// editor's coloured lines) gives null, and the editor pastes its plain
// text as before.

// Elements whose content is never pasted.
const DROPPED = new Set([
  "script",
  "style",
  "noscript",
  "template",
  "head",
  "title",
  "meta",
  "link",
  "base",
  "iframe",
  "frame",
  "frameset",
  "object",
  "embed",
  "param",
  "applet",
  "svg",
  "math",
  "canvas",
  "video",
  "audio",
  "source",
  "track",
  "map",
  "area",
  "button",
  "select",
  "option",
  "optgroup",
  "datalist",
  "textarea",
  "input",
  "dialog",
]);

// Elements that start a block of their own.
const BLOCKS = new Set([
  "address",
  "article",
  "aside",
  "blockquote",
  "body",
  "center",
  "dd",
  "details",
  "dir",
  "div",
  "dl",
  "dt",
  "fieldset",
  "figcaption",
  "figure",
  "footer",
  "form",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "header",
  "hgroup",
  "hr",
  "html",
  "legend",
  "li",
  "main",
  "menu",
  "nav",
  "ol",
  "p",
  "pre",
  "section",
  "summary",
  "table",
  "tbody",
  "td",
  "tfoot",
  "th",
  "thead",
  "tr",
  "ul",
]);

// Elements that carry what markdown keeps; a copy with none of them (and
// no bold or italic style) pastes as plain text.
const STRUCTURE =
  "h1,h2,h3,h4,h5,h6,ul,ol,pre,code,blockquote,table,hr,img,a[href],strong,b,em,i,del,s,strike";

const LIST = new Set(["ul", "ol", "menu", "dir"]);

interface Context {
  // inLink: a link inside a link (or an image's) is its words.
  inLink: boolean;
  bold: boolean;
  italic: boolean;
  strike: boolean;
}

const start: Context = {
  inLink: false,
  bold: false,
  italic: false,
  strike: false,
};

export function htmlToMarkdown(html: string): string | null {
  const doc = new DOMParser().parseFromString(quiet(html), "text/html");
  const body = doc.body;
  if (!body || !hasStructure(body)) return null;
  const markdown = blocks(body, start).join("\n\n").trim();
  return markdown || null;
}

// hasStructure says whether the copy holds anything markdown keeps. A
// code editor colours its lines with spans in a white-space: pre block;
// those spans' bold is not the text's.
function hasStructure(root: Element): boolean {
  for (const element of root.querySelectorAll(STRUCTURE))
    if (!dropped(element) && !normalWeight(element)) return true;
  for (const element of root.querySelectorAll(
    "span[data-fh-style],font[data-fh-style]",
  )) {
    if (!(styledBold(element) || styledItalic(element))) continue;
    let up = element.parentElement;
    let preformatted = false;
    while (up && !preformatted) {
      preformatted = style(up, "white-space") === "pre";
      up = up.parentElement;
    }
    if (!preformatted) return true;
  }
  return false;
}

// Chromium checks each style and on* attribute against the page's CSP as
// it parses it, even into a document that applies and runs nothing, and
// reports every one refused. quiet renames them inside each tag (style to
// data-fh-style) so the parser never meets them. Text between tags is not
// touched, as serialized HTML escapes its <; a value inside a tag that
// happens to read " style=" is restored by attribute.
const INLINE_ATTRIBUTE = /(\s)(style|on[a-z]+)(?=\s*=)/gi;
const RENAMED_ATTRIBUTE = /(\s)data-fh-(style|on[a-z]+)(?=\s*=)/gi;

function quiet(html: string): string {
  return html.replace(/<[a-z][^>]*>/gi, (tag) =>
    tag.replace(INLINE_ATTRIBUTE, "$1data-fh-$2"),
  );
}

// attribute is an attribute's value as it was copied.
function attribute(element: Element, name: string): string {
  return (element.getAttribute(name) ?? "").replace(RENAMED_ATTRIBUTE, "$1$2");
}

// style is the last value given for a property in an element's style
// attribute, read as text (the parsed document computes no styles).
function style(element: Element, property: string): string {
  let value = "";
  for (const declaration of attribute(element, "data-fh-style").split(";")) {
    const colon = declaration.indexOf(":");
    if (colon < 0) continue;
    if (declaration.slice(0, colon).trim().toLowerCase() === property)
      value = declaration
        .slice(colon + 1)
        .replace(/!important/i, "")
        .trim()
        .toLowerCase();
  }
  return value;
}

function styledBold(element: Element): boolean {
  const weight = style(element, "font-weight");
  return weight === "bold" || weight === "bolder" || Number(weight) >= 600;
}

function styledItalic(element: Element): boolean {
  const value = style(element, "font-style");
  return value === "italic" || value === "oblique";
}

// normalWeight is a <b> styled not to be bold (Google Docs wraps a whole
// copy in one).
function normalWeight(element: Element): boolean {
  const weight = style(element, "font-weight");
  return (
    element.localName === "b" &&
    (weight === "normal" || (weight !== "" && Number(weight) < 600))
  );
}

function dropped(element: Element): boolean {
  if (DROPPED.has(element.localName)) return true;
  if (element.hasAttribute("hidden")) return true;
  if (element.getAttribute("aria-hidden") === "true") return true;
  return style(element, "display") === "none";
}

function isBlock(element: Element): boolean {
  return BLOCKS.has(element.localName);
}

// containsBlock says whether an inline element wraps blocks (a link
// around a card, Google Docs' wrapper): it is then read as a block.
function containsBlock(element: Element): boolean {
  for (const child of element.children)
    if (!dropped(child) && (isBlock(child) || containsBlock(child)))
      return true;
  return false;
}

// blocks reads a container's children as markdown blocks.
function blocks(parent: Element, context: Context): string[] {
  const out: string[] = [];
  let line = "";
  const flush = () => {
    const text = tidy(line);
    if (text) out.push(text);
    line = "";
  };
  for (const child of parent.childNodes) {
    if (child.nodeType === child.TEXT_NODE) {
      line += text(child.textContent ?? "");
      continue;
    }
    if (child.nodeType !== child.ELEMENT_NODE) continue;
    const element = child as Element;
    if (dropped(element)) continue;
    if (isBlock(element) || containsBlock(element)) {
      flush();
      out.push(...block(element, context));
    } else line += inline(element, context);
  }
  flush();
  return out;
}

// block is one block element as markdown blocks.
function block(element: Element, context: Context): string[] {
  const name = element.localName;
  const heading = /^h([1-6])$/.exec(name);
  if (heading) {
    const words = oneLine(inlineContent(element, context));
    const level = Math.max(3, Number(heading[1]));
    return words ? [`${"#".repeat(level)} ${words}`] : [];
  }
  if (LIST.has(name)) {
    const list = listOf(element, context);
    return list ? [list] : [];
  }
  if (name === "pre") return [codeBlock(element)];
  if (name === "blockquote") {
    const inner = blocks(element, context).join("\n\n");
    return inner
      ? [
          inner
            .split("\n")
            .map((line) => (line ? `> ${line}` : ">"))
            .join("\n"),
        ]
      : [];
  }
  if (name === "table") {
    const table = tableOf(element, context);
    return table ? [table] : blocks(element, context);
  }
  if (name === "hr") return ["---"];
  return blocks(element, context);
}

// inline is an inline element as markdown text.
function inline(element: Element, context: Context): string {
  const name = element.localName;
  if (name === "br") return "\n";
  if (name === "img") return image(element, context);
  if (name === "a") return link(element, context);
  if (name === "code" || name === "kbd" || name === "samp" || name === "tt")
    return codeSpan(oneLine(collapse(element.textContent ?? "")));
  // Emphasis an element adds (a styled span may add both), never repeated
  // inside the same emphasis.
  const styled = name === "span" || name === "font";
  const bold =
    !context.bold &&
    (((name === "strong" || name === "b") && !normalWeight(element)) ||
      (styled && styledBold(element)));
  const italic =
    !context.italic &&
    (name === "em" || name === "i" || (styled && styledItalic(element)));
  const strike =
    !context.strike && (name === "del" || name === "s" || name === "strike");
  let out = inlineContent(element, {
    ...context,
    bold: context.bold || bold,
    italic: context.italic || italic,
    strike: context.strike || strike,
  });
  if (strike) out = wrap(out, "~~");
  if (italic) out = wrap(out, "*");
  if (bold) out = wrap(out, "**");
  return out;
}

// inlineContent is an element's children as one run of markdown text.
function inlineContent(element: Element, context: Context): string {
  let out = "";
  for (const child of element.childNodes) {
    if (child.nodeType === child.TEXT_NODE)
      out += text(child.textContent ?? "");
    else if (child.nodeType === child.ELEMENT_NODE) {
      const inner = child as Element;
      if (dropped(inner)) continue;
      out += isBlock(inner)
        ? ` ${blocks(inner, context).join(" ")} `
        : inline(inner, context);
    }
  }
  return out;
}

// wrap puts marks around text, leaving its outer spaces outside them.
function wrap(content: string, mark: string): string {
  const match = /^(\s*)([\s\S]*?)(\s*)$/.exec(content);
  if (!match?.[2]) return content;
  return `${match[1]}${mark}${match[2]}${mark}${match[3]}`;
}

// link keeps an allow-listed link; any other keeps its words alone.
function link(element: Element, context: Context): string {
  const words = inlineContent(element, { ...context, inLink: true });
  const href = context.inLink ? null : safeHref(attribute(element, "href"));
  if (!href) return words;
  const match = /^(\s*)([\s\S]*?)(\s*)$/.exec(words);
  const label = oneLine(match?.[2] ?? "");
  if (!label) return "";
  const body = label === href ? href : `[${label}](${href})`;
  return `${match?.[1] ?? ""}${body}${match?.[3] ?? ""}`;
}

// image: a remote image is a link to it, named by its alt text or its
// file; a files/ image stays an image; any other keeps its alt text.
function image(element: Element, context: Context): string {
  const alt = oneLine(collapse(attribute(element, "alt")));
  const src = attribute(element, "src").trim();
  if (FILES.test(src) && !context.inLink) return `![${alt}](${encode(src)})`;
  const href = context.inLink ? null : remoteHref(src);
  if (!href) return escapeTags(alt);
  const name = alt || fileName(href) || "image";
  return `[${escapeTags(name)}](${href})`;
}

function fileName(href: string): string {
  try {
    const segments = new URL(href).pathname.split("/");
    return decodeURIComponent(segments[segments.length - 1] ?? "");
  } catch {
    return "";
  }
}

// FILES is a relative link to the ticket's own files, never above them.
const FILES = /^files\/(?!.*(^|\/)\.\.?(\/|$))[^\s<>]+$/;

// safeHref is a link's address if its scheme is allowed (http, https,
// mailto, or a relative files/ link), else null. The URL parser reads the
// scheme as a browser would (case, spaces, tabs), and its result is what
// is written, so the check and the link agree.
function safeHref(raw: string | null): string | null {
  const value = (raw ?? "").trim();
  if (FILES.test(value)) return encode(value);
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    return null;
  }
  if (!["http:", "https:", "mailto:"].includes(url.protocol)) return null;
  return encode(url.href);
}

function remoteHref(raw: string): string | null {
  const href = safeHref(raw);
  return href?.startsWith("http") ? href : null;
}

// encode keeps a link's address inside markdown's parentheses.
function encode(href: string): string {
  return href.replace(/ /g, "%20").replace(/\(/g, "%28").replace(/\)/g, "%29");
}

// codeSpan fences text in more backticks than it holds.
function codeSpan(content: string): string {
  if (!content) return "";
  const longest = Math.max(
    0,
    ...(content.match(/`+/g) ?? []).map((run) => run.length),
  );
  const fence = "`".repeat(longest + 1);
  const pad = content.startsWith("`") || content.endsWith("`") ? " " : "";
  return `${fence}${pad}${content}${pad}${fence}`;
}

// codeBlock is a <pre> as a fenced block, its language from a class.
function codeBlock(element: Element): string {
  const content = preText(element).replace(/\n$/, "");
  const longest = Math.max(
    2,
    ...(content.match(/`+/g) ?? []).map((run) => run.length),
  );
  const fence = "`".repeat(longest + 1);
  return `${fence}${language(element)}\n${content}\n${fence}`;
}

// preText is preformatted text as written: <br> and a line per block.
function preText(element: Element): string {
  let out = "";
  for (const child of element.childNodes) {
    if (child.nodeType === child.TEXT_NODE) out += child.textContent ?? "";
    else if (child.nodeType === child.ELEMENT_NODE) {
      const inner = child as Element;
      if (dropped(inner)) continue;
      if (inner.localName === "br") out += "\n";
      else if (isBlock(inner) && out && !out.endsWith("\n"))
        out += `\n${preText(inner)}`;
      else out += preText(inner);
    }
  }
  return out;
}

function language(pre: Element): string {
  const owners = [pre, pre.querySelector("code"), pre.parentElement];
  for (const owner of owners) {
    const match = /(?:^|\s)(?:language|lang|highlight-source)-([\w+#-]+)/.exec(
      owner?.getAttribute("class") ?? "",
    );
    if (match?.[1]) return match[1];
  }
  return "";
}

// listOf is a list as markdown, its items numbered from its start and
// nested lists indented under their item. A list that is a child of a
// list (as Google Docs writes it) belongs to the item before it.
function listOf(list: Element, context: Context): string {
  const ordered = list.localName === "ol";
  const first = Number.parseInt(list.getAttribute("start") ?? "1", 10);
  const items: string[][] = [];
  for (const child of list.children) {
    if (dropped(child)) continue;
    if (LIST.has(child.localName)) {
      const nested = listOf(child, context);
      if (!nested) continue;
      const last = items[items.length - 1];
      if (last) last.push(nested);
      else items.push([nested]);
      continue;
    }
    items.push(itemBlocks(child, context));
  }
  const kept = items.filter((item) => item.length > 0);
  if (!kept.length) return "";
  let loose = false;
  const written = kept.map((parts, index) => {
    const marker = ordered
      ? `${(Number.isFinite(first) ? first : 1) + index}. `
      : "- ";
    let body = "";
    parts.forEach((part, at) => {
      if (at > 0) {
        const nested = /^(- |\d+\. )/.test(part) && !part.includes("\n\n");
        if (!nested) loose = true;
        body += nested ? "\n" : "\n\n";
      }
      body += part;
    });
    const pad = " ".repeat(marker.length);
    return (
      marker +
      body
        .split("\n")
        .map((line, at) => (at === 0 || !line ? line : pad + line))
        .join("\n")
    );
  });
  return written.join(loose ? "\n\n" : "\n");
}

// itemBlocks is a list item's blocks, a task's box first.
function itemBlocks(item: Element, context: Context): string[] {
  const parts = blocks(item, context);
  const box = item.querySelector('input[type="checkbox" i]');
  if (box && box.closest("li") === item && parts.length > 0)
    parts[0] = `[${box.hasAttribute("checked") ? "x" : " "}] ${parts[0]}`;
  return parts;
}

// tableOf is a table as a GFM table, its first row the header; a table
// laid out with tables inside it is read as blocks instead.
function tableOf(table: Element, context: Context): string {
  if (table.querySelector("table")) return "";
  const rows: string[][] = [];
  for (const row of table.querySelectorAll("tr")) {
    if (row.closest("table") !== table) continue;
    const cells: string[] = [];
    for (const cell of row.children) {
      if (cell.localName !== "td" && cell.localName !== "th") continue;
      cells.push(
        oneLine(tidy(inlineContent(cell, context))).replace(/\|/g, "\\|"),
      );
    }
    if (cells.length) rows.push(cells);
  }
  if (!rows.length) return "";
  const width = Math.max(...rows.map((row) => row.length));
  const line = (cells: string[]) =>
    `| ${Array.from({ length: width }, (_, at) => cells[at] ?? "").join(" | ")} |`;
  const [header = [], ...body] = rows;
  return [
    line(header),
    line(Array.from({ length: width }, () => "---")),
    ...body.map(line),
  ].join("\n");
}

// text is a text node's words: white space (a non-breaking space too)
// collapsed, and anything that would read as a tag escaped.
function text(value: string): string {
  return escapeTags(collapse(value));
}

function collapse(value: string): string {
  return value.replace(/\s+/g, " ");
}

// escapeTags escapes a < that would start a tag or comment, so no raw
// HTML is ever written.
function escapeTags(value: string): string {
  return value.replace(/<(?=[A-Za-z/!?])/g, "\\<");
}

function oneLine(value: string): string {
  return value
    .replace(/\s*\n\s*/g, " ")
    .replace(/ {2,}/g, " ")
    .trim();
}

// tidy is a paragraph's text: single spaces, none around line breaks.
function tidy(value: string): string {
  return value
    .replace(/ {2,}/g, " ")
    .replace(/ *\n */g, "\n")
    .trim();
}
