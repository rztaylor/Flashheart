package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/rztaylor/flashheart/internal/board"
	"github.com/rztaylor/flashheart/internal/imagefit"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// ProjectImageFile is the name a project's uploaded image is stored under,
// in the project folder, and the value project.yaml's `image` takes
// (FH-151, board-format §project.yaml).
const ProjectImageFile = "image.png"

// MaxProjectImageBytes bounds an uploaded picture before it is decoded.
const MaxProjectImageBytes int64 = 20 << 20

// projectImageSignature reports data that opens as a PNG or JPEG, the two
// kinds a project image may be (board.SniffAttachment, SEC-7).
func projectImageSignature(data []byte) bool {
	sniffed, refusal := board.SniffAttachment("image", data)
	return refusal == "" && (sniffed.Ext == ".png" || sniffed.Ext == ".jpg")
}

// SetProjectImage stores a picture sent from the board as the project's
// image: judged by its bytes (SEC-7), cropped to its centre square, scaled
// to 128×128 and written as ProjectImageFile in one transaction with
// project.yaml's `image` field, under the project lock. An earlier image
// under another name is removed. A picture that is not a JPEG or PNG, or
// cannot be decoded, is ErrTypeNotAllowed; one over the byte or pixel
// limit is ErrTooLarge; each says why in words, and nothing is written.
func (s *Store) SetProjectImage(project string, data []byte) error {
	if err := s.checkProject(project); err != nil {
		return err
	}
	if int64(len(data)) > MaxProjectImageBytes {
		return fmt.Errorf("the image was not saved because it is larger than %s, the limit: %w", board.SizeWords(MaxProjectImageBytes), ErrTooLarge)
	}
	if sniffed, refusal := board.SniffAttachment("image", data); refusal == "" && sniffed.Ext != ".png" && sniffed.Ext != ".jpg" {
		return fmt.Errorf("the image was not saved because it is not a JPEG or PNG image: %w", ErrTypeNotAllowed)
	} else if refusal != "" && !strings.HasPrefix(refusal, "its type") {
		return fmt.Errorf("the image was not saved because %s: %w", refusal, ErrTypeNotAllowed)
	}
	fitted, err := imagefit.Fit(data)
	var refusal *imagefit.Refusal
	switch {
	case errors.As(err, &refusal) && refusal.TooLarge:
		return fmt.Errorf("the image was not saved because %s: %w", refusal.Words, ErrTooLarge)
	case errors.As(err, &refusal):
		return fmt.Errorf("the image was not saved because %s: %w", refusal.Words, ErrTypeNotAllowed)
	case err != nil:
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	name := path.Join(project, "project.yaml")
	t := s.begin()
	current, err := t.read(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	parsed := parseProjectFile(project, current)
	if !parsed.readable("image") {
		return fmt.Errorf("%s: %w: %s", name, mdfile.ErrBrokenFrontmatter, strings.Join(parsed.Warnings, "; "))
	}
	out, err := mdfile.SetYAMLScalar(current, "image", ProjectImageFile)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	t.put(path.Join(project, ProjectImageFile), fitted)
	if string(out) != string(current) {
		t.put(name, out)
	}
	if parsed.Image != "" && parsed.Image != ProjectImageFile {
		t.remove(path.Join(project, parsed.Image))
	}
	return t.commit()
}

// ClearProjectImage removes the project's image: the file project.yaml
// names and the `image` field, under the project lock. A project with no
// image writes nothing.
func (s *Store) ClearProjectImage(project string) error {
	if err := s.checkProject(project); err != nil {
		return err
	}
	release, err := s.lock(project)
	if err != nil {
		return err
	}
	defer release()
	name := path.Join(project, "project.yaml")
	t := s.begin()
	current, err := t.read(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	parsed := parseProjectFile(project, current)
	if !parsed.readable("image") {
		return fmt.Errorf("%s: %w: %s", name, mdfile.ErrBrokenFrontmatter, strings.Join(parsed.Warnings, "; "))
	}
	out, err := mdfile.RemoveYAMLField(current, "image")
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if string(out) == string(current) {
		return nil
	}
	t.put(name, out)
	if parsed.Image != "" {
		t.remove(path.Join(project, parsed.Image))
	}
	return t.commit()
}

// OpenProjectImage opens the project's image file for serving, with its
// content type: a PNG or JPEG in the project folder, by the name
// project.yaml gives (the board never serves any other file there).
func (s *Store) OpenProjectImage(project, file string) (*os.File, string, error) {
	if !ValidProject(project) || !ValidName(file) {
		return nil, "", fmt.Errorf("project image %s/%s: %w", project, file, ErrInvalidName)
	}
	contentType, ok := board.AttachmentType(file)
	if !ok || (contentType != "image/png" && contentType != "image/jpeg") {
		return nil, "", fmt.Errorf("project image %s: %w", file, ErrTypeNotAllowed)
	}
	root, _, err := s.handle()
	if err != nil {
		return nil, "", err
	}
	handle, err := root.Open(path.Join(project, file))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("project image %s: %w", file, ErrNotFound)
	}
	if err != nil {
		return nil, "", fmt.Errorf("project image %s: %w", file, err)
	}
	if info, err := handle.Stat(); err != nil || !info.Mode().IsRegular() {
		handle.Close()
		return nil, "", fmt.Errorf("project image %s: %w", file, ErrNotFound)
	}
	return handle, contentType, nil
}

// readProjectImage checks the image project.yaml names and sets the
// project's Image and ImageVersion, or a warning when it is missing or not
// a PNG or JPEG: the badge then shows the key.
func (r *reader) readProjectImage(project *board.Project, file string) {
	if file == "" {
		return
	}
	name := path.Join(project.Name, file)
	if !r.exists(name) {
		project.Warnings = append(project.Warnings, fmt.Sprintf("project image %s is missing", file))
		return
	}
	data, _, problem := r.read(name)
	if problem != "" {
		project.Warnings = append(project.Warnings, fmt.Sprintf("project image %s: %s", file, problem))
		return
	}
	if !projectImageSignature(data) {
		project.Warnings = append(project.Warnings, fmt.Sprintf("project image %s is not a PNG or JPEG image", file))
		return
	}
	sum := sha256.Sum256(data)
	project.Image, project.ImageVersion = file, hex.EncodeToString(sum[:4])
}
