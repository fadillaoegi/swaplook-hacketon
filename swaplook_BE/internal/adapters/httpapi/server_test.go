package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"swaplook/internal/adapters/httpapi"
	"swaplook/internal/adapters/renderer"
	"swaplook/internal/adapters/storage"
	"swaplook/internal/application"
	"swaplook/internal/domain"
	"swaplook/internal/testkit"
	"testing"
	"time"
)

func TestHTTPAuthOriginValidationAndPrivateSession(t *testing.T) {
	ctx := context.Background()
	svc := &application.Service{Repo: testkit.NewRepository(), Renderer: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	token := "synthetic-test-merchant-token-000000"
	s := httpapi.Server{Service: svc, Config: httpapi.Config{Origin: "http://localhost:5173", MerchantToken: token}}
	handler := s.Handler()
	var cookie *http.Cookie
	call := func(method, path, body, auth, origin string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if auth != "" {
			req.Header.Set("Authorization", "Bearer "+auth)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "/api/session", "", "", ""); w.Code != 401 {
		t.Fatal("missing session should be unauthorized", w.Code)
	}
	w := call("POST", "/api/session", "", "", "")
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	cookie = w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || len(cookie.Value) != 64 {
		t.Fatal("session cookie unsafe")
	}
	if w = call("GET", "/api/merchant/summary", "", "wrong", ""); w.Code != 401 {
		t.Fatal("merchant read unprotected")
	}
	if w = call("PATCH", "/api/merchant/stock", `{"product_id":"SL-002","size":"M","quantity":0}`, token, "https://other-site.test"); w.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
	if w = call("PATCH", "/api/merchant/stock", `{"product_id":"SL-002","size":"M","quantity":-1}`, token, ""); w.Code != 400 {
		t.Fatal("negative stock accepted")
	}
	if w = call("POST", "/api/shortlist", `{"original_id":"SL-001","size":"M","budget":6000,"color":"","material":"","unknown":1}`, "", ""); w.Code != 400 {
		t.Fatal("unknown request field accepted")
	}
	if w = call("POST", "/api/shortlist", `{"original_id":"SL-001","size":"M","budget":6000,"color":"","material":""}`, "", ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = call("POST", "/api/previews", `{"product_id":"SL-002","source_id":"demo"}`, "", ""); w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), "task_id") || strings.Contains(w.Body.String(), cookie.Value) {
		t.Fatal("private fields leaked")
	}
	if w = call("PATCH", "/api/merchant/stock", `{"product_id":"SL-002","size":"M","quantity":0}`, token, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = call("POST", "/api/decisions", `{"product_id":"SL-002","kind":"selected","reason":""}`, "", ""); w.Code != 409 {
		t.Fatal("stale choice did not fail")
	}
	if w = call("POST", "/api/decisions", `{"product_id":"SL-003","kind":"selected","reason":""}`, "", ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call("GET", "/api/merchant/summary", "", token, "")
	var summary domain.Summary
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &summary) != nil || summary.Choices != 1 {
		t.Fatal("summary incorrect")
	}
	state, e := svc.Repo.Session(ctx, cookie.Value, false)
	if e != nil || state.Decision == nil {
		t.Fatal("choice not persisted in repository")
	}
	if w = call("GET", "/api/images/"+application.ID(), "", "", ""); w.Code != 404 {
		t.Fatal("unknown image must remain private")
	}
}

func TestJSONMediaTypeAndReconciliationOwnership(t *testing.T) {
	ctx := context.Background()
	svc := &application.Service{Repo: testkit.NewRepository(), Renderer: renderer.Simulator{}, Storage: storage.Local{Root: t.TempDir()}}
	owner, e := svc.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	other, e := svc.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	api := httpapi.Server{Service: svc, Config: httpapi.Config{Origin: "http://127.0.0.1:5173"}}
	handler := api.Handler()
	body := `{"original_id":"SL-001","size":"M","budget":6000,"color":"","material":""}`
	for _, tc := range []struct {
		kind string
		want int
	}{{"application/jsonx", 415}, {"text/plain", 415}, {"application/json; charset=utf-8", 200}} {
		req := httptest.NewRequest("POST", "/api/shortlist", strings.NewReader(body))
		req.Header.Set("Content-Type", tc.kind)
		req.AddCookie(&http.Cookie{Name: "swaplook_session", Value: owner.ID})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("content type %s: got %d", tc.kind, w.Code)
		}
	}
	job, e := svc.Preview(ctx, owner.ID, "SL-002", "demo")
	if e != nil {
		t.Fatal(e)
	}
	job.Status = "timed_out"
	job.Polls = 60
	job.UpdatedAt = time.Now().Add(-time.Minute)
	if e = svc.Repo.SaveJob(ctx, job); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		session string
		want    int
	}{{other.ID, 404}, {owner.ID, 200}} {
		req := httptest.NewRequest("POST", "/api/previews/"+job.ID+"/reconcile", nil)
		req.AddCookie(&http.Cookie{Name: "swaplook_session", Value: tc.session})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("reconciliation status: got %d", w.Code)
		}
		if strings.Contains(w.Body.String(), "task_id") || strings.Contains(w.Body.String(), job.TaskID) {
			t.Fatal("provider task ID exposed")
		}
	}
	persisted, e := svc.Repo.Job(ctx, owner.ID, job.ID, false)
	if e != nil || persisted.Status != "success" || persisted.Reconciliations != 1 {
		t.Fatal("HTTP did not reconcile the owned task", e)
	}
}
