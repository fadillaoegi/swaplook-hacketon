package httpapi_test

import (
	"net/http/httptest"
	"strings"
	"swaplook/internal/adapters/httpapi"
	"testing"
	"testing/fstest"
)

func TestBuiltFrontendServesShopperMerchantAndAssetsWithoutExposingOtherFiles(t *testing.T) {
	api := httpapi.Server{Config: httpapi.Config{Assets: fstest.MapFS{
		"index.html":                  {Data: []byte("<!doctype html><title>SwapLook</title>")},
		"assets/app-HASH.js":          {Data: []byte("console.log('fixture')")},
		"catalog/SL-001.svg":          {Data: []byte("<svg></svg>")},
		"brand/swaplook-logo.svg":     {Data: []byte("<svg>brand</svg>")},
		"brand/swaplook-icon-192.png": {Data: []byte("PNG_ICON_FIXTURE")},
		"apple-touch-icon.png":        {Data: []byte("PNG_TOUCH_FIXTURE")},
		"favicon.svg":                 {Data: []byte("<svg>icon</svg>")},
		"brand/.private.png":          {Data: []byte("PRIVATE_BRAND_FIXTURE")},
		"brand/source.py":             {Data: []byte("PRIVATE_SOURCE_FIXTURE")},
		".env":                        {Data: []byte("PRIVATE_FILE_FIXTURE")},
		"catalog/.private.png":        {Data: []byte("PRIVATE_IMAGE_FIXTURE")},
	}}}
	handler := api.Handler()
	for _, tc := range []struct {
		path        string
		status      int
		body, cache string
	}{
		{"/", 200, "<title>SwapLook</title>", "no-cache"},
		{"/merchant", 200, "<title>SwapLook</title>", "no-cache"},
		{"/credits", 200, "<title>SwapLook</title>", "no-cache"},
		{"/assets/app-HASH.js", 200, "fixture", "public, max-age=31536000, immutable"},
		{"/catalog/SL-001.svg", 200, "<svg>", "no-cache"},
		{"/brand/swaplook-logo.svg", 200, "brand", "no-cache"},
		{"/brand/swaplook-icon-192.png", 200, "PNG_ICON_FIXTURE", "no-cache"},
		{"/apple-touch-icon.png", 200, "PNG_TOUCH_FIXTURE", "no-cache"},
		{"/favicon.svg", 200, "icon", "no-cache"},
		{"/brand/.private.png", 404, "not found", "no-store"},
		{"/brand/source.py", 404, "not found", "no-store"},
		{"/.env", 404, "not found", "no-store"},
		{"/catalog/.private.png", 404, "not found", "no-store"},
		{"/catalog", 404, "not found", "no-store"},
		{"/assets/app.js.map", 404, "not found", "no-store"},
		{"/api/missing", 404, "API route not found", "no-store"},
		{"/unknown-page", 404, "not found", "no-store"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) || w.Header().Get("Cache-Control") != tc.cache {
				t.Fatalf("unexpected status/body/cache: %d %s %s", w.Code, w.Body.String(), w.Header().Get("Cache-Control"))
			}
			if strings.Contains(w.Body.String(), "PRIVATE_") {
				t.Fatal("private file exposed")
			}
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
				t.Fatal("missing content policy")
			}
		})
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("HEAD", "/merchant", nil))
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD did not match document routing")
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("POST", "/merchant", nil))
	if w.Code != 405 {
		t.Fatal("write method served the frontend")
	}
}
