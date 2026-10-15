package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rztaylor/flashheart/internal/imagefit"
	"github.com/rztaylor/flashheart/internal/mdfile"
)

// picture is a width×height JPEG or PNG of one colour.
func picture(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 90, A: 255})
		}
	}
	var buffer bytes.Buffer
	var err error
	if format == "jpeg" {
		err = jpeg.Encode(&buffer, img, nil)
	} else {
		err = png.Encode(&buffer, img)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// project.yaml names the project's image (FH-151): a plain file name in
// the project folder, read like every other field.
func TestParseProjectFileReadsTheImage(t *testing.T) {
	t.Parallel()

	f := parseProjectFile("app", []byte("name: App\nimage: image.png\n"))
	if f.Image != "image.png" || len(f.Warnings) != 0 || !f.readable("image") {
		t.Errorf("image = %q, warnings %q", f.Image, f.Warnings)
	}
	for _, bad := range []string{"image: ../secret.png\n", "image: a/b.png\n", "image: [x]\n"} {
		f := parseProjectFile("app", []byte("name: App\n"+bad))
		if f.Image != "" || len(f.Warnings) != 1 || !strings.Contains(f.Warnings[0], "image") {
			t.Errorf("%q: image = %q, warnings %q", bad, f.Image, f.Warnings)
		}
	}
	if f := parseProjectFile("app", []byte("name: App\n")); f.Image != "" || len(f.Warnings) != 0 {
		t.Errorf("no image: %+v", f)
	}
}

// A project whose image is missing or unreadable shows its key and a
// warning, never an error; one with a real image carries its name and a
// version that changes with its bytes.
func TestReadProjectWarnsAboutAMissingOrUnreadableImage(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	file := filepath.Join(root, "alpha", "project.yaml")
	data, _ := os.ReadFile(file)
	write(t, file, string(data)+"image: image.png\n")

	project, err := s.ReadProject("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if project.Image != "" || len(project.Warnings) != 1 || project.Warnings[0] != "project image image.png is missing" {
		t.Errorf("missing: image %q, warnings %q", project.Image, project.Warnings)
	}

	write(t, filepath.Join(root, "alpha", "image.png"), "not a picture at all")
	project, _ = s.ReadProject("alpha")
	if project.Image != "" || len(project.Warnings) != 1 || project.Warnings[0] != "project image image.png is not a PNG or JPEG image" {
		t.Errorf("unreadable: image %q, warnings %q", project.Image, project.Warnings)
	}

	write(t, filepath.Join(root, "alpha", "image.png"), string(picture(t, "png", 128, 128)))
	project, _ = s.ReadProject("alpha")
	if project.Image != "image.png" || project.ImageVersion == "" || len(project.Warnings) != 0 {
		t.Fatalf("real: image %q, version %q, warnings %q", project.Image, project.ImageVersion, project.Warnings)
	}
	first := project.ImageVersion
	write(t, filepath.Join(root, "alpha", "image.png"), string(picture(t, "png", 128, 64)))
	if project, _ = s.ReadProject("alpha"); project.ImageVersion == first {
		t.Errorf("the version did not change with the bytes")
	}
	// Beta, with no image field, is as before.
	if beta, _ := s.ReadProject("beta"); beta.Image != "" || len(beta.Warnings) != 0 {
		t.Errorf("beta: image %q, warnings %q", beta.Image, beta.Warnings)
	}
}

// Setting a project's image stores the fitted PNG under ProjectImageFile
// and names it in project.yaml, keeping every other line; replacing it
// leaves one file and no temporary files behind.
func TestSetProjectImageStoresAFittedPNGAndNamesIt(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	file := filepath.Join(root, "alpha", "project.yaml")
	before, _ := os.ReadFile(file)

	if err := s.SetProjectImage("alpha", picture(t, "jpeg", 900, 600)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	if string(after) != string(before)+"image: image.png\n" {
		t.Errorf("project.yaml = %q", after)
	}
	stored, err := os.ReadFile(filepath.Join(root, "alpha", ProjectImageFile))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(stored))
	if err != nil || img.Bounds().Dx() != imagefit.Side || img.Bounds().Dy() != imagefit.Side {
		t.Fatalf("stored image: %v %v", img, err)
	}
	project, err := s.ReadProject("alpha")
	if err != nil || project.Image != "image.png" || project.ImageVersion == "" || len(project.Warnings) != 0 {
		t.Fatalf("project: image %q, version %q, warnings %q, %v", project.Image, project.ImageVersion, project.Warnings, err)
	}
	version := project.ImageVersion

	// Replacing it: the same name, new bytes, no stray files.
	if err := s.SetProjectImage("alpha", picture(t, "png", 20, 50)); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(file); string(again) != string(after) {
		t.Errorf("a replacement rewrote project.yaml: %q", again)
	}
	replaced, _ := os.ReadFile(filepath.Join(root, "alpha", ProjectImageFile))
	if bytes.Equal(replaced, stored) {
		t.Errorf("the image was not replaced")
	}
	if project, _ = s.ReadProject("alpha"); project.ImageVersion == version {
		t.Errorf("the version did not change on replacement")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "alpha"))
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{".archive", ".flashheart", "image.png", "project.yaml", "tickets", "workstreams"}) {
		t.Errorf("alpha/ = %q", names)
	}
}

