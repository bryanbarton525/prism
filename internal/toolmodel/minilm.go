package toolmodel

import (
	"context"
	"fmt"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

// MiniLM runs an ONNX sentence embedding graph through Hugot's pure-Go backend.
type MiniLM struct {
	mu       sync.Mutex
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
}

func LoadMiniLM(modelPath string) (*MiniLM, error) {
	session, err := hugot.NewGoSession()
	if err != nil {
		return nil, err
	}
	pipeline, err := hugot.NewPipeline(session, hugot.FeatureExtractionConfig{
		ModelPath: modelPath, Name: "prism-tool-embeddings", OnnxFilename: "model.onnx",
		Options: []hugot.FeatureExtractionOption{pipelines.WithNormalization()},
	})
	if err != nil {
		_ = session.Destroy()
		return nil, err
	}
	return &MiniLM{session: session, pipeline: pipeline}, nil
}

func (m *MiniLM) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	res, err := m.pipeline.RunPipeline([]string{text})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(res.Embeddings) != 1 || len(res.Embeddings[0]) != 384 {
		return nil, fmt.Errorf("MiniLM returned unexpected embedding dimensions")
	}
	return res.Embeddings[0], nil
}

func (m *MiniLM) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return nil
	}
	err := m.session.Destroy()
	m.session = nil
	return err
}
