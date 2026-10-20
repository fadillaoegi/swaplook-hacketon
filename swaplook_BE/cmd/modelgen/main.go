// modelgen creates fictional, AI-generated model photos with YouCam text-to-image.
// It runs only with -confirm, makes one attempt per missing model and never retries.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"swaplook/internal/adapters/renderer"
	"time"
)

type preset struct{ ID, Subject string }

// Prompts follow the cloth-v4 source rules: one person, standing, facing forward, head to waist, shoulders visible.
const base = "Photorealistic studio photograph of a fictional adult %s. Standing upright, facing the camera directly, body square to the camera, framed from the top of the head to the waist, both shoulders fully visible, arms relaxed straight down at the sides, neutral friendly expression, looking at the camera. Wearing a plain fitted white crew-neck short-sleeve t-shirt with no logo. Plain light grey seamless studio background, soft even front lighting, sharp focus, natural skin texture."
const negative = "crossed arms, hands on chest, hands in pockets, hair covering shoulders, jacket, scarf, necklace, bag, logo, text, watermark, multiple people, cropped head, side view, sitting, distorted hands, extra fingers, blurry, low resolution, cartoon, celebrity"

var presets = []preset{
	{"model-a", "woman of East Asian appearance in her twenties with shoulder-length straight black hair tucked behind her ears"},
	{"model-b", "man of Black African appearance in his thirties with short cropped hair"},
	{"model-c", "woman of South Asian appearance in her thirties with long dark hair tied back in a low ponytail"},
}

type receipt struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
	Started   time.Time `json:"started_at"`
	LatencyMS int64     `json:"latency_ms"`
	Units     string    `json:"units"`
}

func main() {
	confirm := flag.Bool("confirm", false, "confirm spending one text-to-image unit per missing model")
	dir := flag.String("output", "../.data/models/generated", "private output directory")
	flag.Parse()
	key := os.Getenv("YOUCAM_API_KEY")
	if !*confirm || key == "" {
		fmt.Fprintln(os.Stderr, "modelgen requires -confirm and server-side YOUCAM_API_KEY. No request was made.")
		os.Exit(1)
	}
	if e := os.MkdirAll(*dir, 0o700); e != nil {
		fmt.Fprintln(os.Stderr, "cannot create output directory")
		os.Exit(1)
	}
	y := renderer.NewYouCam(key, "")
	ctx := context.Background()
	failed := false
	for _, p := range presets {
		if matches, _ := filepath.Glob(filepath.Join(*dir, p.ID+".*")); len(matches) > 0 {
			fmt.Println(p.ID + ": already generated, skipped (no request)")
			continue
		}
		r := receipt{ID: p.ID, Model: "youcam-image-v2 1104*1472", Started: time.Now().UTC(), Units: "1 per task per provider documentation; not returned by the response"}
		save := func() {
			b, _ := json.MarshalIndent(r, "", "  ")
			_ = os.WriteFile(filepath.Join(*dir, p.ID+".receipt.json"), b, 0o600)
		}
		task, e := y.GenerateImage(ctx, fmt.Sprintf(base, p.Subject), negative, "1104*1472")
		if e != nil {
			r.Status = "submit_failed: " + e.Error()
			save()
			fmt.Println(p.ID + ": submission not confirmed; not retried")
			failed = true
			continue
		}
		r.TaskID, r.Status = task, "pending"
		save()
		for i := 0; i < 18 && r.Status == "pending"; i++ {
			time.Sleep(10 * time.Second)
			res, e := y.PollImage(ctx, task)
			if e != nil {
				continue
			}
			switch res.Status {
			case "success":
				ext := ".jpg"
				if http.DetectContentType(res.Image) == "image/png" {
					ext = ".png"
				}
				if e = os.WriteFile(filepath.Join(*dir, p.ID+ext), res.Image, 0o600); e != nil {
					r.Status = "save_failed"
				} else {
					r.Status = "success"
				}
			case "error":
				r.Status = "error"
				if res.Reason != "" {
					r.Status += ": " + res.Reason
				}
			}
		}
		r.LatencyMS = time.Since(r.Started).Milliseconds()
		save()
		fmt.Printf("%s: %s after %.1fs\n", p.ID, r.Status, float64(r.LatencyMS)/1000)
		if r.Status != "success" {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
