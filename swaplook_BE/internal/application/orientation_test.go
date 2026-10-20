package application

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// exifJPEG inserts an APP1 Exif segment with the given orientation after the SOI marker.
func exifJPEG(t *testing.T, img image.Image, orientation uint16) []byte {
	t.Helper()
	var plain bytes.Buffer
	if e := jpeg.Encode(&plain, img, &jpeg.Options{Quality: 95}); e != nil {
		t.Fatal(e)
	}
	tiff := []byte("MM\x00\x2a\x00\x00\x00\x08\x00\x01")
	entry := make([]byte, 12)
	binary.BigEndian.PutUint16(entry[0:], 0x0112)
	binary.BigEndian.PutUint16(entry[2:], 3)
	binary.BigEndian.PutUint32(entry[4:], 1)
	binary.BigEndian.PutUint16(entry[8:], orientation)
	tiff = append(append(tiff, entry...), 0, 0, 0, 0)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	out := append([]byte{}, plain.Bytes()[:2]...)
	out = append(append(out, app1...), seg...)
	return append(out, plain.Bytes()[2:]...)
}

func TestNormalizeAppliesEXIFOrientationToPortraitPhonePhotos(t *testing.T) {
	// A sideways-stored 300x200 sensor image with a red marker in the top-left corner.
	img := image.NewRGBA(image.Rect(0, 0, 300, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 300; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if x < 40 && y < 40 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	b := exifJPEG(t, img, 6)
	if o := jpegOrientation(b); o != 6 {
		t.Fatal("orientation not read", o)
	}
	out, e := normalize(b, 8<<20, 10<<20)
	if e != nil {
		t.Fatal(e)
	}
	cfg, _, e := image.DecodeConfig(bytes.NewReader(out))
	if e != nil || cfg.Width != 200 || cfg.Height != 300 {
		t.Fatal("portrait photo not rotated upright", cfg.Width, cfg.Height, e)
	}
	decoded, _, _ := image.Decode(bytes.NewReader(out))
	// Orientation 6 rotates 90° clockwise: the top-left marker moves to the top-right.
	if r, g, _, _ := decoded.At(185, 15).RGBA(); r>>8 < 200 || g>>8 > 80 {
		t.Fatal("rotation direction incorrect")
	}
	if jpegOrientation([]byte("not a jpeg")) != 1 || jpegOrientation(exifJPEG(t, img, 1)) != 1 {
		t.Fatal("default orientation incorrect")
	}
}
