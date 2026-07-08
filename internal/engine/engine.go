// internal/engine/engine.go
package engine

import (
	"sync"

	llama "github.com/tcpipuk/llama-go"
)

type Engine struct {
	ChatModel *llama.Model
	EmbedModel *llama.Model
	embedCtx *llama.Context

	// embedMu serializes all embedding operations so they never overlap with
	// chat streaming. The two *llama.Model instances are distinct C++ objects,
	// but llama.cpp's internal thread-pool / BLAS backends are *not* safe for
	// concurrent calls from multiple goroutines.  Serialising the (fast, stateless)
	// embed path is far cheaper than trying to reason about C++ thread safety.
	embedMu sync.Mutex
}

func New(chatPath, embedPath string) (*Engine, error) {
	chatModel, err := llama.LoadModel(chatPath, llama.WithGPULayers(-1))
	if err != nil {
		return nil, err
	}
	embedModel, err := llama.LoadModel(embedPath, llama.WithGPULayers(-1))
	if err != nil {
		chatModel.Close()
		return nil, err
	}
	embedCtx, err := embedModel.NewContext(llama.WithEmbeddings())
	if err != nil {
		chatModel.Close()
		embedModel.Close()
		return nil, err
	}
	return &Engine{ChatModel: chatModel, EmbedModel: embedModel, embedCtx: embedCtx}, nil
}

// Embed serializes access so it never runs concurrently with chat streaming.
func (e *Engine) Embed(text string) ([]float32, error) {
	e.embedMu.Lock()
	defer e.embedMu.Unlock()
	return e.embedCtx.GetEmbeddings(text)
}

func (e *Engine) Close() {
	e.embedMu.Lock()
	defer e.embedMu.Unlock()
	if e.embedCtx != nil {
		e.embedCtx.Close()
	}
	if e.EmbedModel != nil {
		e.EmbedModel.Close()
	}
	if e.ChatModel != nil {
		e.ChatModel.Close()
	}
}
