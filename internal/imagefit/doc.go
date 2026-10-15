// Package imagefit turns a picture a person uploaded into a project's badge
// image (FH-151): a JPEG or PNG, judged by its bytes, checked for size
// before any pixel is decoded (DecodeConfig against MaxPixels), cropped to
// its centre square, scaled to Side×Side and written as a plain PNG, so
// the board keeps a small file and nothing the source carried besides its
// pixels (EXIF, location). A JPEG's EXIF orientation is honoured, read by
// this package's own small scan of the file's segments.
//
// It is pure (no I/O) and knows nothing of projects, files or HTTP: store
// decides where the result lives and api how it is asked for. Anything it
// will not take is refused in words, for the person who sent it.
package imagefit
