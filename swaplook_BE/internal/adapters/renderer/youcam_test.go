package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func TestYouCamContractUploadsOnceAndDoesNotLeakBearerToStorage(t *testing.T) {
	y := NewYouCam("synthetic-test-key", "")
	uploads, tasks, polls := 0, 0, 0
	y.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "yce-api-01.makeupar.com" {
			if r.Header.Get("Authorization") != "Bearer synthetic-test-key" {
				t.Error("missing provider auth")
			}
			switch {
			case r.URL.Path == "/s2s/v2.0/file":
				var in struct {
					Files []struct {
						ContentType string `json:"content_type"`
						Size        int    `json:"file_size"`
					}
				}
				if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
					t.Fatal(e)
				}
				if len(in.Files) != 1 || in.Files[0].ContentType != "image/png" || in.Files[0].Size == 0 {
					t.Fatal("file contract mismatch")
				}
				return response(200, `{"data":{"files":[{"file_id":"private-file","requests":[{"method":"PUT","url":"https://test.s3.amazonaws.com/upload","headers":{"Content-Type":"image/png"}}]}]}}`), nil
			case r.URL.Path == taskPath && r.Method == "POST":
				tasks++
				var in map[string]string
				_ = json.NewDecoder(r.Body).Decode(&in)
				if in["src_file_id"] != "private-file" || in["ref_file_id"] != "private-file" || in["garment_category"] != "upper_body" {
					t.Fatal("task binding contract mismatch")
				}
				return response(200, `{"data":{"task_id":"private-task"}}`), nil
			case strings.HasPrefix(r.URL.Path, taskPath+"/"):
				polls++
				return response(200, `{"data":{"task_status":"success","results":{"url":"https://test.s3.amazonaws.com/result"}}}`), nil
			}
		} else if r.URL.Host == "test.s3.amazonaws.com" {
			if r.Header.Get("Authorization") != "" {
				t.Fatal("bearer leaked to storage")
			}
			if r.Method == "PUT" {
				uploads++
				return response(200, ""), nil
			}
			return response(200, "image bytes fixture"), nil
		}
		t.Fatalf("unexpected provider request: method %s", r.Method)
		return nil, nil
	})
	// Only signature detection is needed at this contract boundary; the application validates decoded images.
	b := append([]byte{137, 80, 78, 71, 13, 10, 26, 10}, bytes.Repeat([]byte{0}, 40)...)
	task, e := y.SubmitImages(context.Background(), b, b)
	if e != nil || task != "private-task" {
		t.Fatal("task creation failed", e)
	}
	result, e := y.Poll(context.Background(), task)
	if e != nil || result.Status != "success" || len(result.Image) == 0 {
		t.Fatal("result contract failed", e)
	}
	if uploads != 2 || tasks != 1 || polls != 1 {
		t.Fatal("unexpected external request counts", uploads, tasks, polls)
	}
}
func TestProviderErrorsRemainVisibleAndUnsafeResultsRejected(t *testing.T) {
	y := NewYouCam("synthetic-test-key", "")
	y.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(401, `{"error":"synthetic auth error"}`), nil
	})
	if _, e := y.Poll(context.Background(), "task"); e == nil {
		t.Fatal("provider auth failure hidden")
	}
	y.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(200, `{"data":{"task_status":"success","results":{"url":"http://127.0.0.1/private"}}}`), nil
	})
	if _, e := y.Poll(context.Background(), "task"); e == nil {
		t.Fatal("unsafe result destination allowed")
	}
	for _, raw := range []string{"https://makeupar.com.evil.test/a", "https://x.amazonaws.com:8443/a", "http://x.amazonaws.com/a", "https://user:password@x.amazonaws.com/a"} {
		if providerURL(raw) {
			t.Fatal("unsafe provider URL allowed")
		}
	}
}
func TestYouCamPollKeepsOnlySafeProviderErrorCode(t *testing.T) {
	y := NewYouCam("synthetic-test-key", "")
	body := `{"data":{"task_status":"error","error":"error_pose","error_message":"free text https://example.invalid"}}`
	y.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
	result, e := y.Poll(context.Background(), "private-task")
	if e != nil || result.Status != "error" || result.Reason != "error_pose" {
		t.Fatal("provider error code not preserved", result, e)
	}
	body = `{"data":{"task_status":"error","error":"Bad Code https://example.invalid"}}`
	if result, _ = y.Poll(context.Background(), "private-task"); result.Reason != "" {
		t.Fatal("unsafe provider error text accepted")
	}
}
func TestYouCamTextToImageContract(t *testing.T) {
	y := NewYouCam("synthetic-test-key", "")
	y.client.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == imagePath && r.Method == "POST":
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in["model"] != "youcam-image-v2" || in["size"] != "1104*1472" || in["prompt_extend"] != false || in["negative_prompt"] != "no text" {
				t.Fatal("text-to-image request contract mismatch", in)
			}
			return response(200, `{"data":{"task_id":"image-task"}}`), nil
		case r.URL.Path == imagePath+"/image-task":
			return response(200, `{"data":{"task_status":"success","results":{"url":"https://test.s3.amazonaws.com/model"}}}`), nil
		case r.URL.Host == "test.s3.amazonaws.com":
			return response(200, "image bytes"), nil
		}
		t.Fatal("unexpected request", r.URL.Path)
		return nil, nil
	})
	id, e := y.GenerateImage(context.Background(), "A fictional person", "no text", "1104*1472")
	if e != nil || id != "image-task" {
		t.Fatal(e)
	}
	if r, e := y.PollImage(context.Background(), id); e != nil || r.Status != "success" || string(r.Image) != "image bytes" {
		t.Fatal("image result not downloaded", e)
	}
	if _, e := y.GenerateImage(context.Background(), "", "", "1104*1472"); e == nil {
		t.Fatal("empty prompt accepted")
	}
}
