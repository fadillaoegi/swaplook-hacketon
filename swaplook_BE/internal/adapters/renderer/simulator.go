package renderer

import (
	"context"
	"swaplook/internal/application"
)

// Simulator returns catalog illustrations only, never an AI-generated or photo-based fit.
type Simulator struct{}

func (Simulator) Name() string { return "simulator" }
func (Simulator) Submit(_ context.Context, in application.RenderInput) (string, error) {
	return "fixture-" + in.SKU, nil
}
func (Simulator) Poll(_ context.Context, _ string) (application.RenderResult, error) {
	return application.RenderResult{Status: "success"}, nil
}
