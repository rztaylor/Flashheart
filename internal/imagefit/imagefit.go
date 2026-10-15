package imagefit

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder
	"image/png"

	"golang.org/x/image/draw"
)

// Side is the width and height of a fitted image, in pixels.
const Side = 128

// MaxPixels bounds the picture decoded: a larger one is refused from its
// header, before its pixels are read (SEC-7).
const MaxPixels = 50_000_000

// Refusal says in words why a picture was not taken, for the person who
// sent it ("it is not a JPEG or PNG image"); TooLarge marks one refused
// for its size rather than its kind.
type Refusal struct {
	Words    string
	TooLarge bool
}

func (r *Refusal) Error() string { return r.Words }

// Fit returns the Side×Side PNG of data's centre square. A picture it will
// not take, not a JPEG or PNG by its bytes, larger than MaxPixels, or one
// that cannot be decoded, is a *Refusal; any other error is this package's
// own failure.
func Fit(data []byte) ([]byte, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, &Refusal{Words: "it is not a JPEG or PNG image"}
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, &Refusal{Words: "it could not be read as an image"}
	}
	if int64(config.Width)*int64(config.Height) > MaxPixels {
		return nil, &Refusal{
			Words:    fmt.Sprintf("its picture is %d×%d pixels, more than the %d megapixel limit", config.Width, config.Height, MaxPixels/1_000_000),
			TooLarge: true,
		}
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, &Refusal{Words: "it could not be read as an image"}
	}
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	square := image.Rect(0, 0, side, side).Add(image.Point{
		X: bounds.Min.X + (bounds.Dx()-side)/2,
		Y: bounds.Min.Y + (bounds.Dy()-side)/2,
	})
	scaled := image.NewRGBA(image.Rect(0, 0, Side, Side))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, square, draw.Src, nil)
	if format == "jpeg" {
		scaled = orient(scaled, exifOrientation(data))
	}
	var out bytes.Buffer
	if err := png.Encode(&out, scaled); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// orient applies an EXIF orientation (1–8) to a square image: the centre
// square of a picture is the centre square of its rotation, so it is
// applied after the crop and scale, to Side×Side pixels.
func orient(src *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	last := Side - 1
	from := [...]func(x, y int) (int, int){
		2: func(x, y int) (int, int) { return last - x, y },        // mirrored
		3: func(x, y int) (int, int) { return last - x, last - y }, // rotated 180°
		4: func(x, y int) (int, int) { return x, last - y },        // flipped
		5: func(x, y int) (int, int) { return y, x },               // transposed
		6: func(x, y int) (int, int) { return y, last - x },        // rotate 90° clockwise to view
		7: func(x, y int) (int, int) { return last - y, last - x }, // transversed
		8: func(x, y int) (int, int) { return last - y, x },        // rotate 90° anticlockwise to view
	}[orientation]
	dst := image.NewRGBA(src.Bounds())
	for y := range Side {
		for x := range Side {
			sx, sy := from(x, y)
			dst.SetRGBA(x, y, src.RGBAAt(sx, sy))
		}
	}
	return dst
}

// exifOrientation reads the Orientation tag (0x0112) of a JPEG's EXIF
// segment, or 1 when there is none or it cannot be read. It walks the
// file's segments up to the start of the picture data, so nothing beyond
// the headers is touched.
func exifOrientation(data []byte) int {
	offset := 2 // after SOI
	for offset+4 <= len(data) && data[offset] == 0xFF {
		marker := data[offset+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			offset += 2 // markers with no payload
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan, end of image
			break
		}
		length := int(binary.BigEndian.Uint16(data[offset+2:]))
		if length < 2 || offset+2+length > len(data) {
			break
		}
		payload := data[offset+4 : offset+2+length]
		if marker == 0xE1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			return tiffOrientation(payload[6:])
		}
		offset += 2 + length
	}
	return 1
}

// tiffOrientation finds the Orientation entry in a TIFF's first IFD.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(tiff[4:8]))
	if ifd < 0 || ifd+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[ifd:]))
	for index := range count {
		entry := ifd + 2 + index*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:]) == 0x0112 && order.Uint16(tiff[entry+2:]) == 3 {
			return int(order.Uint16(tiff[entry+8:]))
		}
	}
	return 1
}
