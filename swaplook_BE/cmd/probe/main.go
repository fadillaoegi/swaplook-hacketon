// The probe performs one paid task or resumes an existing task. Never auto-runs.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"swaplook/internal/adapters/renderer"
	"time"
)

type receipt struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
	Started   time.Time `json:"started_at"`
	LatencyMS int64     `json:"latency_ms"`
	Units     string    `json:"units"`
}

func main() {
	person := flag.String("person", "", "authorized person JPEG/PNG path")
	garment := flag.String("garment", "", "licensed top JPEG/PNG path")
	consent := flag.Bool("consent", false, "confirm rights and provider processing consent")
	dir := flag.String("output", "../.data/probe", "private output directory")
	resume := flag.Bool("resume", false, "poll existing private receipt without submitting a new task")
	check := flag.Bool("check", false, "validate key presence and image files locally; make no provider requests")
	flag.Parse()
	if (!*consent && !*check) || os.Getenv("YOUCAM_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Probe requires consent and server-side YOUCAM_API_KEY.")
		os.Exit(1)
	}
	if *check {
		for _, in := range []struct{ label, path string }{{"Person", *person}, {"Garment", *garment}} {
			if _, e := readImage(in.path); e != nil {
				fmt.Fprintln(os.Stderr, in.label+" image: "+e.Error())
				os.Exit(1)
			}
		}
		fmt.Println("Local checks passed: server key is present and both image files meet the probe format/size limits.")
		fmt.Println("No upload, generation or credit check was performed. Account access, image rights and visible pose still require verification.")
		return
	}
	if e := os.MkdirAll(*dir, 0700); e != nil {
		panic("cannot create private output directory")
	}
	path := filepath.Join(*dir, "receipt.json")
	r := receipt{Model: "cloth-v4/upper_body", Started: time.Now().UTC(), Units: "not provided by this response"}
	provider := renderer.NewYouCam(os.Getenv("YOUCAM_API_KEY"), "")
	ctx := context.Background()
	save := func() {
		b, _ := json.MarshalIndent(r, "", "  ")
		if e := os.WriteFile(path, b, 0600); e != nil {
			panic("cannot save private receipt")
		}
	}
	if *resume {
		b, e := os.ReadFile(path)
		if e != nil || json.Unmarshal(b, &r) != nil || r.TaskID == "" {
			panic("no reconcilable task in private receipt")
		}
	} else {
		if _, e := os.Stat(path); e == nil {
			fmt.Fprintln(os.Stderr, "Receipt already exists. Resume it or choose a new private output directory.")
			os.Exit(1)
		}
		a, e := readImage(*person)
		if e != nil {
			fmt.Fprintln(os.Stderr, "Person image: "+e.Error())
			os.Exit(1)
		}
		b, e := readImage(*garment)
		if e != nil {
			fmt.Fprintln(os.Stderr, "Garment image: "+e.Error())
			os.Exit(1)
		}
		r.Status = "submitting"
		save()
		r.TaskID, e = provider.SubmitImages(ctx, a, b)
		if e != nil {
			r.Status = "unknown"
			save()
			fmt.Fprintln(os.Stderr, "Submission not confirmed. Check provider records; do not submit again automatically.")
			os.Exit(1)
		}
		r.Status = "pending"
		save()
	}
	for i := 0; i < 60; i++ {
		time.Sleep(10 * time.Second)
		out, e := provider.Poll(ctx, r.TaskID)
		if e != nil {
			fmt.Fprintln(os.Stderr, "Poll failed. Resume the retained task later; no new task submitted.")
			os.Exit(1)
		}
		r.Status = out.Status
		r.LatencyMS = time.Since(r.Started).Milliseconds()
		save()
		if out.Status == "success" {
			if e = os.WriteFile(filepath.Join(*dir, "result-image"), out.Image, 0600); e != nil {
				panic("cannot save private result")
			}
			fmt.Println("Actual provider task succeeded. Evidence saved privately; review and redact before sharing.")
			return
		}
		if out.Status == "error" {
			fmt.Fprintln(os.Stderr, "Provider task failed. Receipt retained.")
			os.Exit(1)
		}
	}
	fmt.Fprintln(os.Stderr, "Polling limit reached. Resume the same receipt later.")
	os.Exit(1)
}

func readImage(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, errors.New("file is unavailable")
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if e != nil {
		return nil, errors.New("file could not be read")
	}
	if len(b) == 0 || len(b) > 8<<20 {
		return nil, errors.New("use a nonempty JPEG/PNG no larger than 8 MiB")
	}
	c, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "jpeg" && format != "png") {
		return nil, errors.New("use a valid JPEG or PNG")
	}
	short, long := min(c.Width, c.Height), max(c.Width, c.Height)
	if short < 384 || long < 512 || long > 4096 || c.Width*c.Height > 12000000 {
		return nil, errors.New("use at least 512 by 384 px, at most 4096 px per side and 12 megapixels")
	}
	if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
		return nil, errors.New("image contents are incomplete or invalid")
	}
	return b, nil
}
