import { describe, expect, it } from "vitest";

import { htmlToMarkdown } from "./htmlToMarkdown";

// A browser's DOMParser document loads nothing (frontend/e2e/editor.spec.mjs
// checks it); happy-dom's would load frames and files, so it is told not to.
const happyDOM = (window as { happyDOM?: { settings: object } }).happyDOM;
Object.assign(happyDOM?.settings ?? {}, {
  disableIframePageLoading: true,
  disableJavaScriptFileLoading: true,
  disableCSSFileLoading: true,
  handleDisabledFileLoadingAsSuccess: true,
});

// Pasted web HTML becomes markdown (FH-142): what a copy from a page
// carries, as Chrome, Firefox and Safari put it on the clipboard (a
// <meta charset>, inline styles, highlighter spans, Google Docs' wrapper).
// Links keep only allow-listed schemes, remote images become links, and
// nothing but markdown comes out.
const cases: { name: string; html: string; markdown: string }[] = [
  {
    name: "headings are demoted to levels 3 and 4 so none starts a section",
    html: "<meta charset='utf-8'><h1>Title</h1><h2>Sub</h2><h3>Three</h3><h4>Four</h4><h5>Five</h5>",
    markdown: "### Title\n\n### Sub\n\n### Three\n\n#### Four\n\n##### Five",
  },
  {
    name: "emphasis, strikethrough and inline code",
    html: "<p>Some <strong>bold</strong>, <em>italic</em>, <del>gone</del> and <code>get_ticket</code>.</p>",
    markdown: "Some **bold**, *italic*, ~~gone~~ and `get_ticket`.",
  },
  {
    name: "emphasis keeps its spaces outside the marks, and never doubles",
    html: "<p>a<b> bold </b>b <strong><b>once</b></strong> <i></i>end</p>",
    markdown: "a **bold** b **once** end",
  },
  {
    name: "inline code holding a backtick gets a longer fence",
    html: "<p>Run <code>echo `date`</code> now</p>",
    markdown: "Run `` echo `date` `` now",
  },
  {
    name: "http, https, mailto and relative files/ links are kept",
    html: '<p><a href="https://example.com/a b">docs</a>, <a href="http://example.com/x_(y)">wiki</a>, <a href="mailto:ops@example.com">mail</a>, <a href="files/plan.pdf">plan</a></p>',
    markdown:
      "[docs](https://example.com/a%20b), [wiki](http://example.com/x_%28y%29), [mail](mailto:ops@example.com), [plan](files/plan.pdf)",
  },
  {
    name: "a link whose text is its address stays a bare address",
    html: '<p>See <a href="https://example.com/">https://example.com/</a></p>',
    markdown: "See https://example.com/",
  },
  {
    name: "javascript:, data:, vbscript: and other schemes are removed, keeping the text",
    html: [
      '<p><a href="javascript:alert(1)">one</a>',
      '<a href=" JaVaScRiPt:alert(1)">two</a>',
      '<a href="java&#x09;script:alert(1)">three</a>',
      '<a href="data:text/html,<script>alert(1)</script>">four</a>',
      '<a href="vbscript:msgbox(1)">five</a>',
      '<a href="file:///etc/passwd">six</a>',
      '<a href="blob:https://example.com/1">seven</a>',
      '<a href="/wiki/Relative">eight</a>',
      '<a href="#top">nine</a>',
      '<a href="files/../../secret">ten</a></p>',
    ].join(" "),
    markdown: "one two three four five six seven eight nine ten",
  },
  {
    name: "a link with no words (a heading's anchor icon) is dropped",
    html: '<h2><a class="anchor" href="https://example.com/#install" aria-hidden="true"><svg><path d="M0"></path></svg></a>Install</h2>',
    markdown: "### Install",
  },
  {
    name: "a remote image becomes a link to it, named by its alt text or file",
    html: '<p><img src="https://example.com/cat.png" alt="A cat"> <img src="https://example.com/img/dog.jpg?w=2"></p>',
    markdown:
      "[A cat](https://example.com/cat.png) [dog.jpg](https://example.com/img/dog.jpg?w=2)",
  },
  {
    name: "a data: or other image keeps only its alt text; a files/ image stays an image",
    html: '<p><img src="data:image/png;base64,iVBORw0KGgo=" alt="inline"> <img src="javascript:alert(1)"> <img src="files/shot.png" alt="shot"></p>',
    markdown: "inline ![shot](files/shot.png)",
  },
  {
    name: "an image inside a link is the link's words",
    html: '<p><a href="https://example.com/"><img src="https://example.com/logo.png" alt="Logo"></a></p>',
    markdown: "[Logo](https://example.com/)",
  },
  {
    name: "nested lists, numbered from their start",
    html: "<ul><li>One<ul><li>One a</li><li>One b<ol start='3'><li>Third</li><li>Fourth</li></ol></li></ul></li><li><p>Two</p></li></ul>",
    markdown: "- One\n  - One a\n  - One b\n    3. Third\n    4. Fourth\n- Two",
  },
  {
    name: "a nested list as a sibling of its item (Google Docs) nests under it",
    html: "<ol><li>First</li><ol><li>Inner</li></ol><li>Second</li></ol>",
    markdown: "1. First\n   1. Inner\n2. Second",
  },
  {
    name: "a list item with two paragraphs makes the list loose",
    html: "<ul><li><p>Para one</p><p>Para two</p></li><li>Next</li></ul>",
    markdown: "- Para one\n\n  Para two\n\n- Next",
  },
  {
    name: "task lists keep their boxes",
    html: '<ul class="contains-task-list"><li class="task-list-item"><input type="checkbox" disabled checked> Done</li><li class="task-list-item"><input type="checkbox" disabled> Open</li></ul>',
    markdown: "- [x] Done\n- [ ] Open",
  },
  {
    name: "a highlighted code block keeps its text, lines and language",
    html: '<div class="highlight highlight-source-go"><pre><span class="pl-k">func</span> <span class="pl-en">main</span>() {\n\tfmt.<span class="pl-en">Println</span>(<span class="pl-s">"hi &lt;b&gt;"</span>)\n}\n</pre></div>',
    markdown: '```go\nfunc main() {\n\tfmt.Println("hi <b>")\n}\n```',
  },
  {
    name: "a code block holding a fence gets a longer one, its <br>s as lines",
    html: '<pre><code class="language-markdown">```sh<br>ls<br>```</code></pre>',
    markdown: "````markdown\n```sh\nls\n```\n````",
  },
  {
    name: "a code block inside a list item is indented with it",
    html: "<ol><li>Run:<pre><code>make\nmake test</code></pre></li></ol>",
    markdown: "1. Run:\n\n   ```\n   make\n   make test\n   ```",
  },
  {
    name: "quotes, nested and with paragraphs",
    html: "<blockquote><p>Said this.</p><blockquote><p>And that.</p></blockquote></blockquote>",
    markdown: "> Said this.\n>\n> > And that.",
  },
  {
    name: "tables become GFM tables, pipes escaped",
    html: "<table><thead><tr><th>Name</th><th>Value</th></tr></thead><tbody><tr><td><code>a|b</code></td><td><b>1</b></td></tr><tr><td>only</td></tr></tbody></table>",
    markdown:
      "| Name | Value |\n| --- | --- |\n| `a\\|b` | **1** |\n| only |  |",
  },
  {
    name: "a rule and a line break",
    html: "<p>Line one<br>Line two</p><hr><p>After</p>",
    markdown: "Line one\nLine two\n\n---\n\nAfter",
  },
  {
    name: "unknown elements are reduced to their text; scripts, styles, frames and hidden parts go",
    html: [
      "<div><custom-card>Hello</custom-card> <span style='color:red'>there</span>",
      "<script>alert(1)</script><style>p{color:red}</style>",
      "<iframe>frame</iframe><object data='https://example.com/x'></object>",
      "<svg><text>icon</text></svg><button>Copy</button><span hidden>secret</span>",
      "<span style='display:none'>gone</span><noscript>no</noscript><template><b>t</b></template>",
      "<video src='https://example.com/v.mp4' poster='https://example.com/p.png'>video</video></div>",
      "<p><strong>kept</strong> <font color='red'>font</font> <u>under</u> <sup>[1]</sup></p>",
    ].join(""),
    markdown: "Hello there\n\n**kept** font under [1]",
  },
  {
    name: "text that looks like a tag is escaped, never raw HTML",
    html: "<p>Use <b>&lt;details&gt;</b> and &lt;!-- this --&gt;, but 1 &lt; 2.</p>",
    markdown: "Use **\\<details>** and \\<!-- this -->, but 1 < 2.",
  },
  {
    name: "Google Docs: its normal-weight wrapper is not bold, its styled spans are",
    html: '<meta charset="utf-8"><b style="font-weight:normal;" id="docs-internal-guid-1"><h1 dir="ltr"><span style="font-size:20pt;font-weight:400;white-space:pre;white-space:pre-wrap;">Plan</span></h1><p dir="ltr"><span style="font-weight:700;">Bold</span><span style="font-weight:400;"> and </span><span style="font-style:italic;">italic</span><span style="font-weight:400;"> or </span><span style="font-weight:700;font-style:italic;">both</span></p><ul><li dir="ltr"><p dir="ltr"><span>One</span></p></li><li dir="ltr"><p dir="ltr"><span>Two</span></p></li></ul></b>',
    markdown: "### Plan\n\n**Bold** and *italic* or ***both***\n\n- One\n- Two",
  },
  {
    name: "Google Docs: a line with only styled spans still converts",
    html: '<meta charset="utf-8"><b style="font-weight:normal;" id="docs-internal-guid-2"><span style="font-weight:700;">Only</span><span style="font-weight:400;"> bold here</span></b>',
    markdown: "**Only** bold here",
  },
  {
    // Inline styles and handlers are renamed before parsing (the page's
    // CSP would report each); words in other attributes are left as given.
    name: "a value that reads like a style or handler is kept as given",
    html: '<p><B STYLE="font-weight:bold" ONCLICK="x()">Loud</B> <img src="https://example.com/a.png" alt="set style=bold onload=go"></p>',
    markdown: "**Loud** [set style=bold onload=go](https://example.com/a.png)",
  },
  {
    name: "non-breaking spaces and runs of white space read as one space",
    html: "<p>a&nbsp;b \n\t c</p><ul>\n  <li>\n    x\n  </li>\n</ul>",
    markdown: "a b c\n\n- x",
  },
];

