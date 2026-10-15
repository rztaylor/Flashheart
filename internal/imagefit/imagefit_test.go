package imagefit

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

var (
	red  = color.RGBA{R: 220, A: 255}
	blue = color.RGBA{B: 220, A: 255}
)

// halves draws a width×height picture whose first half (left, or top when
// vertical) is red and the rest blue.
func halves(width, height int, vertical bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(blue), image.Point{}, draw.Src)
	first := image.Rect(0, 0, width/2, height)
	if vertical {
		first = image.Rect(0, 0, width, height/2)
	}
	draw.Draw(img, first, image.NewUniform(red), image.Point{}, draw.Src)
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the result is not a PNG: %v", err)
	}
	return img
}

// reddish reports a pixel nearer red than blue (JPEG blurs the edges).
func reddish(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return r > b
}

// Every accepted picture, whatever its size, shape and format, comes out
// as a Side×Side PNG of its centre square.
func TestFitCentreCropsAndResizesToASquarePNG(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		data     []byte
		vertical bool
	}{
		"large landscape PNG":  {encodePNG(t, halves(1200, 800, false)), false},
		"small landscape PNG":  {encodePNG(t, halves(40, 20, false)), false},
		"portrait PNG":         {encodePNG(t, halves(300, 900, true)), true},
		"square PNG":           {encodePNG(t, halves(64, 64, false)), false},
		"large landscape JPEG": {encodeJPEG(t, halves(1600, 900, false)), false},
		"small portrait JPEG":  {encodeJPEG(t, halves(30, 60, true)), true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := Fit(c.data)
			if err != nil {
				t.Fatalf("Fit: %v", err)
			}
			img := decodePNG(t, out)
			if img.Bounds().Dx() != Side || img.Bounds().Dy() != Side {
				t.Fatalf("size = %v, want %d×%d", img.Bounds(), Side, Side)
			}
			// The centre crop keeps the halves: first half red, the rest blue.
			first, second := img.At(20, 64), img.At(108, 64)
			if c.vertical {
				first, second = img.At(64, 20), img.At(64, 108)
			}
			if !reddish(first) || reddish(second) {
				t.Errorf("first half %v, second half %v", first, second)
			}
		})
	}
}

// The centre crop is taken from the middle of the longer side: a wide
// picture with a red band down its exact centre keeps the band in the
// middle, and what lies outside the central square is gone.
func TestFitTakesTheMiddleOfTheLongerSide(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 900, 300))
	draw.Draw(img, img.Bounds(), image.NewUniform(blue), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(440, 0, 460, 300), image.NewUniform(red), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 0, 100, 300), image.NewUniform(color.RGBA{G: 220, A: 255}), image.Point{}, draw.Src)
	out, err := Fit(encodePNG(t, img))
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	result := decodePNG(t, out)
	if !reddish(result.At(64, 64)) {
		t.Errorf("centre = %v, want red", result.At(64, 64))
	}
	for _, x := range []int{0, 2, 125, 127} {
		if _, g, _, _ := result.At(x, 64).RGBA(); g > 0x4000 {
			t.Errorf("column %d = %v; the green margin lies outside the centre square", x, result.At(x, 64))
		}
	}
}

// A JPEG's EXIF orientation is honoured: a phone photo stored sideways
// with orientation 6 (rotate 90° clockwise to view) comes out upright.
func TestFitHonoursJPEGOrientation(t *testing.T) {
	t.Parallel()

	// Stored: left red, right blue. Viewed (rotated 90° clockwise): top
	// red, bottom blue.
	stored := encodeJPEG(t, halves(400, 200, false))
	out, err := Fit(withOrientation(stored, 6))
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	img := decodePNG(t, out)
	if !reddish(img.At(64, 20)) || reddish(img.At(64, 108)) {
		t.Errorf("top %v, bottom %v: orientation 6 was not applied", img.At(64, 20), img.At(64, 108))
	}

	// Orientation 3 (rotated 180°): left blue, right red.
	out, _ = Fit(withOrientation(stored, 3))
	img = decodePNG(t, out)
	if reddish(img.At(20, 64)) || !reddish(img.At(108, 64)) {
		t.Errorf("orientation 3: left %v, right %v", img.At(20, 64), img.At(108, 64))
	}

	// Orientation 8 (rotate 90° anticlockwise to view): top blue, bottom red.
	out, _ = Fit(withOrientation(stored, 8))
	img = decodePNG(t, out)
	if reddish(img.At(64, 20)) || !reddish(img.At(64, 108)) {
		t.Errorf("orientation 8: top %v, bottom %v", img.At(64, 20), img.At(64, 108))
	}

	// Orientation 1, or none, leaves it as stored.
	out, _ = Fit(withOrientation(stored, 1))
	img = decodePNG(t, out)
	if !reddish(img.At(20, 64)) || reddish(img.At(108, 64)) {
		t.Errorf("orientation 1: left %v, right %v", img.At(20, 64), img.At(108, 64))
	}
}

