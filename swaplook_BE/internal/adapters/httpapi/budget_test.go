package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"swaplook/internal/adapters/httpapi"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"swaplook/internal/testkit"
	"testing"
)

type actualRenderer struct{ submits int }

func (a *actualRenderer) Name() string { return "youcam-v4" }
func (a *actualRenderer) Submit(context.Context, application.RenderInput) (string, error) {
	a.submits++
	return "private-task", nil
}
func (a *actualRenderer) Poll(context.Context, string) (application.RenderResult, error) {
	return application.RenderResult{Status: "pending"}, nil
}

func TestHTTPQuotaIsPerClientAndIgnoresUntrustedForwardedFor(t *testing.T) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 160, 200)))
	model, _ := application.NormalizeModel(b.Bytes())
	r := &actualRenderer{}
	svc := &application.Service{Repo: testkit.NewRepository(), Renderer: r, Storage: storage.Local{Root: t.TempDir()}, Budget: application.Budget{PerClient: 1}, Models: []application.Model{{ID: "ari", Name: "Ari", Note: "Consented test model", Image: model}}}
	handler := (&httpapi.Server{Service: svc, Config: httpapi.Config{Origin: "http://localhost:5173", MerchantToken: "synthetic-test-merchant-token-000000", ClientSalt: []byte("test-salt")}}).Handler()
	call := func(method, path, body string, cookie *http.Cookie, forwarded string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = "203.0.113.7:4242"
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if forwarded != "" {
			req.Header.Set("X-Forwarded-For", forwarded)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	preview := func(forwarded string) *httptest.ResponseRecorder {
		cookie := call("POST", "/api/session", "", nil, "").Result().Cookies()[0]
		if w := call("POST", "/api/shortlist", `{"original_id":"SL-001","size":"M","budget":6000,"color":"","material":""}`, cookie, ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		return call("POST", "/api/previews", `{"product_id":"SL-002","source_id":"model:ari"}`, cookie, forwarded)
	}
	if w := preview(""); w.Code != 202 {
		t.Fatal("first demo-model preview rejected", w.Code, w.Body.String())
	}
	w := preview("198.51.100.9")
	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 429 || body["code"] != "quota" {
		t.Fatal("forwarded header bypassed the per-client allowance", w.Code, w.Body.String())
	}
	if r.submits != 1 {
		t.Fatal("quota rejection reached the provider")
	}
	if w = call("GET", "/api/models/ari/image", "", nil, ""); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("model image not served", w.Code)
	}
	if w = call("GET", "/api/models/..%2fsecret/image", "", nil, ""); w.Code != 404 {
		t.Fatal("unknown model path served", w.Code)
	}
	cookie := call("POST", "/api/session", "", nil, "").Result().Cookies()[0]
	var state struct {
		Quota  map[string]int `json:"quota"`
		Models []struct {
			ID, Name string
		} `json:"models"`
	}
	_ = json.Unmarshal(call("GET", "/api/session", "", cookie, "").Body.Bytes(), &state)
	if len(state.Models) != 1 || state.Models[0].ID != "ari" || state.Quota["client_remaining"] != 0 {
		t.Fatal("session state lacks quota or demo model flag", state)
	}
}

func TestHTTPMerchantAISwitch(t *testing.T) {
	token := "synthetic-test-merchant-token-000000"
	svc := &application.Service{Repo: testkit.NewRepository(), Renderer: &actualRenderer{}, Simulator: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	handler := (&httpapi.Server{Service: svc, Config: httpapi.Config{Origin: "http://localhost:5173", MerchantToken: token}}).Handler()
	call := func(method, path, body, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if w := call("PATCH", "/api/merchant/ai", `{"enabled":false}`, "wrong"); w.Code != 401 || !svc.AIEnabled() {
		t.Fatal("AI switch not protected", w.Code)
	}
	if w := call("PATCH", "/api/merchant/ai", `{}`, token); w.Code != 400 {
		t.Fatal("missing enabled flag accepted", w.Code)
	}
	if w := call("PATCH", "/api/merchant/ai", `{"enabled":false}`, token); w.Code != 200 || svc.AIEnabled() {
		t.Fatal("AI switch failed", w.Code, w.Body.String())
	}
	var health map[string]any
	_ = json.Unmarshal(call("GET", "/api/health", "", "").Body.Bytes(), &health)
	if health["renderer"] != "simulator" || health["ai_available"] != true {
		t.Fatal("health does not reflect the switch", health)
	}
}

func TestHTTPPublicDemoDashboardIsReadOnly(t *testing.T) {
	token := "synthetic-test-merchant-token-000000"
	svc := &application.Service{Repo: testkit.NewRepository(), Renderer: &actualRenderer{}, Simulator: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	handler := (&httpapi.Server{Service: svc, Config: httpapi.Config{Origin: "http://localhost:5173", MerchantToken: token}}).Handler()
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	w := call("GET", "/api/merchant/demo-summary", "")
	var sum struct {
		Sessions int `json:"sessions"`
		Previews struct {
			Available bool `json:"available"`
		} `json:"previews"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &sum) != nil || !sum.Previews.Available {
		t.Fatal("public demo dashboard unavailable", w.Code, w.Body.String())
	}
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/merchant/summary", ""},
		{"PATCH", "/api/merchant/stock", `{"product_id":"SL-002","size":"M","quantity":0}`},
		{"PATCH", "/api/merchant/ai", `{"enabled":false}`},
		{"POST", "/api/merchant/demo-summary", ""},
	} {
		if w := call(c.method, c.path, c.body); w.Code != 401 && w.Code != 404 && w.Code != 405 {
			t.Fatal("write or owner view open without a token", c.method, c.path, w.Code)
		}
	}
	if !svc.AIEnabled() {
		t.Fatal("anonymous request changed the AI switch")
	}
}
