package board

import (
	"path"
	"strings"
)

// attachmentTypes is the REV-2 allow-list: extension → served content type.
// SVG and HTML are deliberately absent.
var attachmentTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".pdf":  "application/pdf",
	".txt":  "text/plain; charset=utf-8",
	".log":  "text/plain; charset=utf-8",
	".md":   "text/markdown; charset=utf-8",
	".json": "application/json",
}

// AttachmentType returns the content type for an allowed attachment file
// name, judged by extension only (REV-2, SEC-4).
func AttachmentType(name string) (string, bool) {
	contentType, ok := attachmentTypes[strings.ToLower(path.Ext(name))]
	return contentType, ok
}