// withOrientation inserts an EXIF APP1 segment holding only the
// orientation tag after the JPEG's SOI marker.
func withOrientation(jpg []byte, orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("II*\x00")                              // little-endian TIFF header
	binary.Write(&tiff, binary.LittleEndian, uint32(8))      // first IFD at offset 8
	binary.Write(&tiff, binary.LittleEndian, uint16(1))      // one entry
	binary.Write(&tiff, binary.LittleEndian, uint16(0x0112)) // Orientation
	binary.Write(&tiff, binary.LittleEndian, uint16(3))      // SHORT
	binary.Write(&tiff, binary.LittleEndian, uint32(1))      // count
	binary.Write(&tiff, binary.LittleEndian, orientation)    // value
	binary.Write(&tiff, binary.LittleEndian, uint16(0))      // padding to 4 bytes
	binary.Write(&tiff, binary.LittleEndian, uint32(0))      // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	segment = append(segment, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, segment...)
	return append(out, jpg[2:]...)
}

// Anything that is not a JPEG or PNG picture is refused in words before
// any pixel is decoded, and so is a picture too large to decode safely.
func TestFitRefusesWhatItCannotTake(t *testing.T) {
	t.Parallel()

	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;")
	cases := map[string]struct {
		data     []byte
		want     string
		tooLarge bool
	}{
		"empty":                   {nil, "it is not a JPEG or PNG image", false},
		"text":                    {[]byte("hello, not a picture"), "it is not a JPEG or PNG image", false},
		"an SVG":                  {[]byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), "it is not a JPEG or PNG image", false},
		"a GIF":                   {gif, "it is not a JPEG or PNG image", false},
		"a truncated JPEG":        {truncated(t, encodeJPEG(t, halves(200, 200, false))), "it could not be read as an image", false},
		"a PNG claiming 100,000²": {hugePNG(100_000, 100_000), "its picture is 100000×100000 pixels, more than the 50 megapixel limit", true},
		"a JPEG claiming 65,535²": {hugeJPEG(t, 65535, 65535), "its picture is 65535×65535 pixels, more than the 50 megapixel limit", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := Fit(c.data)
			var refusal *Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Fit = %d bytes, %v; want a refusal", len(out), err)
			}
			if out != nil || refusal.Words != c.want || refusal.TooLarge != c.tooLarge {
				t.Errorf("Fit = %d bytes, %+v; want nothing, %q (too large %v)", len(out), refusal, c.want, c.tooLarge)
			}
		})
	}
}

// truncated cuts a JPEG a few bytes into its picture data: its headers
// read, its pixels do not.
func truncated(t *testing.T, jpg []byte) []byte {
	t.Helper()
	sos := bytes.Index(jpg, []byte{0xFF, 0xDA})
	if sos < 0 {
		t.Fatal("no SOS marker")
	}
	return jpg[:sos+20]
}

// hugePNG is a PNG signature and IHDR claiming width×height, with no
// picture data: enough for DecodeConfig, nothing to decode.
func hugePNG(width, height uint32) []byte {
	var ihdr bytes.Buffer
	binary.Write(&ihdr, binary.BigEndian, width)
	binary.Write(&ihdr, binary.BigEndian, height)
	ihdr.Write([]byte{8, 2, 0, 0, 0}) // 8-bit RGB
	chunk := append([]byte("IHDR"), ihdr.Bytes()...)
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()))
	out.Write(chunk)
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return out.Bytes()
}

// hugeJPEG is a real small JPEG whose frame header claims width×height.
func hugeJPEG(t *testing.T, width, height uint16) []byte {
	t.Helper()
	data := encodeJPEG(t, halves(8, 8, false))
	sof := bytes.Index(data, []byte{0xFF, 0xC0})
	if sof < 0 {
		t.Fatal("no SOF0 marker")
	}
	binary.BigEndian.PutUint16(data[sof+5:], height)
	binary.BigEndian.PutUint16(data[sof+7:], width)
	return data
}

// The result drops what the source carried besides pixels: a JPEG with an
// EXIF segment gives a plain PNG with no EXIF, and transparency in a PNG
// is kept.
func TestFitWritesAPlainPNG(t *testing.T) {
	t.Parallel()

	out, _ := Fit(withOrientation(encodeJPEG(t, halves(64, 64, false)), 1))
	if bytes.Contains(out, []byte("Exif")) || !strings.HasPrefix(string(out), "\x89PNG\r\n\x1a\n") {
		t.Errorf("the result carries EXIF or is not a PNG")
	}
	clear := image.NewNRGBA(image.Rect(0, 0, 50, 50))
	out, _ = Fit(encodePNG(t, clear))
	if _, _, _, a := decodePNG(t, out).At(10, 10).RGBA(); a != 0 {
		t.Errorf("alpha = %d, want transparent", a)
	}
}
