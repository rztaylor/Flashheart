package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPicture is a small JPEG or PNG of one colour.
func testPicture(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 30, G: 120, B: 200, A: 255})
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

// sendImage puts body as the project's image, as the board does.
func sendImage(t *testing.T, handler http.Handler, project string, body io.Reader, length int64) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPut, "/api/projects/"+project+"/image", body)
	request.ContentLength = length
	request.Header.Set("Content-Type", "application/octet-stream")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func projectSummary(t *testing.T, handler http.Handler, name string) ProjectSummary {
	t.Helper()
	var response ProjectsResponse
	getJSON(t, handler, "/api/projects", http.StatusOK, &response)
	for _, project := range response.Projects {
		if project.Name == name {
			return project
		}
	}
	t.Fatalf("no project %s in %+v", name, response.Projects)
	return ProjectSummary{}
}

// PUT /api/projects/{project}/image stores the picture as the project's
// 128×128 PNG (FH-151): the projects list then carries its URL, which
// serves it with the SEC-4 headers; DELETE takes both away. No event is
// recorded: a project write changes no ticket (agent-protocol §3).
func TestProjectImageIsSetServedAndCleared(t *testing.T) {
	t.Parallel()

	handler, log := activityAPI(t)
	root := projectSummary(t, handler, "alpha")
	if root.Image != "" {
		t.Fatalf("alpha starts with an image: %q", root.Image)
	}
	before := len(boardLog(t, log))

	jpg := testPicture(t, "jpeg", 300, 200)
	response := sendImage(t, handler, "alpha", bytes.NewReader(jpg), int64(len(jpg)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body)
	}
	var saved WriteResponse
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || saved.Revision == 0 || saved.Hash != "" {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	alpha := projectSummary(t, handler, "alpha")
	if !strings.HasPrefix(alpha.Image, "/api/projects/alpha/image?v=") || len(alpha.Warnings) != 0 {
		t.Fatalf("alpha = image %q, warnings %q", alpha.Image, alpha.Warnings)
	}
	served := httptest.NewRecorder()
	handler.ServeHTTP(served, httptest.NewRequest(http.MethodGet, alpha.Image, nil))
	if served.Code != http.StatusOK || served.Header().Get("Content-Type") != "image/png" ||
		served.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(served.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("served %d %v", served.Code, served.Header())
	}
	img, err := png.Decode(bytes.NewReader(served.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 128 || img.Bounds().Dy() != 128 {
		t.Fatalf("served image = %v, %v", img, err)
	}

	// Replacing it changes the version, so the browser's copy is not stale.
	pngData := testPicture(t, "png", 50, 80)
	if response := sendImage(t, handler, "alpha", bytes.NewReader(pngData), -1); response.Code != http.StatusOK {
		t.Fatalf("replace status = %d: %s", response.Code, response.Body)
	}
	if replaced := projectSummary(t, handler, "alpha"); replaced.Image == alpha.Image || replaced.Image == "" {
		t.Errorf("image after replacing = %q, before %q", replaced.Image, alpha.Image)
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/projects/alpha/image", nil)
	cleared := httptest.NewRecorder()
	handler.ServeHTTP(cleared, request)
	if cleared.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", cleared.Code, cleared.Body)
	}
	if after := projectSummary(t, handler, "alpha"); after.Image != "" || len(after.Warnings) != 0 {
		t.Errorf("after clearing: image %q, warnings %q", after.Image, after.Warnings)
	}
	gone := httptest.NewRecorder()
	handler.ServeHTTP(gone, httptest.NewRequest(http.MethodGet, "/api/projects/alpha/image", nil))
	if gone.Code != http.StatusNotFound {
		t.Errorf("serving a cleared image: %d", gone.Code)
	}
	if got := len(boardLog(t, log)); got != before {
		t.Errorf("project image writes recorded %d events", got-before)
	}
}

// What is not a JPEG or PNG, or too large, is refused in words and the
// project is unchanged; an unknown project is 404 and a read-only server
// 501.
func TestProjectImageRefusesBadUploads(t *testing.T) {
	t.Parallel()

	handler, _ := activityAPI(t)
	refusals := map[string]struct {
		body   string
		length int64
		status int
		code   string
		want   string
	}{
		"text":             {"not a picture", -1, http.StatusUnsupportedMediaType, "type_not_allowed", "The image was not saved because it is not a JPEG or PNG image. Choose a JPEG or PNG image."},
		"an SVG":           {"<svg xmlns='http://www.w3.org/2000/svg'/>", -1, http.StatusUnsupportedMediaType, "type_not_allowed", "it is an SVG image"},
		"a declared giant": {"", 21 << 20, http.StatusRequestEntityTooLarge, "too_large", "larger than 20 MB"},
	}
	for name, c := range refusals {
		t.Run(name, func(t *testing.T) {
			response := sendImage(t, handler, "alpha", strings.NewReader(c.body), c.length)
			var body struct {
				Error struct{ Code, Message string } `json:"error"`
			}
			_ = json.Unmarshal(response.Body.Bytes(), &body)
			if response.Code != c.status || body.Error.Code != c.code || !strings.Contains(body.Error.Message, c.want) {
				t.Errorf("response = %d %+v, want %d %s about %q", response.Code, body.Error, c.status, c.code, c.want)
			}
		})
	}
	if alpha := projectSummary(t, handler, "alpha"); alpha.Image != "" || len(alpha.Warnings) != 0 {
		t.Errorf("a refusal changed alpha: image %q, warnings %q", alpha.Image, alpha.Warnings)
	}
	missing := sendImage(t, handler, "nowhere", bytes.NewReader(testPicture(t, "png", 4, 4)), -1)
	if missing.Code != http.StatusNotFound {
		t.Errorf("unknown project: %d", missing.Code)
	}
	none := httptest.NewRecorder()
	handler.ServeHTTP(none, httptest.NewRequest(http.MethodGet, "/api/projects/alpha/image", nil))
	if none.Code != http.StatusNotFound {
		t.Errorf("a project with no image served %d", none.Code)
	}

	readOnly, _ := sampleAPI(t, nil)
	if response := sendImage(t, readOnly, "alpha", bytes.NewReader(testPicture(t, "png", 4, 4)), -1); response.Code != http.StatusNotImplemented {
		t.Errorf("read-only PUT: %d", response.Code)
	}
}

// A project whose image is missing or unreadable shows the warning in its
// summary and no image URL, and the board goes on (never 500).
func TestProjectImageProblemsAreWarnings(t *testing.T) {
	t.Parallel()

	handler, _ := sampleAPI(t, func(root string) {
		file := filepath.Join(root, "alpha", "project.yaml")
		data, _ := os.ReadFile(file)
		writeFile(t, file, string(data)+"image: image.png\n")
		writeFile(t, filepath.Join(root, "beta", "project.yaml"), "key: BE\nname: Beta\nimage: ../alpha/project.yaml\n")
		writeFile(t, filepath.Join(root, "gamma", "project.yaml"), "key: GA\nname: Gamma\nimage: pic.png\n")
		writeFile(t, filepath.Join(root, "gamma", "pic.png"), "words, not pixels")
	})
	want := map[string]string{
		"alpha": "project image image.png is missing",
		"beta":  `image "../alpha/project.yaml" in project.yaml is not a file name in the project folder; the key is shown`,
		"gamma": "project image pic.png is not a PNG or JPEG image",
	}
	for name, warning := range want {
		project := projectSummary(t, handler, name)
		if project.Image != "" || len(project.Warnings) != 1 || project.Warnings[0] != warning {
			t.Errorf("%s = image %q, warnings %q", name, project.Image, project.Warnings)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/projects/"+name+"/image", nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s image served %d", name, response.Code)
		}
	}
}
