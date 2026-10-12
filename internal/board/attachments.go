package board

import (
	"path"
	"strings"
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
