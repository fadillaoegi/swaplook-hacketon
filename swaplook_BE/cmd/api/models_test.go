package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelsValidatesManifestAndFiles(t *testing.T) {
	dir := t.TempDir()
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 160, 200)))
	_ = os.WriteFile(filepath.Join(dir, "ari.png"), b.Bytes(), 0o644)
	write := func(manifest string) { _ = os.WriteFile(filepath.Join(dir, "models.json"), []byte(manifest), 0o644) }
	write(`[{"id":"ari","name":"Ari","note":"Consented","file":"ari.png"}]`)
	if m, e := loadModels(dir); e != nil || len(m) != 1 || m[0].ID != "ari" || len(m[0].Image) == 0 {
		t.Fatal("valid manifest rejected", e)
	}
	for _, bad := range []string{
		`[{"id":"../x","name":"X","file":"ari.png"}]`,
		`[{"id":"x","name":"X","file":"../ari.png"}]`,
		`[{"id":"x","name":"X","file":".hidden"}]`,
		`[{"id":"x","name":"","file":"ari.png"}]`,
		`[{"id":"x","name":"X","file":"missing.png"}]`,
		`[{"id":"x","name":"X","file":"ari.png"},{"id":"x","name":"Y","file":"ari.png"}]`,
		`{"id":"x"}`,
	} {
		write(bad)
		if _, e := loadModels(dir); e == nil {
			t.Fatal("invalid manifest accepted", bad)
		}
	}
}