describe("htmlToMarkdown", () => {
  it.each(cases)("$name", ({ html, markdown }) => {
    expect(htmlToMarkdown(html)).toBe(markdown);
  });

  it("never gives raw HTML: outside code, no tag survives", () => {
    for (const { name, html } of cases) {
      const text = (htmlToMarkdown(html) ?? "")
        .replace(/^(`{3,})[^\n]*\n[\s\S]*?\n\s*\1$/gm, "")
        .replace(/(`+)[^`]*?\1/g, "");
      expect(text, name).not.toMatch(/(^|[^\\])<[a-z!/?]/i);
    }
  });

  // Copies with nothing markdown would keep (a code editor's coloured
  // lines, plain paragraphs) give null, so the plain text is pasted.
  it.each([
    [
      "a code editor's coloured lines",
      '<meta charset="utf-8"><div style="color: #ccc;background-color: #1f1f1f;font-family: Menlo, monospace;white-space: pre;"><div><span style="color: #c586c0;font-weight: bold;">import</span><span> x</span></div><div><span>y</span></div></div>',
    ],
    ["plain paragraphs", "<p>One</p><p>Two</p>"],
    ["a span of text", "<span>just text</span>"],
    ["only scripts", "<script>alert(1)</script>"],
    ["an image with nothing to keep", '<img src="data:image/png;base64,AA">'],
    ["nothing", ""],
  ])("gives null for %s", (_name, html) => {
    expect(htmlToMarkdown(html)).toBeNull();
  });
});
