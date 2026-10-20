package application

import (
	"bytes"
	"image"
	"image/jpeg"
	"strings"
	"testing"
)

func TestProviderFailureExplainsKnownCodesWithoutResubmitting(t *testing.T) {
	if m := providerFailure("error_pose"); !strings.Contains(m, "chest and shoulders") || !strings.Contains(m, "no new generation") {
		t.Fatal("pose failure not explained", m)
	}
	if m := providerFailure(""); m != "The preview provider could not process these images." {
		t.Fatal("generic failure changed", m)
	}
	if m := providerFailure("error_other"); !strings.Contains(m, "error_other") {
		t.Fatal("unknown code hidden", m)
	}
}

func TestNormalizeStoresEightBitPNGForJPEGUploads(t *testing.T) {
	var src bytes.Buffer
	if e := jpeg.Encode(&src, image.NewYCbCr(image.Rect(0, 0, 160, 200), image.YCbCrSubsampleRatio420), nil); e != nil {
		t.Fatal(e)
	}
	out, e := normalize(src.Bytes(), 8<<20, 10<<20)
	if e != nil {
		t.Fatal(e)
	}
	// IHDR bit depth is byte 24 of a PNG file.
	if len(out) < 26 || out[24] != 8 {
		t.Fatal("normalized upload is not an 8-bit PNG")
	}
}
