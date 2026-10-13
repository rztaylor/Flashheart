import { markdownLanguage } from "@codemirror/lang-markdown";
import { LanguageSupport } from "@codemirror/language";

// markdownSupport parses the text as GitHub-flavoured markdown, the
// dialect the view renders with remark-gfm (CARD-2). It is the language
// alone: lang-markdown's markdown() helper would also load HTML,
// JavaScript and CSS parsers for embedded code, which the editor does not
// highlight. Subscript, superscript and emoji, which the parser also
// knows, are left as text by the preview, as the view leaves them (a
// single-tilde span is struck through, as remark-gfm reads it).
export const markdownSupport = new LanguageSupport(markdownLanguage);