// An image set by hand under another name gives way to the upload: the
// field now names image.png and the old file is removed.
func TestSetProjectImageReplacesAnImageUnderAnotherName(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	file := filepath.Join(root, "alpha", "project.yaml")
	data, _ := os.ReadFile(file)
	write(t, file, "image: logo.png\n"+string(data))
	write(t, filepath.Join(root, "alpha", "logo.png"), string(picture(t, "png", 10, 10)))

	if err := s.SetProjectImage("alpha", picture(t, "png", 10, 10)); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(file); string(after) != "image: image.png\n"+string(data) {
		t.Errorf("project.yaml = %q", after)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "logo.png")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("logo.png was kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "image.png")); err != nil {
		t.Errorf("image.png: %v", err)
	}
}

// What is not a JPEG or PNG, by its bytes, or too large, is refused in
// words and nothing is written (SEC-7).
func TestSetProjectImageRefusesAndWritesNothing(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	file := filepath.Join(root, "alpha", "project.yaml")
	before, _ := os.ReadFile(file)
	huge := picture(t, "png", 8, 8)
	cases := map[string]struct {
		data []byte
		kind error
		want string
	}{
		"text":              {[]byte("just some words"), ErrTypeNotAllowed, "it is not a JPEG or PNG image"},
		"an SVG":            {[]byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"), ErrTypeNotAllowed, "it is an SVG image"},
		"a PDF":             {[]byte("%PDF-1.7\n"), ErrTypeNotAllowed, "it is not a JPEG or PNG image"},
		"a GIF":             {[]byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"), ErrTypeNotAllowed, "it is not a JPEG or PNG image"},
		"too many bytes":    {append(huge, make([]byte, MaxProjectImageBytes)...), ErrTooLarge, "larger than 20 MB"},
		"too many pixels":   {hugePNG(), ErrTooLarge, "more than the 50 megapixel limit"},
		"a truncated image": {huge[:len(huge)-10], ErrTypeNotAllowed, "it could not be read as an image"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			err := s.SetProjectImage("alpha", c.data)
			if !errors.Is(err, c.kind) || !strings.Contains(err.Error(), c.want) {
				t.Errorf("SetProjectImage = %v, want %v about %q", err, c.kind, c.want)
			}
		})
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Errorf("a refusal changed project.yaml: %q", after)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ProjectImageFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refusal wrote the image: %v", err)
	}
	if err := s.SetProjectImage("nowhere", huge); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown project: %v", err)
	}
	write(t, file, "image: [oops\n")
	if err := s.SetProjectImage("alpha", huge); !errors.Is(err, mdfile.ErrBrokenFrontmatter) {
		t.Errorf("broken project.yaml: %v", err)
	}
}

// hugePNG is a PNG header claiming 100,000×100,000 pixels.
func hugePNG() []byte {
	ihdr := []byte("IHDR\x00\x01\x86\xa0\x00\x01\x86\xa0\x08\x02\x00\x00\x00")
	out := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\r")
	out = append(out, ihdr...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(ihdr))
}

// Clearing removes the stored file and the field, and nothing else.
func TestClearProjectImageRemovesTheFileAndTheField(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	file := filepath.Join(root, "alpha", "project.yaml")
	before, _ := os.ReadFile(file)
	if err := s.SetProjectImage("alpha", picture(t, "png", 30, 30)); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearProjectImage("alpha"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Errorf("project.yaml = %q, want it as before", after)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", ProjectImageFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("image.png was kept: %v", err)
	}
	project, _ := s.ReadProject("alpha")
	if project.Image != "" || len(project.Warnings) != 0 {
		t.Errorf("after clearing: image %q, warnings %q", project.Image, project.Warnings)
	}

	// Nothing to clear writes nothing; a field naming a missing file is
	// still cleared.
	info, _ := os.Stat(file)
	if err := s.ClearProjectImage("alpha"); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.Stat(file); !again.ModTime().Equal(info.ModTime()) {
		t.Errorf("clearing nothing rewrote project.yaml")
	}
	write(t, file, string(before)+"image: gone.png\n")
	if err := s.ClearProjectImage("alpha"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Errorf("project.yaml = %q", after)
	}
	if err := s.ClearProjectImage("nowhere"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown project: %v", err)
	}
}

// The stored image is opened for serving by the name project.yaml gives,
// a PNG or JPEG in the project folder and nothing else.
func TestOpenProjectImageServesOnlyThatFile(t *testing.T) {
	t.Parallel()

	s, root := writable(t)
	if err := s.SetProjectImage("alpha", picture(t, "png", 30, 30)); err != nil {
		t.Fatal(err)
	}
	handle, contentType, err := s.OpenProjectImage("alpha", "image.png")
	if err != nil || contentType != "image/png" {
		t.Fatalf("OpenProjectImage = %v, %q", err, contentType)
	}
	handle.Close()
	for _, bad := range []string{"../alpha/image.png", "project.yaml", "missing.png", "tickets"} {
		if handle, _, err := s.OpenProjectImage("alpha", bad); err == nil {
			handle.Close()
			t.Errorf("%q was served", bad)
		}
	}
	if _, _, err := s.OpenProjectImage("alpha", "project.yaml"); !errors.Is(err, ErrTypeNotAllowed) {
		t.Errorf("project.yaml: %v", err)
	}
	if _, _, err := s.OpenProjectImage("alpha", "missing.png"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	write(t, filepath.Join(root, "alpha", "logo.jpg"), string(picture(t, "jpeg", 4, 4)))
	handle, contentType, err = s.OpenProjectImage("alpha", "logo.jpg")
	if err != nil || contentType != "image/jpeg" {
		t.Fatalf("logo.jpg = %v, %q", err, contentType)
	}
	handle.Close()
}
