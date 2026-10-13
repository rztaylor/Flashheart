package board

import (
	"strings"
	"testing"
)

// A file from the browser is judged by its bytes, not its name (REV-2,
// FH-18): an image or PDF by its signature, whatever it is called; text by
// being text, kept under its own text extension or .txt; markup (SVG,
// HTML, XML) and anything else refused.
func TestSniffAttachmentJudgesTheContent(t *testing.T) {
	t.Parallel()

	png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	for _, tc := range []struct {
		name, data         string
		ext, contentType   string
		refused, refusedAs string
	}{
		{name: "shot.png", data: png, ext: ".png", contentType: "image/png"},
		{name: "notes.txt", data: png, ext: ".png", contentType: "image/png"},
		{name: "image", data: png, ext: ".png", contentType: "image/png"},
		{name: "photo.JPEG", data: "\xff\xd8\xff\xe0\x00\x10JFIF", ext: ".jpg", contentType: "image/jpeg"},
		{name: "loop.gif", data: "GIF89a\x01\x00", ext: ".gif", contentType: "image/gif"},
		{name: "a.webp", data: "RIFF\x24\x00\x00\x00WEBPVP8 ", ext: ".webp", contentType: "image/webp"},
		{name: "spec.pdf", data: "%PDF-1.7\n%\xe2\xe3", ext: ".pdf", contentType: "application/pdf"},
		{name: "build.log", data: "\x1b[32mok\x1b[0m\n\tdone\r\n", ext: ".log", contentType: "text/plain; charset=utf-8"},
		{name: "README.md", data: "\xef\xbb\xbf# Title\n\n<p>inline html in markdown</p>\n", ext: ".md", contentType: "text/markdown; charset=utf-8"},
		{name: "data.json", data: `{"a": 1}`, ext: ".json", contentType: "application/json"},
		{name: "table.csv", data: "a,b\n1,2\n", ext: ".txt", contentType: "text/plain; charset=utf-8"},
		{name: "empty.txt", data: "", ext: ".txt", contentType: "text/plain; charset=utf-8"},
		{name: "drawing.png", data: `<svg xmlns="http://www.w3.org/2000/svg"></svg>`, refusedAs: "SVG"},
		{name: "drawing.txt", data: "\n  <?xml version=\"1.0\"?><svg/>", refusedAs: "XML"},
		{name: "page.png", data: "<!DOCTYPE html><html></html>", refusedAs: "HTML"},
		{name: "page.md", data: "\xef\xbb\xbf  <HTML><script>alert(1)</script>", refusedAs: "HTML"},
		{name: "frag.txt", data: "<script>alert(1)</script>", refusedAs: "HTML"},
		{name: "frag.log", data: "<!-- a comment -->", refusedAs: "HTML"},
		{name: "page.html", data: "just words", refusedAs: "HTML"},
		{name: "drawing.svg", data: "just words", refusedAs: "SVG"},
		{name: "app.exe", data: "MZ\x90\x00\x03\x00\x00\x00", refusedAs: "not"},
		{name: "utf16.txt", data: "\xff\xfeh\x00i\x00", refusedAs: "not"},
		{name: "bad.txt", data: "caf\xe9", refusedAs: "not"},
	} {
		sniffed, refusal := SniffAttachment(tc.name, []byte(tc.data))
		if tc.refusedAs != "" {
			if refusal == "" || !strings.Contains(refusal, tc.refusedAs) {
				t.Errorf("%s: sniffed %+v, refusal %q, want one naming %s", tc.name, sniffed, refusal, tc.refusedAs)
			}
			continue
		}
		if refusal != "" || sniffed.Ext != tc.ext || sniffed.ContentType != tc.contentType {
			t.Errorf("%s: sniffed %+v, refusal %q, want %s %s", tc.name, sniffed, refusal, tc.ext, tc.contentType)
		}
		// What it is stored as is served as what it was judged to be.
		if served, ok := AttachmentType("x" + sniffed.Ext); !ok || served != sniffed.ContentType {
			t.Errorf("%s: stored as %s is served as %q", tc.name, sniffed.Ext, served)
		}
	}
}
