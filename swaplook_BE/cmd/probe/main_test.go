package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeImageValidationBeforeProviderUpload(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name          string
		width, height int
		valid         bool
	}{{"landscape", 1024, 768, true}, {"portrait", 768, 1024, true}, {"small", 128, 128, false}, {"too-wide", 4097, 384, false}} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, tc.width, tc.height)))
			p := filepath.Join(root, tc.name+".png")
			if e := os.WriteFile(p, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			_, e := readImage(p)
			if (e == nil) != tc.valid {
				t.Fatal("unexpected image validation outcome", e)
			}
		})
	}
	p := filepath.Join(root, "invalid.png")
	_ = os.WriteFile(p, []byte("not an image"), 0600)
	if _, e := readImage(p); e == nil {
		t.Fatal("invalid image accepted")
	}
	if _, e := readImage(filepath.Join(root, "missing.png")); e == nil {
		t.Fatal("missing image accepted")
	}
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1024, 768)))
	truncated := filepath.Join(root, "truncated.png")
	_ = os.WriteFile(truncated, b.Bytes()[:33], 0600)
	if _, e := readImage(truncated); e == nil {
		t.Fatal("header-only image accepted before paid provider upload")
	}
}
