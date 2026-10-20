package renderer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"swaplook/internal/application"
	"time"
)

const taskPath = "/s2s/v2.0/task/cloth-v4"

type YouCam struct {
	key        string
	base       string
	garmentDir string
	client     *http.Client
}

func NewYouCam(key, dir string) *YouCam {
	return &YouCam{key: key, base: "https://yce-api-01.makeupar.com", garmentDir: dir, client: &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (y *YouCam) Name() string { return "youcam-v4" }
func (y *YouCam) json(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, y.base+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+y.key)
	req.Header.Set("Content-Type", "application/json")
	res, e := y.client.Do(req)
	if e != nil {
		return errors.New("provider network request not confirmed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("provider HTTP %d", res.StatusCode)
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out); e != nil {
		return errors.New("provider response invalid")
	}
	return nil
}
func providerURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.HasSuffix(host, ".amazonaws.com") || strings.HasSuffix(host, ".makeupar.com") || strings.HasSuffix(host, ".perfectcorp.com")
}
func (y *YouCam) upload(ctx context.Context, b []byte) (string, error) {
	if len(b) == 0 || len(b) >= 10<<20 {
		return "", errors.New("provider image size invalid")
	}
	kind := http.DetectContentType(b)
	if kind != "image/png" && kind != "image/jpeg" {
		return "", errors.New("provider image type invalid")
	}
	in := map[string]any{"files": []any{map[string]any{"content_type": kind, "file_name": "image" + map[string]string{"image/png": ".png", "image/jpeg": ".jpg"}[kind], "file_size": len(b)}}}
	var out struct {
		Data struct {
			Files []struct {
				ID       string `json:"file_id"`
				Requests []struct {
					Method  string            `json:"method"`
					URL     string            `json:"url"`
					Headers map[string]string `json:"headers"`
				} `json:"requests"`
			} `json:"files"`
		} `json:"data"`
	}
	if e := y.json(ctx, "POST", "/s2s/v2.0/file", in, &out); e != nil {
		return "", e
	}
	if len(out.Data.Files) != 1 || out.Data.Files[0].ID == "" || len(out.Data.Files[0].Requests) != 1 {
		return "", errors.New("provider upload contract invalid")
	}
	f := out.Data.Files[0]
	upload := f.Requests[0]
	if upload.Method != "PUT" || !providerURL(upload.URL) {
		return "", errors.New("provider upload destination rejected")
	}
	req, e := http.NewRequestWithContext(ctx, "PUT", upload.URL, bytes.NewReader(b))
	if e != nil {
		return "", errors.New("provider upload request invalid")
	}
	for k, v := range upload.Headers {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		if strings.EqualFold(k, "Authorization") {
			return "", errors.New("unexpected provider upload auth header")
		}
		req.Header.Set(k, v)
	}
	res, e := y.client.Do(req)
	if e != nil {
		return "", errors.New("provider upload failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("provider upload HTTP %d", res.StatusCode)
	}
	return f.ID, nil
}

var skuPattern = regexp.MustCompile(`^SL-[0-9]{3}$`)
var reasonPattern = regexp.MustCompile(`^[a-z0-9_]{1,48}$`)

func (y *YouCam) Submit(ctx context.Context, in application.RenderInput) (string, error) {
	if !skuPattern.MatchString(in.SKU) {
		return "", errors.New("invalid SKU")
	}
	var garment []byte
	var e error
	for _, ext := range []string{".png", ".jpg", ".jpeg"} {
		garment, e = os.ReadFile(filepath.Join(y.garmentDir, in.SKU+ext))
		if e == nil {
			break
		}
	}
	if e != nil {
		return "", errors.New("licensed garment file missing")
	}
	return y.SubmitImages(ctx, in.Source, garment)
}

// SubmitImages is also used by the explicitly invoked, single-generation probe.
func (y *YouCam) SubmitImages(ctx context.Context, person, garment []byte) (string, error) {
	src, e := y.upload(ctx, person)
	if e != nil {
		return "", e
	}
	ref, e := y.upload(ctx, garment)
	if e != nil {
		return "", e
	}
	var out struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	e = y.json(ctx, "POST", taskPath, map[string]string{"src_file_id": src, "ref_file_id": ref, "garment_category": "upper_body"}, &out)
	if e == nil && out.Data.TaskID == "" {
		e = errors.New("provider task ID missing")
	}
	return out.Data.TaskID, e
}
func (y *YouCam) Poll(ctx context.Context, id string) (application.RenderResult, error) {
	return y.poll(ctx, taskPath, id)
}

// poll checks one task under path and downloads a successful single-URL result.
func (y *YouCam) poll(ctx context.Context, path, id string) (application.RenderResult, error) {
	result := application.RenderResult{Status: "pending"}
	if id == "" {
		return result, errors.New("missing provider task ID")
	}
	var out struct {
		Data struct {
			Status  string `json:"task_status"`
			Error   string `json:"error"`
			Results struct {
				URL string `json:"url"`
			} `json:"results"`
		} `json:"data"`
	}
	if e := y.json(ctx, "GET", path+"/"+url.PathEscape(id), nil, &out); e != nil {
		return result, e
	}
	switch out.Data.Status {
	case "error":
		result.Status = "error"
		if reasonPattern.MatchString(out.Data.Error) {
			result.Reason = out.Data.Error
		}
		return result, nil
	case "success":
	case "running", "pending", "queued", "processing":
		return result, nil
	default:
		return result, errors.New("unrecognized provider task status")
	}
	if !providerURL(out.Data.Results.URL) {
		return result, errors.New("provider result destination rejected")
	}
	req, e := http.NewRequestWithContext(ctx, "GET", out.Data.Results.URL, nil)
	if e != nil {
		return result, errors.New("provider result request invalid")
	}
	res, e := y.client.Do(req)
	if e != nil {
		return result, errors.New("provider result download failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return result, fmt.Errorf("provider download HTTP %d", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (10<<20)+1))
	if e != nil || len(b) > 10<<20 {
		return result, errors.New("provider result size invalid")
	}
	result.Status = "success"
	result.Image = b
	return result, nil
}
