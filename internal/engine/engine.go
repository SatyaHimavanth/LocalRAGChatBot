package engine

import (
	"path/filepath"
	"sync"

	llama "github.com/tcpipuk/llama-go"
)

// EmbeddingProvider exposes the embedding metadata that higher layers need
// without coupling callers to the concrete llama.cpp-backed engine.
type EmbeddingProvider interface {
	Embed(text string) ([]float32, error)
	EmbeddingModelName() string
	EmbeddingDimensions() int
	EmbeddingBackend() string
}

type Engine struct {
	ChatModel  *llama.Model
	EmbedModel *llama.Model
	embedCtx   *llama.Context

	embedPath      string
	embedModelName string
	embedDims      int

	// embedMu serializes all embedding operations so they never overlap with
	// chat streaming. The two *llama.Model instances are distinct C++ objects,
	// but llama.cpp's internal thread-pool / BLAS backends are *not* safe for
	// concurrent calls from multiple goroutines. Serialising the (fast, stateless)
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
	return &Engine{
		ChatModel:      chatModel,
		EmbedModel:     embedModel,
		embedCtx:       embedCtx,
		embedPath:      embedPath,
		embedModelName: safeBaseName(embedPath),
	}, nil
}

// Embed serializes access so it never runs concurrently with chat streaming.
func (e *Engine) Embed(text string) ([]float32, error) {
	e.embedMu.Lock()
	defer e.embedMu.Unlock()
	return e.embedCtx.GetEmbeddings(text)
}

// EmbeddingModelName returns a human-readable name for the active embedding model.
func (e *Engine) EmbeddingModelName() string {
	if e == nil {
		return ""
	}
	if e.embedModelName != "" {
		return e.embedModelName
	}
	return safeBaseName(e.embedPath)
}

// EmbeddingDimensions returns the configured embedding dimensionality when known.
func (e *Engine) EmbeddingDimensions() int {
	if e == nil {
		return 0
	}
	return e.embedDims
}

// EmbeddingBackend identifies the active vector embedding backend.
func (e *Engine) EmbeddingBackend() string {
	return "llama-go"
}

// EmbeddingProfile returns the runtime embedding configuration.
func (e *Engine) EmbeddingProfile() (model string, dims int, backend string) {
	if e == nil {
		return "", 0, ""
	}
	return e.EmbeddingModelName(), e.EmbeddingDimensions(), e.EmbeddingBackend()
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

func safeBaseName(path string) string {
	if path == "" {
		return ""
	}
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		return path
	}
	return name
}
