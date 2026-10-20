package renderer

import (
	"context"
	"errors"
	"swaplook/internal/application"
)

const imagePath = "/s2s/v2.0/task/text-to-image/youcam"

// GenerateImage submits one text-to-image task (youcam-image-v2). It is used only by the
// explicitly invoked model generator, never by shopper requests.
func (y *YouCam) GenerateImage(ctx context.Context, prompt, negative, size string) (string, error) {
	if prompt == "" || len(prompt) > 800 || len(negative) > 500 {
		return "", errors.New("prompt length invalid")
	}
	in := map[string]any{"model": "youcam-image-v2", "prompt": prompt, "size": size, "prompt_extend": false}
	if negative != "" {
		in["negative_prompt"] = negative
	}
	var out struct {
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	if e := y.json(ctx, "POST", imagePath, in, &out); e != nil {
		return "", e
	}
	if out.Data.TaskID == "" {
		return "", errors.New("provider task ID missing")
	}
	return out.Data.TaskID, nil
}

// PollImage checks a text-to-image task and downloads its result when ready.
func (y *YouCam) PollImage(ctx context.Context, id string) (application.RenderResult, error) {
	return y.poll(ctx, imagePath, id)
}
