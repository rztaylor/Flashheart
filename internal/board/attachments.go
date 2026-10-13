package board

import (
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// attachmentTypes is the REV-2 allow-list, in the order messages name it:
// extension → served content type. SVG and HTML are deliberately absent.
var attachmentTypes = []struct{ ext, contentType string }{
	{".png", "image/png"},
	{".jpg", "image/jpeg"},
	{".jpeg", "image/jpeg"},
	{".gif", "image/gif"},
	{".webp", "image/webp"},
	{".pdf", "application/pdf"},
	{".txt", "text/plain; charset=utf-8"},
	{".log", "text/plain; charset=utf-8"},
	{".md", "text/markdown; charset=utf-8"},
	{".json", "application/json"},
}

// AttachmentKinds are the kinds a files index records.
var AttachmentKinds = []string{"screenshot", "log", "other"}

// AttachmentKind guesses an attachment's kind from its file name:
// screenshot for images, log for plain text, else other.
func AttachmentKind(name string) string {
	contentType, _ := AttachmentType(name)
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "screenshot"
	case strings.HasPrefix(contentType, "text/plain"):
		return "log"
	}
	return "other"
}

// AttachmentKindFor is kind when it is one of AttachmentKinds, the guess
// from name when it is empty, else other.
func AttachmentKindFor(kind, name string) string {
	switch {
	case kind == "":
		return AttachmentKind(name)
	case kind == "screenshot", kind == "log", kind == "other":
		return kind
	}
	return "other"
}

// AttachmentType returns the content type for an allowed attachment file
// name, judged by extension only (REV-2, SEC-4).
func AttachmentType(name string) (string, bool) {
	ext := strings.ToLower(path.Ext(name))
	for _, t := range attachmentTypes {
		if t.ext == ext {
			return t.contentType, true
		}
	}
	return "", false
}

// AttachmentScrubbed reports a content type whose text is scrubbed of
// likely secrets before it is stored (SEC-3).
func AttachmentScrubbed(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") || contentType == "application/json"
}

// AllowedAttachments names the allowed extensions for a message: "png,
// jpg, …".
func AllowedAttachments() string {
	names := make([]string, len(attachmentTypes))
	for index, t := range attachmentTypes {
		names[index] = strings.TrimPrefix(t.ext, ".")
	}
	return strings.Join(names, ", ")
}

// SizeWords is a size limit as a person reads it: "20 MB", or bytes when
// it is not whole megabytes.
func SizeWords(limit int64) string {
	if limit >= 1<<20 && limit%(1<<20) == 0 {
		return strconv.FormatInt(limit>>20, 10) + " MB"
	}
	return strconv.FormatInt(limit, 10) + " bytes"
}

// SniffedAttachment is what a file's bytes were judged to be (REV-2): the
// extension it is stored under, which serving reads back, and that
// extension's content type.
type SniffedAttachment struct {
	Ext, ContentType string
}

// signatures are the allow-listed binary types by their first bytes (and,
// for WebP, the bytes at an offset too).
var signatures = []struct {
	prefix, also string
	at           int
	ext          string
}{
	{prefix: "\x89PNG\r\n\x1a\n", ext: ".png"},
	{prefix: "\xff\xd8\xff", ext: ".jpg"},
	{prefix: "GIF87a", ext: ".gif"},
	{prefix: "GIF89a", ext: ".gif"},
	{prefix: "RIFF", also: "WEBP", at: 8, ext: ".webp"},
	{prefix: "%PDF-", ext: ".pdf"},
}

// textExts are the allow-listed text extensions a text file keeps; other
// text is stored as .txt.
var textExts = []string{".txt", ".log", ".md", ".json"}

// markupNames are names whose text a browser would take as markup.
var markupNames = map[string]string{
	".svg": "an SVG image", ".svgz": "an SVG image",
	".html": "an HTML page", ".htm": "an HTML page", ".xhtml": "an HTML page",
	".xml": "an XML document",
}

// htmlTags open an HTML document as the WHATWG MIME sniffing standard
// recognises one (§7.1), when followed by a space or ">".
var htmlTags = []string{
	"<!doctype html", "<html", "<head", "<script", "<iframe", "<h1", "<div",
	"<font", "<table", "<a", "<style", "<title", "<b", "<body", "<br", "<p",
}

// SniffAttachment judges a file sent from the browser by its content, not
// its name (REV-2, FH-18): PNG, JPEG, GIF, WebP and PDF by their
// signatures, whatever the name says; text (UTF-8 with no binary control
// bytes) under its own text extension or .txt. SVG, HTML and XML, by
// their opening or by a name that says so, and everything else are
// refused: refusal says why in words, "" when the file is allowed.
func SniffAttachment(name string, data []byte) (SniffedAttachment, string) {
	content := string(data)
	for _, signature := range signatures {
		if strings.HasPrefix(content, signature.prefix) &&
			strings.HasPrefix(content[min(signature.at, len(content)):], signature.also) {
			contentType, _ := AttachmentType(signature.ext)
			return SniffedAttachment{Ext: signature.ext, ContentType: contentType}, ""
		}
	}
	if !isText(content) {
		return SniffedAttachment{}, "its type is not allowed"
	}
	if markup := markupOf(content); markup != "" {
		return SniffedAttachment{}, "it is " + markup
	}
	ext := strings.ToLower(path.Ext(name))
	if markup, ok := markupNames[ext]; ok {
		return SniffedAttachment{}, "it is " + markup
	}
	if !slices.Contains(textExts, ext) {
		ext = ".txt"
	}
	contentType, _ := AttachmentType(ext)
	return SniffedAttachment{Ext: ext, ContentType: contentType}, ""
}

// isText reports UTF-8 text with none of the control bytes the WHATWG
// standard counts as binary (tab, line feed, form feed, carriage return
// and escape are text).
func isText(content string) bool {
	if !utf8.ValidString(content) {
		return false
	}
	for index := 0; index < len(content); index++ {
		switch b := content[index]; {
		case b == '\t', b == '\n', b == '\f', b == '\r', b == 0x1b:
		case b < 0x20:
			return false
		}
	}
	return true
}

const utf8BOM = "\xef\xbb\xbf"

// markupOf names the markup text opens with ("an HTML page"), or "".
func markupOf(content string) string {
	opening := strings.ToLower(strings.TrimLeft(strings.TrimPrefix(content, utf8BOM), " \t\n\r\f"))
	switch {
	case strings.HasPrefix(opening, "<svg"):
		return "an SVG image"
	case strings.HasPrefix(opening, "<?xml"):
		return "an XML document, possibly SVG"
	case strings.HasPrefix(opening, "<!--"):
		return "an HTML page"
	}
	for _, tag := range htmlTags {
		if rest, ok := strings.CutPrefix(opening, tag); ok && (rest == "" || rest[0] == ' ' || rest[0] == '>') {
			return "an HTML page"
		}
	}
	return ""
}
